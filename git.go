package git

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

type Repo struct {
	path string
	packs []*pack
}

// A git repository requires:
// - Either an objects directory or the GIT_OBJECT_DIRECTORY environment variable
// - A refs directory
// - Either a HEAD symlink or a HEAD file that is formatted properly
func IsRepo(dir string) bool {
	// TODO: Check for symlink?
	head, err := ioutil.ReadFile(dir + "/HEAD")
	if err != nil {
		return false
	}
	// We'll just assume that anything starting with "ref: " is good enough
	if string(head[:5]) != "ref: " {
		for _, c := range head {
			if c < '0' || c > 'f' || (c > '9' && c < 'a') {
				// Not a valid SHA-1
				return false
			}
		}
	}
	stat, err := os.Lstat(dir + "/objects")
	if err != nil || !stat.IsDirectory() {
		return false
	}
	stat, err = os.Lstat(dir + "/refs")
	if err != nil || !stat.IsDirectory() {
		return false
	}
	return true
}

// Create .git directory
// Create empty .git/objects, .git/refs
// Create HEAD containing "ref: refs/heads/master"
func InitRepo(path string, bare bool) *Repo {
	r := NewRepo(path)
	if r != nil {
		return r
	}
	os.Mkdir(path, 0666)
	os.Mkdir(r.file("/objects"), 0666)
	os.Mkdir(r.file("refs"), 0666)
	ioutil.WriteFile(r.file("/HEAD"), []byte("ref: refs/heads/master"), 0666)
	return &Repo{path: path}
}

func NewRepo(path string) *Repo {
	if !IsRepo(path) {
		return nil
	}
	return &Repo{path: path}
}

func (r *Repo) file(path string) string {
	return filepath.Join(r.path, path)
}

func (r *Repo) GetObject(id Id) Object {
	obj := r.getLooseObject(id)
	if obj == nil {
		obj = r.getPackedObject(id)
	}
	return obj
}

func parse(raw []byte) Object {
	i := bytes.IndexByte(raw, ' ')
	null := bytes.IndexByte(raw, '\x00')
	sizeStr := raw[i+1:null]
	size, err := strconv.Atoi(string(sizeStr))
	if err != nil {
		panic("whaa?")
	}
	switch string(raw[:i]) {
		case "blob":
			return &Blob{raw[null+1:null+1+size]}
		case "tree":
			return parseTree(raw[null+1:null+1+size])
		case "commit":
			return parseCommit(raw[null+1:null+1+size])
		default:
			panic("What the heck?")
	}
	return nil
}

func parseTree(raw []byte) *Tree {
	t := NewTree(0)
	for len(raw) > 0 {
		pos := bytes.IndexByte(raw, 0)
		name := string(raw[7:pos])
		id := Id(string(raw[pos + 1:pos+21]))
		raw = raw[pos+21:]
		t.Add(name, id)
	}
	return t
}

func parseCommit(raw []byte) *Commit {
	msgPos := bytes.Index(raw, []byte("\n\n"))
	if msgPos < 0 {
		panic("no message?")
	}
	lines := bytes.Split(raw[:msgPos], []byte{'\n'}, -1)
	c := &Commit{}
	for _, line := range lines {
		pos := bytes.IndexByte(line, ' ')
		if pos < 0 {
			panic("bad commit format")
		}
		switch string(line[:pos]) {
		case "tree":
			c.tree = IdFromBytes(line[pos+1:])
		case "parent":
			parentId := IdFromBytes(line[pos+1:])
			c.parents = append(c.parents, parentId)
		case "author":
			c.authorName, c.authorEmail = parseIdentity(line[pos+1:])
		case "committer":
			c.committerName, c.committerEmail = parseIdentity(line[pos+1:])
		}
	}
	c.msg = string(raw[msgPos+2:])
	return c
}

func parseIdentity(line []byte) (string, string) {
	pos := bytes.IndexByte(line, '<')
	name := string(line[:pos-1])
	addrEnd := bytes.IndexByte(line, '>')
	addr := string(line[pos+1:addrEnd])
	return name, addr
}

func (r *Repo) loosePath(id Id) string {
	sha1 := id.String()
	return filepath.Join(r.file("objects"), sha1[0:2], sha1[2:])
}

func (r *Repo) getLooseObject(id Id) Object {
	path := r.loosePath(id)
	f, err := os.Open(path, os.O_RDONLY, 0)
	defer f.Close()
	if err != nil {
		return nil
	}
	z, _ := zlib.NewReader(f)
	defer z.Close()
	// TODO: Size it right
	b := make([]byte, 1024)
	z.Read(b)
	return parse(b)
}

// parse and cache index info
func (r *Repo) findPacks() {
	if len(r.packs) > 0 {
		// TODO: it's probably legal to have 0 packs
		return
	}
	packDir := filepath.Join(r.file("objects"), "pack")
	dir, err := os.Open(packDir, os.O_RDONLY, 0)
	if err != nil {
		panic(err.String())
		return
	}
	defer dir.Close()
	files, err := dir.Readdirnames(-1)
	if err != nil {
		panic(err.String())
		return
	}
	for _, f := range files {
		ext := filepath.Ext(f)
		if ext == ".idx" {
			base := f[:len(f)-len(ext)]
			r.packs = append(r.packs, newPack(r, base))
		}
	}
}

func (r *Repo) getPackedObject(id Id) Object {
	r.findPacks()
	for _, p := range r.packs {
		if obj := p.getObject(id); obj != nil {
			return obj
		}
	}
	return nil
}

// Save adds an Object to the repository.
func (r *Repo) Save(obj Object) os.Error {
	// It's easy to create a loose object. Let's do that.
	path := r.loosePath(ObjectId(obj))
	f, err := os.Open(path, os.O_CREATE | os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	content := obj.Raw()
	wr, err := zlib.NewWriter(f)
	if err != nil {
		return err
	}
	defer wr.Close()
	wr.Write(content)
	return nil
}

// binary representation of an id
// len == 20, always (except for invalid ids)
// it's a string so it can be used as a map key
type Id string

func IdFromBytes(sha1 []byte) Id {
	if len(sha1) != 40 {
		return ""
	}
	decoded := make([]byte, 20)
	hex.Decode(decoded, sha1)
	return Id(string(decoded))
}

func IdFromString(sha1 string) Id {
	if len(sha1) != 40 {
		return ""
	}
	decoded, err := hex.DecodeString(sha1)
	if err != nil {
		return ""
	}
	return Id(string(decoded))
}

func (id Id) String() string {
	return hex.EncodeToString([]byte(string(id)))
}
