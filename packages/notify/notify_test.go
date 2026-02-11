package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---- helpers ----

// captureSlackPayload starts an httptest server that records the JSON body
// sent to it and returns (server, pointer-to-captured-message).
func captureSlackPayload(t *testing.T) (*httptest.Server, *slackMessage) {
	t.Helper()
	var captured slackMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("unmarshal slack payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

// captureTeamsPayload starts an httptest server that records the JSON body
// sent to it and returns (server, pointer-to-captured-message).
func captureTeamsPayload(t *testing.T) (*httptest.Server, *teamsMessage) {
	t.Helper()
	var captured teamsMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("unmarshal teams payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

func fieldValue(fields []slackField, title string) string {
	for _, f := range fields {
		if f.Title == title {
			return f.Value
		}
	}
	return ""
}

// ---- Slack tests ----

func TestSlackNotifier_AllPassed(t *testing.T) {
	srv, captured := captureSlackPayload(t)
	notifier := NewSlackNotifier(srv.URL, WithSlackChannel("#ci"), WithSlackUsername("bot"))

	summary := &RunSummary{
		TotalFiles:  2,
		TotalTests:  10,
		PassedTests: 10,
		FailedTests: 0,
		Duration:    1500 * time.Millisecond,
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	if len(captured.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(captured.Attachments))
	}
	att := captured.Attachments[0]

	// Color should be green ("good")
	if att.Color != "good" {
		t.Errorf("expected color 'good', got %q", att.Color)
	}
	// Title should indicate all passed
	if !strings.Contains(att.Title, "All tests passed") {
		t.Errorf("expected title to contain 'All tests passed', got %q", att.Title)
	}
	if !strings.Contains(att.Title, ":white_check_mark:") {
		t.Errorf("expected title to contain check mark emoji, got %q", att.Title)
	}

	// Verify channel/username options
	if captured.Channel != "#ci" {
		t.Errorf("expected channel '#ci', got %q", captured.Channel)
	}
	if captured.Username != "bot" {
		t.Errorf("expected username 'bot', got %q", captured.Username)
	}

	// Verify fields
	if v := fieldValue(att.Fields, "Total Tests"); v != "10" {
		t.Errorf("expected Total Tests '10', got %q", v)
	}
	if v := fieldValue(att.Fields, "Passed"); v != "10" {
		t.Errorf("expected Passed '10', got %q", v)
	}
	if v := fieldValue(att.Fields, "Failed"); v != "0" {
		t.Errorf("expected Failed '0', got %q", v)
	}
	if v := fieldValue(att.Fields, "Duration"); v != "1.5s" {
		t.Errorf("expected Duration '1.5s', got %q", v)
	}
}

func TestSlackNotifier_SomeFailed(t *testing.T) {
	srv, captured := captureSlackPayload(t)
	notifier := NewSlackNotifier(srv.URL)

	summary := &RunSummary{
		TotalFiles:  3,
		TotalTests:  20,
		PassedTests: 17,
		FailedTests: 3,
		Duration:    2 * time.Second,
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	att := captured.Attachments[0]

	if att.Color != "danger" {
		t.Errorf("expected color 'danger', got %q", att.Color)
	}
	if !strings.Contains(att.Title, "3 test(s) failed") {
		t.Errorf("expected title to mention failures, got %q", att.Title)
	}
	if !strings.Contains(att.Title, ":x:") {
		t.Errorf("expected title to contain :x: emoji, got %q", att.Title)
	}
}

func TestSlackNotifier_Recovery(t *testing.T) {
	srv, captured := captureSlackPayload(t)
	notifier := NewSlackNotifier(srv.URL)

	summary := &RunSummary{
		TotalTests:  10,
		PassedTests: 10,
		FailedTests: 0,
		Duration:    500 * time.Millisecond,
		IsRecovery:  true,
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	att := captured.Attachments[0]

	// Recovery should still be green
	if att.Color != "good" {
		t.Errorf("expected color 'good' for recovery, got %q", att.Color)
	}
	if !strings.Contains(att.Title, "Tests recovered!") {
		t.Errorf("expected title 'Tests recovered!', got %q", att.Title)
	}
	if !strings.Contains(att.Title, ":tada:") {
		t.Errorf("expected title to contain :tada: emoji, got %q", att.Title)
	}
}

func TestSlackNotifier_FailedTestDetails(t *testing.T) {
	srv, captured := captureSlackPayload(t)
	notifier := NewSlackNotifier(srv.URL)

	summary := &RunSummary{
		TotalTests:  5,
		PassedTests: 3,
		FailedTests: 2,
		Duration:    1 * time.Second,
		FailedResults: []FailedTest{
			{
				Name:   "GET /users",
				File:   "users.http",
				Errors: []string{"status 500 != 200", "body missing 'id'"},
			},
			{
				Name:   "POST /login",
				File:   "auth.http",
				Errors: []string{"timeout after 5s"},
			},
		},
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	att := captured.Attachments[0]

	if !strings.Contains(att.Text, "Failed tests:") {
		t.Errorf("expected text to contain 'Failed tests:', got %q", att.Text)
	}
	if !strings.Contains(att.Text, "`GET /users`") {
		t.Errorf("expected text to contain test name 'GET /users', got %q", att.Text)
	}
	if !strings.Contains(att.Text, "(users.http)") {
		t.Errorf("expected text to contain file 'users.http', got %q", att.Text)
	}
	if !strings.Contains(att.Text, "status 500 != 200") {
		t.Errorf("expected text to contain error detail, got %q", att.Text)
	}
	if !strings.Contains(att.Text, "`POST /login`") {
		t.Errorf("expected text to contain test name 'POST /login', got %q", att.Text)
	}
	if !strings.Contains(att.Text, "timeout after 5s") {
		t.Errorf("expected text to contain error detail, got %q", att.Text)
	}
}

func TestSlackNotifier_EnvironmentField(t *testing.T) {
	srv, captured := captureSlackPayload(t)
	notifier := NewSlackNotifier(srv.URL)

	summary := &RunSummary{
		TotalTests:  5,
		PassedTests: 5,
		FailedTests: 0,
		Duration:    1 * time.Second,
		Environment: "staging",
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	att := captured.Attachments[0]

	if v := fieldValue(att.Fields, "Environment"); v != "staging" {
		t.Errorf("expected Environment 'staging', got %q", v)
	}
}

func TestSlackNotifier_NoEnvironmentFieldWhenEmpty(t *testing.T) {
	srv, captured := captureSlackPayload(t)
	notifier := NewSlackNotifier(srv.URL)

	summary := &RunSummary{
		TotalTests:  5,
		PassedTests: 5,
		FailedTests: 0,
		Duration:    1 * time.Second,
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	att := captured.Attachments[0]

	if v := fieldValue(att.Fields, "Environment"); v != "" {
		t.Errorf("expected no Environment field, but got %q", v)
	}
}

func TestSlackNotifier_DefaultOptions(t *testing.T) {
	srv, captured := captureSlackPayload(t)
	notifier := NewSlackNotifier(srv.URL)

	summary := &RunSummary{
		TotalTests:  1,
		PassedTests: 1,
		Duration:    100 * time.Millisecond,
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	if captured.Username != "hitspec" {
		t.Errorf("expected default username 'hitspec', got %q", captured.Username)
	}
	if captured.IconEmoji != ":test_tube:" {
		t.Errorf("expected default icon_emoji ':test_tube:', got %q", captured.IconEmoji)
	}
	// Channel should be empty (omitted) when not set
	if captured.Channel != "" {
		t.Errorf("expected empty channel by default, got %q", captured.Channel)
	}
}

func TestSlackNotifier_IconEmojiOption(t *testing.T) {
	srv, captured := captureSlackPayload(t)
	notifier := NewSlackNotifier(srv.URL, WithSlackIconEmoji(":rocket:"))

	summary := &RunSummary{TotalTests: 1, PassedTests: 1, Duration: 100 * time.Millisecond}
	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	if captured.IconEmoji != ":rocket:" {
		t.Errorf("expected icon_emoji ':rocket:', got %q", captured.IconEmoji)
	}
}

func TestSlackNotifier_Name(t *testing.T) {
	n := NewSlackNotifier("http://example.com")
	if n.Name() != "slack" {
		t.Errorf("expected Name() == 'slack', got %q", n.Name())
	}
}

// ---- Teams tests ----

func TestTeamsNotifier_AllPassed(t *testing.T) {
	srv, captured := captureTeamsPayload(t)
	notifier := NewTeamsNotifier(srv.URL)

	summary := &RunSummary{
		TotalTests:  8,
		PassedTests: 8,
		FailedTests: 0,
		Duration:    750 * time.Millisecond,
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	if captured.Type != "message" {
		t.Errorf("expected message type 'message', got %q", captured.Type)
	}
	if len(captured.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(captured.Attachments))
	}
	card := captured.Attachments[0]
	if card.ContentType != "application/vnd.microsoft.card.adaptive" {
		t.Errorf("unexpected content type: %q", card.ContentType)
	}
	if card.Content.Version != "1.2" {
		t.Errorf("expected Adaptive Card version '1.2', got %q", card.Content.Version)
	}

	// First block should be the title
	if len(card.Content.Body) < 1 {
		t.Fatal("expected at least 1 body block")
	}
	titleBlock := card.Content.Body[0]
	if !strings.Contains(titleBlock.Text, "All tests passed") {
		t.Errorf("expected title 'All tests passed', got %q", titleBlock.Text)
	}
	if titleBlock.Color != "good" {
		t.Errorf("expected color 'good', got %q", titleBlock.Color)
	}
}

func TestTeamsNotifier_SomeFailed(t *testing.T) {
	srv, captured := captureTeamsPayload(t)
	notifier := NewTeamsNotifier(srv.URL)

	summary := &RunSummary{
		TotalTests:  15,
		PassedTests: 12,
		FailedTests: 3,
		Duration:    3 * time.Second,
		FailedResults: []FailedTest{
			{Name: "GET /health", File: "health.http", Errors: []string{"expected 200, got 503"}},
		},
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	card := captured.Attachments[0]
	titleBlock := card.Content.Body[0]

	if titleBlock.Color != "attention" {
		t.Errorf("expected color 'attention' for failures, got %q", titleBlock.Color)
	}
	if !strings.Contains(titleBlock.Text, "3 test(s) failed") {
		t.Errorf("expected title to mention failures, got %q", titleBlock.Text)
	}

	// Find block mentioning the failed test
	found := false
	for _, block := range card.Content.Body {
		if strings.Contains(block.Text, "`GET /health`") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected body to contain failed test name 'GET /health'")
	}

	// Find block mentioning the error
	foundErr := false
	for _, block := range card.Content.Body {
		if strings.Contains(block.Text, "expected 200, got 503") {
			foundErr = true
			break
		}
	}
	if !foundErr {
		t.Error("expected body to contain error detail 'expected 200, got 503'")
	}
}

func TestTeamsNotifier_Recovery(t *testing.T) {
	srv, captured := captureTeamsPayload(t)
	notifier := NewTeamsNotifier(srv.URL)

	summary := &RunSummary{
		TotalTests:  5,
		PassedTests: 5,
		FailedTests: 0,
		Duration:    1 * time.Second,
		IsRecovery:  true,
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	card := captured.Attachments[0]
	titleBlock := card.Content.Body[0]

	if !strings.Contains(titleBlock.Text, "Tests recovered!") {
		t.Errorf("expected title 'Tests recovered!', got %q", titleBlock.Text)
	}
	if titleBlock.Color != "good" {
		t.Errorf("expected color 'good' for recovery, got %q", titleBlock.Color)
	}
}

func TestTeamsNotifier_EnvironmentField(t *testing.T) {
	srv, captured := captureTeamsPayload(t)
	notifier := NewTeamsNotifier(srv.URL)

	summary := &RunSummary{
		TotalTests:  5,
		PassedTests: 5,
		Duration:    1 * time.Second,
		Environment: "production",
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}

	card := captured.Attachments[0]
	found := false
	for _, block := range card.Content.Body {
		if strings.Contains(block.Text, "production") && strings.Contains(block.Text, "Environment") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected body to contain Environment block with 'production'")
	}
}

func TestTeamsNotifier_AcceptedStatus(t *testing.T) {
	// Teams webhook can return 202 Accepted
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	notifier := NewTeamsNotifier(srv.URL)
	summary := &RunSummary{TotalTests: 1, PassedTests: 1, Duration: 100 * time.Millisecond}

	if err := notifier.Notify(summary); err != nil {
		t.Fatalf("expected no error for 202 status, got: %v", err)
	}
}

func TestTeamsNotifier_Name(t *testing.T) {
	n := NewTeamsNotifier("http://example.com")
	if n.Name() != "teams" {
		t.Errorf("expected Name() == 'teams', got %q", n.Name())
	}
}

// ---- Manager tests ----

// mockNotifier records whether Notify was called and the summaries it received.
type mockNotifier struct {
	name      string
	calls     []*RunSummary
	returnErr error
}

func (m *mockNotifier) Notify(summary *RunSummary) error {
	m.calls = append(m.calls, summary)
	return m.returnErr
}

func (m *mockNotifier) Name() string {
	return m.name
}

func TestManager_NotifyAlways(t *testing.T) {
	mock := &mockNotifier{name: "mock"}
	mgr := NewManager(NotifyAlways, mock)

	// All pass
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5}); err != nil {
		t.Fatal(err)
	}
	// Some fail
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 3, FailedTests: 2}); err != nil {
		t.Fatal(err)
	}
	// All pass again
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5}); err != nil {
		t.Fatal(err)
	}

	if len(mock.calls) != 3 {
		t.Errorf("expected 3 calls with NotifyAlways, got %d", len(mock.calls))
	}
}

func TestManager_NotifyFailure(t *testing.T) {
	mock := &mockNotifier{name: "mock"}
	mgr := NewManager(NotifyFailure, mock)

	// Pass - should NOT notify
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5}); err != nil {
		t.Fatal(err)
	}
	if len(mock.calls) != 0 {
		t.Fatalf("expected 0 calls after pass, got %d", len(mock.calls))
	}

	// Fail - should notify
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 3, FailedTests: 2}); err != nil {
		t.Fatal(err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call after failure, got %d", len(mock.calls))
	}

	// Fail again - should notify again
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 4, FailedTests: 1}); err != nil {
		t.Fatal(err)
	}
	if len(mock.calls) != 2 {
		t.Errorf("expected 2 calls after second failure, got %d", len(mock.calls))
	}
}

func TestManager_NotifySuccess(t *testing.T) {
	mock := &mockNotifier{name: "mock"}
	mgr := NewManager(NotifySuccess, mock)

	// Fail - should NOT notify
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 3, FailedTests: 2}); err != nil {
		t.Fatal(err)
	}
	if len(mock.calls) != 0 {
		t.Fatalf("expected 0 calls after failure, got %d", len(mock.calls))
	}

	// Pass - should notify
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5}); err != nil {
		t.Fatal(err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call after pass, got %d", len(mock.calls))
	}

	// Pass again - should notify again
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5}); err != nil {
		t.Fatal(err)
	}
	if len(mock.calls) != 2 {
		t.Errorf("expected 2 calls after second pass, got %d", len(mock.calls))
	}
}

func TestManager_NotifyRecovery(t *testing.T) {
	mock := &mockNotifier{name: "mock"}
	mgr := NewManager(NotifyRecovery, mock)

	// Pass first (initial state is success, so no recovery) - should NOT notify
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5}); err != nil {
		t.Fatal(err)
	}
	if len(mock.calls) != 0 {
		t.Fatalf("expected 0 calls after initial pass (no recovery), got %d", len(mock.calls))
	}

	// Fail - should notify (recovery mode also notifies on failure)
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 3, FailedTests: 2}); err != nil {
		t.Fatal(err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call after failure, got %d", len(mock.calls))
	}
	if mock.calls[0].IsRecovery {
		t.Error("failure notification should NOT have IsRecovery set")
	}

	// Pass after failure = recovery - should notify with IsRecovery=true
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5}); err != nil {
		t.Fatal(err)
	}
	if len(mock.calls) != 2 {
		t.Fatalf("expected 2 calls after recovery, got %d", len(mock.calls))
	}
	if !mock.calls[1].IsRecovery {
		t.Error("recovery notification should have IsRecovery set to true")
	}

	// Pass again (no longer recovery) - should NOT notify
	if err := mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5}); err != nil {
		t.Fatal(err)
	}
	if len(mock.calls) != 2 {
		t.Errorf("expected still 2 calls (pass after pass is not recovery), got %d", len(mock.calls))
	}
}

func TestManager_RecoveryThenFailureCycle(t *testing.T) {
	mock := &mockNotifier{name: "mock"}
	mgr := NewManager(NotifyRecovery, mock)

	// Fail
	_ = mgr.Notify(&RunSummary{TotalTests: 5, FailedTests: 1, PassedTests: 4})
	// Recover
	_ = mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5})
	// Fail again
	_ = mgr.Notify(&RunSummary{TotalTests: 5, FailedTests: 2, PassedTests: 3})
	// Recover again
	_ = mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5})

	// Expected: fail(1) + recover(2) + fail(3) + recover(4) = 4 calls
	if len(mock.calls) != 4 {
		t.Errorf("expected 4 calls in fail/recover cycle, got %d", len(mock.calls))
	}
	// Verify the recovery flags
	if mock.calls[0].IsRecovery {
		t.Error("first fail should not be recovery")
	}
	if !mock.calls[1].IsRecovery {
		t.Error("first recover should be recovery")
	}
	if mock.calls[2].IsRecovery {
		t.Error("second fail should not be recovery")
	}
	if !mock.calls[3].IsRecovery {
		t.Error("second recover should be recovery")
	}
}

func TestManager_AddNotifier(t *testing.T) {
	mock1 := &mockNotifier{name: "mock1"}
	mock2 := &mockNotifier{name: "mock2"}
	mgr := NewManager(NotifyAlways, mock1)
	mgr.AddNotifier(mock2)

	if err := mgr.Notify(&RunSummary{TotalTests: 1, PassedTests: 1}); err != nil {
		t.Fatal(err)
	}

	if len(mock1.calls) != 1 {
		t.Errorf("expected mock1 to be called once, got %d", len(mock1.calls))
	}
	if len(mock2.calls) != 1 {
		t.Errorf("expected mock2 to be called once, got %d", len(mock2.calls))
	}
}

func TestManager_MultipleNotifiers_OneErrors(t *testing.T) {
	mock1 := &mockNotifier{name: "ok"}
	mock2 := &mockNotifier{name: "err", returnErr: http.ErrServerClosed}
	mgr := NewManager(NotifyAlways, mock1, mock2)

	err := mgr.Notify(&RunSummary{TotalTests: 1, PassedTests: 1})
	if err == nil {
		t.Fatal("expected error from manager when a notifier fails")
	}

	// Both should still be called (manager iterates all)
	if len(mock1.calls) != 1 {
		t.Errorf("expected mock1 to be called, got %d calls", len(mock1.calls))
	}
	if len(mock2.calls) != 1 {
		t.Errorf("expected mock2 to be called, got %d calls", len(mock2.calls))
	}
}

func TestManager_NoNotifiers(t *testing.T) {
	mgr := NewManager(NotifyAlways)
	// Should not panic with zero notifiers
	if err := mgr.Notify(&RunSummary{TotalTests: 1, PassedTests: 1}); err != nil {
		t.Fatalf("expected no error with zero notifiers, got: %v", err)
	}
}

// ---- Error cases ----

func TestSlackNotifier_WebhookError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	t.Cleanup(srv.Close)

	notifier := NewSlackNotifier(srv.URL)
	summary := &RunSummary{TotalTests: 1, PassedTests: 1, Duration: 100 * time.Millisecond}

	err := notifier.Notify(summary)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status code 500, got: %v", err)
	}
	if !strings.Contains(err.Error(), "internal error") {
		t.Errorf("expected error to contain response body, got: %v", err)
	}
}

func TestSlackNotifier_NetworkError(t *testing.T) {
	// Use a server that's immediately closed to force a network error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // Close it immediately

	notifier := NewSlackNotifier(srv.URL)
	summary := &RunSummary{TotalTests: 1, PassedTests: 1, Duration: 100 * time.Millisecond}

	err := notifier.Notify(summary)
	if err == nil {
		t.Fatal("expected network error for closed server")
	}
	if !strings.Contains(err.Error(), "failed to send Slack notification") {
		t.Errorf("expected 'failed to send Slack notification' in error, got: %v", err)
	}
}

func TestTeamsNotifier_WebhookError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}))
	t.Cleanup(srv.Close)

	notifier := NewTeamsNotifier(srv.URL)
	summary := &RunSummary{TotalTests: 1, PassedTests: 1, Duration: 100 * time.Millisecond}

	err := notifier.Notify(summary)
	if err == nil {
		t.Fatal("expected error for 502 response")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("expected error to mention status code 502, got: %v", err)
	}
	if !strings.Contains(err.Error(), "bad gateway") {
		t.Errorf("expected error to contain response body, got: %v", err)
	}
}

func TestTeamsNotifier_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	notifier := NewTeamsNotifier(srv.URL)
	summary := &RunSummary{TotalTests: 1, PassedTests: 1, Duration: 100 * time.Millisecond}

	err := notifier.Notify(summary)
	if err == nil {
		t.Fatal("expected network error for closed server")
	}
	if !strings.Contains(err.Error(), "failed to send Teams notification") {
		t.Errorf("expected 'failed to send Teams notification' in error, got: %v", err)
	}
}

// ---- Payload structure verification ----

func TestSlackNotifier_PayloadStructure(t *testing.T) {
	var rawPayload json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %q", ct)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST method, got %q", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		rawPayload = body
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	notifier := NewSlackNotifier(srv.URL, WithSlackChannel("#test"))
	summary := &RunSummary{
		TotalTests:  10,
		PassedTests: 10,
		Duration:    1 * time.Second,
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatal(err)
	}

	// Verify the raw JSON structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawPayload, &parsed); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}

	if _, ok := parsed["attachments"]; !ok {
		t.Error("payload missing 'attachments' key")
	}
	if _, ok := parsed["channel"]; !ok {
		t.Error("payload missing 'channel' key")
	}
	if _, ok := parsed["username"]; !ok {
		t.Error("payload missing 'username' key")
	}
}

func TestTeamsNotifier_PayloadStructure(t *testing.T) {
	var rawPayload json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %q", ct)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST method, got %q", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		rawPayload = body
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	notifier := NewTeamsNotifier(srv.URL)
	summary := &RunSummary{
		TotalTests:  10,
		PassedTests: 10,
		Duration:    1 * time.Second,
	}

	if err := notifier.Notify(summary); err != nil {
		t.Fatal(err)
	}

	// Verify the raw JSON structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawPayload, &parsed); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}

	if parsed["type"] != "message" {
		t.Errorf("expected type 'message', got %v", parsed["type"])
	}
	attachments, ok := parsed["attachments"].([]interface{})
	if !ok || len(attachments) == 0 {
		t.Fatal("expected non-empty 'attachments' array")
	}
	card, ok := attachments[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected first attachment to be an object")
	}
	if card["contentType"] != "application/vnd.microsoft.card.adaptive" {
		t.Errorf("expected contentType to be adaptive card, got %v", card["contentType"])
	}
}

// ---- Integration: Manager with real Slack/Teams notifiers ----

func TestManager_NotifyAlways_WithSlackAndTeams(t *testing.T) {
	slackSrv, slackMsg := captureSlackPayload(t)
	teamsSrv, teamsMsg := captureTeamsPayload(t)

	slack := NewSlackNotifier(slackSrv.URL)
	teams := NewTeamsNotifier(teamsSrv.URL)
	mgr := NewManager(NotifyAlways, slack, teams)

	summary := &RunSummary{
		TotalTests:  10,
		PassedTests: 8,
		FailedTests: 2,
		Duration:    3 * time.Second,
		Environment: "ci",
		FailedResults: []FailedTest{
			{Name: "GET /api/v1/items", File: "items.http", Errors: []string{"expected 200"}},
		},
	}

	if err := mgr.Notify(summary); err != nil {
		t.Fatal(err)
	}

	// Verify Slack received the message
	if len(slackMsg.Attachments) != 1 {
		t.Fatal("Slack did not receive expected attachment")
	}
	if slackMsg.Attachments[0].Color != "danger" {
		t.Errorf("Slack: expected danger color, got %q", slackMsg.Attachments[0].Color)
	}

	// Verify Teams received the message
	if len(teamsMsg.Attachments) != 1 {
		t.Fatal("Teams did not receive expected attachment")
	}
	titleBlock := teamsMsg.Attachments[0].Content.Body[0]
	if titleBlock.Color != "attention" {
		t.Errorf("Teams: expected attention color, got %q", titleBlock.Color)
	}
}

func TestManager_NotifyRecovery_FullCycleWithRealNotifiers(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	slack := NewSlackNotifier(srv.URL)
	mgr := NewManager(NotifyRecovery, slack)

	// Initial pass: no notification (not a recovery)
	_ = mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5})
	if callCount != 0 {
		t.Fatalf("expected 0 calls after initial pass, got %d", callCount)
	}

	// Failure: should notify
	_ = mgr.Notify(&RunSummary{TotalTests: 5, FailedTests: 1, PassedTests: 4})
	if callCount != 1 {
		t.Fatalf("expected 1 call after failure, got %d", callCount)
	}

	// Recovery: should notify
	_ = mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5})
	if callCount != 2 {
		t.Fatalf("expected 2 calls after recovery, got %d", callCount)
	}

	// Continued pass: no notification
	_ = mgr.Notify(&RunSummary{TotalTests: 5, PassedTests: 5})
	if callCount != 2 {
		t.Errorf("expected still 2 calls after continued pass, got %d", callCount)
	}
}
