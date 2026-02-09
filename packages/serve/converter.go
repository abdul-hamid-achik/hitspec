package serve

import (
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
)

func convertFile(f *parser.File) *ParsedFileDTO {
	dto := &ParsedFileDTO{
		Path:      f.Path,
		Variables: make([]VariableDTO, 0, len(f.Variables)),
		Requests:  make([]RequestDTO, 0, len(f.Requests)),
	}
	for _, v := range f.Variables {
		dto.Variables = append(dto.Variables, VariableDTO{Name: v.Name, Value: v.Value, Line: v.Line})
	}
	for _, r := range f.Requests {
		dto.Requests = append(dto.Requests, convertRequest(r))
	}
	return dto
}

func convertRequest(r *parser.Request) RequestDTO {
	dto := RequestDTO{
		Name:        r.Name,
		Description: r.Description,
		Tags:        r.Tags,
		Method:      r.Method,
		URL:         r.URL,
		Line:        r.Line,
	}
	for _, h := range r.Headers {
		dto.Headers = append(dto.Headers, HeaderDTO{Key: h.Key, Value: h.Value, Line: h.Line})
	}
	for _, q := range r.QueryParams {
		dto.QueryParams = append(dto.QueryParams, QueryDTO{Key: q.Key, Value: q.Value, Line: q.Line})
	}
	if r.Body != nil {
		dto.Body = &BodyDTO{
			ContentType: bodyTypeString(r.Body.ContentType),
			Raw:         r.Body.Raw,
			Line:        r.Body.Line,
		}
	}
	for _, a := range r.Assertions {
		dto.Assertions = append(dto.Assertions, AssertionDTO{
			Subject:  a.Subject,
			Operator: a.Operator.String(),
			Expected: a.Expected,
			Line:     a.Line,
		})
	}
	for _, c := range r.Captures {
		dto.Captures = append(dto.Captures, CaptureDTO{
			Name:   c.Name,
			Source: c.Source.String(),
			Path:   c.Path,
			Line:   c.Line,
		})
	}
	if r.Metadata != nil {
		dto.Metadata = convertMetadata(r.Metadata)
	}
	return dto
}

func convertMetadata(m *parser.RequestMetadata) *MetadataDTO {
	dto := &MetadataDTO{
		Skip:    m.Skip,
		Only:    m.Only,
		Timeout: m.Timeout,
		Retry:   m.Retry,
		Depends: m.Depends,
	}
	if m.Auth != nil {
		dto.Auth = &AuthDTO{
			Type:   authTypeString(m.Auth.Type),
			Params: m.Auth.Params,
		}
	}
	return dto
}

func convertRunResult(r *runner.RunResult) *RunResultDTO {
	dto := &RunResultDTO{
		File:     r.File,
		Duration: float64(r.Duration.Milliseconds()),
		Passed:   r.Passed,
		Failed:   r.Failed,
		Skipped:  r.Skipped,
		Results:  make([]RequestResultDTO, 0, len(r.Results)),
	}
	for _, rr := range r.Results {
		dto.Results = append(dto.Results, convertRequestResult(rr))
	}
	return dto
}

func convertRequestResult(rr *runner.RequestResult) RequestResultDTO {
	dto := RequestResultDTO{
		Name:       rr.Name,
		Passed:     rr.Passed,
		Skipped:    rr.Skipped,
		SkipReason: rr.SkipReason,
		Duration:   float64(rr.Duration.Milliseconds()),
		Captures:   rr.Captures,
	}
	if rr.Error != nil {
		dto.Error = rr.Error.Error()
	}
	if rr.Request != nil {
		dto.Request = &HTTPRequestDTO{
			Method:  rr.Request.Method,
			URL:     rr.Request.URL,
			Headers: rr.Request.Headers,
		}
	}
	if rr.Response != nil {
		dto.Response = &HTTPResponseDTO{
			StatusCode: rr.Response.StatusCode,
			Status:     rr.Response.Status,
			Headers:    rr.Response.Headers,
			Body:       string(rr.Response.Body),
			Duration:   float64(rr.Response.Duration.Milliseconds()),
			Size:       int64(len(rr.Response.Body)),
		}
	}
	for _, a := range rr.Assertions {
		dto.Assertions = append(dto.Assertions, AssertionResultDTO{
			Subject:  a.Subject,
			Operator: a.Operator,
			Expected: a.Expected,
			Actual:   a.Actual,
			Passed:   a.Passed,
			Message:  a.Message,
		})
	}
	return dto
}

func bodyTypeString(bt parser.BodyType) string {
	switch bt {
	case parser.BodyJSON:
		return "json"
	case parser.BodyForm:
		return "form"
	case parser.BodyFormBlock:
		return "formBlock"
	case parser.BodyMultipart:
		return "multipart"
	case parser.BodyRaw:
		return "raw"
	case parser.BodyXML:
		return "xml"
	case parser.BodyGraphQL:
		return "graphql"
	default:
		return "none"
	}
}

func authTypeString(at parser.AuthType) string {
	switch at {
	case parser.AuthBasic:
		return "basic"
	case parser.AuthBearer:
		return "bearer"
	case parser.AuthAPIKey:
		return "apiKey"
	case parser.AuthAPIKeyQuery:
		return "apiKeyQuery"
	case parser.AuthDigest:
		return "digest"
	case parser.AuthAWS:
		return "aws"
	case parser.AuthOAuth2ClientCredentials:
		return "oauth2_client_credentials"
	case parser.AuthOAuth2Password:
		return "oauth2_password"
	default:
		return "none"
	}
}
