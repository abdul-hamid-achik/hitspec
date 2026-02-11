package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- helpers ----------

// newTestServer returns an httptest.Server with a simple mux that serves
// GET /users, POST /users, GET /users/1, GET /health, and GET /error.
func newTestServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
		})
	})

	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   3,
			"name": "Charlie",
		})
	})

	mux.HandleFunc("GET /users/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   1,
			"name": "Alice",
		})
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("GET /error", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	})

	return httptest.NewServer(mux)
}

// writeHTTPFile is a small helper that writes content into dir/name and returns the full path.
func writeHTTPFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

// ---------- NewVerifier ----------

func TestNewVerifier_Defaults(t *testing.T) {
	v := NewVerifier()
	assert.NotNil(t, v)
	assert.NotNil(t, v.client)
	assert.NotNil(t, v.resolver)
	assert.Empty(t, v.providerURL)
	assert.Empty(t, v.stateHandler)
	assert.False(t, v.verbose)
}

func TestNewVerifier_WithOptions(t *testing.T) {
	v := NewVerifier(
		WithProviderURL("http://localhost:9999"),
		WithStateHandler("/tmp/state.sh"),
		WithVerbose(true),
	)
	assert.Equal(t, "http://localhost:9999", v.providerURL)
	assert.Equal(t, "/tmp/state.sh", v.stateHandler)
	assert.True(t, v.verbose)
}

// ---------- Functional option coverage ----------

func TestWithProviderURL(t *testing.T) {
	v := &Verifier{}
	WithProviderURL("http://example.com")(v)
	assert.Equal(t, "http://example.com", v.providerURL)
}

func TestWithStateHandler(t *testing.T) {
	v := &Verifier{}
	WithStateHandler("/bin/handler")(v)
	assert.Equal(t, "/bin/handler", v.stateHandler)
}

func TestWithVerbose(t *testing.T) {
	v := &Verifier{}
	WithVerbose(true)(v)
	assert.True(t, v.verbose)
	WithVerbose(false)(v)
	assert.False(t, v.verbose)
}

// ---------- Verify (parsed file, mock provider) ----------

func TestVerify_SimpleGET_NoAssertions_Success(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:     "Get Users",
				Method:   "GET",
				URL:      ts.URL + "/users",
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 0, result.Skipped)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Passed)
	assert.Equal(t, "Get Users", result.Results[0].Name)
}

func TestVerify_DefaultSuccessCheck_FailsOnServerError(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:     "Error Endpoint",
				Method:   "GET",
				URL:      ts.URL + "/error",
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 1, result.Failed)
	require.Len(t, result.Results, 1)
	assert.False(t, result.Results[0].Passed)
}

func TestVerify_WithAssertions_AllPass(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Get User 1",
				Method: "GET",
				URL:    ts.URL + "/users/1",
				Headers: []*parser.Header{
					{Key: "Accept", Value: "application/json"},
				},
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpEquals, Expected: 200},
					{Subject: "body.name", Operator: parser.OpEquals, Expected: "Alice"},
				},
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Failed)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Passed)
	require.Len(t, result.Results[0].Assertions, 2)
	for _, a := range result.Results[0].Assertions {
		assert.True(t, a.Passed, "assertion should have passed: %s", a.Message)
	}
}

func TestVerify_WithAssertions_OneFails(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Get User 1",
				Method: "GET",
				URL:    ts.URL + "/users/1",
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpEquals, Expected: 200},
					{Subject: "body.name", Operator: parser.OpEquals, Expected: "WRONG"},
				},
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 1, result.Failed)
	require.Len(t, result.Results, 1)
	assert.False(t, result.Results[0].Passed)
	// The first assertion (status) should pass, the second (body.name) should fail
	require.Len(t, result.Results[0].Assertions, 2)
	assert.True(t, result.Results[0].Assertions[0].Passed)
	assert.False(t, result.Results[0].Assertions[1].Passed)
}

func TestVerify_MultipleRequests_Aggregation(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:     "Health",
				Method:   "GET",
				URL:      ts.URL + "/health",
				Metadata: &parser.RequestMetadata{},
			},
			{
				Name:   "Get Users",
				Method: "GET",
				URL:    ts.URL + "/users",
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpEquals, Expected: 200},
				},
				Metadata: &parser.RequestMetadata{},
			},
			{
				Name:     "Server Error",
				Method:   "GET",
				URL:      ts.URL + "/error",
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	// Health (200 no assertions -> pass), Get Users (assertion passes -> pass), Server Error (500 no assertions -> fail)
	assert.Equal(t, 2, result.Passed)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, 0, result.Skipped)
	require.Len(t, result.Results, 3)
	assert.True(t, result.Duration > 0)
}

func TestVerify_ProviderURL_OverridesBaseUrl(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Variables: []*parser.Variable{
			{Name: "baseUrl", Value: "http://should-be-overridden.invalid"},
		},
		Requests: []*parser.Request{
			{
				Name:     "Health",
				Method:   "GET",
				URL:      "{{baseUrl}}/health",
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier(WithProviderURL(ts.URL))
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Failed)
}

func TestVerify_Variables_Resolved(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Variables: []*parser.Variable{
			{Name: "baseUrl", Value: ts.URL},
		},
		Requests: []*parser.Request{
			{
				Name:     "Health",
				Method:   "GET",
				URL:      "{{baseUrl}}/health",
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
}

func TestVerify_ContractFile_SetOnResult(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "/some/path/contract.http",
		Requests: []*parser.Request{
			{
				Name:     "Health",
				Method:   "GET",
				URL:      ts.URL + "/health",
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, "/some/path/contract.http", result.ContractFile)
}

func TestVerify_HTTPRequestFailure(t *testing.T) {
	// Point to a non-existent server
	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:     "Bad Request",
				Method:   "GET",
				URL:      "http://127.0.0.1:1/nonexistent",
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err) // Verify itself does not error; individual interactions do
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 1, result.Failed)
	require.Len(t, result.Results, 1)
	assert.NotNil(t, result.Results[0].Error)
	assert.False(t, result.Results[0].Passed)
}

func TestVerify_EmptyFile_NoRequests(t *testing.T) {
	file := &parser.File{
		Path:     "empty.http",
		Requests: nil,
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 0, result.Skipped)
	assert.Empty(t, result.Results)
}

// ---------- State extraction ----------

func TestExtractState_FromCustomAnnotation(t *testing.T) {
	v := NewVerifier()
	req := &parser.Request{
		Metadata: &parser.RequestMetadata{
			Custom: map[string]string{
				"contract.state": "user exists",
			},
		},
	}
	state := v.extractState(req)
	assert.Equal(t, "user exists", state)
}

func TestExtractState_FromDescription(t *testing.T) {
	v := NewVerifier()
	req := &parser.Request{
		Description: "state:user exists",
		Metadata:    &parser.RequestMetadata{},
	}
	state := v.extractState(req)
	assert.Equal(t, "user exists", state)
}

func TestExtractState_CustomAnnotationTakesPrecedence(t *testing.T) {
	v := NewVerifier()
	req := &parser.Request{
		Description: "state:from description",
		Metadata: &parser.RequestMetadata{
			Custom: map[string]string{
				"contract.state": "from annotation",
			},
		},
	}
	state := v.extractState(req)
	assert.Equal(t, "from annotation", state)
}

func TestExtractState_NoState(t *testing.T) {
	v := NewVerifier()

	// No metadata
	req := &parser.Request{}
	assert.Empty(t, v.extractState(req))

	// With metadata but no custom and no matching description
	req2 := &parser.Request{
		Description: "just a description",
		Metadata:    &parser.RequestMetadata{},
	}
	assert.Empty(t, v.extractState(req2))

	// Nil custom map
	req3 := &parser.Request{
		Metadata: &parser.RequestMetadata{
			Custom: nil,
		},
	}
	assert.Empty(t, v.extractState(req3))
}

// ---------- Contract provider metadata ----------

func TestVerify_ContractProviderExtracted(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Health",
				Method: "GET",
				URL:    ts.URL + "/health",
				Metadata: &parser.RequestMetadata{
					Custom: map[string]string{
						"contract.provider": "user-service",
					},
				},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.Equal(t, "user-service", result.Results[0].Provider)
}

// ---------- State setup ----------

func TestSetupState_NoHandler_NoError(t *testing.T) {
	v := NewVerifier()
	err := v.setupState("some state")
	assert.NoError(t, err)
}

func TestSetupState_WithHandler_ScriptNotFound(t *testing.T) {
	v := NewVerifier(WithStateHandler("/nonexistent/state-handler.sh"))
	err := v.setupState("some state")
	assert.Error(t, err)
}

func TestSetupState_WithHandler_Success(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "handler.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755))

	v := NewVerifier(WithStateHandler(script))
	err := v.setupState("user exists")
	assert.NoError(t, err)
}

func TestSetupState_WithHandler_Failure(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "handler.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755))

	v := NewVerifier(WithStateHandler(script))
	err := v.setupState("bad state")
	assert.Error(t, err)
}

func TestVerify_StateSetupFailure_RecordedInResult(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "handler.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755))

	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Get User",
				Method: "GET",
				URL:    ts.URL + "/users/1",
				Metadata: &parser.RequestMetadata{
					Custom: map[string]string{
						"contract.state": "user exists",
					},
				},
			},
		},
	}

	v := NewVerifier(WithStateHandler(script))
	result, err := v.Verify(file)
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.False(t, result.Results[0].Passed)
	assert.NotNil(t, result.Results[0].Error)
	assert.Contains(t, result.Results[0].Error.Error(), "failed to set up state")
	assert.Equal(t, "user exists", result.Results[0].State)
}

// ---------- VerifyFile ----------

func TestVerifyFile_Success(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()
	content := "### Health Check\nGET " + ts.URL + "/health\n"
	path := writeHTTPFile(t, dir, "health.http", content)

	v := NewVerifier()
	result, err := v.VerifyFile(path)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Failed)
}

func TestVerifyFile_WithAssertions(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()
	content := "### Get User\nGET " + ts.URL + "/users/1\n\n>>>\nexpect status == 200\nexpect body.name == \"Alice\"\n<<<\n"
	path := writeHTTPFile(t, dir, "user.http", content)

	v := NewVerifier()
	result, err := v.VerifyFile(path)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Failed)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Passed)
}

func TestVerifyFile_FileNotFound(t *testing.T) {
	v := NewVerifier()
	result, err := v.VerifyFile("/nonexistent/path/test.http")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse contract file")
}

func TestVerifyFile_ParseError(t *testing.T) {
	dir := t.TempDir()
	// A file that will cause a parse error: annotation without a method
	content := "### Missing Method\n# @skip\n\n>>>\nexpect status == 200\n<<<\n"
	path := writeHTTPFile(t, dir, "bad.http", content)

	v := NewVerifier()
	result, err := v.VerifyFile(path)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse contract file")
}

// ---------- VerifyDirectory ----------

func TestVerifyDirectory_MultipleFiles(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()

	writeHTTPFile(t, dir, "health.http", "### Health\nGET "+ts.URL+"/health\n")
	writeHTTPFile(t, dir, "users.hitspec", "### Users\nGET "+ts.URL+"/users\n\n>>>\nexpect status == 200\n<<<\n")

	v := NewVerifier()
	results, err := v.VerifyDirectory(dir)
	require.NoError(t, err)
	require.Len(t, results, 2)

	totalPassed := 0
	for _, r := range results {
		totalPassed += r.Passed
	}
	assert.Equal(t, 2, totalPassed)
}

func TestVerifyDirectory_IgnoresNonHTTPFiles(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()

	writeHTTPFile(t, dir, "readme.txt", "not an http file")
	writeHTTPFile(t, dir, "data.json", `{"key":"value"}`)
	writeHTTPFile(t, dir, "health.http", "### Health\nGET "+ts.URL+"/health\n")

	v := NewVerifier()
	results, err := v.VerifyDirectory(dir)
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestVerifyDirectory_Subdirectories(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(subdir, 0o755))

	writeHTTPFile(t, dir, "root.http", "### Root Health\nGET "+ts.URL+"/health\n")
	writeHTTPFile(t, subdir, "sub.http", "### Sub Health\nGET "+ts.URL+"/health\n")

	v := NewVerifier()
	results, err := v.VerifyDirectory(dir)
	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestVerifyDirectory_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	v := NewVerifier()
	results, err := v.VerifyDirectory(dir)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestVerifyDirectory_NonExistentDirectory(t *testing.T) {
	v := NewVerifier()
	results, err := v.VerifyDirectory("/nonexistent/directory")
	assert.Error(t, err)
	assert.Nil(t, results)
}

// ---------- Assertions evaluation ----------

func TestVerify_AssertionContains(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Health",
				Method: "GET",
				URL:    ts.URL + "/health",
				Assertions: []*parser.Assertion{
					{Subject: "body.status", Operator: parser.OpContains, Expected: "ok"},
				},
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
}

func TestVerify_AssertionNotEquals(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Health",
				Method: "GET",
				URL:    ts.URL + "/health",
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpNotEquals, Expected: 500},
				},
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
}

func TestVerify_AssertionExists(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Get User",
				Method: "GET",
				URL:    ts.URL + "/users/1",
				Assertions: []*parser.Assertion{
					{Subject: "body.id", Operator: parser.OpExists},
					{Subject: "body.name", Operator: parser.OpExists},
				},
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
}

func TestVerify_AssertionGreaterThan(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Get User",
				Method: "GET",
				URL:    ts.URL + "/users/1",
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpGreaterOrEqual, Expected: 200},
					{Subject: "status", Operator: parser.OpLessThan, Expected: 300},
				},
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
}

// ---------- InteractionResult fields ----------

func TestVerify_InteractionResult_Duration(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:     "Health",
				Method:   "GET",
				URL:      ts.URL + "/health",
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Duration > 0)
}

func TestVerify_InteractionResult_Description(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:        "Health",
				Description: "Check the health endpoint",
				Method:      "GET",
				URL:         ts.URL + "/health",
				Metadata:    &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.Equal(t, "Check the health endpoint", result.Results[0].Description)
}

// ---------- POST with body ----------

func TestVerify_POSTWithBody(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Create User",
				Method: "POST",
				URL:    ts.URL + "/users",
				Headers: []*parser.Header{
					{Key: "Content-Type", Value: "application/json"},
				},
				Body: &parser.Body{
					ContentType: parser.BodyJSON,
					Raw:         `{"name":"Charlie"}`,
				},
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpEquals, Expected: 201},
					{Subject: "body.name", Operator: parser.OpEquals, Expected: "Charlie"},
				},
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Failed)
}

// ---------- VerificationResult type ----------

func TestVerificationResult_ZeroValue(t *testing.T) {
	r := VerificationResult{}
	assert.Empty(t, r.ContractFile)
	assert.Equal(t, 0, r.Passed)
	assert.Equal(t, 0, r.Failed)
	assert.Equal(t, 0, r.Skipped)
	assert.Nil(t, r.Results)
}

func TestInteractionResult_ZeroValue(t *testing.T) {
	r := InteractionResult{}
	assert.Empty(t, r.Name)
	assert.Empty(t, r.Description)
	assert.Empty(t, r.Provider)
	assert.Empty(t, r.State)
	assert.False(t, r.Passed)
	assert.Nil(t, r.Error)
	assert.Nil(t, r.Assertions)
}

// ---------- VerifyFile with contract annotations ----------

func TestVerifyFile_WithContractAnnotations(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()
	content := "### Get User\n# @contract.provider user-service\n# @contract.state user exists\nGET " + ts.URL + "/users/1\n\n>>>\nexpect status == 200\n<<<\n"
	path := writeHTTPFile(t, dir, "contract.http", content)

	v := NewVerifier()
	result, err := v.VerifyFile(path)
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.Equal(t, "user-service", result.Results[0].Provider)
	assert.Equal(t, "user exists", result.Results[0].State)
	assert.True(t, result.Results[0].Passed)
}

// ---------- VerifyFile with variables ----------

func TestVerifyFile_WithVariables(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()
	content := "@baseUrl = " + ts.URL + "\n\n### Health\nGET {{baseUrl}}/health\n\n>>>\nexpect status == 200\n<<<\n"
	path := writeHTTPFile(t, dir, "vars.http", content)

	v := NewVerifier()
	result, err := v.VerifyFile(path)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
}

// ---------- VerifyFile with provider URL override ----------

func TestVerifyFile_ProviderURLOverridesVariable(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()
	// Variable points to a bad URL, but provider URL should override
	content := "@baseUrl = http://127.0.0.1:1\n\n### Health\nGET {{baseUrl}}/health\n"
	path := writeHTTPFile(t, dir, "override.http", content)

	v := NewVerifier(WithProviderURL(ts.URL))
	result, err := v.VerifyFile(path)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
}

// ---------- Multiple requests mixed pass/fail ----------

func TestVerify_MixedPassFail_Aggregation(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Pass: status 200",
				Method: "GET",
				URL:    ts.URL + "/health",
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpEquals, Expected: 200},
				},
				Metadata: &parser.RequestMetadata{},
			},
			{
				Name:   "Fail: wrong status",
				Method: "GET",
				URL:    ts.URL + "/health",
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpEquals, Expected: 404},
				},
				Metadata: &parser.RequestMetadata{},
			},
			{
				Name:   "Pass: exists check",
				Method: "GET",
				URL:    ts.URL + "/users/1",
				Assertions: []*parser.Assertion{
					{Subject: "body.id", Operator: parser.OpExists},
				},
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Passed)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, 0, result.Skipped)
}

// ---------- Assertions Result type exposed ----------

func TestVerify_AssertionResultsExposed(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "User",
				Method: "GET",
				URL:    ts.URL + "/users/1",
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpEquals, Expected: 200, Line: 5},
					{Subject: "body.name", Operator: parser.OpEquals, Expected: "Alice", Line: 6},
				},
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	require.Len(t, result.Results, 1)

	interResult := result.Results[0]
	require.Len(t, interResult.Assertions, 2)

	// Check that assertions.Result fields are populated
	a0 := interResult.Assertions[0]
	assert.True(t, a0.Passed)
	assert.Equal(t, "status", a0.Subject)

	a1 := interResult.Assertions[1]
	assert.True(t, a1.Passed)
	assert.Equal(t, "body.name", a1.Subject)
}

// Ensure assertions.Result type is the one from the assertions package
func TestVerify_AssertionResultType(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "User",
				Method: "GET",
				URL:    ts.URL + "/users/1",
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpEquals, Expected: 200},
				},
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	require.Len(t, result.Results[0].Assertions, 1)

	// Verify it is *assertions.Result
	var _ *assertions.Result = result.Results[0].Assertions[0]
}

// ---------- VerifyDirectory with parse error in one file ----------

func TestVerifyDirectory_ParseErrorStopsProcessing(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()
	writeHTTPFile(t, dir, "good.http", "### Health\nGET "+ts.URL+"/health\n")
	// This file has a separator but no method, causing a parse error
	writeHTTPFile(t, dir, "bad.http", "### Missing Method\n# @skip\n\n>>>\nexpect status == 200\n<<<\n")

	v := NewVerifier()
	_, err := v.VerifyDirectory(dir)
	assert.Error(t, err)
}

// ---------- State with verbose ----------

func TestSetupState_Verbose_NoHandler(t *testing.T) {
	// Just verify it doesn't panic when verbose is true and no handler is set
	v := NewVerifier(WithVerbose(true))
	err := v.setupState("some state")
	assert.NoError(t, err)
}

// ---------- Header forwarding ----------

func TestVerify_HeadersForwarded(t *testing.T) {
	var receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Auth Request",
				Method: "GET",
				URL:    ts.URL + "/protected",
				Headers: []*parser.Header{
					{Key: "Authorization", Value: "Bearer my-token"},
				},
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpEquals, Expected: 200},
				},
				Metadata: &parser.RequestMetadata{},
			},
		},
	}

	v := NewVerifier()
	result, err := v.Verify(file)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, "Bearer my-token", receivedAuth)
}

// ---------- Method and URL on the request ----------

func TestVerify_MethodForwarded(t *testing.T) {
	var receivedMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			file := &parser.File{
				Path: "test.http",
				Requests: []*parser.Request{
					{
						Name:     method + " test",
						Method:   method,
						URL:      ts.URL + "/endpoint",
						Metadata: &parser.RequestMetadata{},
					},
				},
			}

			v := NewVerifier()
			_, err := v.Verify(file)
			require.NoError(t, err)
			assert.Equal(t, method, receivedMethod)
		})
	}
}
