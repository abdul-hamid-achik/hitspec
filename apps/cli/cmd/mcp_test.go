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
