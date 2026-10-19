package runner

import (
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateCondition(t *testing.T) {
	r := &Runner{resolver: env.NewResolver()}
	r.resolver.SetWarnFunc(func(string, ...any) {})
	r.resolver.SetVariables(map[string]any{
		"env":     "prod",
		"quoted":  `"prod"`,
		"empty":   "",
		"flag":    "true",
		"off":     "false",
		"count":   "5",
		"eqtext":  "a == b",
		"version": "2.0",
	})

	tests := []struct {
		expr string
		want bool
	}{
		// bare truthiness (documented)
		{`{{flag}}`, true},
		{`{{off}}`, false},
		{`{{empty}}`, false},
		{`{{missing}}`, false},
		{`{{env}}`, true},
		// comparisons: the resolved value used to be tested for non-emptiness,
		// so any string made == "x" true
		{`{{env}} == "prod"`, true},
		{`{{env}} == "staging"`, false},
		{`{{env}} == prod`, true},
		{`{{env}} != "prod"`, false},
		{`{{env}} != "staging"`, true},
		{`'prod' == {{quoted}}`, true},
		{`{{count}} == 5`, true},
		{`{{count}} == 5.0`, true},
		{`{{version}} == 2`, true},
		{`{{count}} != 6`, true},
		{`{{missing}} == "prod"`, false},
		{`{{missing}} != "prod"`, true},
		{`{{missing}} == ""`, true},
		// a resolved value containing == does not confuse the split
		{`{{eqtext}} == "a == b"`, true},
		{`{{eqtext}}`, true},
		// combinations
		{`{{env}} == "prod" && {{flag}}`, true},
		{`{{env}} == "prod" && {{off}}`, false},
		{`{{env}} == "dev" || {{flag}}`, true},
		{`{{env}} == "dev" || {{off}}`, false},
		{`!{{off}}`, true},
		{`!{{flag}}`, false},
		{`! {{env}} == "prod"`, false},
		{`{{env}} != "prod" || {{env}} == "prod" && {{flag}}`, true},
		{`{{env}} == "a||b"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := r.evaluateCondition(&parser.Condition{Type: parser.ConditionIf, Expression: tt.expr})
			assert.Equal(t, tt.want, got)
			unless := r.evaluateCondition(&parser.Condition{Type: parser.ConditionUnless, Expression: tt.expr})
			assert.Equal(t, !tt.want, unless)
		})
	}
	assert.True(t, r.evaluateCondition(nil))
}
