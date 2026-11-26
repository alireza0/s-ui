package nodes

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProvisionHostRequirements(t *testing.T) {
	cases := []struct {
		name, probe, arch, packages, errorCode string
	}{
		{"Debian", "Linux\nx86_64\n257.7-1\nID=debian\nVERSION_ID=13\n", "amd64", "apt", ""},
		{"Ubuntu", "Linux\nx86_64\n255.4-1\nID=ubuntu\nVERSION_ID=\"24.04\"\n", "amd64", "apt", ""},
		{"AlmaLinux", "Linux\naarch64\n252 (252-1.el9)\nID=\"almalinux\"\n", "arm64", "rpm", ""},
		{"Rocky", "Linux\nx86_64\nv252-1.el9\nID=\"rocky\"\n", "amd64", "rpm", ""},
		{"Oracle ID", "Linux\nx86_64\n252\nID=ol\n", "amd64", "rpm", ""},
		{"CentOS Stream", "Linux\nx86_64\n252\nID=\"centos\"\n", "amd64", "rpm", ""},
		{"No automatic package changes on an unlisted image", "Linux\nx86_64\n252\nID_LIKE=debian\nID=other\n", "amd64", "", ""},
		{"Old manager", "Linux\nx86_64\n239\nID=centos\n", "", "", "node_systemd_too_old"},
		{"Missing manager version", "Linux\nx86_64\n\nID=debian\n", "", "", "node_systemd_unavailable"},
		{"Wrong architecture", "Linux\ns390x\n252\nID=debian\n", "", "", "unsupported_architecture"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host, err := parseProvisionHost(tc.probe)
			if tc.errorCode != "" {
				if err == nil || err.Error() != tc.errorCode {
					t.Fatalf("got %v, want %q", err, tc.errorCode)
				}
				return
			}
			if err != nil || host.architecture != tc.arch || host.packages != tc.packages {
				t.Fatalf("unexpected host: %+v, %v", host, err)
			}
		})
	}
}

func TestProvisionPythonPackageCommands(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("package scripts run on Linux")
	}
	cases := []provisionPythonCase{
		{name: "Existing Python", family: "rpm", managers: []string{"dnf"}, pythonPresent: true},
		{name: "APT", family: "apt", managers: []string{"apt-get"}, commands: "apt-get\napt-get\n"},
		{name: "DNF preferred", family: "rpm", managers: []string{"dnf", "yum"}, commands: "dnf\n"},
		{name: "YUM fallback", family: "rpm", managers: []string{"yum"}, commands: "yum\n"},
		{name: "Unsupported package family", managers: []string{"apt-get"}, wantExit: 64},
		{name: "Package manager timed out", family: "rpm", managers: []string{"dnf"}, commands: "dnf\n", managerExit: "124", wantExit: 62},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkProvisionPythonCommands(t, tc)
		})
	}
}

type provisionPythonCase struct {
	name, family, commands string
	managers               []string
	pythonPresent          bool
	managerExit            string
	wantExit               int
}

func checkProvisionPythonCommands(t *testing.T, tc provisionPythonCase) {
	t.Helper()
	dir := t.TempDir()
	ready, trace := filepath.Join(dir, "python-ready"), filepath.Join(dir, "packages")
	write := func(name, script string) {
		t.Helper()
		requireNodeTestSuccess(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0700))
	}
	write("python3", "#!/bin/sh\n[ -f \"$PYTHON_READY\" ]\n")
	write("timeout", "#!/bin/sh\nshift 3\nexec \"$@\"\n")
	if tc.pythonPresent {
		write("python-ready", "")
	}
	for _, name := range tc.managers {
		write(name, `#!/bin/sh
printf '%s\n' "${0##*/}" >> "$PACKAGE_TRACE"
if [ -n "$PACKAGE_EXIT" ]; then exit "$PACKAGE_EXIT"; fi
case " $* " in *" install python3 "*) : > "$PYTHON_READY" ;; esac
`)
	}
	cmd := exec.CommandContext(t.Context(), "/bin/sh", "-s")
	cmd.Env = []string{"PATH=" + dir, "PYTHON_READY=" + ready, "PACKAGE_TRACE=" + trace, "PACKAGE_EXIT=" + tc.managerExit}
	cmd.Stdin = strings.NewReader("packages=" + tc.family + "\n" + installProvisionPython)
	err := cmd.Run()
	if tc.wantExit == 0 {
		requireNodeTestSuccess(t, err)
	} else {
		var result *exec.ExitError
		if !errors.As(err, &result) || result.ExitCode() != tc.wantExit {
			t.Fatalf("unexpected exit: %v, want %d", err, tc.wantExit)
		}
	}
	got, err := os.ReadFile(trace)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if string(got) != tc.commands {
		t.Fatalf("got package commands %q, want %q", got, tc.commands)
	}
}
