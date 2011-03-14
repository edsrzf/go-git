package git

import (
	"bytes"
	"crypto/sha1"
	"strconv"
)

type Type int
const (
	BlobType Type = iota
	TreeType
	CommitType
	TagType
)

type Object interface {
	Type() Type
	Raw() []byte
	Id() *Id
}

// Returns an object ID given an object header an the object's content.
func idHelper(header string, content []byte) *Id {
	h := sha1.New()
	h.Write([]byte(header + " "))
	h.Write([]byte(strconv.Itoa(len(content))))
	h.Write([]byte{0})
	h.Write(content)
	return IdFromBytes(h.Sum())
}

type Blob struct {
	raw []byte
}

func NewBlob(raw []byte) *Blob {
	return &Blob{raw}
}

func (b *Blob) Type() Type { return BlobType }

func (b *Blob) Id() *Id { return idHelper("blob", b.Raw()) }

func (b *Blob) Raw() []byte {
	return b.raw
}

type Tree struct {
	names []string
	children []Object
}

func NewTree(cap int) *Tree {
	t := &Tree{}
	if cap > 0 {
		t.names = make([]string, 0, cap)
		t.children = make([]Object, 0, cap)
	}
	return t
}

func (t *Tree) Add(name string, child Object) {
	n := len(t.names)
	if n == cap(t.names) {
		tempNames := make([]string, n, 2*n)
		copy(tempNames, t.names)
		t.names = tempNames
		tempObjects := make([]Object, n, 2*n)
		copy(tempObjects, t.children)
		t.children = tempObjects
	}
	t.names = t.names[0:n+1]
	t.names[n] = name
	t.children = t.children[0:n+1]
	t.children[n] = child
}

func (t *Tree) Id() *Id { return idHelper("tree", t.Raw()) }

func (t *Tree) Raw() []byte {
	content := bytes.NewBuffer(nil)
	// TODO: Print out the children in sorted order
	for i := range t.names {
		// TODO: Fix permissions
		content.WriteString("100644 ")
		content.WriteString(t.names[i])
		content.WriteByte('\x00')
		content.Write(t.children[i].Id()[:])
	}
	return content.Bytes()
}

type time struct {
	seconds int64
	offset int // time zone offset in minutes
}

func (t *time) String() string {
	pre := " "
	offset := t.offset
	if t.offset < 0 {
		pre += "-"
		offset = -t.offset
	}
	tz := strconv.Itoa(offset)
	for i := 0; len(tz) + i < 4; i++ {
		pre += "0"
	}
	return strconv.Itoa64(t.seconds) + pre + tz
}

type Commit struct {
	authorName string
	authorEmail string
	authorTime *time
	committerName string
	committerEmail string
	committerTime *time
	tree *Id
	parents []*Id
	message string
}

func NewCommit(authorName, authorEmail string, authorTime *time, committerName, committerEmail string, committerTime *time, tree *Id, parents []*Id, message string) *Commit {
	// TODO: Set unset things
	return &Commit{authorName, authorEmail, authorTime, committerName, committerEmail, committerTime, tree, parents, message}
}

func NewCommitSimple(name, email string, time *time, tree *Id, parent *Id) *Commit {
	return &Commit{name, email, time, name, email, time, tree, []*Id{parent}, "empty message"}
}

func (c *Commit) Type() Type {
	return CommitType
}

func (c *Commit) Id() *Id { return idHelper("commit", c.Raw()) }

func (c *Commit) Raw() []byte {
	content := "tree " + c.tree.String()
	for i := range c.parents {
		content += "\nparent " + c.parents[i].String()
	}
	content += "\nauthor " + c.authorName + " <" + c.authorEmail + "> " + c.authorTime.String()
	content += "\ncommitter " + c.committerName + " <" + c.committerEmail + "> " + c.committerTime.String() + "\n\n"
	content += c.message
	return []byte(content)
}
