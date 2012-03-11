package git

import (
	"bytes"
	"crypto/sha1"
	"strconv"
)

type Object interface {
	Header() string
	Raw() []byte
}

func ObjectId(obj Object) Id {
	h := sha1.New()
	h.Write(ObjectFull(obj))
	return IdFromBytes(h.Sum(nil))
}

func ObjectFull(obj Object) []byte {
	content := obj.Raw()
	full := [][]byte{
		[]byte(obj.Header()),
		{' '},
		[]byte(strconv.Itoa(len(content))),
		{0},
		content,
	}
	return bytes.Join(full, nil)
}

type Blob struct {
	raw []byte
}

func NewBlob(raw []byte) *Blob {
	return &Blob{raw}
}

func (b *Blob) Header() string { return "blob" }

func (b *Blob) Raw() []byte {
	return b.raw
}

type Tree struct {
	names    []string
	children []Id
}

func NewTree(cap int) *Tree {
	t := &Tree{}
	if cap > 0 {
		t.names = make([]string, 0, cap)
		t.children = make([]Id, 0, cap)
	}
	return t
}

func (t *Tree) Add(name string, child Id) {
	t.names = append(t.names, name)
	t.children = append(t.children, child)
}

func (t *Tree) Header() string { return "tree" }

func (t *Tree) Raw() []byte {
	content := bytes.NewBuffer(nil)
	// TODO: Print out the children in sorted order
	for i := range t.names {
		// TODO: Fix permissions
		content.WriteString("100644 ")
		content.WriteString(t.names[i])
		content.WriteByte('\x00')
		content.WriteString(t.children[i].String())
	}
	return content.Bytes()
}

type time struct {
	seconds int64
	offset  int // time zone offset in minutes
}

func (t *time) String() string {
	if t == nil {
		return ""
	}
	pre := " "
	offset := t.offset
	if t.offset < 0 {
		pre += "-"
		offset = -t.offset
	}
	tz := strconv.Itoa(offset)
	for i := 0; len(tz)+i < 4; i++ {
		pre += "0"
	}
	return strconv.FormatInt(t.seconds, 10) + pre + tz
}

type Commit struct {
	authorName     string
	authorEmail    string
	authorTime     *time
	committerName  string
	committerEmail string
	committerTime  *time
	tree           Id
	parents        []Id
	msg            string
}

func NewCommit(authorName, authorEmail string, authorTime *time, committerName, committerEmail string, commitTime *time, tree Id, parents []Id, msg string) *Commit {
	// TODO: Set unset things
	return &Commit{authorName, authorEmail, authorTime, committerName, committerEmail, commitTime, tree, parents, msg}
}

func NewCommitSimple(name, email string, time *time, tree Id, parent Id) *Commit {
	return &Commit{name, email, time, name, email, time, tree, []Id{parent}, "empty message"}
}

func (c *Commit) Header() string { return "commit" }

func (c *Commit) Raw() []byte {
	content := "tree " + c.tree.String()
	for i := range c.parents {
		content += "\nparent " + c.parents[i].String()
	}
	content += "\nauthor " + c.authorName + " <" + c.authorEmail + "> " + c.authorTime.String()
	content += "\ncommitter " + c.committerName + " <" + c.committerEmail + "> " + c.committerTime.String() + "\n\n"
	content += c.msg
	return []byte(content)
}
