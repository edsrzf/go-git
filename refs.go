package git

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
)

func (r *Repo) resolveRef(name string) (id Id) {
	if id := r.refs[name]; id != "" {
		return id
	}
	content, err := ioutil.ReadFile(r.file(name))
	if err != nil {
		panic(err.String())
		return ""
	}
	content = bytes.TrimSpace(content)
	if bytes.HasPrefix(content, []byte("ref: ")) {
		// TODO: probably shouldn't cache symrefs -- at least not this way
		id = r.resolveRef(string(content[5:]))
	} else {
		id = IdFromString(string(content))
	}
	if id != "" {
		r.refs[name] = id
	}
	return
}

// Head returns the Id of the HEAD ref.
func (r *Repo) Head() Id {
	return r.resolveRef("HEAD")
}

func (r *Repo) packedRefs() {
	if r.refs == nil {
		r.refs = map[string]Id{}
	}
	packedRefs := r.file("packed-refs")
	content, err := ioutil.ReadFile(packedRefs)
	if err != nil {
		return
	}
	lines := bytes.Split(content, []byte{'\n'}, -1)
	for _, line := range lines {
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		parts := bytes.Split(line, []byte{' '}, 2)
		if len(parts[0]) != 40 {
			continue
		}
		id := IdFromString(string(parts[0]))
		refname := string(parts[1])
		r.refs[refname] = id
	}
	return
}

// Refs returns a map of ref names to Ids.
func (r *Repo) Refs() map[string]Id {
	v := &refVisitor{r: r}
	r.packedRefs()
	if head := r.Head(); head != "" {
		r.refs["HEAD"] = head
	}
	filepath.Walk(r.file("refs"), v, nil)
	return r.refs
}

type refVisitor struct {
	r *Repo
}

func (v *refVisitor) VisitDir(path string, f *os.FileInfo) bool {
	return true
}

func (v *refVisitor) VisitFile(path string, f *os.FileInfo) {
	v.r.resolveRef(path[5:])
}
