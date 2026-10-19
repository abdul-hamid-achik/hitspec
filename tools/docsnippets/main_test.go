package main

import (
	"strings"
	"testing"
)

func TestExtract(t *testing.T) {
	doc := "intro\n" +
		"```http\nGET http://x/y\n```\n" +
		"```bash\nhitspec run a.http\n```\n" +
		"```json\n{}\n```\n" +
		"{/* docsnippets: skip */}\n" +
		"```http\nGARBAGE\n```\n" +
		"<Steps>\n  <Step>\n    ```http\n    GET http://x/z\n    ```\n  </Step>\n</Steps>\n"
	got := extract("doc.mdx", doc)
	if len(got) != 4 {
		t.Fatalf("got %d snippets, want 4: %+v", len(got), got)
	}
	if got[0].lang != "http" || got[0].line != 2 || got[0].skip {
		t.Errorf("first snippet = %+v", got[0])
	}
	if got[1].lang != "bash" {
		t.Errorf("second snippet lang = %q", got[1].lang)
	}
	if !got[2].skip {
		t.Errorf("snippet after the skip marker should be skipped: %+v", got[2])
	}
	if got[3].body != "GET http://x/z\n" {
		t.Errorf("indented fence body not dedented: %q", got[3].body)
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string // substring of a problem; "" means clean
	}{
		{"valid request", "### A\nGET http://x/y\n\n>>>\nexpect status 200\n<<<\n", ""},
		{"valid variable fragment", "@baseUrl = http://x\n", ""},
		{"valid contract annotation", "### A\n# @contract.state ready\nGET http://x/y\n", ""},
		{"unknown directive", "### A\n# @assert status 200\nGET http://x/y\n", "unknown directive"},
		{"at-line is not syntax", "### A\nGET http://x/y\n\n@capture t <- jsonpath $.t\n", "not hitspec syntax"},
		{"bad timeout", "### A\n# @timeout soon\nGET http://x/y\n", "parser warning"},
		{"misspelled directive", "### A\n# @tgas smoke\nGET http://x/y\n", "parser warning"},
		{"bad think time", "### A\n# @stress.think 500ms\nGET http://x/y\n", "needs an integer"},
		{"unclosed assertion block", "### A\nGET http://x/y\n\n>>>\nexpect status 200\n", "parse error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := check(snippet{file: "doc.md", line: 1, lang: "http", body: tt.body})
			joined := strings.Join(problems, "\n")
			switch {
			case tt.want == "" && len(problems) > 0:
				t.Fatalf("unexpected problems: %s", joined)
			case tt.want != "" && !strings.Contains(joined, tt.want):
				t.Fatalf("problems %q do not contain %q", joined, tt.want)
			}
		})
	}
}
