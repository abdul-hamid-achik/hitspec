package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TeamsNotifier sends notifications to Microsoft Teams via webhook
type TeamsNotifier struct {
	webhookURL string
	client     *http.Client
}

// TeamsOption is a functional option for TeamsNotifier
type TeamsOption func(*TeamsNotifier)

// NewTeamsNotifier creates a new Teams notifier
func NewTeamsNotifier(webhookURL string, opts ...TeamsOption) *TeamsNotifier {
	t := &TeamsNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Name returns the name of the notifier
func (t *TeamsNotifier) Name() string {
	return "teams"
}

// teamsMessage represents a Microsoft Teams Adaptive Card message
type teamsMessage struct {
	Type        string       `json:"type"`
	Attachments []teamsCard  `json:"attachments"`
}

// teamsCard represents an Adaptive Card
type teamsCard struct {
	ContentType string           `json:"contentType"`
	ContentURL  *string          `json:"contentUrl"`
	Content     teamsCardContent `json:"content"`
}

// teamsCardContent is the content of an Adaptive Card
type teamsCardContent struct {
	Schema  string        `json:"$schema"`
	Type    string        `json:"type"`
	Version string        `json:"version"`
	Body    []teamsBlock  `json:"body"`
}

// teamsBlock represents a block in the Adaptive Card
type teamsBlock struct {
	Type      string       `json:"type"`
	Size      string       `json:"size,omitempty"`
	Weight    string       `json:"weight,omitempty"`
	Text      string       `json:"text,omitempty"`
	Color     string       `json:"color,omitempty"`
	Wrap      bool         `json:"wrap,omitempty"`
	Columns   []teamsColumn `json:"columns,omitempty"`
	Items     []teamsBlock  `json:"items,omitempty"`
	Spacing   string        `json:"spacing,omitempty"`
	Separator bool          `json:"separator,omitempty"`
}

// teamsColumn represents a column in a ColumnSet
type teamsColumn struct {
	Type  string       `json:"type"`
	Width string       `json:"width"`
	Items []teamsBlock `json:"items"`
}

// Notify sends a notification to Microsoft Teams
func (t *TeamsNotifier) Notify(summary *RunSummary) error {
	color := "good"
	title := "All tests passed!"
	emoji := "✓"

	if summary.FailedTests > 0 {
		color = "attention"
		title = fmt.Sprintf("%d test(s) failed", summary.FailedTests)
		emoji = "✗"
	} else if summary.IsRecovery {
		color = "good"
		title = "Tests recovered!"
		emoji = "🎉"
	}

	body := []teamsBlock{
		{
			Type:   "TextBlock",
			Size:   "Large",
			Weight: "Bolder",
			Text:   fmt.Sprintf("%s %s", emoji, title),
			Color:  color,
		},
		{
			Type:      "ColumnSet",
			Separator: true,
			Spacing:   "Medium",
			Columns: []teamsColumn{
				{
					Type:  "Column",
					Width: "stretch",
					Items: []teamsBlock{
						{Type: "TextBlock", Text: "**Total Tests**", Wrap: true},
						{Type: "TextBlock", Text: fmt.Sprintf("%d", summary.TotalTests), Wrap: true},
					},
				},
				{
					Type:  "Column",
					Width: "stretch",
					Items: []teamsBlock{
						{Type: "TextBlock", Text: "**Passed**", Wrap: true},
						{Type: "TextBlock", Text: fmt.Sprintf("%d", summary.PassedTests), Color: "good", Wrap: true},
					},
				},
				{
					Type:  "Column",
					Width: "stretch",
					Items: []teamsBlock{
						{Type: "TextBlock", Text: "**Failed**", Wrap: true},
						{Type: "TextBlock", Text: fmt.Sprintf("%d", summary.FailedTests), Color: "attention", Wrap: true},
					},
				},
				{
					Type:  "Column",
					Width: "stretch",
					Items: []teamsBlock{
						{Type: "TextBlock", Text: "**Duration**", Wrap: true},
						{Type: "TextBlock", Text: summary.Duration.Round(time.Millisecond).String(), Wrap: true},
					},
				},
			},
		},
	}

	// Add environment if present
	if summary.Environment != "" {
		body = append(body, teamsBlock{
			Type: "TextBlock",
			Text: fmt.Sprintf("**Environment:** %s", summary.Environment),
			Wrap: true,
		})
	}

	// Add failed test details if any
	if len(summary.FailedResults) > 0 {
		body = append(body, teamsBlock{
			Type:      "TextBlock",
			Text:      "**Failed Tests:**",
			Separator: true,
			Spacing:   "Medium",
		})

		for _, ft := range summary.FailedResults {
			text := fmt.Sprintf("- `%s`", ft.Name)
			if ft.File != "" {
				text += fmt.Sprintf(" (%s)", ft.File)
			}
			body = append(body, teamsBlock{
				Type: "TextBlock",
				Text: text,
				Wrap: true,
			})
			for _, err := range ft.Errors {
				body = append(body, teamsBlock{
					Type: "TextBlock",
					Text: fmt.Sprintf("  - %s", err),
					Wrap: true,
				})
			}
		}
	}

	// Add footer
	body = append(body, teamsBlock{
		Type:      "TextBlock",
		Text:      fmt.Sprintf("_hitspec - %s_", time.Now().Format(time.RFC3339)),
		Separator: true,
		Spacing:   "Medium",
	})

	msg := teamsMessage{
		Type: "message",
		Attachments: []teamsCard{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				ContentURL:  nil,
				Content: teamsCardContent{
					Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
					Type:    "AdaptiveCard",
					Version: "1.2",
					Body:    body,
				},
			},
		},
	}

	return t.send(msg)
}

func (t *TeamsNotifier) send(msg teamsMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal Teams message: %w", err)
	}

	req, err := http.NewRequest("POST", t.webhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Teams notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Teams API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
