package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SlackNotifier sends notifications to Slack via webhook
type SlackNotifier struct {
	webhookURL string
	channel    string
	username   string
	iconEmoji  string
	client     *http.Client
}

// SlackOption is a functional option for SlackNotifier
type SlackOption func(*SlackNotifier)

// WithSlackChannel sets the Slack channel
func WithSlackChannel(channel string) SlackOption {
	return func(s *SlackNotifier) {
		s.channel = channel
	}
}

// WithSlackUsername sets the Slack bot username
func WithSlackUsername(username string) SlackOption {
	return func(s *SlackNotifier) {
		s.username = username
	}
}

// WithSlackIconEmoji sets the Slack bot icon emoji
func WithSlackIconEmoji(emoji string) SlackOption {
	return func(s *SlackNotifier) {
		s.iconEmoji = emoji
	}
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(webhookURL string, opts ...SlackOption) *SlackNotifier {
	s := &SlackNotifier{
		webhookURL: webhookURL,
		username:   "hitspec",
		iconEmoji:  ":test_tube:",
		client:     &http.Client{Timeout: 10 * time.Second},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Name returns the name of the notifier
func (s *SlackNotifier) Name() string {
	return "slack"
}

// slackMessage represents a Slack webhook message
type slackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	Attachments []slackAttachment `json:"attachments"`
}

// slackAttachment represents a Slack message attachment
type slackAttachment struct {
	Color      string       `json:"color"`
	Title      string       `json:"title"`
	Text       string       `json:"text,omitempty"`
	Fields     []slackField `json:"fields,omitempty"`
	Footer     string       `json:"footer,omitempty"`
	FooterIcon string       `json:"footer_icon,omitempty"`
	TS         int64        `json:"ts,omitempty"`
}

// slackField represents a field in a Slack attachment
type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Notify sends a notification to Slack
func (s *SlackNotifier) Notify(summary *RunSummary) error {
	color := "good" // green
	title := "All tests passed!"
	emoji := ":white_check_mark:"

	if summary.FailedTests > 0 {
		color = "danger" // red
		title = fmt.Sprintf("%d test(s) failed", summary.FailedTests)
		emoji = ":x:"
	} else if summary.IsRecovery {
		color = "good"
		title = "Tests recovered!"
		emoji = ":tada:"
	}

	fields := []slackField{
		{Title: "Total Tests", Value: fmt.Sprintf("%d", summary.TotalTests), Short: true},
		{Title: "Passed", Value: fmt.Sprintf("%d", summary.PassedTests), Short: true},
		{Title: "Failed", Value: fmt.Sprintf("%d", summary.FailedTests), Short: true},
		{Title: "Duration", Value: summary.Duration.Round(time.Millisecond).String(), Short: true},
	}

	if summary.Environment != "" {
		fields = append(fields, slackField{
			Title: "Environment",
			Value: summary.Environment,
			Short: true,
		})
	}

	// Add failed test details if any
	var text string
	if len(summary.FailedResults) > 0 {
		text = "*Failed tests:*\n"
		for _, ft := range summary.FailedResults {
			text += fmt.Sprintf("• `%s`", ft.Name)
			if ft.File != "" {
				text += fmt.Sprintf(" (%s)", ft.File)
			}
			text += "\n"
			for _, err := range ft.Errors {
				text += fmt.Sprintf("  - %s\n", err)
			}
		}
	}

	attachment := slackAttachment{
		Color:      color,
		Title:      fmt.Sprintf("%s %s", emoji, title),
		Text:       text,
		Fields:     fields,
		Footer:     "hitspec",
		FooterIcon: "https://github.com/abdul-hamid-achik/hitspec",
		TS:         time.Now().Unix(),
	}

	msg := slackMessage{
		Channel:     s.channel,
		Username:    s.username,
		IconEmoji:   s.iconEmoji,
		Attachments: []slackAttachment{attachment},
	}

	return s.send(msg)
}

func (s *SlackNotifier) send(msg slackMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	req, err := http.NewRequest("POST", s.webhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
