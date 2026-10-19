package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/clientmgr"
)

func TestStressFormStartsWithProfileField(t *testing.T) {
	m := stressModel(t)
	require.Len(t, m.formInputs, 5)
	var labels []string
	for _, in := range m.formInputs {
		labels = append(labels, strings.TrimSuffix(in.Prompt, ": "))
	}
	assert.Equal(t, []string{"duration", "rate", "vus", "max vus", "profile"}, labels)
	assert.Equal(t, "none", m.formInputs[4].Placeholder)
	assert.Contains(t, m.formInputs[0].Placeholder, "profile's")
}

func TestStressStartReqWithoutProfileKeepsDefaults(t *testing.T) {
	req, err := stressStartReq(map[string]string{}, []string{"a.http"})
	require.NoError(t, err)
	assert.Equal(t, clientmgr.StressStartReq{Files: []string{"a.http"}, Duration: "30s", Rate: 5}, req)
}

func TestStressStartReqWithProfileSendsOnlyTypedValues(t *testing.T) {
	req, err := stressStartReq(map[string]string{"profile": " smoke "}, []string{"a.http"})
	require.NoError(t, err)
	assert.Equal(t, "smoke", req.Profile)
	assert.Empty(t, req.Duration, "a blank duration must not override the profile's")
	assert.Zero(t, req.Rate)
	assert.Zero(t, req.VUs)
	assert.Zero(t, req.MaxVUs)
}

func TestStressStartReqTypedValuesWinOverProfile(t *testing.T) {
	req, err := stressStartReq(map[string]string{
		"profile": "smoke", "duration": " 2s ", "rate": "7", "vus": "3", "max vus": "9",
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, clientmgr.StressStartReq{Profile: "smoke", Duration: "2s", Rate: 7, VUs: 3, MaxVUs: 9}, req)
}

func TestStressStartReqRejectsBadNumbers(t *testing.T) {
	_, err := stressStartReq(map[string]string{"profile": "p", "rate": "fast"}, nil)
	require.Error(t, err)
}

// profileModel is a stress screen over a workspace whose hitspec.yaml has the
// given stress profiles.
func profileModel(t *testing.T, names ...string) model {
	t.Helper()
	mgr, dir := newTestManagerDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "p.http"), []byte("### P\n# @name p\nGET http://localhost:1/\n"), 0o600))
	for _, n := range names {
		require.NoError(t, mgr.PutStressProfile(context.Background(), clientmgr.StressProfileReq{Name: n, Duration: "2s", Rate: 2}))
	}
	m := newModel(context.Background(), mgr, Options{})
	m.color = false
	m.width, m.height = 100, 30
	m.resize()
	m.setScreen(screenStress)
	return m
}

func TestStressIdleListsSavedProfiles(t *testing.T) {
	m := profileModel(t, "soak", "smoke")
	assert.Equal(t, []string{"smoke", "soak"}, m.stressProfiles)
	out := plain(m.stressContent())
	assert.Contains(t, out, "No stress test running")
	assert.Contains(t, out, "Profiles: smoke, soak")
	assert.Contains(t, plain(m.secondaryView()), "Profiles: smoke, soak")
}

func TestStressIdleWithoutProfilesShowsNoProfilesLine(t *testing.T) {
	m := profileModel(t)
	assert.Empty(t, m.stressProfiles)
	assert.NotContains(t, plain(m.stressContent()), "Profiles:")
}

// Submitting a profile that does not exist reaches the user as an error toast
// that names it.
func TestStressUnknownProfileShowsErrorToast(t *testing.T) {
	m := profileModel(t, "smoke")
	m.selected = "p.http"
	m.formInputs[4].SetValue("nope")
	cmd := m.submitFormCmd()
	require.NotNil(t, cmd)
	msg, ok := cmd().(simpleMsg)
	require.True(t, ok)
	require.Error(t, msg.err)
	next, _ := m.Update(msg)
	nm := next.(model)
	assert.Contains(t, nm.err, `"nope"`)
	assert.Contains(t, toastTexts(nm), `"nope"`)
}

// A known profile with blank fields starts the run with the profile's values.
func TestStressKnownProfileStartsWithBlankFields(t *testing.T) {
	m := profileModel(t, "smoke")
	m.selected = "p.http"
	m.formInputs[4].SetValue("smoke")
	cmd := m.submitFormCmd()
	require.NotNil(t, cmd)
	msg := cmd().(simpleMsg)
	require.NoError(t, msg.err)
	t.Cleanup(func() { _ = m.mgr.StopStress(context.Background()) })
	st := m.mgr.StressStatus(context.Background())
	assert.True(t, st.Running)
	assert.InDelta(t, 2, st.Duration, 0.01, "the profile's 2s applies, not the 30s default")
}
