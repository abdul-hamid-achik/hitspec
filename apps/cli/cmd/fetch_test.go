package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchCommandDirectRawAndFailSemantics(t *testing.T) {
	body := []byte{0, 1, 0xff}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			w.WriteHeader(http.StatusNotFound)
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()

	command := newFetchCommand()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{server.URL})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != string(body) {
		t.Fatalf("stdout = %v, want %v", stdout.Bytes(), body)
	}

	command = newFetchCommand()
	stdout.Reset()
	command.SetOut(&stdout)
	command.SetArgs([]string{server.URL + "/missing", "--fail"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "404") || stdout.String() != string(body) {
		t.Fatalf("stdout=%v err=%v", stdout.Bytes(), err)
	}
}

func TestFetchCommandSavedRequestAndAtomicOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer server.Close()
	directory := t.TempDir()
	requestFile := filepath.Join(directory, "api.http")
	content := "@base = " + server.URL + "\n\n### First\n# @name first\nGET {{base}}/first\n\n### Second\n# @name second\nGET {{base}}/second\n"
	if err := os.WriteFile(requestFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(directory, "response.txt")
	command := newFetchCommand()
	command.SetArgs([]string{requestFile, "--name", "second", "--output-file", output})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil || string(got) != "/second" {
		t.Fatalf("output=%q err=%v", got, err)
	}
	command = newFetchCommand()
	command.SetArgs([]string{requestFile, "--name", "first", "--output-file", output})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v, want no-overwrite error", err)
	}
	command = newFetchCommand()
	command.SetArgs([]string{requestFile, "--name", "first", "--output-file", output, "--force"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(output)
	if string(got) != "/first" {
		t.Fatalf("forced output = %q", got)
	}
}

func TestFetchCommandMarkdownSanitizesSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<h1>Docs</h1>`))
	}))
	defer server.Close()
	command := newFetchCommand()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{server.URL + "/page?token=secret", "--format", "markdown"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "# Docs") || strings.Contains(stdout.String(), "token=") {
		t.Fatalf("unexpected Markdown: %s", stdout.String())
	}
}
