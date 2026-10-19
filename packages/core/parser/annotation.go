package parser

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// parseAnnotation parses a single annotation directive and applies it to the request.
func (p *Parser) parseAnnotation(req *Request) error {
	name := strings.ToLower(p.curToken.Value)
	value := ""
	if p.curToken.Literal != nil {
		value = p.curToken.Literal.(string)
	}

	switch name {
	case "name":
		// @name always overrides the separator name for better DX
		// This allows readable separators (### Get User Profile) with simple identifiers (@name getUser)
		req.Name = value
	case "description":
		req.Description = value
	case "tags":
		tags := strings.Split(value, ",")
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t != "" {
				req.Tags = append(req.Tags, t)
			}
		}
	case "skip":
		req.Metadata.Skip = value
		if req.Metadata.Skip == "" {
			req.Metadata.Skip = "skipped"
		}
	case "only":
		req.Metadata.Only = true
	case "timeout":
		if v, err := strconv.Atoi(value); err == nil {
			req.Metadata.Timeout = v
		} else if value != "" {
			fmt.Fprintf(os.Stderr, "warning: invalid timeout value %q (expected integer): %v\n", value, err)
		}
	case "retry":
		if v, err := strconv.Atoi(value); err == nil {
			req.Metadata.Retry = v
		} else if value != "" {
			fmt.Fprintf(os.Stderr, "warning: invalid retry value %q (expected integer): %v\n", value, err)
		}
	case "retrydelay":
		if v, err := strconv.Atoi(value); err == nil {
			req.Metadata.RetryDelay = v
		} else if value != "" {
			fmt.Fprintf(os.Stderr, "warning: invalid retryDelay value %q (expected integer): %v\n", value, err)
		}
	case "retryon":
		parts := strings.Split(value, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if code, err := strconv.Atoi(part); err == nil {
				req.Metadata.RetryOn = append(req.Metadata.RetryOn, code)
			} else {
				fmt.Fprintf(os.Stderr, "warning: invalid retryOn status code %q: %v\n", part, err)
			}
		}
	case "depends":
		deps := strings.Split(value, ",")
		for _, d := range deps {
			d = strings.TrimSpace(d)
			if d != "" {
				req.Metadata.Depends = append(req.Metadata.Depends, d)
			}
		}
	case "if":
		req.Metadata.Condition = &Condition{
			Type:       ConditionIf,
			Expression: value,
		}
	case "unless":
		req.Metadata.Condition = &Condition{
			Type:       ConditionUnless,
			Expression: value,
		}
	case "auth":
		auth, err := parseAuthConfig(value)
		if err != nil {
			return &ParseError{
				File:    p.file,
				Line:    p.curToken.Line,
				Column:  p.curToken.Column,
				Message: err.Error(),
				Snippet: p.getSnippet(),
			}
		}
		req.Metadata.Auth = auth
	case "before":
		hook := &Hook{Type: HookExec, Command: value}
		req.Metadata.PreHooks = append(req.Metadata.PreHooks, hook)
	case "after":
		hook := &Hook{Type: HookExec, Command: value, Always: true}
		req.Metadata.PostHooks = append(req.Metadata.PostHooks, hook)
	case "db":
		req.Metadata.DBConnection = value
	case "waitfor":
		parts := strings.Fields(value)
		if len(parts) >= 1 {
			cfg := &WaitForConfig{
				URL:      parts[0],
				Status:   200,   // default
				Timeout:  30000, // default 30s
				Interval: 1000,  // default 1s
			}
			if len(parts) >= 2 {
				if status, err := strconv.Atoi(parts[1]); err == nil {
					cfg.Status = status
				}
			}
			if len(parts) >= 3 {
				if timeout, err := strconv.Atoi(parts[2]); err == nil {
					cfg.Timeout = timeout
				}
			}
			if len(parts) >= 4 {
				if interval, err := strconv.Atoi(parts[3]); err == nil {
					cfg.Interval = interval
				}
			}
			req.Metadata.WaitFor = cfg
		}
	case "stress.weight":
		if req.Metadata.Stress == nil {
			req.Metadata.Stress = &StressMetadata{}
		}
		if v, err := strconv.Atoi(value); err == nil {
			req.Metadata.Stress.Weight = v
		}
	case "stress.think":
		if req.Metadata.Stress == nil {
			req.Metadata.Stress = &StressMetadata{}
		}
		if v, err := strconv.Atoi(value); err == nil {
			req.Metadata.Stress.Think = v
		}
	case "stress.skip":
		if req.Metadata.Stress == nil {
			req.Metadata.Stress = &StressMetadata{}
		}
		req.Metadata.Stress.Skip = true
	case "stress.setup":
		if req.Metadata.Stress == nil {
			req.Metadata.Stress = &StressMetadata{}
		}
		req.Metadata.Stress.Setup = true
	case "stress.teardown":
		if req.Metadata.Stress == nil {
			req.Metadata.Stress = &StressMetadata{}
		}
		req.Metadata.Stress.Teardown = true
	default:
		// Store unrecognized annotations as custom annotations
		// This supports @contract.state, @contract.provider, @x-custom, etc.
		if req.Metadata.Custom == nil {
			req.Metadata.Custom = make(map[string]string)
		}
		req.Metadata.Custom[name] = value
	}

	return nil
}

// authSpec describes the argument shape of one auth type.
type authSpec struct {
	typ   AuthType
	args  []string // names of the required arguments, for error messages
	scope bool     // extra arguments after the required ones are scopes
}

var authSpecs = map[string]authSpec{
	"basic":        {typ: AuthBasic, args: []string{"username", "password"}},
	"bearer":       {typ: AuthBearer, args: []string{"token"}},
	"apikey":       {typ: AuthAPIKey, args: []string{"headerName", "value"}},
	"apikey-query": {typ: AuthAPIKeyQuery, args: []string{"paramName", "value"}},
	"apikeyquery":  {typ: AuthAPIKeyQuery, args: []string{"paramName", "value"}},
	"digest":       {typ: AuthDigest, args: []string{"username", "password"}},
	"aws":          {typ: AuthAWS, args: []string{"accessKey", "secretKey", "region", "service"}},
}

var oauth2Specs = map[string]authSpec{
	"client_credentials": {typ: AuthOAuth2ClientCredentials, args: []string{"tokenUrl", "clientId", "clientSecret"}, scope: true},
	"password":           {typ: AuthOAuth2Password, args: []string{"tokenUrl", "clientId", "clientSecret", "username", "password"}, scope: true},
}

// parseAuthConfig parses an @auth annotation value into an AuthConfig.
//
// Arguments are comma-separated ("basic alice, s3cret") when the value has a
// comma outside {{ }}, otherwise whitespace-separated ("basic alice s3cret").
// The argument count is validated per type and unknown types are rejected, so a
// typo fails at parse time instead of silently sending no credentials.
func parseAuthConfig(value string) (*AuthConfig, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("@auth needs a type: basic, bearer, apiKey, apiKeyQuery, digest, aws, oauth2 client_credentials or oauth2 password")
	}

	kind, rest := value, ""
	if i := strings.IndexAny(value, " \t"); i >= 0 {
		kind, rest = value[:i], strings.TrimSpace(value[i+1:])
	}
	label := kind

	spec, ok := authSpecs[strings.ToLower(kind)]
	if strings.EqualFold(kind, "oauth2") {
		grant, grantRest := rest, ""
		if i := strings.IndexAny(rest, " \t"); i >= 0 {
			grant, grantRest = rest[:i], strings.TrimSpace(rest[i+1:])
		}
		spec, ok = oauth2Specs[strings.ToLower(grant)]
		if !ok {
			return nil, fmt.Errorf("@auth oauth2 needs a grant type: client_credentials or password (got %q)", grant)
		}
		label = "oauth2 " + strings.ToLower(grant)
		rest = grantRest
	}
	if !ok {
		return nil, fmt.Errorf("unknown @auth type %q (expected basic, bearer, apiKey, apiKeyQuery, digest, aws, oauth2 client_credentials or oauth2 password)", kind)
	}

	args := splitAuthArgs(rest)
	n := len(spec.args)
	if len(args) < n || (!spec.scope && len(args) > n) {
		want := fmt.Sprintf("%d", n)
		if spec.scope {
			want += " or more"
		}
		return nil, fmt.Errorf("@auth %s expects %s arguments (%s), got %d", label, want, strings.Join(spec.args, ", "), len(args))
	}
	if spec.scope && len(args) > n {
		// Everything after the required arguments is the scope list, which the
		// runner reads as one comma-separated parameter.
		args = append(args[:n:n], strings.Join(args[n:], ","))
	}
	return &AuthConfig{Type: spec.typ, Params: args}, nil
}

// splitAuthArgs splits @auth arguments on commas outside {{ }} when there is
// one, else on whitespace outside {{ }}. Pieces are trimmed.
func splitAuthArgs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	isComma := func(c byte) bool { return c == ',' }
	parts := splitOutsideBraces(s, isComma, false)
	if len(parts) < 2 {
		parts = fieldsOutsideBraces(s)
	}
	out := make([]string, len(parts))
	for i, f := range parts {
		out[i] = strings.TrimSpace(f.text)
	}
	return out
}
