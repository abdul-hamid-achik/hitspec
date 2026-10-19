package runner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/capture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_WarningsOnResult(t *testing.T) {
	old := capture.Warn
	capture.Warn = nil
	defer func() { capture.Warn = old }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()

	path := writeHTTPFile(t, `### one
GET `+srv.URL+`/a?t={{nobody.token}}
X-Token: {{nobody.token}}

{{nobody.token}} {{$HITSPEC_B1_UNSET}}

>>>
expect status 200
<<<

>>>capture
ghost from body.nope
good from body.id
<<<

### two
GET `+srv.URL+`/b

>>>
expect status 200
<<<
`)
	res, err := NewRunner(&Config{}).RunFile(path)
	require.NoError(t, err)
	require.Len(t, res.Results, 2)

	one, two := res.Results[0], res.Results[1]
	assert.True(t, one.Passed, "warnings never change pass/fail")
	assert.Equal(t, 0, res.Failed)
	assert.Contains(t, one.Warnings, "{{nobody.token}} was not resolved and was sent literally (in the URL)")
	assert.Contains(t, one.Warnings, "{{nobody.token}} was not resolved and was sent literally (in header X-Token)")
	assert.Contains(t, one.Warnings, "{{nobody.token}} was not resolved and was sent literally (in the body)")
	assert.Contains(t, one.Warnings, "{{$HITSPEC_B1_UNSET}} was not resolved and was sent literally (in the body)")
	var miss bool
	for _, w := range one.Warnings {
		if strings.Contains(w, `capture "ghost"`) {
			miss = true
		}
	}
	assert.True(t, miss, "capture miss in %v", one.Warnings)
	assert.EqualValues(t, "1", fmt.Sprint(one.Captures["good"]))
	assert.Empty(t, two.Warnings)
}

func TestRunner_WarningsOnErroredRequest(t *testing.T) {
	path := writeHTTPFile(t, "### down\nGET http://127.0.0.1:1/{{nobody.id}}\n")
	res, err := NewRunner(&Config{}).RunFile(path)
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	assert.Error(t, res.Results[0].Error)
	assert.NotEmpty(t, res.Results[0].Warnings, "unresolved URL reference reported even when the request errors")
}

func TestPlanFile(t *testing.T) {
	path := writeHTTPFile(t, `@base = http://example.test

### login
# @tags smoke, auth
POST {{base}}/login?v=1

>>>capture
token from body.token
<<<

### me
# @depends login
GET {{base}}/me?t={{login.token}}

### old
# @skip gone
GET {{base}}/old

### maybe
# @if {{login.token}} == "x"
GET {{base}}/maybe

### never
# @if {{undefinedvar}} == "x"
GET {{base}}/never

### after-never
# @depends never
GET {{base}}/after
`)
	r := NewRunner(&Config{})
	plan, err := r.PlanFile(path)
	require.NoError(t, err)

	byName := map[string]PlannedRequest{}
	var order []string
	for _, p := range plan.Requests {
		byName[p.Name] = p
		order = append(order, p.Name)
	}
	assert.Less(t, indexOfName(order, "login"), indexOfName(order, "me"), "dependencies first")
	assert.Equal(t, "http://example.test/login?v=1", byName["login"].URL)
	assert.Equal(t, []string{"smoke", "auth"}, byName["login"].Tags)
	assert.Equal(t, "POST", byName["login"].Method)
	assert.Equal(t, []string{"login"}, byName["me"].Depends)
	assert.Equal(t, "http://example.test/me?t={{login.token}}", byName["me"].URL)
	assert.Equal(t, "gone", byName["old"].SkipReason)
	assert.Empty(t, byName["maybe"].SkipReason)
	assert.Contains(t, byName["maybe"].Pending, "decided at run time")
	assert.Equal(t, "condition not met", byName["never"].SkipReason)
	assert.Equal(t, "dependency will not run", byName["after-never"].SkipReason)
	assert.Equal(t, 6, plan.Matched)

	// Filters apply as in a real run: --tags pulls in nothing but its matches.
	r = NewRunner(&Config{TagsFilter: []string{"smoke"}})
	plan, err = r.PlanFile(path)
	require.NoError(t, err)
	assert.Equal(t, 1, plan.Matched)
	for _, p := range plan.Requests {
		if p.Name == "login" {
			assert.Empty(t, p.SkipReason)
		} else {
			assert.Equal(t, "filtered out", p.SkipReason, p.Name)
		}
	}
}

func indexOfName(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}

func TestPlanFile_BrokenFile(t *testing.T) {
	path := writeHTTPFile(t, "### x\nGET http://e.test\n\n>>>\nexpect status frobnicates 200\n<<<\n")
	_, err := NewRunner(&Config{}).PlanFile(path)
	assert.Error(t, err)
}
