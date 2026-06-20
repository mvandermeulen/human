package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestGoNativeIndex_genericsDegradeGracefully indexes a package that uses
// generics, which makes the x/tools CHA call-graph builder panic. The backend
// must recover and still emit symbols/references rather than aborting.
func TestGoNativeIndex_genericsDegradeGracefully(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/gen\n\ngo 1.21\n",
		"main.go": `package main

// Map is generic, which trips the x/tools call-graph builder.
func Map[T any](xs []T) []T { return xs }

func use() { _ = Map([]int{1, 2, 3}) }

func main() { use() }
`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sink := newCollectSink()
	scan := RepoScan{Project: "gen", Root: dir}
	if err := (GoNative{}).Index(context.Background(), scan, sink); err != nil {
		t.Fatalf("Index returned error instead of degrading: %v", err)
	}
	if _, ok := sink.symbols["example.com/gen.Map"]; !ok {
		t.Errorf("generic symbol Map was not indexed (got %d symbols)", len(sink.symbols))
	}
}
