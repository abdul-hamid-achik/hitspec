package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

var adhocMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true,
}

// parseAdHocLine splits a quick-request line into method + URL. A leading known
// method is honored ("POST https://…"); otherwise the whole line is the URL and
// the method defaults to GET.
func parseAdHocLine(line string) (method, url string) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return "", ""
	}
	if len(fields) >= 2 && adhocMethods[strings.ToUpper(fields[0])] {
		return strings.ToUpper(fields[0]), fields[1]
	}
	return "GET", fields[0]
}

// adhocCmd executes a one-off request typed into the quick-request prompt.
func adhocCmd(ctx context.Context, mgr *clientmgr.Manager, line string) tea.Cmd {
	return func() tea.Msg {
		method, url := parseAdHocLine(line)
		if url == "" {
			return runDoneMsg{err: fmt.Errorf("enter a URL, optionally prefixed with a method")}
		}
		result, err := mgr.ExecuteAdHoc(ctx, clientmgr.AdHocReq{Method: method, URL: url})
		return runDoneMsg{result: result, err: err, adhoc: true}
	}
}
