package cmd

import "testing"

func TestRootCmdRegistersSubcommands(t *testing.T) {
	want := map[string]bool{"check": false, "plan": false, "apply": false, "rollback": false}
	for _, c := range rootCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("expected subcommand %q to be registered on rootCmd", name)
		}
	}
}
