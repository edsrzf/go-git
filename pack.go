package git

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"github.com/edsrzf/mmap-go"
)

var order = binary.BigEndian

type pack struct {
	idxPath   string
	indexFile *os.File
	index     mmap.MMap

	dataPath string
	dataFile *os.File
	data     mmap.MMap
}

func newPack(r *Repo, base string) *pack {
	basePath := filepath.Join(r.file("objects"), "pack", base)
	return &pack{idxPath: basePath + ".idx", dataPath: basePath + ".pack"}
}

const indexHeader = "\xFF\x74\x4F\x63\x00\x00\x00\x02"

func (p *pack) readIndex() {
	if p.indexFile != nil {
		return
	}
	var err os.Error
	p.indexFile, err = os.Open(p.idxPath)
	if err != nil {
		panic(err.String())
		return
	}
	p.index, err = mmap.Map(p.indexFile, mmap.RDONLY, 0)
	if err != nil {
		panic("error mapping")
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
	p.dataFile, err = os.Open(p.dataPath)
	if err != nil {
		return
	}
	p.data, err = mmap.Map(p.dataFile, mmap.RDONLY, 0)
	if err != nil {
		return
	}
	if string([]byte(p.data[:8])) != packHeader {
		panic("bad packfile header")
	}
}

const (
	_OBJ_COMMIT = iota + 1
	_OBJ_TREE
	_OBJ_BLOB
	_OBJ_TAG
	_
	_OBJ_OFS_DELTA
	_OBJ_REF_DELTA
)

func (p *pack) getObject(id Id) Object {
	offset := p.offset(id)
	return p.readObject(offset)
}

func (p *pack) offset(id Id) uint32 {
	idBytes := []byte(string(id))
	p.readIndex()
	// 255 uint32s
	fan := p.index[8:1032]
	size := order.Uint32(fan[1020:])
	// TODO: Make sure fan[id[0]] > fan[id[0] - 1]
	//       Otherwise our object's definitely not here
	cnt := order.Uint32(fan[4*id[0]:])
	// TODO: split up this line so it's easier to read
	loc := 8 + 1024 + cnt*20
	suspect := p.index[loc : loc+20]
	cmp := bytes.Compare(idBytes, suspect)
	// TODO: allow for failure
	for lo, hi := uint32(0), size; cmp != 0; cmp = bytes.Compare(idBytes, suspect) {
		if cmp < 0 {
			hi = cnt
		} else {
			lo = cnt + 1
		}
		if lo >= hi {
			return 0
		}
		cnt = (lo + hi) / 2
		loc = 8 + 1024 + cnt*20
		suspect = p.index[loc : loc+20]
	}

	// TODO: check for 64-bit offset
	// calculate which sha1 we looked at
	n := (loc - 1032) / 20
	offsetBase := 1032 + 20*size + 4*size
	offset := order.Uint32(p.index[offsetBase+4*n:])
	return offset
}

func (p *pack) readObject(offset uint32) Object {
	objType, obj := p.readRaw(offset)

	switch objType {
	case _OBJ_COMMIT:
		println("It's a commit")
		return parseCommit(obj)
	case _OBJ_TREE:
		println("It's a tree")
		return parseTree(obj)
	case _OBJ_BLOB:
		println("It's a blob")
		return &Blob{obj}
	case _OBJ_TAG:
		println("It's a tag")
	default:
		println("It's something else")
		panic("we don't know about this type yet")
	}

	return nil
}

func (p *pack) readRaw(offset uint32) (int, []byte) {
	p.readData()
	objHeader := p.data[offset]
	objType := int(objHeader & 0x71 >> 4)

	// size when uncompressed
	// TODO: should be uint64?
	objSize := uint32(objHeader & 0x0F)
	i := uint32(0)
	shift := uint32(4)
	for objHeader&0x80 != 0 {
		i++
		objHeader = p.data[offset+i]
		objSize |= uint32(objHeader&0x7F) << shift
		shift += 7
	}

	var rawBase []byte
	if objType == _OBJ_OFS_DELTA {
		i++
		b := p.data[offset+i]
		baseOffset := uint32(b & 0x7F)
		for b&0x80 != 0 {
			i++
			b = p.data[offset+i]
			baseOffset = ((baseOffset + 1) << 7) | uint32(b&0x7F)
		}
		if baseOffset > uint32(len(p.data)) || baseOffset > offset {
			panic("bad offset")
		}
		objType, rawBase = p.readRaw(offset - baseOffset)
	} else if objType == _OBJ_REF_DELTA {
		baseId := Id(string([]byte(p.data[offset+i+1 : offset+i+21])))
		i += 20
		baseOffset := p.offset(baseId)
		objType, rawBase = p.readRaw(baseOffset)
	}

	obj := make([]byte, objSize)
	buf := bytes.NewBuffer(p.data[offset+i+1:])
	r, err := zlib.NewReader(buf)
	if err != nil {
		panic(err.String())
	}
	r.Read(obj)
	r.Close()

	if rawBase != nil {
		// apply delta to base
		obj = applyDelta(rawBase, obj)
	}

	return objType, obj
}

func applyDelta(base, patch []byte) []byte {
	// base length; TODO: use for bounds checking
	baseLength, n := decodeVarint(patch)
	if baseLength != uint64(len(base)) {
		// TODO: return error
		panic("base mismatch")
		return nil
	}
	patch = patch[n:]
	resultLength, n := decodeVarint(patch)
	patch = patch[n:]
	result := make([]byte, resultLength)
	loc := uint(0)
	for len(patch) > 0 {
		i := uint(1)
		op := patch[0]
		if op == 0 {
			// reserved
			// TODO: better error, don't panic
			panic("delta opcode 0")
		} else if op&0x80 == 0 {
			// insert
			n := uint(op)
			copy(result[loc:], patch[i:i+n])
			loc += n
			patch = patch[i+n:]
			continue
		}
		copyOffset := uint(0)
		for j := uint(0); j < 4; j++ {
			if op&(1<<j) != 0 {
				x := patch[i]
				i++
				copyOffset |= uint(x) << (j * 8)
			}
		}
		copyLength := uint(0)
		for j := uint(0); j < 3; j++ {
			if op&(1<<(4+j)) != 0 {
				x := patch[i]
				i++
				copyLength |= uint(x) << (j * 8)
			}
		}
		if copyLength == 0 {
			copyLength = 1 << 16
		}
		if copyOffset+copyLength > uint(len(base)) || copyLength > uint(len(result[loc:])) {
			panic("oops, that's not good")
		}
		copy(result[loc:], base[copyOffset:copyOffset+copyLength])
		loc += copyLength
		patch = patch[i:]
	}
	return result
}

func decodeVarint(buf []byte) (x uint64, n int) {
	shift := uint64(0)
	for {
		b := buf[n]
		n++
		x |= uint64(b&0x7F) << shift
		shift += 7
		if b&0x80 == 0 {
			return
		}
	}
	return
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
