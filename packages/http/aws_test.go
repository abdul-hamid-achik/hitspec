package http

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AWS Signature Version 4 test suite (aws-sig-v4-test-suite), credentials
// AKIDEXAMPLE, region us-east-1, service "service", 20150830T123600Z.
// These vectors predate x-amz-content-sha256 so it is left out of the signed
// headers (SignContentSHA false).
func TestSigV4_AWSTestSuiteVectors(t *testing.T) {
	creds := AWSAuthCredentials{
		AccessKey: "AKIDEXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		Region:    "us-east-1",
		Service:   "service",
	}
	const date = "20150830T123600Z"
	emptyHash := sha256Hash("")

	cases := []struct {
		name, method, url string
		want              string
	}{
		{
			name: "get-vanilla", method: "GET", url: "https://example.amazonaws.com/",
			want: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31",
		},
		{
			name: "post-vanilla", method: "POST", url: "https://example.amazonaws.com/",
			want: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5da7c1a2acd57cee7505fc6676e4e544621c30862966e37dddb68e92efbe5d6b",
		},
		{
			name: "get-vanilla-query-order-key-case", method: "GET", url: "https://example.amazonaws.com/?Param2=value2&Param1=value1",
			want: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=b97d918cfa904a5beff61c982a1b6f458b799221646efd99d3219ec94cdf2500",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.url)
			require.NoError(t, err)
			got := sigV4Sign(sigV4Input{
				Method: tc.method, URL: u, PayloadHash: emptyHash,
				Creds: creds, AmzDate: date,
			})
			assert.Equal(t, tc.want, got)
		})
	}
}

func fixedAWSNow(t *testing.T) {
	t.Helper()
	old := awsNow
	awsNow = func() time.Time { return time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC) }
	t.Cleanup(func() { awsNow = old })
}

func TestSignAWSRequest_SignsContentHashAndSetsHeaders(t *testing.T) {
	fixedAWSNow(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SESSION_TOKEN", "")
	req := NewRequest("POST", "https://example.amazonaws.com/")
	req.Body = `{"a":1}`
	req.AWSAuth = &AWSAuthCredentials{AccessKey: "AK", SecretKey: "SK", Region: "us-east-1", Service: "service"}

	auth, err := SignAWSRequest(req)
	require.NoError(t, err)
	assert.Contains(t, auth, "SignedHeaders=host;x-amz-content-sha256;x-amz-date,")
	assert.Equal(t, sha256Hash(`{"a":1}`), req.Headers["X-Amz-Content-Sha256"])
	assert.Equal(t, "20150830T123600Z", req.Headers["X-Amz-Date"])
	assert.Equal(t, "example.amazonaws.com", req.Headers["Host"])
	assert.NotContains(t, req.Headers, "X-Amz-Security-Token")
}

func TestSignAWSRequest_NilHeadersDoNotPanic(t *testing.T) {
	fixedAWSNow(t)
	req := &Request{Method: "GET", URL: "https://example.amazonaws.com/", AWSAuth: &AWSAuthCredentials{AccessKey: "AK", SecretKey: "SK", Region: "r", Service: "s"}}
	_, err := SignAWSRequest(req)
	require.NoError(t, err)
}

func TestSignAWSRequest_SessionToken(t *testing.T) {
	fixedAWSNow(t)
	newReq := func() *Request {
		req := NewRequest("GET", "https://example.amazonaws.com/")
		req.AWSAuth = &AWSAuthCredentials{AccessKey: "AK", SecretKey: "SK", Region: "us-east-1", Service: "service"}
		return req
	}

	t.Run("explicit", func(t *testing.T) {
		t.Setenv("AWS_ACCESS_KEY_ID", "")
		t.Setenv("AWS_SESSION_TOKEN", "")
		req := newReq()
		req.AWSAuth.SessionToken = "tok/en=="
		auth, err := SignAWSRequest(req)
		require.NoError(t, err)
		assert.Equal(t, "tok/en==", req.Headers["X-Amz-Security-Token"])
		assert.Contains(t, auth, "SignedHeaders=host;x-amz-content-sha256;x-amz-date;x-amz-security-token,")

		plain := newReq()
		plainAuth, err := SignAWSRequest(plain)
		require.NoError(t, err)
		assert.NotEqual(t, sigOf(plainAuth), sigOf(auth), "token is part of the signature")
	})

	t.Run("from environment when the access key matches", func(t *testing.T) {
		t.Setenv("AWS_ACCESS_KEY_ID", "AK")
		t.Setenv("AWS_SESSION_TOKEN", "env-token")
		req := newReq()
		_, err := SignAWSRequest(req)
		require.NoError(t, err)
		assert.Equal(t, "env-token", req.Headers["X-Amz-Security-Token"])
	})

	t.Run("environment token ignored for other credentials", func(t *testing.T) {
		t.Setenv("AWS_ACCESS_KEY_ID", "SOMEONE-ELSE")
		t.Setenv("AWS_SESSION_TOKEN", "env-token")
		req := newReq()
		_, err := SignAWSRequest(req)
		require.NoError(t, err)
		assert.NotContains(t, req.Headers, "X-Amz-Security-Token")
	})
}

func sigOf(auth string) string {
	return auth[strings.LastIndex(auth, "Signature=")+len("Signature="):]
}

func TestSignAWSRequest_SignsAmzAndContentTypeHeaders(t *testing.T) {
	fixedAWSNow(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	req := NewRequest("POST", "https://dynamodb.us-east-1.amazonaws.com/")
	req.AWSAuth = &AWSAuthCredentials{AccessKey: "AK", SecretKey: "SK", Region: "us-east-1", Service: "dynamodb"}
	req.Headers["Content-Type"] = "application/x-amz-json-1.0"
	req.Headers["X-Amz-Target"] = "DynamoDB_20120810.ListTables"
	req.Headers["X-Custom"] = "not signed"
	auth, err := SignAWSRequest(req)
	require.NoError(t, err)
	assert.Contains(t, auth, "SignedHeaders=content-type;host;x-amz-content-sha256;x-amz-date;x-amz-target,")
}

func TestCanonicalURI(t *testing.T) {
	cases := []struct {
		url, service, want string
	}{
		{"https://h", "service", "/"},
		{"https://h/", "service", "/"},
		{"https://h/a/b", "service", "/a/b"},
		// non-S3: the sent (escaped) path, encoded once more
		{"https://h/a%20b/c", "service", "/a%2520b/c"},
		{"https://h/%E1%88%B4", "service", "/%25E1%2588%25B4"},
		{"https://h/a+b", "service", "/a%2Bb"},
		{"https://h/a:b", "service", "/a%3Ab"},
		// non-S3: normalized
		{"https://h/a/./b//c/../d/", "service", "/a/b/d/"},
		// S3: the sent path verbatim, not normalized
		{"https://h/a%20b/c", "s3", "/a%20b/c"},
		{"https://h/%E1%88%B4", "s3", "/%E1%88%B4"},
		{"https://h/a+b", "s3", "/a+b"},
		{"https://h/a%2Bb", "s3", "/a%2Bb"},
		{"https://h/a~b-c_d.e", "s3", "/a~b-c_d.e"},
		{"https://h//double//slash", "s3", "//double//slash"},
	}
	for _, tc := range cases {
		u, err := url.Parse(tc.url)
		require.NoError(t, err)
		assert.Equal(t, tc.want, canonicalURI(u, tc.service), "%s (%s)", tc.url, tc.service)
	}
}

func TestCanonicalQueryString(t *testing.T) {
	u, _ := url.Parse("https://h/?b=2&a=1&a=0&sp=a%20b&plus=a%2Bb&tilde=~&star=*")
	// RFC 3986: space is %20 (not "+"), "~" is unreserved, "*" is encoded.
	assert.Equal(t, "a=0&a=1&b=2&plus=a%2Bb&sp=a%20b&star=%2A&tilde=~", createCanonicalQueryString(u.Query()))
	assert.Equal(t, "", createCanonicalQueryString(url.Values{}))
}
