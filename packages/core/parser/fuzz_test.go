package parser

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzParse checks that no input makes the parser panic. The verbatim
// readers seek the lexer back to line starts by byte offset, so malformed
// input (stray quotes, CR-only line endings, unterminated blocks) is the
// interesting space.
func FuzzParse(f *testing.F) {
	seeds, _ := filepath.Glob(filepath.Join("..", "..", "..", "examples", "*", "*.http"))
	for _, path := range seeds {
		if data, err := os.ReadFile(path); err == nil {
			f.Add(string(data))
		}
	}
	f.Add("GET https://x.test/a HTTP/1.1\r\nX-A: \"q\"\r\n# c\r\nX-B: 'it's\r\n\r\n{\"a\":1}\r\n>>>\r\nexpect status 200\r\n<<<\r\n")
	f.Add("### a\rPOST {{ base }}/x?q={{$urlEncode(a b)}}\r# @auth basic a, b\rX: #fff\r\r'unterminated")
	f.Add("@v = {{v}}\n### r\n# @auth oauth2 client_credentials, {{u}}, id, secret, s1, s2\nGET /x\n")
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = Parse(input, "fuzz.http")
	})
}
