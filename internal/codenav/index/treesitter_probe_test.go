package index

import (
	"strings"
	"testing"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// TestGotreesitterProbe validates the gotreesitter dependency end-to-end:
// detect language by filename, load grammar + bundled tags query, and extract
// definitions/references. This is the foundation the TreeSitter backend rests on.
func TestGotreesitterProbe(t *testing.T) {
	type probe struct {
		file     string
		src      string
		wantDefs []string // asserted (the curated v1 set)
		wantRefs []string
	}
	// Validated set — these must extract defs and refs.
	validated := []probe{
		{"x.py", "def alpha():\n    beta()\n\ndef beta():\n    pass\n", []string{"alpha", "beta"}, []string{"beta"}},
		{"x.ts", "export function alpha() { beta(); }\nfunction beta() {}\n", []string{"alpha", "beta"}, []string{"beta"}},
		{"x.js", "function alpha() { beta(); }\nfunction beta() {}\n", []string{"alpha", "beta"}, []string{"beta"}},
	}
	// Candidates — logged only, to decide whether they join the curated set.
	candidates := []probe{
		{"x.java", "class C {\n void alpha() { beta(); }\n void beta() {}\n}\n", nil, nil},
		{"x.php", "<?php\nfunction alpha() { beta(); }\nfunction beta() {}\n", nil, nil},
		{"x.rb", "def alpha\n  beta\nend\n\ndef beta\nend\n", nil, nil},
		{"x.rs", "fn alpha() { beta(); }\nfn beta() {}\n", nil, nil},
	}
	run := func(c probe, assert bool) {
		entry := grammars.DetectLanguage(c.file)
		if entry == nil {
			if assert {
				t.Errorf("%s: no language detected", c.file)
			}
			return
		}
		tagsQ := grammars.ResolveTagsQuery(*entry)
		tagger, err := gts.NewTagger(entry.Language(), tagsQ)
		if err != nil {
			t.Logf("%s (%s): NewTagger error: %v", c.file, entry.Name, err)
			if assert {
				t.Errorf("%s: NewTagger: %v", c.file, err)
			}
			return
		}
		defs, refs := map[string]bool{}, map[string]bool{}
		for _, tg := range tagger.Tag([]byte(c.src)) {
			switch {
			case strings.HasPrefix(tg.Kind, "definition"):
				defs[tg.Name] = true
			case strings.HasPrefix(tg.Kind, "reference"):
				refs[tg.Name] = true
			}
		}
		t.Logf("%s (%s): tagsQuery=%dB defs=%v refs=%v", c.file, entry.Name, len(tagsQ), defs, refs)
		if assert {
			for _, w := range c.wantDefs {
				if !defs[w] {
					t.Errorf("%s (%s): missing def %q (defs=%v refs=%v)", c.file, entry.Name, w, defs, refs)
				}
			}
			for _, w := range c.wantRefs {
				if !refs[w] {
					t.Errorf("%s (%s): missing ref %q (defs=%v refs=%v)", c.file, entry.Name, w, defs, refs)
				}
			}
		}
	}
	for _, c := range validated {
		run(c, true)
	}
	for _, c := range candidates {
		run(c, false)
	}
}
