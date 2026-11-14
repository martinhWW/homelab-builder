package main

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
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
	want := "Creating a new homelab environment: homelab\nProvider: local\nNodes: 1\n" +
		"multipass launch 24.04 --name homelab --cpus 2 --memory 4G --disk 30G\n"
	if buf.String() != want {
		t.Errorf("Expected output: %q, got: %q", want, buf.String())
	}
}

func TestCreateWithName(t *testing.T) {
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create", "--name", "mylab"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Error executing command: %v", err)
	}
	want := "multipass launch 24.04 --name mylab --cpus 2 --memory 4G --disk 30G\n"
	if !strings.Contains(buf.String(), want) {
		t.Errorf("Expected output to contain %q, got: %q", want, buf.String())
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
	want := "Creating a new homelab environment: homelab\nProvider: aws\nNodes: 3\n"
	if buf.String() != want {
		t.Errorf("Expected output: %q, got: %q", want, buf.String())
	}
}

func TestCreateInvalidInput(t *testing.T) {
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create", "--provider", "banana"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected an error for invalid provider, got nil")
	}
	if buf.Len() != 0 {
		t.Errorf("Expected no output for invalid input, got: %q", buf.String())
	}
}

func TestValidateCreateInputsNodes(t *testing.T) {
	tests := []struct {
		nodes   int
		wantErr bool
	}{
		{-5, true},
		{0, true},
		{1, false},
		{15, false},
		{16, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("nodes=%d", tt.nodes), func(t *testing.T) {
			err := validateCreateInputs("local", tt.nodes, "homelab")
			if tt.wantErr && err == nil {
				t.Errorf("Expected an error for %d nodes, got nil", tt.nodes)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error for %d nodes, got: %v", tt.nodes, err)
			}
		})
	}
}

func TestValidateCreateInputsProvider(t *testing.T) {
	tests := []struct {
		provider string
		wantErr  bool
	}{
		{"local", false},
		{"aws", false},
		{"gcp", false},
		{"", true},
		{"banana", true},
		{"AWS", true},
	}
	for _, tt := range tests {
		t.Run("provider="+tt.provider, func(t *testing.T) {
			err := validateCreateInputs(tt.provider, 1, "homelab")
			if tt.wantErr && err == nil {
				t.Errorf("Expected an error for provider %q, got nil", tt.provider)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error for provider %q, got: %v", tt.provider, err)
			}
		})
	}
}

func TestBuildMultipassLaunchArgs(t *testing.T) {
	got := buildMultipassLaunchArgs("homelab-node-1")
	want := []string{
		"launch", "24.04",
		"--name", "homelab-node-1",
		"--cpus", "2",
		"--memory", "4G",
		"--disk", "30G",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Expected args: %q, got: %q", want, got)
	}
}

func TestValidateCreateInputsName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"homelab", false},
		{"my-lab-2", false},
		{"a", false},
		{strings.Repeat("a", 63), false},
		{"", true},
		{"my lab", true},
		{"my_lab", true},
		{"lab!", true},
		{"2lab", true},
		{"-lab", true},
		{"lab-", true},
		{"MyLab", true},
		{"homelaB", true},
		{strings.Repeat("a", 64), true},
	}
	for _, tt := range tests {
		t.Run("name="+tt.name, func(t *testing.T) {
			err := validateCreateInputs("local", 1, tt.name)
			if tt.wantErr && err == nil {
				t.Errorf("Expected an error for name %q, got nil", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error for name %q, got: %v", tt.name, err)
			}
		})
	}
}

func TestCreateHeader(t *testing.T) {
	got := createHeader("aws", 3, "mylab")
	want := "Creating a new homelab environment: mylab\nProvider: aws\nNodes: 3\n"
	if got != want {
		t.Errorf("Expected header: %q, got: %q", want, got)
	}
}

func TestInstanceExists(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		lookup  string
		want    bool
		wantErr bool
	}{
		{"empty list", `{"list": []}`, "homelab", false, false},
		{"match", `{"list": [{"name": "homelab", "state": "Running"}]}`, "homelab", true, false},
		{"no match", `{"list": [{"name": "other"}]}`, "homelab", false, false},
		{"match among several", `{"list": [{"name": "a"}, {"name": "homelab"}]}`, "homelab", true, false},
		{"prefix is not a match", `{"list": [{"name": "homelab-2"}]}`, "homelab", false, false},
		{"invalid json", `not json`, "homelab", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := instanceExists([]byte(tt.data), tt.lookup)
			if (err != nil) != tt.wantErr {
				t.Fatalf("instanceExists() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("instanceExists() = %v, want %v", got, tt.want)
			}
		})
	}
}
