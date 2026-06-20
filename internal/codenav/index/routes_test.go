package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectRoutes(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/svc\n\ngo 1.21\n",
		"main.go": `package main

import "net/http"

type router struct{}

func (router) Get(path string, h http.HandlerFunc) {}

func getUser(w http.ResponseWriter, r *http.Request)    {}
func createUser(w http.ResponseWriter, r *http.Request) {}

func main() {
	http.HandleFunc("/users", getUser)
	r := router{}
	r.Get("/users/new", createUser)
}
`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sink := newCollectSink()
	if err := (GoNative{}).Index(context.Background(), RepoScan{Project: "svc", Root: dir}, sink); err != nil {
		t.Fatal(err)
	}

	type want struct{ method, pattern, handler string }
	wants := []want{
		{"ANY", "/users", "example.com/svc.getUser"},
		{"GET", "/users/new", "example.com/svc.createUser"},
	}
	for _, w := range wants {
		found := false
		for _, r := range sink.routes {
			if r.Method == w.method && r.Pattern == w.pattern && r.HandlerQName == w.handler {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing route %s %s -> %s (got %+v)", w.method, w.pattern, w.handler, sink.routes)
		}
	}
}
