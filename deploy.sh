#!/usr/bin/env bash
# Custom Linux/systemd installer. Keep upstream install.sh unchanged.
set -euo pipefail

install_dependencies() {
    local package
    local required=(python3 openssl ca-certificates curl tar)
    local missing=()

    if command -v apt-get >/dev/null; then
        for package in "${required[@]}"; do
            if [[ $(dpkg-query -W -f='${db:Status-Status}' "$package" 2>/dev/null || true) != installed ]]; then
                missing+=("$package")
            fi
        done
        if [[ ${#missing[@]} -gt 0 ]]; then
            apt-get update || return
            # An installer should not upgrade already installed prerequisites.
            apt-get install -y --no-upgrade "${missing[@]}" || return
        fi
    elif command -v dnf >/dev/null || command -v yum >/dev/null; then
        for package in "${required[@]}"; do
            rpm -q --quiet "$package" || missing+=("$package")
        done
        if [[ ${#missing[@]} -gt 0 ]]; then
            if command -v dnf >/dev/null; then
                dnf install -y "${missing[@]}" || return
            else
                yum install -y "${missing[@]}" || return
            fi
        fi
    fi

    for package in python3 openssl curl tar; do
        command -v "$package" >/dev/null || {
            echo "缺少必需工具：$package。请修复软件源并安装后重试。" >&2
            return 1
        }
    done
}

# Allow isolated dependency tests to source this file without running deployment.
if [[ ${BASH_SOURCE[0]} != "$0" ]]; then
    return 0
fi

[[ ${EUID} -eq 0 ]] || { echo '请使用 root 运行。' >&2; exit 1; }
command -v systemctl >/dev/null && [[ -d /run/systemd/system ]] || {
    echo '此部署入口需要 Linux + systemd。' >&2; exit 1;
}
install_dependencies
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# Adjacent files are trusted only when the caller explicitly requests local mode.
# A downloaded installer in /tmp must not pick up someone else's /tmp/scripts.
if [[ ${1:-} == --local ]]; then
    exec python3 "$script_dir/scripts/deploy.py"
fi
deploy_dir=$(mktemp -d)
trap 'rm -rf "$deploy_dir"' EXIT
deploy_ref=${SUI_DEPLOY_REF:-dev}
curl --fail --location --show-error --retry 3 \
    "https://raw.githubusercontent.com/wuquanlin/s-ui/${deploy_ref}/scripts/deploy.py" \
    -o "$deploy_dir/deploy.py"
python3 "$deploy_dir/deploy.py"
