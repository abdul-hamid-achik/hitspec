// Package mcpclient provides a small, transport-independent client for probing
// MCP servers and invoking their tools.
package mcpclient

import (
	"encoding/json"
	"io"
)

// Target identifies exactly one MCP server transport.
type Target struct {
	URL     string
	Command []string
	Dir     string
	Env     []string
	Headers []string
	Stderr  io.Writer
}

// ServerInfo is the negotiated server identity.
type ServerInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

// Tool describes one tool advertised by the MCP server.
type Tool struct {
	Name         string `json:"name"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	InputSchema  any    `json:"inputSchema"`
	OutputSchema any    `json:"outputSchema,omitempty"`
}

// ProbeReport is the stable result of an MCP handshake and full tools/list
// traversal.
type ProbeReport struct {
	OK              bool       `json:"ok"`
	Transport       string     `json:"transport"`
	ProtocolVersion string     `json:"protocolVersion"`
	Server          ServerInfo `json:"server"`
	Capabilities    []string   `json:"capabilities"`
	Tools           []Tool     `json:"tools"`
	MissingTools    []string   `json:"missingTools,omitempty"`
	Warnings        []string   `json:"warnings,omitempty"`
}

// CallReport is the stable result of one explicit tools/call request.
type CallReport struct {
	OK                bool              `json:"ok"`
	Transport         string            `json:"transport"`
	ProtocolVersion   string            `json:"protocolVersion"`
	Server            ServerInfo        `json:"server"`
	Tool              string            `json:"tool"`
	Content           []json.RawMessage `json:"content"`
	StructuredContent any               `json:"structuredContent,omitempty"`
	IsError           bool              `json:"isError"`
}
