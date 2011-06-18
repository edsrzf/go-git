package git

// This file implements the git protocol and is intended to be transport-independent.
// Useful resources:
//	http://www.kernel.org/pub/software/scm/git/docs/technical/pack-protocol.txt
//	http://www.kernel.org/pub/software/scm/git/docs/technical/protocol-capabilities.txt
//	http://www.kernel.org/pub/software/scm/git/docs/technical/protocol-common.txt

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
)

const zeroId Id = "0000000000000000000000000000000000000000"

// write refs in the format of git receive-pack --stateless-rpc --advertise-refs
// format of ref is: SHA-1 " " name "\x00" capability { " " capability }
// capabilities are sent only with the first ref
// possibilities are:
// - report-status - 
// - delete-refs - can accept a zero-id value as the target value of a reference update
// - side-band - multi-plexed progress reports are allowed; broken into packets
// - side-band-64k - same as above with a larger packet size; mutually exclusive
// - multi_ack - allows server to cut off the client when it finds a common base
// - ofs-delta - prefer offset deltas in pack files; settable in .gitconfig
// - thin-pack - server can send a 'thin' pack which doesn't contain base objects.
// - shallow - client can fetch shallow clones
// - no-progress - 
// - include-tag - 
func (r *Repo) advertiseRefs(w io.Writer) {
	refs := r.Refs()
	if len(refs) == 0 {
		packet := []byte(zeroId.String() + " capabilities^{}\x00\n")
		writePacket(w, packet)
	} else {
		sentCaps := false
		// HEAD has to be first
		if head := refs["HEAD"]; head != "" {
			writePacket(w, []byte(head.String()+" HEAD\x00\n"))
			sentCaps = true
		}
		for name, id := range refs {
			if name == "HEAD" {
				continue
			}
			payload := []byte(id.String() + " " + name)
			if !sentCaps {
				payload = append(payload, '\x00')
				sentCaps = true
			}
			payload = append(payload, '\n')
			writePacket(w, payload)
		}
	}
	flush(w)
}

// negotiate initiates packfile negotiation from the server's perspective.
// The server expects some "want" lines and "have" lines from r and writes
// out the necessary packfiles to w.
func (repo *Repo) negotiate(w io.Writer, r io.Reader) {
	//capFlags := 0
	haveCaps := false

	var wants []Id
	for {
		packet, _ := readPacket(r)
		if packet == nil {
			break
		}
		if len(packet) < 45 || !bytes.HasPrefix(packet, []byte("want ")) {
			// error
		}
		id := IdFromString(string(packet[5:45]))
		wants = append(wants, id)
		if !haveCaps {
			haveCaps = true
			if len(packet) < 45 || packet[45] != ' ' {
				// error
			}
			//caps := bytes.Split(packet[46:], []byte{' '}, -1)
		}
	}

	var haves []Id
	for {
		packet, _ := readPacket(r)
		if packet == nil {
			break
		}
		if len(packet) < 45 || !bytes.HasPrefix(packet, []byte("have ")) {
			// error
		}
		haves = append(haves, IdFromString(string(packet[5:45])))
	}

	// hack alert
	nak(w)
	f, err := os.Open(repo.file("objects/pack/pack-2f9aa945c499706d76fa3807faac9e8f01e48dd7.pack"))
	if err != nil {
		panic(err.String())
	}
	io.Copy(w, f)
	f.Close()
}

func readPacket(r io.Reader) (b []byte, err os.Error) {
	n := make([]byte, 4)
	_, err = r.Read(n)
	if err != nil {
		return
	}
	n2, err := strconv.Btoui64(string(n), 16)
	if err != nil {
		return
	}
	// flush
	if n2 == 0 {
		return
	}
	b = make([]byte, n2-4)
	_, err = r.Read(b)
	if err != nil {
		b = nil
		return
	}
	return
}

func writePacket(w io.Writer, payload []byte) {
	fmt.Fprintf(w, "%04X", len(payload)+4)
	w.Write(payload)
}

func flush(w io.Writer) {
	w.Write([]byte("0000"))
}

func (r *Repo) want(w io.Writer, ids []Id) {
}

func (r *Repo) have(w io.Writer, ids []Id) {
}

func ack(w io.Writer, id Id) {
	writePacket(w, []byte("ACK "+id.String()+"\n"))
}

func ackMulti(w io.Writer, id Id, status string) {
	writePacket(w, []byte("ACK "+id.String()+" "+status+"\n"))
}

func nak(w io.Writer) {
	writePacket(w, []byte("NAK\n"))
}
