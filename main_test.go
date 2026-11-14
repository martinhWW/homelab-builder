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

func TestCreate(t *testing.T) {
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Error executing command: %v", err)
	}
	want := "Creating a new homelab environment...\nProvider: local\nNodes: 1\n"
	if buf.String() != want {
		t.Errorf("Expected output: %q, got: %q", want, buf.String())
	}
}

func TestCreateWithFlags(t *testing.T) {
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create", "--provider", "aws", "-n", "3"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Error executing command: %v", err)
	}
	want := "Creating a new homelab environment...\nProvider: aws\nNodes: 3\n"
	if buf.String() != want {
		t.Errorf("Expected output: %q, got: %q", want, buf.String())
	}
}

func TestCreateInvalidProvider(t *testing.T) {
	rootCmd := newRootCommand()
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetArgs([]string{"create", "--provider", "banana"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected an error for invalid provider, got nil")
	}
}

func TestCreateNodeLimits(t *testing.T) {
	tests := []struct {
		nodes   string
		wantErr bool
	}{
		{"-5", true},
		{"0", true},
		{"1", false},
		{"15", false},
		{"16", true},
	}
	for _, tt := range tests {
		t.Run("nodes="+tt.nodes, func(t *testing.T) {
			rootCmd := newRootCommand()
			rootCmd.SetOut(&bytes.Buffer{})
			rootCmd.SetArgs([]string{"create", "--nodes=" + tt.nodes})
			err := rootCmd.Execute()
			if tt.wantErr && err == nil {
				t.Errorf("Expected an error for %s nodes, got nil", tt.nodes)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error for %s nodes, got: %v", tt.nodes, err)
			}
		})
	}
}
