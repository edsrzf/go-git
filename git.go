package git

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
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
	if string(head[0:5]) != "ref: " {
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
	os.Mkdir(path + "/objects", 0666)
	os.Mkdir(path + "/refs", 0666)
	ioutil.WriteFile(path + "/HEAD", []byte("ref: refs/heads/master"), 0666)
	return &Repo{path: path}
}

func NewRepo(path string) *Repo {
	if !IsRepo(path) {
		return nil
	}
	return &Repo{path: path}
}

func (r *Repo) GetObject(id *Id) Object {
	raw := r.getLooseObject(id)
	if raw == nil {
		raw = r.getPackedObject(id)
	}
	return parse(raw)
}

func parse(raw []byte) Object {
	fmt.Println(string(raw))
	return nil
	i := bytes.IndexByte(raw, ' ')
	//null := bytes.IndexByte(raw[i:], '\x00')
	switch string(raw[0:i]) {
		case "blob":
		case "tree":
		case "commit":
		default:
			panic("What the heck?")
	}
	return nil
}

func (r *Repo) getLooseObject(id *Id) []byte {
	sha1 := id.String()
	path := r.path + "/objects/" + sha1[0:2] + "/" + sha1[2:]
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
	return b
}

// parse and cache index info
func (r *Repo) findPacks() {
	if len(r.packs) > 0 {
		// TODO: it's probably legal to have 0 packs
		return
	}
	packDir := r.path + "/objects/pack/"
	dir, err := os.Open(packDir, os.O_RDONLY, 0)
	if err != nil {
		return
	}
	defer dir.Close()
	files, err := dir.Readdirnames(-1)
	if err != nil {
		return
	}
	for _, f := range files {
		ext := path.Ext(f)
		if ext == "idx" {
			base := path.Base(f)
			r.packs = append(r.packs, newPack(r, base))
		}
	}
}

func (r *Repo) getPackedObject(id *Id) []byte {
	r.findPacks()
	for _, p := range r.packs {
		if raw := p.getObject(id); raw != nil {
			return raw
		}
	}
	return nil
}

type Id [20]byte

func IdFromBytes(sha1 []byte) *Id {
	if len(sha1) != 20 {
		return nil
	}
	id := new(Id)
	copy(id[0:], sha1)
	return id
}

func IdFromString(sha1 string) *Id {
	if len(sha1) != 40 {
		return nil
	}
	id := new(Id)
	decoded, err := hex.DecodeString(sha1)
	if err != nil {
		return nil
	}
	copy(id[0:], decoded)
	return id
}

func (id *Id) String() string {
	return hex.EncodeToString(id[:])
}

func (x *Id) Same(y *Id) bool {
	a, b := *x, *y
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
