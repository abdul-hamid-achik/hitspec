package parser

import (
	"regexp"
	"strings"
)

var captureLineRe = regexp.MustCompile(`^([\p{L}_][\p{L}\p{N}_.\-]*)\s+(?i:from)\s+(.+)$`)

// parseCaptures parses a >>>capture ... <<< block into a slice of Capture.
// Lines are read raw: a path may contain quotes, '#' and spaces
// (items.#(name=="Jane Doe").id).
func (p *Parser) parseCaptures() ([]*Capture, error) {
	var captures []*Capture
	err := p.readBlock("capture", func(lineNo int, raw string) error {
		s := &exprScanner{p: p, line: lineNo, text: raw}
		s.skipSpace()
		text := strings.TrimSpace(raw)
		m := captureLineRe.FindStringSubmatch(stripTrailingComment(text))
		if m == nil {
			return s.fail(s.pos, "invalid capture %q: expected \"<name> from <source>\" (e.g. token from body.access_token)", text)
		}
		c, perr := parseCaptureSource(m[1], strings.TrimSpace(m[2]), lineNo)
		if perr != nil {
			return s.fail(s.pos, "%v", perr)
		}
		if c.Source == CaptureBody && c.Path != "" {
			probe := &exprScanner{text: c.Path}
			if probe.scanSubject() != c.Path {
				col := strings.Index(raw, m[2]) + 1
				p.warnAt(lineNo, col, "capture %q: the path %q contains spaces, so it will never match (put comments after #)", c.Name, c.Path)
			}
		}
		captures = append(captures, c)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return captures, nil
}

// parseCaptureSource turns the source text of "name from <source>" into a Capture.
func parseCaptureSource(name, path string, line int) (*Capture, error) {
	c := &Capture{Name: name, Source: CaptureBody, Line: line}
	lower := strings.ToLower(path)
	switch {
	case lower == "header" || strings.HasPrefix(lower, "header "):
		c.Source = CaptureHeader
		c.Path = strings.TrimSpace(path[len("header"):])
	case lower == "status":
		c.Source = CaptureStatus
	case lower == "duration":
		c.Source = CaptureDuration
	case lower == "body":
		c.Path = ""
	case strings.HasPrefix(lower, "body."):
		c.Path = path[len("body."):]
	case strings.HasPrefix(lower, "body["):
		c.Path = path[len("body"):]
	case strings.HasPrefix(lower, "jsonpath "):
		gp, err := JSONPathToGJSON(path[len("jsonpath "):])
		if err != nil {
			return nil, err
		}
		c.Path = gp
	default:
		c.Path = path
	}
	return c, nil
}
