package http

import (
	"io"
	nethttp "net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

func identity(s string) string { return s }

func build(t *testing.T, src, baseDir string) *Request {
	t.Helper()
	f, err := parser.Parse(src, "t.http")
	require.NoError(t, err)
	return BuildRequestFromASTWithBaseDir(f.Requests[0], identity, baseDir)
}

func TestBuildURL_KeepsOrderDuplicatesAndEncodesSpaces(t *testing.T) {
	r := build(t, "### q\nGET http://h/p?z=1\n? name = John Smith\n? tag = x\n? tag = y\n? s = a&b=c+d\n", "")
	assert.Equal(t, "http://h/p?z=1&name=John%20Smith&tag=x&tag=y&s=a%26b%3Dc%2Bd", r.URL)

	// No query in the URL yet: the first parameter starts it.
	r = build(t, "### q\nGET http://h/p\n? b = 2\n? a = 1\n", "")
	assert.Equal(t, "http://h/p?b=2&a=1", r.URL, "order is the order written")
}

func TestBuildURL_EscapesOnlyWhatCannotBeSent(t *testing.T) {
	for in, want := range map[string]string{
		"http://h/a b?x=John Smith":      "http://h/a%20b?x=John%20Smith",
		"http://h/?a=1%202&b=3":          "http://h/?a=1%202&b=3", // existing escape kept
		"http://h/?p=1+2&q=a:b/c,d;e=f":  "http://h/?p=1+2&q=a:b/c,d;e=f",
		"http://h/?u=ü":                  "http://h/?u=%C3%BC",
		"http://h/?q=\"x\"&r=<y>":        "http://h/?q=%22x%22&r=%3Cy%3E",
		"http://h/?pct=100%":             "http://h/?pct=100%25",
		"http://h/?a[0]=1&a[1]=2":        "http://h/?a[0]=1&a[1]=2",
		"http://h/p#frag ment":           "http://h/p#frag%20ment",
		"http://[::1]:8080/x y":          "http://[::1]:8080/x%20y",
		"http://user@h:1/":               "http://user@h:1/",
		"http://h":                       "http://h",
		"http://h?a=b c":                 "http://h?a=b%20c",
		"{{unresolved}}/x y":             "{{unresolved}}/x y", // not a URL yet: untouched
		"http://h/?x=1&y={{unresolved}}": "http://h/?x=1&y=%7B%7Bunresolved%7D%7D",
	} {
		r := &Request{URL: in}
		assert.Equal(t, want, r.BuildURL(), in)
	}
}

func TestBuildURL_ParamsAfterFragmentlessAndFragmentURLs(t *testing.T) {
	r := NewRequest("GET", "http://h/p?a=1#top")
	r.AddQueryParam("b", "x y")
	assert.Equal(t, "http://h/p?a=1&b=x%20y#top", r.BuildURL())

	r = NewRequest("GET", "http://h/p?")
	r.AddQueryParam("b", "1")
	assert.Equal(t, "http://h/p?b=1", r.BuildURL())

	r = NewRequest("GET", "http://h/p")
	r.SetQueryParam("k", "1").SetQueryParam("j", "2").SetQueryParam("k", "3")
	assert.Equal(t, "http://h/p?k=3&j=2", r.BuildURL(), "SetQueryParam replaces in place")
}

func TestFormBlockBodyIsEncodedAndKeepsSpaces(t *testing.T) {
	r := build(t, "### f\nPOST http://h/\n\n& name = John Smith\n& tag = x\n& tag = y\n& note = a&b=c\n", "")
	assert.Equal(t, "name=John%20Smith&tag=x&tag=y&note=a%26b%3Dc", r.Body)
	assert.Equal(t, "application/x-www-form-urlencoded", r.Headers["Content-Type"])
}

func TestBodyFileMissingIsARequestError(t *testing.T) {
	dir := t.TempDir()
	r := build(t, "### b\nPOST http://127.0.0.1:1/\n\n< ./nope.json\n", dir)
	require.Error(t, r.Err)
	assert.Contains(t, r.Err.Error(), "nope.json")
	assert.Empty(t, r.Body, "the literal '< ./nope.json' text must not become the body")

	_, err := NewClient().Do(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope.json")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "ok.json"), []byte(`{"a":1}`), 0o600))
	r = build(t, "### b\nPOST http://127.0.0.1:1/\n\n< ./ok.json\n", dir)
	require.NoError(t, r.Err)
	assert.Equal(t, `{"a":1}`, r.Body)
}

func TestMultipartMissingFileNamesThePath(t *testing.T) {
	dir := t.TempDir()
	r := build(t, "### m\nPOST http://127.0.0.1:1/\n\n>>>multipart\nfile doc @./gone.png\n<<<\n", dir)
	_, err := NewClient().Do(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gone.png")
}

func TestMultipartFilePartCarriesTheFieldName(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600))
	for line, want := range map[string]string{
		"file avatar @./a.txt": "avatar",
		"file @./a.txt":        "file",
		"file ./a.txt":         "file",
	} {
		var gotName, gotFile string
		srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
			require.NoError(t, r.ParseMultipartForm(1<<20))
			for name, files := range r.MultipartForm.File {
				gotName, gotFile = name, files[0].Filename
			}
		}))
		r := build(t, "### m\nPOST "+srv.URL+"/\n\n>>>multipart\n"+line+"\n<<<\n", dir)
		_, err := NewClient().Do(r)
		srv.Close()
		require.NoError(t, err, line)
		assert.Equal(t, want, gotName, line)
		assert.Equal(t, "a.txt", gotFile, line)
	}
}

func TestApplyAuthAWSSessionToken(t *testing.T) {
	r := build(t, "### a\n# @auth aws AK, SK, us-east-1, s3, the-token\nGET http://h/\n", "")
	require.NotNil(t, r.AWSAuth)
	assert.Equal(t, "the-token", r.AWSAuth.SessionToken)

	r = build(t, "### a\n# @auth aws AK, SK, us-east-1, s3\nGET http://h/\n", "")
	assert.Empty(t, r.AWSAuth.SessionToken)
}

// The Authorization header must match the bytes that go on the wire, also for
// a multipart body that is assembled at send time with a random boundary.
func TestAWSAuthSignsTheMultipartBodyThatIsSent(t *testing.T) {
	fixedAWSNow(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SESSION_TOKEN", "")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600))

	var (
		authz, sentHash, ctype, token string
		body                          []byte
	)
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		authz, sentHash = r.Header.Get("Authorization"), r.Header.Get("X-Amz-Content-Sha256")
		ctype, token = r.Header.Get("Content-Type"), r.Header.Get("X-Amz-Security-Token")
		body, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()

	src := "### up\n# @auth aws AK, SK, us-east-1, s3, sess-token\nPUT " + srv.URL + "/bucket/key\n\n" +
		">>>multipart\nfile doc @./a.txt\nfield note = hi\n<<<\n"
	r := build(t, src, dir)
	_, err := NewClient().Do(r)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(ctype, "multipart/form-data; boundary="))
	assert.Contains(t, string(body), "hello")
	assert.Equal(t, sha256Hash(string(body)), sentHash, "x-amz-content-sha256 must hash the bytes actually sent")
	assert.Equal(t, "sess-token", token)

	// Recompute the signature the way a verifying server would: from what it received.
	u := srv.Listener.Addr().String()
	parsed := mustParse(t, srv.URL+"/bucket/key")
	want := sigV4Sign(sigV4Input{
		Method:         "PUT",
		URL:            parsed,
		Headers:        map[string]string{"Content-Type": ctype, "X-Amz-Content-Sha256": sentHash},
		PayloadHash:    sha256Hash(string(body)),
		Creds:          AWSAuthCredentials{AccessKey: "AK", SecretKey: "SK", Region: "us-east-1", Service: "s3", SessionToken: "sess-token"},
		AmzDate:        "20150830T123600Z",
		SignContentSHA: true,
	})
	assert.Equal(t, want, authz, "signature for host %s", u)
}

// Known-answer test: independently computed (HMAC-SHA256 key derivation in
// Python) SigV4 signature for a POST with a session token at a fixed clock.
func TestSignAWSRequest_SessionTokenKnownSignature(t *testing.T) {
	fixedAWSNow(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SESSION_TOKEN", "")
	r := build(t, "### a\n# @auth aws AKIDEXAMPLE, wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY, us-east-1, service, session/token==\nPOST https://example.amazonaws.com/\n\n{\"a\":1}\n", "")
	delete(r.Headers, "Content-Type") // the JSON body got one; the reference signature has none
	got, err := SignAWSRequest(r)
	require.NoError(t, err)
	assert.Equal(t, "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, "+
		"SignedHeaders=host;x-amz-content-sha256;x-amz-date;x-amz-security-token, "+
		"Signature=b810728d480b2a3765d0a9854382a37f4a01bea9773f877e179b4140eb1b3e10", got)
	assert.Equal(t, "session/token==", r.Headers["X-Amz-Security-Token"])
}

func mustParse(t *testing.T, raw string) *neturl.URL {
	t.Helper()
	u, err := neturl.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestAPIKeyQueryReplacesTheSameNamedURLParam(t *testing.T) {
	r := build(t, "### k\n# @auth apikey-query api_key, new\nGET http://h/x?a=1&api_key=old&b=2#top\n? c = 3\n", "")
	assert.Equal(t, "http://h/x?a=1&b=2&c=3&api_key=new#top", r.URL)

	r = build(t, "### k\n# @auth apikey-query api_key, new\nGET http://h/x?api_key=old\n", "")
	assert.Equal(t, "http://h/x?api_key=new", r.URL)
}
