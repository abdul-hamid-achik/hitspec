package artifact

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

type fakeRunner struct {
	stdout []byte
	stderr []byte
	err    error
	args   []string
	env    []string
}

func (r *fakeRunner) Run(_ context.Context, _ string, args, env []string) ([]byte, []byte, error) {
	r.args, r.env = append([]string(nil), args...), append([]string(nil), env...)
	return r.stdout, r.stderr, r.err
}

func TestFcheapSaveReturnsKnownPartialReceiptWithoutRetryError(t *testing.T) {
	runner := &fakeRunner{
		stdout: []byte(`{"id":"stash-1","content_hash":"abc","file_count":1,"total_size":12,"status":"saved_with_failures","indexed":false,"failed":[{"stage":"index","error":"busy"}]}`),
		err:    errors.New("exit status 1"),
	}
	sink := &Fcheap{path: "/fixed/fcheap", run: runner}
	receipt, err := sink.Save(context.Background(), Input{
		Filename: "response.md", Content: []byte("hello"), Name: "hitspec-webpage",
		Tags: []string{"hitspec", "web", "fetch"}, TTL: "30d", Index: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Storage != StorageSucceeded || receipt.Index != IndexFailed || receipt.StashID != "stash-1" || len(receipt.Failures) != 1 {
		t.Fatalf("receipt = %#v", receipt)
	}
	if receipt.Status != "saved_with_failures" || receipt.IndexRequested != nil || receipt.Failures[0].ID != "" {
		t.Fatalf("additive legacy fields = %#v", receipt)
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"index_requested"`) {
		t.Fatalf("legacy receipt gained a field the child did not emit: %s", encoded)
	}
	if strings.Join(runner.args, " ") == "" || !containsSequence(runner.args, "--ttl", "30d") || !containsSequence(runner.args, "--index") {
		t.Fatalf("args = %#v", runner.args)
	}
}

func TestFcheapSavePreservesCompactReceiptFields(t *testing.T) {
	runner := &fakeRunner{stdout: []byte(`{
		"id":"stash-1",
		"name":"captured webpage",
		"status":"saved_with_failures",
		"created_at":"2026-07-13T12:34:56Z",
		"expires_at":"2026-08-12T12:34:56Z",
		"tags":["hitspec","web","webpage"],
		"index_requested":false,
		"indexed":false,
		"content_hash":"sha256:abc",
		"file_count":1,
		"total_size":12,
		"failed":[{"id":"stash-1","stage":"compress","error":"busy"}]
	}`)}
	sink := &Fcheap{path: "/fixed/fcheap", run: runner}
	receipt, err := sink.Save(context.Background(), Input{
		Filename: "response.md", Content: []byte("hello"), Name: "captured webpage",
		Tags: []string{"hitspec", "web", "webpage"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Name != "captured webpage" || receipt.Status != "saved_with_failures" ||
		receipt.CreatedAt != "2026-07-13T12:34:56Z" || receipt.ExpiresAt != "2026-08-12T12:34:56Z" {
		t.Fatalf("receipt metadata = %#v", receipt)
	}
	if receipt.Storage != StorageSucceeded || receipt.Index != IndexSkipped || receipt.ContentHash != "sha256:abc" {
		t.Fatalf("internal outcome = %#v", receipt)
	}
	if receipt.IndexRequested == nil || *receipt.IndexRequested || strings.Join(receipt.Tags, ",") != "hitspec,web,webpage" {
		t.Fatalf("receipt flags/tags = %#v", receipt)
	}
	if len(receipt.Failures) != 1 || receipt.Failures[0].ID != "stash-1" || receipt.Failures[0].Stage != "compress" {
		t.Fatalf("receipt failures = %#v", receipt.Failures)
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"index_requested":false`) {
		t.Fatalf("receipt JSON lost emitted false: %s", encoded)
	}
}

func TestFcheapSaveBoundsAndSanitizesReceiptFields(t *testing.T) {
	tags := make([]string, maxReceiptTags+8)
	for index := range tags {
		tags[index] = "tag\n" + strings.Repeat("é", maxReceiptTagBytes)
	}
	failures := make([]map[string]string, maxReceiptFailures+4)
	for index := range failures {
		failures[index] = map[string]string{
			"id":    "stash\n" + strings.Repeat("é", maxReceiptIDBytes),
			"stage": "index\t" + strings.Repeat("é", maxFailureStageBytes),
			"error": "busy\r\n" + strings.Repeat("é", maxFailureErrorBytes),
		}
	}
	requested := true
	output := map[string]any{
		"id":              "stash\n" + strings.Repeat("é", maxReceiptIDBytes),
		"name":            "name\n" + strings.Repeat("é", maxReceiptNameBytes),
		"status":          "saved\t" + strings.Repeat("é", maxReceiptStatusBytes),
		"created_at":      "created\r\n" + strings.Repeat("é", maxReceiptTimestampBytes),
		"expires_at":      "expires\n" + strings.Repeat("é", maxReceiptTimestampBytes),
		"content_hash":    strings.Repeat("é", maxReceiptHashBytes),
		"tags":            tags,
		"index_requested": requested,
		"failed":          failures,
	}
	stdout, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	sink := &Fcheap{path: "/fixed/fcheap", run: &fakeRunner{stdout: stdout}}
	receipt, err := sink.Save(context.Background(), Input{
		Filename: "response.md", Content: []byte("hello"), Name: "captured webpage",
	})
	if err != nil {
		t.Fatal(err)
	}
	boundedFields := []struct {
		name  string
		value string
		limit int
	}{
		{"stash_id", receipt.StashID, maxReceiptIDBytes},
		{"name", receipt.Name, maxReceiptNameBytes},
		{"status", receipt.Status, maxReceiptStatusBytes},
		{"created_at", receipt.CreatedAt, maxReceiptTimestampBytes},
		{"expires_at", receipt.ExpiresAt, maxReceiptTimestampBytes},
		{"content_hash", receipt.ContentHash, maxReceiptHashBytes},
	}
	for _, field := range boundedFields {
		if len(field.value) > field.limit || !utf8.ValidString(field.value) || strings.ContainsAny(field.value, "\r\n\t") {
			t.Errorf("%s not bounded/sanitized: %q", field.name, field.value)
		}
	}
	if len(receipt.Tags) != maxReceiptTags {
		t.Fatalf("tags = %d, want %d", len(receipt.Tags), maxReceiptTags)
	}
	for _, tag := range receipt.Tags {
		if len(tag) > maxReceiptTagBytes || !utf8.ValidString(tag) || strings.ContainsAny(tag, "\r\n\t") {
			t.Errorf("tag not bounded/sanitized: %q", tag)
		}
	}
	if len(receipt.Failures) != maxReceiptFailures {
		t.Fatalf("failures = %d, want %d", len(receipt.Failures), maxReceiptFailures)
	}
	for _, failure := range receipt.Failures {
		if len(failure.ID) > maxReceiptIDBytes || len(failure.Stage) > maxFailureStageBytes || len(failure.Error) > maxFailureErrorBytes {
			t.Errorf("failure not bounded: %#v", failure)
		}
		if !utf8.ValidString(failure.ID+failure.Stage+failure.Error) || strings.ContainsAny(failure.ID+failure.Stage+failure.Error, "\r\n\t") {
			t.Errorf("failure not sanitized: %#v", failure)
		}
	}
}

func TestFcheapSaveReportsUnknownWithoutReceipt(t *testing.T) {
	runner := &fakeRunner{stderr: []byte("child failed\n"), err: errors.New("exit status 1")}
	sink := &Fcheap{path: "/fixed/fcheap", run: runner}
	receipt, err := sink.Save(context.Background(), Input{
		Filename: "search.json", Content: []byte(`{"results":[]}`), Name: "hitspec-web-search",
		Tags: []string{"hitspec", "web", "search"}, Index: true,
	})
	if !IsOutcomeUnknown(err) || receipt.Storage != StorageUnknown || receipt.OperationID == "" {
		t.Fatalf("receipt=%#v err=%v", receipt, err)
	}
}

func TestFcheapEnvironmentExcludesProviderSecrets(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "sentinel-secret")
	t.Setenv("FCHEAP_TEST_SETTING", "allowed")
	env := strings.Join(fcheapEnvironment(), "\n")
	if strings.Contains(env, "sentinel-secret") || strings.Contains(env, "TAVILY_API_KEY") {
		t.Fatalf("provider secret leaked to child env: %s", env)
	}
	if !strings.Contains(env, "FCHEAP_TEST_SETTING=allowed") {
		t.Fatalf("file.cheap setting was not preserved: %s", env)
	}
}

func TestValidateTTL(t *testing.T) {
	for _, value := range []string{"", "24h", "30d", "2w", "6m", "1y", "2026-12-31", " 30d ", "\t2024-02-29\n"} {
		if err := ValidateTTL(value); err != nil {
			t.Errorf("ValidateTTL(%q): %v", value, err)
		}
	}
	for _, value := range []string{
		"0d", "--help", "tomorrow", "30 days", strings.Repeat("1", 20) + "d",
		"2023-02-29", "2024-02-30", "2026-04-31", "2026-13-01", "2026-00-01",
	} {
		if err := ValidateTTL(value); err == nil {
			t.Errorf("ValidateTTL(%q) unexpectedly succeeded", value)
		}
	}
}

func TestFcheapSaveNormalizesTTLBeforeBuildingArguments(t *testing.T) {
	runner := &fakeRunner{stdout: []byte(`{"id":"stash-1"}`)}
	sink := &Fcheap{path: "/fixed/fcheap", run: runner}
	_, err := sink.Save(context.Background(), Input{
		Filename: "response.md",
		Content:  []byte("hello"),
		Name:     "captured webpage",
		TTL:      " \t30d\n ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsSequence(runner.args, "--ttl", "30d") {
		t.Fatalf("args = %#v, want normalized ttl", runner.args)
	}
	for _, arg := range runner.args {
		if strings.ContainsAny(arg, "\r\n\t") {
			t.Fatalf("argv contains unnormalized whitespace: %#v", runner.args)
		}
	}
}

func TestFcheapSaveRejectsTooManyTags(t *testing.T) {
	sink := &Fcheap{path: "/fixed/fcheap", run: &fakeRunner{}}
	receipt, err := sink.Save(context.Background(), Input{
		Filename: "response.md",
		Content:  []byte("hello"),
		Name:     "captured webpage",
		Tags:     make([]string, maxReceiptTags+1),
	})
	if err == nil || receipt.Storage != StorageFailed || !strings.Contains(err.Error(), "at most 32") {
		t.Fatalf("receipt=%#v err=%v", receipt, err)
	}
}

func containsSequence(values []string, sequence ...string) bool {
	if len(sequence) == 0 {
		return true
	}
	for index := 0; index+len(sequence) <= len(values); index++ {
		match := true
		for offset := range sequence {
			if values[index+offset] != sequence[offset] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
