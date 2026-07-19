package cmd

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type rootCommandFlagDefault struct {
	flag      *pflag.Flag
	value     string
	changed   bool
	slice     []string
	sliceFlag bool
}

var rootCommandFlagDefaults []rootCommandFlagDefault

func captureRootCommandFlagDefaults(command *cobra.Command) []rootCommandFlagDefault {
	seen := make(map[*pflag.Flag]bool)
	var defaults []rootCommandFlagDefault
	var capture func(*cobra.Command)
	capture = func(current *cobra.Command) {
		collect := func(flags *pflag.FlagSet) {
			flags.VisitAll(func(flag *pflag.Flag) {
				if seen[flag] {
					return
				}
				seen[flag] = true
				state := rootCommandFlagDefault{
					flag: flag, value: flag.Value.String(), changed: flag.Changed,
				}
				if value, ok := flag.Value.(pflag.SliceValue); ok {
					state.sliceFlag = true
					state.slice = append([]string(nil), value.GetSlice()...)
				}
				defaults = append(defaults, state)
			})
		}
		collect(current.Flags())
		collect(current.PersistentFlags())
		for _, child := range current.Commands() {
			capture(child)
		}
	}
	capture(command)
	return defaults
}

func restoreRootCommandDefaults() error {
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	for _, state := range rootCommandFlagDefaults {
		var err error
		if state.sliceFlag {
			value := state.flag.Value.(pflag.SliceValue)
			err = value.Replace(append([]string(nil), state.slice...))
		} else {
			err = state.flag.Value.Set(state.value)
		}
		if err != nil {
			return fmt.Errorf("restore --%s: %w", state.flag.Name, err)
		}
		state.flag.Changed = state.changed
	}
	return nil
}

func resetRootCommandForTest(t *testing.T) {
	t.Helper()
	if err := restoreRootCommandDefaults(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := restoreRootCommandDefaults(); err != nil {
			t.Errorf("restore root command: %v", err)
		}
	})
}

func TestRootCommandResetRestoresFlagDefaults(t *testing.T) {
	resetRootCommandForTest(t)
	flags := []*pflag.Flag{
		validateCmd.Flags().Lookup("json"),
		runCmd.Flags().Lookup("quiet"),
	}
	before := make(map[*pflag.Flag]rootCommandFlagDefault, len(flags))
	for _, flag := range flags {
		for _, state := range rootCommandFlagDefaults {
			if state.flag == flag {
				before[flag] = state
				break
			}
		}
		next := "true"
		if flag.Value.String() == "true" {
			next = "false"
		}
		if err := flag.Value.Set(next); err != nil {
			t.Fatal(err)
		}
		flag.Changed = true
	}
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := restoreRootCommandDefaults(); err != nil {
		t.Fatal(err)
	}
	for _, flag := range flags {
		want := before[flag]
		if flag.Value.String() != want.value || flag.Changed != want.changed {
			t.Errorf("--%s state = (%q, changed=%t), want (%q, changed=%t)",
				flag.Name, flag.Value.String(), flag.Changed, want.value, want.changed)
		}
	}
	if rootCmd.OutOrStdout() != os.Stdout || rootCmd.OutOrStderr() != os.Stderr {
		t.Fatal("root command writers were not restored")
	}
}
