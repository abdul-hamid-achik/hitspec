package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadYAML writes a hitspec.yaml in a fresh directory and loads it, so
// outputDir resolves against that directory like it does for a real run.
func loadYAML(t *testing.T, yaml string) (*config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hitspec.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o644))
	loaded, err := config.Load(path)
	require.NoError(t, err)
	// t.TempDir may be a symlinked path; the config records the absolute one.
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	return loaded.Config, abs
}

func TestPlanFromConfig(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		want         []reportTarget // File is relative to the config dir
		dir          string         // expected outputDir relative to the config dir
		listFailures bool
		errContains  string
	}{
		{name: "default", yaml: "", want: []reportTarget{{Format: "console"}}},
		{name: "empty list is console", yaml: "reporters: []\n", want: []reportTarget{{Format: "console"}}},
		{name: "single structured to stdout", yaml: "reporters: [junit]\n", want: []reportTarget{{Format: "junit"}}},
		{
			name: "console and junit in a dir",
			yaml: "reporters: [console, junit]\noutputDir: reports\n",
			want: []reportTarget{{Format: "console"}, {Format: "junit", File: "reports/hitspec-junit.xml"}},
			dir:  "reports",
		},
		{
			name: "all file names",
			yaml: "reporters: [json, junit, tap, html]\noutputDir: out\n",
			want: []reportTarget{
				{Format: "json", File: "out/hitspec-results.json"},
				{Format: "junit", File: "out/hitspec-junit.xml"},
				{Format: "tap", File: "out/hitspec-results.tap"},
				{Format: "html", File: "out/hitspec-report.html"},
			},
			dir:          "out",
			listFailures: true,
		},
		{
			name: "case and duplicates",
			yaml: "reporters: [JSON, json, Console, CONSOLE]\noutputDir: o\n",
			want: []reportTarget{{Format: "json", File: "o/hitspec-results.json"}, {Format: "console"}},
			dir:  "o",
		},
		{
			name:         "dir with only a file reporter lists failures",
			yaml:         "reporters: [junit]\noutputDir: reports\n",
			want:         []reportTarget{{Format: "junit", File: "reports/hitspec-junit.xml"}},
			dir:          "reports",
			listFailures: true,
		},
		{name: "console with dir writes no file", yaml: "outputDir: unused\n", want: []reportTarget{{Format: "console"}}},
		{name: "unknown reporter", yaml: "reporters: [xml]\n", errContains: `hitspec.yaml: unknown reporter "xml" (valid: console, json, junit, tap, html)`},
		{name: "two on stdout", yaml: "reporters: [console, junit]\n", errContains: "reporters console and junit would all write to stdout; set outputDir"},
		{name: "three on stdout", yaml: "reporters: [json, tap, html]\n", errContains: "reporters json, tap and html would all write to stdout"},
		{name: "duplicates are not two", yaml: "reporters: [junit, JUnit]\n", want: []reportTarget{{Format: "junit"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, dir := loadYAML(t, tt.yaml)
			plan, err := planFromConfig(cfg, "hitspec.yaml")
			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			require.NoError(t, err)
			var want []reportTarget
			for _, w := range tt.want {
				if w.File != "" {
					w.File = filepath.Join(dir, filepath.FromSlash(w.File))
				}
				want = append(want, w)
			}
			assert.Equal(t, want, plan.Targets)
			wantDir := ""
			if tt.dir != "" {
				wantDir = filepath.Join(dir, tt.dir)
			}
			assert.Equal(t, wantDir, plan.Dir)
			assert.Equal(t, tt.listFailures, plan.ListFailures)
		})
	}
}

func TestPlanFromConfigAbsoluteOutputDir(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "abs")
	cfg, _ := loadYAML(t, "reporters: [json]\noutputDir: "+`"`+filepath.ToSlash(abs)+`"`+"\n")
	plan, err := planFromConfig(cfg, "hitspec.yaml")
	require.NoError(t, err)
	assert.Equal(t, abs, plan.Dir)
	assert.Equal(t, filepath.Join(abs, "hitspec-results.json"), plan.Targets[0].File)
}

func TestReportFlagsGiven(t *testing.T) {
	noEnv := func(string) (string, bool) { return "", false }
	env := func(set map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) { v, ok := set[k]; return v, ok }
	}
	parse := func(args ...string) *pflag.FlagSet {
		fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
		t.Cleanup(func() { registerRunFlags(pflag.NewFlagSet("reset", pflag.ContinueOnError)) })
		registerRunFlags(fs)
		require.NoError(t, fs.Parse(args))
		return fs
	}

	assert.False(t, reportFlagsGiven(parse(), noEnv), "nothing given")
	assert.False(t, reportFlagsGiven(nil, noEnv), "nil flags")
	assert.True(t, reportFlagsGiven(parse("-o", "json"), noEnv))
	assert.True(t, reportFlagsGiven(parse("--output-file", "x"), noEnv))
	assert.True(t, reportFlagsGiven(nil, env(map[string]string{"HITSPEC_OUTPUT": "json"})))
	assert.True(t, reportFlagsGiven(nil, env(map[string]string{"HITSPEC_OUTPUT_FILE": "x"})))
	assert.False(t, reportFlagsGiven(nil, env(map[string]string{"HITSPEC_OUTPUT": "", "HITSPEC_OUTPUT_FILE": ""})), "empty values do not count")
}

func TestResolveReportPlanCommandLineWins(t *testing.T) {
	// Even an invalid reporters list is not read when -o was given.
	cfg, _ := loadYAML(t, "reporters: [xml]\n")
	oldOut, oldFile := outputFlag, outputFileFlag
	t.Cleanup(func() { outputFlag, outputFileFlag = oldOut, oldFile })
	outputFlag, outputFileFlag = "tap", "x.tap"

	plan, err := resolveReportPlan(nil, func(k string) (string, bool) {
		return "tap", k == "HITSPEC_OUTPUT"
	}, cfg, "hitspec.yaml")
	require.NoError(t, err)
	assert.Equal(t, []reportTarget{{Format: "tap", File: "x.tap"}}, plan.Targets)
	assert.Empty(t, plan.Dir)
	assert.False(t, plan.ListFailures)

	_, err = resolveReportPlan(nil, func(string) (string, bool) { return "", false }, cfg, "hitspec.yaml")
	assert.Error(t, err, "without an override the invalid list is read")
}

func TestReportPlanEnsureDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")
	require.NoError(t, (&reportPlan{Dir: dir}).ensureDir())
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	require.NoError(t, (&reportPlan{}).ensureDir(), "unset outputDir is a no-op")

	// A file where the directory should be fails.
	file := filepath.Join(t.TempDir(), "f")
	require.NoError(t, os.WriteFile(file, nil, 0o644))
	assert.Error(t, (&reportPlan{Dir: filepath.Join(file, "sub")}).ensureDir())
}

func TestOpenReports_QuietKeepsFiles(t *testing.T) {
	dir := t.TempDir()
	plan := &reportPlan{Targets: []reportTarget{
		{Format: "console"},
		{Format: "junit", File: filepath.Join(dir, "j.xml")},
	}}
	set, err := openReports(plan, 0, true, true)
	require.NoError(t, err)
	assert.Len(t, set.reports, 1, "the stdout report is dropped, the file is kept")
	require.NoError(t, set.Close())

	set, err = openReports(&reportPlan{Targets: []reportTarget{{Format: "json"}}}, 0, true, true)
	require.NoError(t, err)
	assert.False(t, set.Active())
	assert.NoError(t, set.Flush(0))
	assert.NoError(t, set.Close())
}

func TestReportSet_FormatBrokenPerTarget(t *testing.T) {
	dir := t.TempDir()
	console := filepath.Join(dir, "console.txt")
	junit := filepath.Join(dir, "j.xml")
	plan := &reportPlan{Targets: []reportTarget{
		{Format: "console", File: console},
		{Format: "junit", File: junit},
	}}
	set, err := openReports(plan, 0, true, false)
	require.NoError(t, err)
	set.FormatBroken("bad.http", errors.New("bad.http:1: syntax error"))
	require.NoError(t, set.Flush(0))
	require.NoError(t, set.Close())

	text, err := os.ReadFile(console)
	require.NoError(t, err)
	assert.Contains(t, string(text), "syntax error", "a console report in a file carries the error")
	xml, err := os.ReadFile(junit)
	require.NoError(t, err)
	assert.Contains(t, string(xml), "bad.http", "a structured report lists the file as a failure")
}

// A --watch pass picks up an edited `reporters` list, and keeps the previous
// settings (an error) while the new one is invalid.
func TestReloadSettingsReloadsReporters(t *testing.T) {
	t.Setenv("HITSPEC_OUTPUT", "")
	t.Setenv("HITSPEC_OUTPUT_FILE", "")
	t.Cleanup(func() { registerRunFlags(pflag.NewFlagSet("reset", pflag.ContinueOnError)) })

	path := filepath.Join(t.TempDir(), "hitspec.yaml")
	reload := func() (*reportPlan, error) {
		t.Helper()
		fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
		registerRunFlags(fs)
		require.NoError(t, fs.Parse([]string{"--config", path}))
		_, _, _, plan, err := reloadSettings(fs, nil, nil)
		return plan, err
	}

	require.NoError(t, os.WriteFile(path, []byte("reporters: [json]\n"), 0o644))
	plan, err := reload()
	require.NoError(t, err)
	assert.Equal(t, []reportTarget{{Format: "json"}}, plan.Targets)

	require.NoError(t, os.WriteFile(path, []byte("reporters: [junit]\n"), 0o644))
	plan, err = reload()
	require.NoError(t, err)
	assert.Equal(t, []reportTarget{{Format: "junit"}}, plan.Targets)

	require.NoError(t, os.WriteFile(path, []byte("reporters: [nope]\n"), 0o644))
	_, err = reload()
	assert.ErrorContains(t, err, "unknown reporter")
}
