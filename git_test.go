package git

import (
	"testing"
)

var varintTests = []struct {
	encoded []byte
	decoded uint64
}{
	{[]byte{0}, 0},
	{[]byte{1}, 1},
	{[]byte{0x80, 0x01}, 0x80},
}

func TestVarint(t *testing.T) {
	for _, test := range varintTests {
		dec, n := decodeVarint(test.encoded)
		if dec != test.decoded {
			t.Errorf("got %d, wanted %d", dec, test.decoded)
		}
		if n != len(test.encoded) {
			t.Errorf("didn't consume all the bytes")
		}
	}
}

// It's probably a bad idea to write tests this way, but it's just so convenient.
func TestObjects(t *testing.T) {
	r := NewRepo(".git")
	if r == nil {
		t.Fatal("cannot find repo")
	}
	id := IdFromString("5740508db83a6f137c346e240607f51261633e51")

	obj := r.GetObject(id)
	if _, ok := obj.(*Commit); !ok {
		t.Errorf("%s isn't a commit!", id)
	}

	id = IdFromString("80d3035b39f0f6346a1b666c9bc49896c41e89df")
	obj = r.GetObject(id)
	if _, ok := obj.(*Tree); !ok {
		t.Errorf("%s isn't a tree!", id)
	}

	id = IdFromString("f9f3ec33496f036295663b178b96f6863c303b8f")
	obj = r.GetObject(id)
	if _, ok := obj.(*Blob); !ok {
		t.Errorf("%s isn't a blob!", id)
	}

	id = IdFromString("020ec0fea03988331291de2148f52c0b2351fbbe")
	obj = r.GetObject(id)
	if _, ok := obj.(*Blob); !ok {
		t.Errorf("%s isn't a blob!", id)
	}
}
