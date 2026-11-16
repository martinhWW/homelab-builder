package main

import (
	"io"
	"slices"
	"testing"
)

// stubLaunch replaces launch with a fake that records its args and returns err,
// so tests never start a real VM. The original is restored when the test ends.
func stubLaunch(t *testing.T, err error) *[]string {
	t.Helper()
	var gotArgs []string
	orig := launch
	launch = func(w io.Writer, args []string) error {
		gotArgs = args
		return err
	}
	t.Cleanup(func() { launch = orig })
	return &gotArgs
}

// stubVerify replaces verify with a fake that records the instance name and
// returns err, so tests never run `multipass exec`. Restored when the test ends.
func stubVerify(t *testing.T, err error) *string {
	t.Helper()
	var gotName string
	orig := verify
	verify = func(name string) error {
		gotName = name
		return err
	}
	t.Cleanup(func() { verify = orig })
	return &gotName
}

// stubVerifyK3s replaces verifyK3s with a fake that records the instance name
// and returns installed and err, so tests never run `multipass exec`.
func stubVerifyK3s(t *testing.T, installed bool, err error) *string {
	t.Helper()
	var gotName string
	orig := verifyK3s
	verifyK3s = func(name string) (bool, error) {
		gotName = name
		return installed, err
	}
	t.Cleanup(func() { verifyK3s = orig })
	return &gotName
}

// stubInstallK3s replaces installK3s with a fake that records the instance name
// and returns err, so tests never install k3s. Restored when the test ends.
// The returned bool reports whether it was called.
func stubInstallK3s(t *testing.T, err error) (*string, *bool) {
	t.Helper()
	var gotName string
	var called bool
	orig := installK3s
	installK3s = func(w io.Writer, name string) error {
		gotName = name
		called = true
		return err
	}
	t.Cleanup(func() { installK3s = orig })
	return &gotName, &called
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
