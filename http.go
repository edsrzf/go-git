package git

// This file implements HTTP (both "smart" and "dumb") transport for the
// Git protocol.
import (
	"http"
	"io"
	"os"
	"regexp"
	"strings"
)

// HttpHandler provides an interface to both smart and dumb HTTP protocols.
type HttpHandler struct {
	Repo *Repo // the git repository to operate on
	// AllowDumb specifies whether or not to allow the "dumb" HTTP protocol.
	// This version of the protocol supports older clients, but can use
	// much more bandwidth than the "smart" version.
	AllowDumb bool
}

var routes = []struct {
	pattern *regexp.Regexp
	handler func(*Repo, http.ResponseWriter, *http.Request)
}{
	// POST-only
	{regexp.MustCompile("/git-upload-pack$"), uploadPack},
	{regexp.MustCompile("/git-receive-pack$"), receivePack},
	// GET-only
	{regexp.MustCompile("/info/refs(\\?.*)?$"), serveRefs},
	{regexp.MustCompile("/HEAD$"), serveText},
	{regexp.MustCompile("/objects/info/alternates$"), serveText},
	{regexp.MustCompile("/objects/info/http-alternates$"), serveText},
	{regexp.MustCompile("/objects/info/packs$"), servePacks},
	{regexp.MustCompile("/objects/info/[^/]*$"), serveText},
	// this last bracket expression should really be matched 38 times
	{regexp.MustCompile("/objects/info/[0-9a-f][0-9a-f]/[0-9a-f]+$"), serveLoose},
	// this one should be matched 40 times
	{regexp.MustCompile("/objects/pack/pack-[0-9a-f]+\\.pack$"), servePackFile},
	// same here
	{regexp.MustCompile("/objects/pack/pack-[0-9a-f]+\\.idx$"), serveIndex},
}

func (h *HttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Write(os.Stdout)
	for _, route := range routes {
		if route.pattern.MatchString(r.RawURL) {
			route.handler(h.Repo, w, r)
			break
		}
	}
}

func uploadPack(repo *Repo, w http.ResponseWriter, r *http.Request) {
	repo.negotiate(w, r.Body)
}

func receivePack(repo *Repo, w http.ResponseWriter, r *http.Request) {
}

func serveRefs(repo *Repo, w http.ResponseWriter, r *http.Request) {
	service := r.FormValue("service")
	if service == "" {
		serveText(repo, w, r)
		return
	}

	if !strings.HasPrefix(service, "git-") || r.Method != "GET" {
		// forbidden
		return
	}

	w.Header().Set("Content-Type", "application/x-" + service + "-advertisement")
	writePacket(w, []byte("# service=" + service))
	flush(w)
	repo.advertiseRefs(w)
}

func serveText(repo *Repo, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	serveFile(repo, w, r)
}

func servePacks(repo *Repo, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	serveFile(repo, w, r)
}

func serveLoose(repo *Repo, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-git-loose-object")
	serveFile(repo, w, r)
}

func servePackFile(repo *Repo, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-git-packed-objects")
	serveFile(repo, w, r)
}

func serveIndex(repo *Repo, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-git-packed-objects-toc")
	serveFile(repo, w, r)
}

// http.ServeFile won't let us set our own Content-Type
func serveFile(repo *Repo, w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		// forbidden?
		return
	}
	name := repo.file(r.URL.Path)
	f, err := os.Open(name, os.O_RDONLY, 0)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	io.Copy(w, f)
}
