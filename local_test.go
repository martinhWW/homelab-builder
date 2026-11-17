package main

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
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

// fakeKubeconfig is what k3s writes inside the VM: the server points at
// localhost, which createLocal must rewrite to the VM's IP.
const fakeKubeconfig = "clusters:\n- cluster:\n    server: https://127.0.0.1:6443\n  name: default\n"

// stubGetVMInfo replaces getVmInfo with a fake that returns data and err,
// so tests never run `multipass info`. Restored when the test ends.
func stubGetVMInfo(t *testing.T, data string, err error) {
	t.Helper()
	orig := getVmInfo
	getVmInfo = func(name string) ([]byte, error) {
		return []byte(data), err
	}
	t.Cleanup(func() { getVmInfo = orig })
}

// stubGetKubeconfig replaces getKubeconfig with a fake that returns data and
// err, so tests never read from a VM. Restored when the test ends.
func stubGetKubeconfig(t *testing.T, data string, err error) {
	t.Helper()
	orig := getKubeconfig
	getKubeconfig = func(name string) ([]byte, error) {
		return []byte(data), err
	}
	t.Cleanup(func() { getKubeconfig = orig })
}

// stubWaitForNodeReady replaces waitForNodeReadyCheck with a fake that returns
// err immediately, so tests never poll a real VM. Restored when the test ends.
func stubWaitForNodeReady(t *testing.T, err error) {
	t.Helper()
	orig := waitForNodeReadyCheck
	waitForNodeReadyCheck = func(name string) error {
		return err
	}
	t.Cleanup(func() { waitForNodeReadyCheck = orig })
}

// stubKubeconfigSteps stubs the happy path for fetching the kubeconfig and
// waiting for the node, and points HOME at a temp dir so tests never write to
// the real ~/.homelab. It returns the temp home dir.
func stubKubeconfigSteps(t *testing.T, name string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubGetVMInfo(t, `{"info": {"`+name+`": {"ipv4": ["192.168.64.5"]}}}`, nil)
	stubGetKubeconfig(t, fakeKubeconfig, nil)
	stubWaitForNodeReady(t, nil)
	return home
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

func TestFindInstance(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		lookup    string
		wantFound bool
		wantState string
		wantIPv4  []string
		wantErr   bool
	}{
		{"running with ip", `{"list": [{"name": "homelab", "state": "Running", "ipv4": ["192.168.64.5"]}]}`, "homelab", true, "Running", []string{"192.168.64.5"}, false},
		{"stopped without ip", `{"list": [{"name": "homelab", "state": "Stopped", "ipv4": []}]}`, "homelab", true, "Stopped", []string{}, false},
		{"picks the named instance", `{"list": [{"name": "other", "state": "Running"}, {"name": "homelab", "state": "Suspended"}]}`, "homelab", true, "Suspended", nil, false},
		{"missing", `{"list": [{"name": "other", "state": "Running"}]}`, "homelab", false, "", nil, false},
		{"invalid json", `not json`, "homelab", false, "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found, err := findInstance([]byte(tt.data), tt.lookup)
			if (err != nil) != tt.wantErr {
				t.Fatalf("findInstance() error = %v, wantErr %v", err, tt.wantErr)
			}
			if found != tt.wantFound {
				t.Fatalf("findInstance() found = %v, want %v", found, tt.wantFound)
			}
			if got.State != tt.wantState {
				t.Errorf("findInstance() State = %q, want %q", got.State, tt.wantState)
			}
			if !slices.Equal(got.IPv4, tt.wantIPv4) {
				t.Errorf("findInstance() IPv4 = %v, want %v", got.IPv4, tt.wantIPv4)
			}
		})
	}
}

func TestParseVMIP(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		lookup  string
		want    string
		wantErr bool
	}{
		{"single ip", `{"errors": [], "info": {"mylab": {"ipv4": ["192.168.64.5"], "state": "Running"}}}`, "mylab", "192.168.64.5", false},
		{"first of several ips", `{"info": {"mylab": {"ipv4": ["192.168.64.5", "10.42.0.0"]}}}`, "mylab", "192.168.64.5", false},
		{"vm not in output", `{"info": {"other": {"ipv4": ["192.168.64.9"]}}}`, "mylab", "", true},
		{"no ipv4 addresses", `{"info": {"mylab": {"ipv4": []}}}`, "mylab", "", true},
		{"invalid json", `not json`, "mylab", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVMIP([]byte(tt.data), tt.lookup)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseVMIP() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseVMIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetKubeconfigServer(t *testing.T) {
	data := []byte("clusters:\n- cluster:\n    certificate-authority-data: LS0tLS1CRUdJTg==\n    server: https://127.0.0.1:6443\n  name: default\n")
	want := "clusters:\n- cluster:\n    certificate-authority-data: LS0tLS1CRUdJTg==\n    server: https://192.168.64.5:6443\n  name: default\n"
	got := setKubeconfigServer(data, "192.168.64.5")
	if string(got) != want {
		t.Errorf("setKubeconfigServer() = %q, want %q", got, want)
	}
}

// stubListInstances replaces listInstances with a fake that returns data and
// err, so tests never run `multipass list`. Restored when the test ends.
func stubListInstances(t *testing.T, data string, err error) {
	t.Helper()
	orig := listInstances
	listInstances = func() ([]byte, error) {
		return []byte(data), err
	}
	t.Cleanup(func() { listInstances = orig })
}

// stubCheckNodeReady replaces checkNodeReady with a fake that returns ready and
// err, so tests never run kubectl in a VM. The returned bool reports whether it
// was called. Restored when the test ends.
func stubCheckNodeReady(t *testing.T, ready bool, err error) *bool {
	t.Helper()
	var called bool
	orig := checkNodeReady
	checkNodeReady = func(name string) (bool, error) {
		called = true
		return ready, err
	}
	t.Cleanup(func() { checkNodeReady = orig })
	return &called
}

func TestStatusLocal(t *testing.T) {
	const running = `{"list": [{"name": "homelab", "state": "Running", "ipv4": ["192.168.64.5"]}]}`
	tests := []struct {
		name          string
		list          string
		listErr       error
		ready         bool
		readyErr      error
		want          string
		wantErr       string
		wantNodeCheck bool
	}{
		{
			name:          "running and ready",
			list:          running,
			ready:         true,
			want:          "State: Running\nIP: 192.168.64.5\nK3s Node Status: Ready\n",
			wantNodeCheck: true,
		},
		{
			name:          "running but not ready",
			list:          running,
			ready:         false,
			want:          "State: Running\nIP: 192.168.64.5\nK3s Node Status: NotReady\n",
			wantNodeCheck: true,
		},
		{
			name:          "running but unreachable",
			list:          running,
			readyErr:      errors.New("no route to host"),
			want:          "State: Running\nIP: 192.168.64.5\nK3s Node Status: unknown (no route to host)\n",
			wantNodeCheck: true,
		},
		{
			name: "stopped skips node check",
			list: `{"list": [{"name": "homelab", "state": "Stopped", "ipv4": []}]}`,
			want: "State: Stopped\nIP: none\nK3s Node Status: n/a\n",
		},
		{
			name:    "missing",
			list:    `{"list": [{"name": "other", "state": "Running"}]}`,
			wantErr: `homelab "homelab" not found`,
		},
		{
			name:    "multipass list fails",
			listErr: errors.New("multipass not found"),
			wantErr: "multipass not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubListInstances(t, tt.list, tt.listErr)
			called := stubCheckNodeReady(t, tt.ready, tt.readyErr)
			var buf bytes.Buffer

			err := statusLocal(&buf, "homelab")

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("statusLocal() error = %v, want it to contain %q", err, tt.wantErr)
				}
				if buf.Len() != 0 {
					t.Errorf("Expected no output on error, got: %q", buf.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("statusLocal() unexpected error: %v", err)
			}
			if buf.String() != tt.want {
				t.Errorf("statusLocal() output = %q, want %q", buf.String(), tt.want)
			}
			if *called != tt.wantNodeCheck {
				t.Errorf("node readiness checked = %v, want %v", *called, tt.wantNodeCheck)
			}
		})
	}
}
