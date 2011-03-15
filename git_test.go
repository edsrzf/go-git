package git

import (
	"testing"
)

func TestEverything(t *testing.T) {
	r := NewRepo(".git")
	if r == nil {
		t.Fatal("cannot find repo")
	}
	//id := IdFromString("a67e49e02cb9a85ed457829c163c1162e15fcdc2")
	id := IdFromString("2bbe5b5277c79042662facf6941d15a50c046b09")
	obj := r.GetObject(id)
	if _, ok := obj.(*Commit); !ok {
		t.Errorf("%s isn't a commit!", id)
	}
}
