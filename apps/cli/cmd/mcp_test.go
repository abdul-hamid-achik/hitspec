package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestMCPServeCommandValidatesLimitsWithoutStartingTransport(t *testing.T) {
	command := newMCPServeCommand()
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"--max-body-bytes", "0"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "max-body-bytes") {
		t.Fatalf("error = %v, want max-body-bytes validation", err)
	}
}

func TestMCPServeCommandFailsClosedForInvalidServerOwnedProviders(t *testing.T) {
	t.Setenv("HITSPEC_SEARCH_PROVIDER", "")
	t.Setenv("HITSPEC_FCHEAP_PATH", "")
	t.Setenv("TAVILY_API_KEY", "")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown search provider", args: []string{"--search-provider", "model-choice"}, want: "unsupported search provider"},
		{name: "missing tavily key", args: []string{"--search-provider", "tavily"}, want: "TAVILY_API_KEY is required"},
		{name: "missing artifact executable", args: []string{"--fcheap-path", "hitspec-definitely-missing-fcheap"}, want: "resolve file.cheap executable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := newMCPServeCommand()
			command.SetOut(&bytes.Buffer{})
			command.SetErr(&bytes.Buffer{})
			command.SetArgs(test.args)
			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
