package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// awsNow is the clock used for signing; tests replace it.
var awsNow = time.Now

// SignAWSRequest signs a request using AWS Signature Version 4.
//
// Note: This function mutates req.Headers as a side effect, setting Host,
// X-Amz-Date, X-Amz-Content-Sha256 and (when a session token is configured)
// X-Amz-Security-Token, which are required for AWS signature verification.
// The returned string is the Authorization header value.
//
// What is signed:
//   - Signed headers are host, x-amz-content-sha256, x-amz-date,
//     x-amz-security-token (only with a session token), plus content-type,
//     content-md5 and any other x-amz-* header already present on the request.
//   - The canonical path is built from the path that is actually sent
//     (RFC 3986: only A-Z a-z 0-9 - _ . ~ stay unescaped, "/" separates
//     segments). For every service except s3 the path is normalized (dot
//     segments and duplicate slashes removed) and URI-encoded a second time,
//     as the SigV4 specification requires; s3 uses the single-encoded path
//     verbatim.
//   - The canonical query string is RFC 3986 encoded (a space is %20, never
//     "+"), sorted by encoded name then value.
//
// A session token (temporary credentials) is taken from
// AWSAuthCredentials.SessionToken, or, when that is empty and the access key
// equals $AWS_ACCESS_KEY_ID, from $AWS_SESSION_TOKEN.
func SignAWSRequest(req *Request) (string, error) {
	if req.AWSAuth == nil {
		return "", fmt.Errorf("AWS auth credentials not provided")
	}

	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return "", err
	}

	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}

	creds := *req.AWSAuth
	if creds.SessionToken == "" && creds.AccessKey != "" && creds.AccessKey == os.Getenv("AWS_ACCESS_KEY_ID") {
		creds.SessionToken = os.Getenv("AWS_SESSION_TOKEN")
	}

	t := awsNow().UTC()
	amzDate := t.Format("20060102T150405Z")
	payloadHash := sha256Hash(req.Body)

	signed := sigV4Sign(sigV4Input{
		Method:         req.Method,
		URL:            parsedURL,
		Headers:        req.Headers,
		PayloadHash:    payloadHash,
		Creds:          creds,
		AmzDate:        amzDate,
		SignContentSHA: true,
	})

	// Set required headers on the request
	req.Headers["Host"] = parsedURL.Host
	req.Headers["X-Amz-Date"] = amzDate
	req.Headers["X-Amz-Content-Sha256"] = payloadHash
	if creds.SessionToken != "" {
		req.Headers["X-Amz-Security-Token"] = creds.SessionToken
	}

	return signed, nil
}

type sigV4Input struct {
	Method      string
	URL         *url.URL
	Headers     map[string]string // user headers; only content-type, content-md5 and x-amz-* are signed
	PayloadHash string
	Creds       AWSAuthCredentials
	AmzDate     string
	// SignContentSHA adds x-amz-content-sha256 to the signed headers. It is
	// always true for real requests; the AWS test-suite vectors omit it.
	SignContentSHA bool
}

// sigV4Sign returns the Authorization header value for in.
func sigV4Sign(in sigV4Input) string {
	dateStamp := in.AmzDate[:8]

	headers := map[string]string{
		"host":       in.URL.Host,
		"x-amz-date": in.AmzDate,
	}
	if in.SignContentSHA {
		headers["x-amz-content-sha256"] = in.PayloadHash
	}
	if in.Creds.SessionToken != "" {
		headers["x-amz-security-token"] = in.Creds.SessionToken
	}
	for k, v := range in.Headers {
		lk := strings.ToLower(k)
		if lk == "content-type" || lk == "content-md5" || strings.HasPrefix(lk, "x-amz-") {
			if _, set := headers[lk]; !set {
				headers[lk] = v
			}
		}
	}

	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)

	var canonicalHeaders strings.Builder
	for _, k := range names {
		canonicalHeaders.WriteString(k)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(trimHeaderValue(headers[k]))
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(names, ";")

	canonicalRequest := strings.Join([]string{
		in.Method,
		canonicalURI(in.URL, in.Creds.Service),
		createCanonicalQueryString(in.URL.Query()),
		canonicalHeaders.String(),
		signedHeaders,
		in.PayloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request",
		dateStamp, in.Creds.Region, in.Creds.Service)

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		in.AmzDate,
		credentialScope,
		sha256Hash(canonicalRequest),
	}, "\n")

	signingKey := getSignatureKey(in.Creds.SecretKey, dateStamp, in.Creds.Region, in.Creds.Service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	return fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		in.Creds.AccessKey, credentialScope, signedHeaders, signature)
}

// trimHeaderValue trims a header value and collapses runs of spaces, as the
// SigV4 canonical-header rules require.
func trimHeaderValue(v string) string {
	return strings.Join(strings.Fields(v), " ")
}

// canonicalURI returns the SigV4 canonical URI for u. See SignAWSRequest.
func canonicalURI(u *url.URL, service string) string {
	if u.Path == "" {
		return "/"
	}
	if service == "s3" {
		return rfc3986EscapePath(u.Path)
	}
	// Every other service: normalize, then encode the (already encoded) path
	// a second time.
	return rfc3986EscapePath(rfc3986EscapePath(normalizePath(u.Path)))
}

// normalizePath removes dot segments and duplicate slashes, keeping a trailing
// slash and always returning an absolute path.
func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	trailing := strings.HasSuffix(p, "/")
	var out []string
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "", ".":
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		default:
			out = append(out, seg)
		}
	}
	res := "/" + strings.Join(out, "/")
	if trailing && res != "/" {
		res += "/"
	}
	return res
}

// rfc3986Escape percent-encodes everything except the RFC 3986 unreserved set
// (A-Z a-z 0-9 - _ . ~) using upper-case hex.
func rfc3986Escape(s string) string {
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexDigits[c>>4])
		b.WriteByte(hexDigits[c&15])
	}
	return b.String()
}

// rfc3986EscapePath encodes each segment of a decoded path, keeping "/".
func rfc3986EscapePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = rfc3986Escape(s)
	}
	return strings.Join(segs, "/")
}

func createCanonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	type pair struct{ k, v string }
	var pairs []pair
	for k, vals := range values {
		for _, v := range vals {
			pairs = append(pairs, pair{rfc3986Escape(k), rfc3986Escape(v)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].k != pairs[j].k {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v < pairs[j].v
	})

	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.k + "=" + p.v
	}
	return strings.Join(out, "&")
}

func sha256Hash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func getSignatureKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}
