package cmd

import "testing"

func TestTaskContextIsFileOnlyPersistentFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("task-context")
	if flag == nil {
		t.Fatal("missing --task-context flag")
	}
	if rootCmd.PersistentFlags().Lookup("task-context-json") != nil {
		t.Fatal("inline task context flag must not be exposed")
	}
}
