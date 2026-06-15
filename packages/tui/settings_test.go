package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

func TestSetEnvVarReadModifyWrite(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	if err := setEnvVar(ctx, mgr, "staging", "token", "abc123"); err != nil {
		t.Fatalf("setEnvVar (create): %v", err)
	}
	env, err := mgr.GetEnvironment(ctx, "staging")
	if err != nil {
		t.Fatalf("get env: %v", err)
	}
	if env.Variables["token"] != "abc123" {
		t.Fatalf("variable not set: %+v", env.Variables)
	}

	// A second variable must not clobber the first (read-modify-write).
	if err := setEnvVar(ctx, mgr, "staging", "baseUrl", "http://x"); err != nil {
		t.Fatalf("setEnvVar (update): %v", err)
	}
	env, _ = mgr.GetEnvironment(ctx, "staging")
	if env.Variables["token"] != "abc123" || env.Variables["baseUrl"] != "http://x" {
		t.Fatalf("read-modify-write lost a variable: %+v", env.Variables)
	}
}

func TestSetEnvVarRequiresName(t *testing.T) {
	if err := setEnvVar(context.Background(), newTestManager(t), "  ", "k", "v"); err == nil {
		t.Fatal("expected an error when the environment name is blank")
	}
}

// TestSettingsConfigMsgKeepsForm guards the regression where loading the config
// (configMsg) on the settings screen overwrote the preview with the form-less
// settingsContent, making the editable fields vanish after entry.
func TestSettingsConfigMsgKeepsForm(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	m := newModel(ctx, mgr, Options{})
	m.width, m.height = 100, 30
	m.resize()
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.setScreen(screenSettings)

	updated, _ := m.Update(configMsg{
		config: clientmgr.ConfigDTO{DefaultEnvironment: "dev", Timeout: 30000},
		envs:   []clientmgr.EnvironmentDTO{{Name: "dev"}},
	})
	view := plain(updated.(model).preview.View())
	for _, label := range []string{"default env:", "timeout ms:", "set var key:"} {
		if !strings.Contains(view, label) {
			t.Fatalf("settings preview dropped form field %q after configMsg:\n%s", label, view)
		}
	}
}

func TestSettingsFormSetsEnvVar(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	m := newModel(ctx, mgr, Options{})
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.setScreen(screenSettings)

	// Fields: 0 default env, 1 timeout, 2 retries, 3 concurrency, 4 set var env, 5 key, 6 value
	m.formInputs[4].SetValue("dev")
	m.formInputs[5].SetValue("apiKey")
	m.formInputs[6].SetValue("secret-1")

	msg := m.submitFormCmd()()
	if cm, ok := msg.(configMsg); !ok {
		t.Fatalf("submit returned %T, want configMsg", msg)
	} else if cm.err != nil {
		t.Fatalf("submit errored: %v", cm.err)
	}

	env, err := mgr.GetEnvironment(ctx, "dev")
	if err != nil {
		t.Fatalf("get env: %v", err)
	}
	if env.Variables["apiKey"] != "secret-1" {
		t.Fatalf("settings form did not set the env var: %+v", env.Variables)
	}
}
