package git

import (
	"bufio"
	"bytes"
	"http"
	"io"
	"os"
)

func Clone(url, path string) *Repo {
	r := InitRepo(path, false)
	refsUrl := url + "/info/refs?service=git-upload-pack"
	//resp, _, err := http.Get(url + "/info/refs?service=git-upload-pack")
	resp, err := http.Get(refsUrl)
	//resp, _, err := http.Get(url + "/info/refs?service=git-receive-pack")
	if err != nil {
		panic(err.String())
		return nil
	}
	buf := bufio.NewReader(resp.Body)
	// TODO: check that this is the '#' packet
	readPacket(buf)
	// TODO: check that this is a flush
	readPacket(buf)
	refs := readRefs(buf)
	b := bytes.NewBuffer(nil)
	wants := make([]Id, 0, len(refs))
	for _, id := range refs {
		wants = append(wants, id)
	}
	writeWants(b, wants, nil)
	resp, err = http.Post(url + "/git-upload-pack", "application/x-git-upload-pack-request", b)
	io.Copy(os.Stdout, resp.Body)
	return r
}
