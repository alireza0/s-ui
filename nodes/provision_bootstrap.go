package nodes

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type provisionHost struct {
	architecture string
	packages     string
}

const probeProvisionHost = `[ "$(id -u)" = 0 ] || exit 60
kernel=$(uname -s)
[ "$kernel" = Linux ] || exit 66
printf '%s\n' "$kernel" "$(uname -m)"
[ -d /run/systemd/system ] || exit 61
systemctl --system --no-pager show --property=Version --value || exit 61
if [ -r /etc/os-release ]; then
    cat /etc/os-release
elif [ -r /usr/lib/os-release ]; then
    cat /usr/lib/os-release
fi
`

func inspectProvisionHost(ctx context.Context, client *ssh.Client, user string) (provisionHost, error) {
	output, err := runProvisionShell(ctx, client, user, probeProvisionHost, 30*time.Second, "node_preflight")
	if err != nil {
		return provisionHost{}, err
	}
	return parseProvisionHost(string(output))
}

func parseProvisionHost(output string) (provisionHost, error) {
	lines := strings.SplitN(output, "\n", 4)
	if len(lines) != 4 {
		return provisionHost{}, errors.New("node_preflight_failed")
	}
	if lines[0] != "Linux" {
		return provisionHost{}, errors.New("node_linux_required")
	}
	var host provisionHost
	switch lines[1] {
	case "x86_64":
		host.architecture = "amd64"
	case "aarch64":
		host.architecture = "arm64"
	default:
		return provisionHost{}, errors.New("unsupported_architecture")
	}
	version := strings.TrimPrefix(strings.TrimSpace(lines[2]), "systemd ")
	version = strings.TrimPrefix(version, "v")
	if end := strings.IndexFunc(version, func(character rune) bool { return character < '0' || character > '9' }); end >= 0 {
		version = version[:end]
	}
	major, err := strconv.Atoi(version)
	if err != nil {
		return provisionHost{}, errors.New("node_systemd_unavailable")
	}
	// The agent unit uses LoadCredential to supply its mTLS keys to DynamicUser.
	if major < 247 {
		return provisionHost{}, errors.New("node_systemd_too_old")
	}
	for _, line := range strings.Split(lines[3], "\n") {
		key, value, _ := strings.Cut(line, "=")
		if key == "ID" {
			host.packages = nodePackageFamily(strings.Trim(value, "\"'"))
		}
	}
	return host, nil
}

func nodePackageFamily(id string) string {
	switch id {
	case "ubuntu", "debian", "armbian":
		return "apt"
	case "almalinux", "rocky", "centos", "rhel", "fedora", "ol", "oracle":
		return "rpm"
	default:
		return ""
	}
}

func ensureRemotePython(ctx context.Context, client *ssh.Client, user string, host provisionHost) error {
	script := fmt.Sprintf("packages=%q\n", host.packages) + installProvisionPython
	_, err := runProvisionShell(ctx, client, user, script, 8*time.Minute, "node_dependencies")
	return err
}

const installProvisionPython = `python_ready() {
    python3 -c 'import sys; sys.exit(sys.version_info < (3, 7))' >/dev/null 2>&1
}
run_package() {
    limit=$1
    shift
    timeout -k 15s "$limit" "$@" </dev/null >/dev/null 2>&1
    case "$?" in
        0) ;;
        124|137) exit 62 ;;
        *) exit 63 ;;
    esac
}
if python_ready; then exit 0; fi
export LC_ALL=C
case "$packages" in
    apt)
        command -v apt-get >/dev/null 2>&1 || exit 64
        export DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l
        run_package 180s apt-get -o Acquire::Retries=2 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 update
        run_package 240s apt-get -o DPkg::Lock::Timeout=60 -y --no-install-recommends install python3
        ;;
    rpm)
        if command -v dnf >/dev/null 2>&1; then
            run_package 420s dnf -y install python3
        elif command -v yum >/dev/null 2>&1; then
            run_package 420s yum -y install python3
        else
            exit 64
        fi
        ;;
    *) exit 64 ;;
esac
python_ready || exit 65
`

func runProvisionShell(parent context.Context, client *ssh.Client, user, script string, budget time.Duration, step string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, budget)
	defer cancel()
	// A deadline can also expire while opening the SSH channel, before a
	// session is available to close. The caller abandons this connection.
	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stop()
	session, err := client.NewSession()
	if err != nil {
		return nil, provisionShellError(ctx, err, step)
	}
	defer session.Close()
	var output limitedBuffer
	session.Stdin = strings.NewReader(script)
	session.Stdout = &output
	command := "exec sh -s"
	if user != "root" {
		// sudo uses exit 1 when it cannot run the command. The bootstrap
		// scripts reserve 60–66 for failures inside the privileged shell.
		command = `command -v sudo >/dev/null 2>&1 || exit 60; sudo -n sh -s; result=$?; [ "$result" != 1 ] || exit 60; exit "$result"`
	}
	if err = session.Run(command); err != nil {
		return nil, provisionShellError(ctx, err, step)
	}
	return output.Bytes(), nil
}

func provisionShellError(ctx context.Context, err error, step string) error {
	switch ctx.Err() {
	case context.DeadlineExceeded:
		return errors.New(step + "_timeout")
	case context.Canceled:
		return context.Canceled
	}
	var exit *ssh.ExitError
	if errors.As(err, &exit) {
		switch exit.ExitStatus() {
		case 60:
			return errors.New("node_sudo_required")
		case 61:
			return errors.New("node_systemd_unavailable")
		case 62:
			return errors.New("node_dependencies_timeout")
		case 63:
			return errors.New("node_dependencies_failed")
		case 64:
			return errors.New("node_python_bootstrap_unavailable")
		case 65:
			return errors.New("node_python_version_unsupported")
		case 66:
			return errors.New("node_linux_required")
		}
	}
	return errors.New(step + "_failed")
}
