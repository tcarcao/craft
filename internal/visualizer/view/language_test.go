package view

import "testing"

func TestLookupLanguage_MatchesWholeWordsOnly(t *testing.T) {
	cases := map[string]string{
		"go":          "go",
		"Golang/gin":  "go",
		"java":        "java",
		"javascript":  "nodejs",
		"Node.js":     "nodejs",
		"TypeScript":  "javascript",
		"C# 12":       "dot_net",
		"django":      "", // must not match "go"
		"mongo-shell": "", // must not match "go"
	}
	for lang, want := range cases {
		l, ok := lookupLanguage(lang)
		if want == "" {
			if ok {
				t.Errorf("lookupLanguage(%q) = %q, want no match", lang, l.sprite)
			}
			continue
		}
		if !ok || l.sprite != want {
			t.Errorf("lookupLanguage(%q) = %q (ok=%v), want %q", lang, l.sprite, ok, want)
		}
	}
}
