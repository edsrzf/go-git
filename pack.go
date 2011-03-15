package git

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"os"
	"github.com/edsrzf/go-mmap"
)

var order = binary.BigEndian

type pack struct {
	idxPath string
	indexFile *os.File
	index mmap.MMap

	dataPath string
	dataFile *os.File
	data mmap.MMap
}

func newPack(r *Repo, base string) *pack {
	basePath := r.path + base
	return &pack{idxPath: basePath + ".idx", dataPath: basePath + ".pack"}
}

const indexHeader = "\xFF\x74\x4F\x63\x00\x00\x00\x02"

func (p *pack) readIndex() {
	if p.indexFile != nil {
		return
	}
	var err os.Error
	p.indexFile, err = os.Open(p.idxPath, os.O_RDONLY, 0)
	if err != nil {
		println("error opening")
		return
	}
	p.index, err = mmap.Map(p.indexFile, mmap.RDONLY)
	if err != nil {
		println("error mapping")
		return
	}
	if string([]byte(p.index[:8])) != indexHeader {
		panic("bad index header")
	}
}

const packHeader = "PACK\x00\x00\x00\x02"

func (p *pack) readData() {
	if p.dataFile != nil {
		return
	}
	var err os.Error
	p.dataFile, err = os.Open(p.dataPath, os.O_RDONLY, 0)
	if err != nil {
		return
	}
	p.data, err = mmap.Map(p.dataFile, mmap.RDONLY)
	if err != nil {
		return
	}
	if string([]byte(p.data[:8])) != packHeader {
		panic("bad packfile header")
	}
}

func (p *pack) getObject(id *Id) []byte {
	p.readIndex()
	// 255 uint32s
	fan := p.index[8:1032]
	size := order.Uint32(fan[1028:])
	// TODO: Make sure fan[id[0]] > fan[id[0] - 1]
	//       Otherwise our object's definitely not here
	cnt := order.Uint32(fan[4*id[0]:])
	// TODO: split up this line so it's easier to read
	loc := 8 + 1024 + cnt*20
	suspect := p.index[loc:loc+20]
	cmp := bytes.Compare(id[:], suspect)
	lo, hi := uint32(0), size
	// TODO: allow for failure
	for ; cmp != 0; cmp = bytes.Compare(id[:], suspect) {
		if cmp < 0 {
			hi = cnt
		} else {
			lo = cnt
		}
		cnt = (lo + hi) / 2
		loc := 8 + 1024 + cnt*20
		suspect = p.index[loc:loc+20]
	}

	// TODO: check for 64-bit offset
	offset := order.Uint32(p.index[loc + 24*size - 20*cnt:])
	p.readData()
	objHeader := p.data[offset]
	objType := objHeader & 0x70 >> 4

	switch objType {
	case 0x01:
		println("It's a commit")
	case 0x02:
		println("It's a tree")
	case 0x03:
		println("It's a blog")
	case 0x04:
		println("It's a tag")
	default:
		println("It's something else")
	}

	// size when uncompressed
	// TODO: should be uint64?
	objSize := uint32(objHeader & 0x0F)
	i := uint32(0)
	for objHeader & 0x80 != 0 {
		objHeader = p.data[offset+i]
		objSize |= uint32(objHeader & 0x7F) << 7*(i+1)
		i++
	}

	obj := make([]byte, objSize)
	buf := bytes.NewBuffer(p.data[offset+i:])
	r, err := zlib.NewReader(buf)
	if err != nil {
		panic(err.String())
	}
	r.Read(obj)
	r.Close()

	fmt.Printf("data: %q\n", obj)

	return obj
}

func (p *pack) Close() {
	if p.indexFile != nil {
		p.index.Unmap()
		p.indexFile.Close()
	}
	if p.dataFile != nil {
		p.data.Unmap()
		p.dataFile.Close()
	}
}
