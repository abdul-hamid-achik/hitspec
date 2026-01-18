package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// SignAWSRequest signs a request using AWS Signature Version 4.
// Note: This function mutates req.Headers as a side effect, setting Host, X-Amz-Date,
// and X-Amz-Content-Sha256 headers which are required for AWS signature verification.
// The returned string is the Authorization header value.
func SignAWSRequest(req *Request) (string, error) {
	if req.AWSAuth == nil {
		return "", fmt.Errorf("AWS auth credentials not provided")
	}

	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return "", err
	}

	t := time.Now().UTC()
	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")

	// Get host from URL
	host := parsedURL.Host

	// Create canonical headers
	signedHeaders := "host;x-amz-date"
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-date:%s\n", host, amzDate)

	// Calculate payload hash
	payloadHash := sha256Hash(req.Body)

	// Create canonical URI
	canonicalURI := parsedURL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	// Create canonical query string
	canonicalQueryString := createCanonicalQueryString(parsedURL.Query())

	// Create canonical request
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	// Create string to sign
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request",
		dateStamp, req.AWSAuth.Region, req.AWSAuth.Service)

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hash(canonicalRequest),
	}, "\n")

	// Calculate signature
	signingKey := getSignatureKey(req.AWSAuth.SecretKey, dateStamp, req.AWSAuth.Region, req.AWSAuth.Service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	// Create authorization header
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		req.AWSAuth.AccessKey, credentialScope, signedHeaders, signature)

	// Set required headers on the request
	req.Headers["Host"] = host
	req.Headers["X-Amz-Date"] = amzDate
	req.Headers["X-Amz-Content-Sha256"] = payloadHash

	return authHeader, nil
}

func createCanonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	var keys []string
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		vals := values[k]
		sort.Strings(vals)
		for _, v := range vals {
			pairs = append(pairs, fmt.Sprintf("%s=%s",
				url.QueryEscape(k),
				url.QueryEscape(v)))
		}
	}

	return strings.Join(pairs, "&")
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
