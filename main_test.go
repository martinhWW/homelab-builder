package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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
	stubListInstances(t, `{"list": []}`, nil)
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
	want := "→ Creating homelab \"homelab\"...\n  Name:      homelab\n  Provider:  local\n  Nodes:     1\n" +
		"→ Launching VM \"homelab\"...\n" +
		"  multipass launch 24.04 --name homelab --cpus 2 --memory 4G --disk 30G\n" +
		"✓ VM \"homelab\" is up and reachable\n" +
		"→ Installing k3s...\n" +
		"✓ k3s installed\n" +
		"→ Waiting for node to be ready...\n" +
		"🚀 Homelab \"homelab\" is ready!\n" +
		"  Connect with:\n"
	kubeconfigPath := filepath.Join(home, ".homelab", "homelab", "kubeconfig")
	want += fmt.Sprintf("  export KUBECONFIG=%q\n", kubeconfigPath)
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
	stubListInstances(t, `{"list": []}`, nil)
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
	stubListInstances(t, `{"list": []}`, nil)
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
	stubListInstances(t, `{"list": []}`, nil)
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
	stubListInstances(t, `{"list": []}`, nil)
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
	if !strings.Contains(buf.String(), "• k3s already installed, skipping") {
		t.Errorf("Expected already-installed message, got: %q", buf.String())
	}
	// Re-running create must still save the kubeconfig.
	if _, err := os.Stat(filepath.Join(home, ".homelab", "k3slab", "kubeconfig")); err != nil {
		t.Errorf("Expected kubeconfig to be saved when k3s was already installed: %v", err)
	}
}

func TestCreateVMInfoFails(t *testing.T) {
	stubLaunch(t, nil)
	stubListInstances(t, `{"list": []}`, nil)
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
	stubListInstances(t, `{"list": []}`, nil)
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
	stubListInstances(t, `{"list": []}`, nil)
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
	stubListInstances(t, `{"list": []}`, nil)
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

func TestUnsupportedProviderRejected(t *testing.T) {
	for _, command := range []string{"create", "status"} {
		for _, provider := range []string{"aws", "gcp"} {
			t.Run(command+"/"+provider, func(t *testing.T) {
				rootCmd := newRootCommand()
				var buf bytes.Buffer
				rootCmd.SetOut(&buf)
				rootCmd.SetArgs([]string{command, "--provider", provider})
				err := rootCmd.Execute()
				if err == nil || !strings.Contains(err.Error(), "not yet supported") {
					t.Fatalf("Expected a not-yet-supported error, got: %v", err)
				}
				if buf.Len() != 0 {
					t.Errorf("Expected no output for unsupported provider, got: %q", buf.String())
				}
			})
		}
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

func TestValidateNodesFlag(t *testing.T) {
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
			err := validateNodesFlag(tt.nodes)
			if tt.wantErr && err == nil {
				t.Errorf("Expected an error for %d nodes, got nil", tt.nodes)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error for %d nodes, got: %v", tt.nodes, err)
			}
		})
	}
}

func TestValidateProviderFlag(t *testing.T) {
	tests := []struct {
		provider string
		wantErr  bool
	}{
		{"local", false},
		{"aws", true},
		{"gcp", true},
		{"", true},
		{"banana", true},
		{"AWS", true},
	}
	for _, tt := range tests {
		t.Run("provider="+tt.provider, func(t *testing.T) {
			err := validateProviderFlag(tt.provider)
			if tt.wantErr && err == nil {
				t.Errorf("Expected an error for provider %q, got nil", tt.provider)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error for provider %q, got: %v", tt.provider, err)
			}
		})
	}
}

func TestValidateNameFlag(t *testing.T) {
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
			err := validateNameFlag(tt.name)
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
	want := "→ Creating homelab \"mylab\"...\n  Name:      mylab\n  Provider:  aws\n  Nodes:     3\n"
	if got != want {
		t.Errorf("Expected header: %q, got: %q", want, got)
	}
}

func TestEnvDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := envDir("mylab")
	if err != nil {
		t.Fatalf("envDir() error = %v", err)
	}
	want := filepath.Join(home, ".homelab", "mylab")
	if got != want {
		t.Errorf("envDir() = %q, want %q", got, want)
	}
}

func TestEnvDirNoHome(t *testing.T) {
	t.Setenv("HOME", "")
	_, err := envDir("mylab")
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
	stubListInstances(t, `{"list": []}`, nil)
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

func TestSaveState(t *testing.T) {
	// Nested path: state is written right after launch, before saveKubeconfig
	// has created the environment directory.
	dir := filepath.Join(t.TempDir(), ".homelab", "mylab")
	want := HomelabState{
		Name:      "mylab",
		Provider:  "local",
		CreatedAt: time.Date(2026, 11, 18, 10, 30, 0, 0, time.UTC),
	}

	if err := saveState(dir, want); err != nil {
		t.Fatalf("saveState() error = %v", err)
	}

	path := filepath.Join(dir, "state.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat state.json: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("state.json permissions = %o, want 600", perm)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	var got HomelabState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("state.json is not valid JSON: %v", err)
	}
	if got.Name != want.Name || got.Provider != want.Provider || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("state = %+v, want %+v", got, want)
	}
}

func TestLoadState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mylab")
	want := HomelabState{
		Name:      "mylab",
		Provider:  "local",
		CreatedAt: time.Date(2026, 11, 18, 10, 30, 0, 0, time.UTC),
	}
	if err := saveState(dir, want); err != nil {
		t.Fatalf("saveState() error = %v", err)
	}

	got, err := loadState(dir)
	if err != nil {
		t.Fatalf("loadState() error = %v", err)
	}
	if got.Name != want.Name || got.Provider != want.Provider || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("loadState() = %+v, want %+v", got, want)
	}
}

func TestLoadStateMissing(t *testing.T) {
	// destroy relies on this to tell "unmanaged" apart from other failures.
	_, err := loadState(t.TempDir())
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("loadState() error = %v, want fs.ErrNotExist", err)
	}
}

func TestLoadStateCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{not json"), 0600); err != nil {
		t.Fatalf("write state.json: %v", err)
	}

	_, err := loadState(dir)
	if err == nil {
		t.Fatal("loadState() error = nil, want error for corrupt file")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("loadState() error = %v, corrupt file must not look like a missing file", err)
	}
}

func TestConfirmDestroy(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"y", "y\n", true},
		{"yes", "yes\n", true},
		{"uppercase with spaces", "  YES \n", true},
		{"n", "n\n", false},
		{"empty line defaults to no", "\n", false},
		{"other word", "nope\n", false},
		{"y without newline", "y", true},
		{"no input at all", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := confirmDestroy(strings.NewReader(tt.input), &out, "mylab")
			if err != nil {
				t.Fatalf("confirmDestroy() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("confirmDestroy(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if !strings.Contains(out.String(), "mylab") {
				t.Errorf("prompt %q does not name the homelab", out.String())
			}
		})
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestConfirmDestroyReadError(t *testing.T) {
	got, err := confirmDestroy(errReader{}, io.Discard, "mylab")
	if err == nil {
		t.Fatal("confirmDestroy() error = nil, want read error")
	}
	if got {
		t.Error("confirmDestroy() = true on read error, want false")
	}
}

func runDestroy(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()
	rootCmd := newRootCommand()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetIn(strings.NewReader(input))
	rootCmd.SetArgs(append([]string{"destroy", "--name", "mylab"}, args...))
	err := rootCmd.Execute()
	return buf.String(), err
}

func TestDestroyPromptsAndAborts(t *testing.T) {
	dir := setupDestroy(t, listWithMylab, true)
	gotDelete := stubDeleteVM(t, nil)

	out, err := runDestroy(t, "n\n")
	if err != nil {
		t.Fatalf("destroy error = %v", err)
	}
	if !strings.Contains(out, "[y/N]") {
		t.Errorf("expected a confirmation prompt, got: %q", out)
	}
	if !strings.Contains(out, "aborted") {
		t.Errorf("expected an aborted message, got: %q", out)
	}
	if *gotDelete != "" {
		t.Errorf("deleteVM called with %q after the user said no", *gotDelete)
	}
	if _, err := loadState(dir); err != nil {
		t.Errorf("state removed after abort: %v", err)
	}
}

func TestDestroyPromptsAndConfirms(t *testing.T) {
	setupDestroy(t, listWithMylab, true)
	gotDelete := stubDeleteVM(t, nil)

	out, err := runDestroy(t, "y\n")
	if err != nil {
		t.Fatalf("destroy error = %v", err)
	}
	if !strings.Contains(out, "[y/N]") {
		t.Errorf("expected a confirmation prompt, got: %q", out)
	}
	if *gotDelete != "mylab" {
		t.Errorf("deleteVM called with %q, want %q", *gotDelete, "mylab")
	}
}

func TestDestroyForceSkipsPrompt(t *testing.T) {
	setupDestroy(t, listWithMylab, true)
	gotDelete := stubDeleteVM(t, nil)

	// No input: with --force, destroy must not wait for an answer.
	out, err := runDestroy(t, "", "--force")
	if err != nil {
		t.Fatalf("destroy --force error = %v", err)
	}
	if strings.Contains(out, "[y/N]") {
		t.Errorf("--force should skip the prompt, got: %q", out)
	}
	if *gotDelete != "mylab" {
		t.Errorf("deleteVM called with %q, want %q", *gotDelete, "mylab")
	}
}
