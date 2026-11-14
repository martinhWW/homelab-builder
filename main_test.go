package main

import (
	"bytes"
	"testing"
)

func TestVersion(t *testing.T) {
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"version"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Error executing command: %v", err)
	}
	if buf.String() != "Homelab v0.1.0\n" {
		t.Errorf("Expected output: %q, got: %q", "Homelab v0.1.0\n", buf.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	rootCmd := newRootCommand()
	rootCmd.SetArgs([]string{"bogus"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected an error for unknown command, got nil")
	}
}
