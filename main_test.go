package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	gotArgs := stubLaunch(t, nil)
	gotName := stubVerify(t, nil)
	gotCheckName := stubVerifyK3s(t, false, nil)
	gotInstallName, _ := stubInstallK3s(t, nil)
	home := stubKubeconfigSteps(t, "homelab")
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Error executing command: %v", err)
	}
	want := "Creating a new homelab environment: homelab\nProvider: local\nNodes: 1\n" +
		"multipass launch 24.04 --name homelab --cpus 2 --memory 4G --disk 30G\n" +
		"vm \"homelab\" is ready\n" +
		"k3s has been installed on VM \"homelab\"\n" +
		"waiting for node to be ready...\n"
	kubeconfigPath := filepath.Join(home, ".homelab", "homelab", "kubeconfig")
	want += fmt.Sprintf("connect to k3s cluster using kubeconfig: export KUBECONFIG=%q\n", kubeconfigPath)
	saved, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		t.Fatalf("reading saved kubeconfig: %v", err)
	}
	if !strings.Contains(string(saved), "server: https://192.168.64.5:6443") {
		t.Errorf("saved kubeconfig server not rewritten to VM IP, got: %q", saved)
	}
	if *gotCheckName != "homelab" {
		t.Errorf("verifyK3s called with %q, want %q", *gotCheckName, "homelab")
	}
	if *gotInstallName != "homelab" {
		t.Errorf("installK3s called with %q, want %q", *gotInstallName, "homelab")
	}
	if buf.String() != want {
		t.Errorf("Expected output: %q, got: %q", want, buf.String())
	}
	if !slices.Equal(*gotArgs, buildMultipassLaunchArgs("homelab")) {
		t.Errorf("launch called with %v, want %v", *gotArgs, buildMultipassLaunchArgs("homelab"))
	}
	if *gotName != "homelab" {
		t.Errorf("verify called with %q, want %q", *gotName, "homelab")
	}
}

func TestCreateVerifyFails(t *testing.T) {
	stubLaunch(t, nil)
	stubVerify(t, errors.New("not reachable"))
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create", "--name", "faillab"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("Expected verify error to be returned, got: %v", err)
	}
	if strings.Contains(buf.String(), "is ready") {
		t.Errorf("Expected no ready message when verify fails, got: %q", buf.String())
	}
}

func TestCreateLaunchFails(t *testing.T) {
	stubLaunch(t, errors.New("boom"))
	rootCmd := newRootCommand()
	rootCmd.SetOut(io.Discard)
	rootCmd.SetArgs([]string{"create", "--name", "faillab"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Expected launch error to be returned, got: %v", err)
	}
}

func TestCreateWithName(t *testing.T) {
	gotArgs := stubLaunch(t, nil)
	gotName := stubVerify(t, nil)
	stubVerifyK3s(t, false, nil)
	gotInstallName, _ := stubInstallK3s(t, nil)
	stubKubeconfigSteps(t, "mylab")
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
	if !slices.Equal(*gotArgs, buildMultipassLaunchArgs("mylab")) {
		t.Errorf("launch called with %v, want %v", *gotArgs, buildMultipassLaunchArgs("mylab"))
	}
	if *gotName != "mylab" {
		t.Errorf("verify called with %q, want %q", *gotName, "mylab")
	}
	if *gotInstallName != "mylab" {
		t.Errorf("installK3s called with %q, want %q", *gotInstallName, "mylab")
	}
}

func TestCreateK3sAlreadyInstalled(t *testing.T) {
	stubLaunch(t, nil)
	stubVerify(t, nil)
	stubVerifyK3s(t, true, nil)
	_, called := stubInstallK3s(t, nil)
	home := stubKubeconfigSteps(t, "k3slab")
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create", "--name", "k3slab"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Error executing command: %v", err)
	}
	if *called {
		t.Error("installK3s was called, but k3s was already installed")
	}
	if !strings.Contains(buf.String(), "k3s is already installed on VM \"k3slab\"") {
		t.Errorf("Expected already-installed message, got: %q", buf.String())
	}
	// Re-running create must still save the kubeconfig.
	if _, err := os.Stat(filepath.Join(home, ".homelab", "k3slab", "kubeconfig")); err != nil {
		t.Errorf("Expected kubeconfig to be saved when k3s was already installed: %v", err)
	}
}

func TestCreateVMInfoFails(t *testing.T) {
	stubLaunch(t, nil)
	stubVerify(t, nil)
	stubVerifyK3s(t, true, nil)
	stubInstallK3s(t, nil)
	stubKubeconfigSteps(t, "k3slab")
	stubGetVMInfo(t, "", errors.New("info broke"))
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create", "--name", "k3slab"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "info broke") {
		t.Fatalf("Expected vm info error to be returned, got: %v", err)
	}
	if strings.Contains(buf.String(), "KUBECONFIG") {
		t.Errorf("Expected no connect message when vm info fails, got: %q", buf.String())
	}
}

func TestCreateGetKubeconfigFails(t *testing.T) {
	stubLaunch(t, nil)
	stubVerify(t, nil)
	stubVerifyK3s(t, true, nil)
	stubInstallK3s(t, nil)
	home := stubKubeconfigSteps(t, "k3slab")
	stubGetKubeconfig(t, "", errors.New("read broke"))
	rootCmd := newRootCommand()
	rootCmd.SetOut(io.Discard)
	rootCmd.SetArgs([]string{"create", "--name", "k3slab"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "read broke") {
		t.Fatalf("Expected kubeconfig read error to be returned, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".homelab", "k3slab", "kubeconfig")); err == nil {
		t.Error("Expected no kubeconfig file when reading it from the VM fails")
	}
}

func TestCreateK3sCheckFails(t *testing.T) {
	stubLaunch(t, nil)
	stubVerify(t, nil)
	stubVerifyK3s(t, false, errors.New("check broke"))
	_, called := stubInstallK3s(t, nil)
	rootCmd := newRootCommand()
	rootCmd.SetOut(io.Discard)
	rootCmd.SetArgs([]string{"create", "--name", "k3slab"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "check broke") {
		t.Fatalf("Expected verifyK3s error to be returned, got: %v", err)
	}
	if *called {
		t.Error("installK3s was called after the k3s check failed")
	}
}

func TestCreateK3sInstallFails(t *testing.T) {
	stubLaunch(t, nil)
	stubVerify(t, nil)
	stubVerifyK3s(t, false, nil)
	stubInstallK3s(t, errors.New("install broke"))
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create", "--name", "k3slab"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "install broke") {
		t.Fatalf("Expected installK3s error to be returned, got: %v", err)
	}
	if strings.Contains(buf.String(), "has been installed") {
		t.Errorf("Expected no success message when install fails, got: %q", buf.String())
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

func TestEnvironmentDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := environmentDir("mylab")
	if err != nil {
		t.Fatalf("environmentDir() error = %v", err)
	}
	want := filepath.Join(home, ".homelab", "mylab")
	if got != want {
		t.Errorf("environmentDir() = %q, want %q", got, want)
	}
}

func TestEnvironmentDirNoHome(t *testing.T) {
	t.Setenv("HOME", "")
	_, err := environmentDir("mylab")
	if err == nil || !strings.Contains(err.Error(), "finding home directory") {
		t.Fatalf("Expected home directory error, got: %v", err)
	}
}

func TestSaveKubeconfig(t *testing.T) {
	// Nested path checks that missing parents (like ~/.homelab) are created.
	dir := filepath.Join(t.TempDir(), ".homelab", "mylab")
	data := []byte("apiVersion: v1\n")

	gotPath, err := saveKubeconfig(dir, data)
	if err != nil {
		t.Fatalf("saveKubeconfig() error = %v", err)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0700 {
		t.Errorf("dir permissions = %o, want 700", perm)
	}

	path := filepath.Join(dir, "kubeconfig")
	if gotPath != path {
		t.Errorf("saveKubeconfig() path = %q, want %q", gotPath, path)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat kubeconfig: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0600 {
		t.Errorf("kubeconfig permissions = %o, want 600", perm)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read kubeconfig: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("kubeconfig contents = %q, want %q", got, data)
	}
}

func TestSaveKubeconfigExistingDir(t *testing.T) {
	// Re-running create on an existing environment must not fail.
	dir := filepath.Join(t.TempDir(), "mylab")
	if _, err := saveKubeconfig(dir, []byte("old")); err != nil {
		t.Fatalf("first saveKubeconfig() error = %v", err)
	}
	if _, err := saveKubeconfig(dir, []byte("new")); err != nil {
		t.Fatalf("second saveKubeconfig() error = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "kubeconfig"))
	if err != nil {
		t.Fatalf("read kubeconfig: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("kubeconfig contents = %q, want %q", got, "new")
	}
}

func TestCreateNodeNotReady(t *testing.T) {
	stubLaunch(t, nil)
	stubVerify(t, nil)
	stubVerifyK3s(t, true, nil)
	stubInstallK3s(t, nil)
	home := stubKubeconfigSteps(t, "k3slab")
	stubWaitForNodeReady(t, errors.New("node not ready"))
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create", "--name", "k3slab"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "node not ready") {
		t.Fatalf("Expected node readiness error to be returned, got: %v", err)
	}
	if strings.Contains(buf.String(), "KUBECONFIG") {
		t.Errorf("Expected no connect message when node is not ready, got: %q", buf.String())
	}
	// The kubeconfig is saved before waiting, so it's there for debugging.
	if _, err := os.Stat(filepath.Join(home, ".homelab", "k3slab", "kubeconfig")); err != nil {
		t.Errorf("Expected kubeconfig to be saved even when the node is not ready: %v", err)
	}
}
