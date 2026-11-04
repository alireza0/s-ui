#!/usr/bin/env bash
# Custom Linux/systemd installer. Keep upstream install.sh unchanged.
set -euo pipefail
[[ ${EUID} -eq 0 ]] || { echo '请使用 root 运行。' >&2; exit 1; }
command -v systemctl >/dev/null && [[ -d /run/systemd/system ]] || {
    echo '此部署入口需要 Linux + systemd。' >&2; exit 1;
}
if command -v apt-get >/dev/null; then
    apt-get update
    apt-get install -y python3 openssl ca-certificates curl tar
elif command -v dnf >/dev/null; then
    dnf install -y python3 openssl ca-certificates curl tar
elif command -v yum >/dev/null; then
    yum install -y python3 openssl ca-certificates curl tar
else
    for tool in python3 openssl curl tar; do command -v "$tool" >/dev/null; done
fi
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
