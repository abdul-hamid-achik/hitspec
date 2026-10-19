package openapi

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

const tinySpec = `openapi: 3.0.0
info: {title: T, version: "1"}
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "200": {description: ok}
`

func TestConvertFile_FromURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(tinySpec))
	}))
	defer srv.Close()

	out, err := NewConverter().ConvertFile(srv.URL + "/openapi.yaml")
	if err != nil {
		t.Fatalf("ConvertFile(url): %v", err)
	}
	if !strings.Contains(out, "/pets") {
		t.Errorf("converted output lacks /pets:\n%s", out)
	}
}

func TestConvertFile_FromURLFollowsRelativeRefs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`openapi: 3.0.0
info: {title: T, version: "1"}
paths:
  /pets:
    $ref: "./paths/pets.yaml"
`))
	})
	mux.HandleFunc("/api/paths/pets.yaml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`get:
  operationId: listPets
  responses:
    "200": {description: ok}
`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	out, err := NewConverter().ConvertFile(srv.URL + "/api/openapi.yaml")
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "GET") || !strings.Contains(out, "/pets") {
		t.Errorf("$ref over HTTP was not followed:\n%s", out)
	}
}

func TestConvertFile_URLTimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	start := time.Now()
	_, err := NewConverter(WithTimeout(300*time.Millisecond)).ConvertFile(srv.URL + "/hang.yaml")
	if err == nil {
		t.Fatal("a hanging server must produce an error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %s; the timeout was not applied", elapsed)
	}
}

func TestConvertFile_URLBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	_, err := NewConverter().ConvertFile(srv.URL + "/missing.yaml")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("want a 404 error, got %v", err)
	}
}

func TestDefaultLoadTimeout(t *testing.T) {
	if got := NewConverter().timeout; got != 30*time.Second {
		t.Errorf("default timeout = %s, want 30s", got)
	}
	if got := NewConverter(WithTimeout(0)).timeout; got != DefaultLoadTimeout {
		t.Errorf("WithTimeout(0) should keep the default, got %s", got)
	}
}

// The spec client must pick up HTTP_PROXY/HTTPS_PROXY/NO_PROXY.
func TestSpecClientHonorsProxyEnvironment(t *testing.T) {
	tr, ok := newSpecClient(time.Second).Transport.(*http.Transport)
	if !ok || tr.Proxy == nil {
		t.Fatal("spec client has no proxy func")
	}
	if reflect.ValueOf(tr.Proxy).Pointer() != reflect.ValueOf(http.ProxyFromEnvironment).Pointer() {
		t.Error("spec client proxy is not http.ProxyFromEnvironment")
	}
}
