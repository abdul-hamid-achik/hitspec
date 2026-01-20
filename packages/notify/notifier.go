// Package notify provides notification functionality for hitspec test results.
package notify

import (
	"time"
)

// NotifyOn specifies when to send notifications
type NotifyOn string

const (
	// NotifyAlways sends notifications for every run
	NotifyAlways NotifyOn = "always"
	// NotifyFailure sends notifications only when tests fail
	NotifyFailure NotifyOn = "failure"
	// NotifySuccess sends notifications only when tests pass
	NotifySuccess NotifyOn = "success"
	// NotifyRecovery sends notifications when tests recover from failure
	NotifyRecovery NotifyOn = "recovery"
)

// TestResult represents the result of a test run
type TestResult struct {
	Name        string
	Passed      bool
	Failed      bool
	Duration    time.Duration
	TotalTests  int
	PassedTests int
	FailedTests int
	SkippedTests int
	Errors      []string
}

// RunSummary represents the summary of a test run for notifications
type RunSummary struct {
	TotalFiles    int           `json:"total_files"`
	TotalTests    int           `json:"total_tests"`
	PassedTests   int           `json:"passed_tests"`
	FailedTests   int           `json:"failed_tests"`
	SkippedTests  int           `json:"skipped_tests"`
	Duration      time.Duration `json:"duration"`
	Environment   string        `json:"environment,omitempty"`
	FailedResults []FailedTest  `json:"failed_results,omitempty"`
	IsRecovery    bool          `json:"is_recovery,omitempty"`
}

// FailedTest represents a failed test for notifications
type FailedTest struct {
	Name     string   `json:"name"`
	File     string   `json:"file"`
	Errors   []string `json:"errors,omitempty"`
}

// Notifier is the interface for notification services
type Notifier interface {
	// Notify sends a notification about test results
	Notify(summary *RunSummary) error

	// Name returns the name of the notifier
	Name() string
}

// Manager manages multiple notifiers
type Manager struct {
	notifiers []Notifier
	notifyOn  NotifyOn
	lastState bool // true if last run was successful
}

// NewManager creates a new notification manager
func NewManager(notifyOn NotifyOn, notifiers ...Notifier) *Manager {
	return &Manager{
		notifiers: notifiers,
		notifyOn:  notifyOn,
		lastState: true, // Assume success initially
	}
}

// AddNotifier adds a notifier to the manager
func (m *Manager) AddNotifier(n Notifier) {
	m.notifiers = append(m.notifiers, n)
}

// Notify sends notifications based on the configured policy
func (m *Manager) Notify(summary *RunSummary) error {
	shouldNotify := false
	currentSuccess := summary.FailedTests == 0

	switch m.notifyOn {
	case NotifyAlways:
		shouldNotify = true
	case NotifyFailure:
		shouldNotify = summary.FailedTests > 0
	case NotifySuccess:
		shouldNotify = currentSuccess
	case NotifyRecovery:
		// Notify if recovering from failure
		if !m.lastState && currentSuccess {
			shouldNotify = true
			summary.IsRecovery = true
		}
		// Also notify on failure
		if summary.FailedTests > 0 {
			shouldNotify = true
		}
	}

	m.lastState = currentSuccess

	if !shouldNotify {
		return nil
	}

	var lastErr error
	for _, n := range m.notifiers {
		if err := n.Notify(summary); err != nil {
			lastErr = err
		}
	}

	return lastErr
}
