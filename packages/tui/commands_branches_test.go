package tui

import (
	"context"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// TestSelectEnvCmdExecutes runs the env-switch closure against a real manager:
// it activates the environment and returns the refreshed config/env/workspace.
func TestSelectEnvCmdExecutes(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if err := setEnvVar(ctx, mgr, "dev", "k", "v"); err != nil {
		t.Fatalf("seed env: %v", err)
	}
	msg := selectEnvCmd(ctx, mgr, "dev")()
	es, ok := msg.(envSelectedMsg)
	if !ok {
		t.Fatalf("selectEnvCmd -> %T, want envSelectedMsg", msg)
	}
	if es.err != nil {
		t.Fatalf("selectEnvCmd errored: %v", es.err)
	}
	if es.name != "dev" {
		t.Fatalf("selectEnvCmd name = %q, want dev", es.name)
	}
}

// TestRunDoneAndCopyMsgArms covers the ad-hoc focus switch in runDoneMsg and the
// content surfacing in copyMsg.
func TestRunDoneAndCopyMsgArms(t *testing.T) {
	// ad-hoc run surfaces the response pane (screen → workspace, focus → response)
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.screen = screenStress
	next, _ := m.Update(runDoneMsg{result: sampleResult(), adhoc: true})
	nm := next.(model)
	if nm.screen != screenWorkspace || nm.focus != focusResponse {
		t.Fatalf("ad-hoc run should focus the response pane, got screen=%v focus=%v", nm.screen, nm.focus)
	}

	// a copy with content surfaces it in the response viewer on the workspace
	m2 := newModel(context.Background(), newTestManager(t), Options{})
	m2.screen = screenWorkspace
	next2, _ := m2.Update(copyMsg{title: "copied", content: "curl https://x/y"})
	if got := plain(next2.(model).respView.view()); got == "" {
		t.Fatal("copyMsg content should be surfaced in the response viewer")
	}

	// a copy error sets m.err
	m3 := newModel(context.Background(), newTestManager(t), Options{})
	next3, _ := m3.Update(copyMsg{err: errBoom})
	if next3.(model).err == "" {
		t.Fatal("copyMsg error should set m.err")
	}
}

// TestContractMsgUpdatesPreview covers the contractMsg success arm.
func TestContractMsgUpdatesPreview(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenContract)
	next, _ := m.Update(contractMsg{results: []clientmgr.ContractResultDTO{
		{File: "api.http", Passed: 1, Failed: 0},
	}})
	nm := next.(model)
	if nm.contracts == nil || len(nm.contracts) != 1 {
		t.Fatalf("contractMsg should store results, got %+v", nm.contracts)
	}
}
