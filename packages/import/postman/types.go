package postman

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/result"
)

// collection is a Postman Collection v2.x document. The same struct also
// reads a Postman environment export (name + values).
type collection struct {
	Info     info       `json:"info"`
	Item     []item     `json:"item"`
	Variable []variable `json:"variable"`
	Auth     *auth      `json:"auth"`
	Event    []event    `json:"event"`

	// Environment export fields.
	Name   string     `json:"name"`
	Values []envValue `json:"values"`
}

type info struct {
	Name string `json:"name"`
}

type variable struct {
	Key      string `json:"key"`
	Value    any    `json:"value"`
	Disabled bool   `json:"disabled"`
}

type envValue struct {
	Key     string `json:"key"`
	Value   any    `json:"value"`
	Enabled *bool  `json:"enabled"`
}

type item struct {
	Name    string          `json:"name"`
	Item    []item          `json:"item"`
	Request json.RawMessage `json:"request"`
	Event   []event         `json:"event"`
	Auth    *auth           `json:"auth"`
}

type request struct {
	Method string
	Header []header
	Body   *postmanBody
	URL    postmanURL
	Auth   *auth
}

type header struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
}

type kv struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
}

type formData struct {
	Key      string          `json:"key"`
	Value    string          `json:"value"`
	Type     string          `json:"type"`
	Src      json.RawMessage `json:"src"`
	Disabled bool            `json:"disabled"`
}

// srcPath returns the first file path of a form file field (src may be a
// string or a list of strings).
func (f formData) srcPath() string {
	var s string
	if json.Unmarshal(f.Src, &s) == nil {
		return s
	}
	var list []string
	if json.Unmarshal(f.Src, &list) == nil && len(list) > 0 {
		return list[0]
	}
	return ""
}

type graphqlBody struct {
	Query     string          `json:"query"`
	Variables json.RawMessage `json:"variables"`
}

// variables returns the GraphQL variables as JSON text (Postman stores them as
// a string or an object).
func (g graphqlBody) variables() string {
	var s string
	if json.Unmarshal(g.Variables, &s) == nil {
		return s
	}
	if len(g.Variables) > 0 && string(g.Variables) != "null" {
		return string(g.Variables)
	}
	return ""
}

type postmanBody struct {
	Mode       string       `json:"mode"`
	Raw        string       `json:"raw"`
	URLEncoded []kv         `json:"urlencoded"`
	FormData   []formData   `json:"formdata"`
	GraphQL    *graphqlBody `json:"graphql"`
	File       *struct {
		Src string `json:"src"`
	} `json:"file"`
	Disabled bool `json:"disabled"`
	Options  struct {
		Raw struct {
			Language string `json:"language"`
		} `json:"raw"`
	} `json:"options"`
}

// postmanURL is a request URL: a plain string or an object.
type postmanURL struct {
	Raw      string
	Protocol string
	Host     []string
	Path     []string
	Query    []kv
	Variable []variable
}

func (u *postmanURL) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		u.Raw = s
		return nil
	}
	var o struct {
		Raw      string          `json:"raw"`
		Protocol string          `json:"protocol"`
		Host     json.RawMessage `json:"host"`
		Path     json.RawMessage `json:"path"`
		Query    []kv            `json:"query"`
		Variable []variable      `json:"variable"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	u.Raw, u.Protocol, u.Query, u.Variable = o.Raw, o.Protocol, o.Query, o.Variable
	u.Host = stringList(o.Host)
	u.Path = stringList(o.Path)
	return nil
}

// stringList reads a JSON string or list of strings (path segments may also
// be objects with a value field).
func stringList(b json.RawMessage) []string {
	var s string
	if json.Unmarshal(b, &s) == nil && s != "" {
		return []string{s}
	}
	var list []json.RawMessage
	if json.Unmarshal(b, &list) != nil {
		return nil
	}
	var out []string
	for _, e := range list {
		var str string
		if json.Unmarshal(e, &str) == nil {
			out = append(out, str)
			continue
		}
		var o struct {
			Value string `json:"value"`
		}
		if json.Unmarshal(e, &o) == nil {
			out = append(out, o.Value)
		}
	}
	return out
}

// build assembles a URL from its parts when raw is empty.
func (u postmanURL) build() string {
	var sb strings.Builder
	if u.Protocol != "" {
		sb.WriteString(u.Protocol + "://")
	}
	sb.WriteString(strings.Join(u.Host, "."))
	if len(u.Path) > 0 {
		sb.WriteString("/" + strings.Join(u.Path, "/"))
	}
	sep := "?"
	for _, q := range u.Query {
		if q.Disabled || q.Key == "" {
			continue
		}
		sb.WriteString(sep + q.Key + "=" + q.Value)
		sep = "&"
	}
	return sb.String()
}

// parseRequest decodes an item's request, which is either an object or just a
// URL string.
func parseRequest(raw json.RawMessage) (*request, error) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return &request{Method: "GET", URL: postmanURL{Raw: s}}, nil
	}
	var o struct {
		Method string          `json:"method"`
		Header json.RawMessage `json:"header"`
		Body   *postmanBody    `json:"body"`
		URL    postmanURL      `json:"url"`
		Auth   *auth           `json:"auth"`
	}
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	r := &request{Method: o.Method, Body: o.Body, URL: o.URL, Auth: o.Auth}
	// header is normally a list but may be a raw header string.
	var hs []header
	if json.Unmarshal(o.Header, &hs) == nil {
		r.Header = hs
	} else {
		var text string
		if json.Unmarshal(o.Header, &text) == nil {
			for _, line := range strings.Split(text, "\n") {
				if k, v, ok := strings.Cut(line, ":"); ok {
					r.Header = append(r.Header, header{Key: strings.TrimSpace(k), Value: strings.TrimSpace(v)})
				}
			}
		}
	}
	return r, nil
}

// auth is a Postman auth block: {"type": "bearer", "bearer": [{key, value}]}
// (v2.1) or {"type": "bearer", "bearer": {"token": "..."}} (v2.0).
type auth struct {
	Type string
	raw  map[string]json.RawMessage
}

func (a *auth) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &a.raw); err != nil {
		return err
	}
	if t, ok := a.raw["type"]; ok {
		_ = json.Unmarshal(t, &a.Type)
	}
	return nil
}

// params flattens the parameters of the active auth type.
func (a *auth) params() map[string]string {
	out := map[string]string{}
	raw, ok := a.raw[a.Type]
	if !ok {
		return out
	}
	var list []struct {
		Key   string `json:"key"`
		Value any    `json:"value"`
	}
	if json.Unmarshal(raw, &list) == nil {
		for _, e := range list {
			out[e.Key] = result.Scalar(e.Value)
		}
		return out
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		for k, v := range obj {
			out[k] = result.Scalar(v)
		}
	}
	return out
}

type event struct {
	Listen string `json:"listen"`
	Script script `json:"script"`
}

// script holds a script's source: exec is a list of lines or one string.
type script struct {
	Exec json.RawMessage `json:"exec"`
}

func (s script) lines() []string {
	var list []string
	if json.Unmarshal(s.Exec, &list) == nil {
		return list
	}
	var one string
	if json.Unmarshal(s.Exec, &one) == nil {
		return strings.Split(one, "\n")
	}
	return nil
}
