package mcpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// WriteJSON writes one stable, indented JSON report.
func WriteJSON(writer io.Writer, report any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// WriteProbeHuman writes a concise, terminal-safe probe report.
func WriteProbeHuman(writer io.Writer, report *ProbeReport) error {
	if _, err := fmt.Fprintf(writer, "Connected: %s %s\nProtocol: %s\nTransport: %s\n",
		safeLine(report.Server.Name), safeLine(report.Server.Version),
		safeLine(report.ProtocolVersion), report.Transport,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Tools (%d):\n", len(report.Tools)); err != nil {
		return err
	}
	for _, tool := range report.Tools {
		description := safeLine(tool.Description)
		if description == "" {
			if _, err := fmt.Fprintf(writer, "  - %s\n", safeLine(tool.Name)); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(writer, "  - %s — %s\n", safeLine(tool.Name), description); err != nil {
			return err
		}
	}
	for _, warning := range report.Warnings {
		if _, err := fmt.Fprintf(writer, "Warning: %s\n", safeLine(warning)); err != nil {
			return err
		}
	}
	if len(report.MissingTools) > 0 {
		safeNames := make([]string, len(report.MissingTools))
		for index, name := range report.MissingTools {
			safeNames[index] = safeLine(name)
		}
		if _, err := fmt.Fprintf(writer, "Missing required tools: %s\n", strings.Join(safeNames, ", ")); err != nil {
			return err
		}
		_, err := fmt.Fprintln(writer, "Probe: FAILED")
		return err
	}
	_, err := fmt.Fprintln(writer, "Probe: OK")
	return err
}

// WriteCallHuman writes text content readably and preserves non-text content as
// indented JSON.
func WriteCallHuman(writer io.Writer, report *CallReport) error {
	status := "OK"
	if report.IsError {
		status = "ERROR"
	}
	if _, err := fmt.Fprintf(writer, "Tool: %s\nStatus: %s\n", safeLine(report.Tool), status); err != nil {
		return err
	}
	for _, raw := range report.Content {
		var text struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &text) == nil && text.Type == "text" {
			if _, err := fmt.Fprintln(writer, safeMultiline(text.Text)); err != nil {
				return err
			}
			continue
		}
		var formatted bytes.Buffer
		if err := json.Indent(&formatted, raw, "", "  "); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(writer, formatted.String()); err != nil {
			return err
		}
	}
	if report.StructuredContent != nil {
		encoded, err := json.MarshalIndent(report.StructuredContent, "", "  ")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "Structured content:\n%s\n", encoded); err != nil {
			return err
		}
	}
	return nil
}

func safeLine(value string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value))
}

func safeMultiline(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}
