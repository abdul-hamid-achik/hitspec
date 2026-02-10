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
			return err
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

// parseAuthConfig parses an @auth annotation value into an AuthConfig.
func parseAuthConfig(value string) (*AuthConfig, error) {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return nil, nil
	}

	auth := &AuthConfig{}
	switch strings.ToLower(parts[0]) {
	case "basic":
		auth.Type = AuthBasic
		auth.Params = parts[1:]
	case "bearer":
		auth.Type = AuthBearer
		auth.Params = parts[1:]
	case "apikey":
		auth.Type = AuthAPIKey
		auth.Params = parts[1:]
	case "apikey-query", "apikeyquery":
		auth.Type = AuthAPIKeyQuery
		auth.Params = parts[1:]
	case "digest":
		auth.Type = AuthDigest
		auth.Params = parts[1:]
	case "aws":
		auth.Type = AuthAWS
		auth.Params = parts[1:]
	case "oauth2":
		if len(parts) >= 2 {
			switch strings.ToLower(parts[1]) {
			case "client_credentials":
				auth.Type = AuthOAuth2ClientCredentials
				auth.Params = parts[2:] // tokenUrl, clientId, clientSecret, [scopes]
			case "password":
				auth.Type = AuthOAuth2Password
				auth.Params = parts[2:] // tokenUrl, clientId, clientSecret, username, password, [scopes]
			}
		}
	}

	return auth, nil
}
