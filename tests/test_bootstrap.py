"""Exercise dependency installation with fake package tools, never the host APT."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


BOOTSTRAP = Path(__file__).resolve().parents[1] / 'deploy.sh'
REQUIRED = 'python3 openssl ca-certificates curl tar'


class DependencyTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        self.log = self.root / 'apt.log'
        self.fake('dpkg-query', '''#!/bin/bash
package="${@: -1}"
if [[ "$package" == "${FAKE_PARTIAL:-}" ]]; then
    printf unpacked
    exit 0
fi
case " ${FAKE_INSTALLED:-} " in
    *" $package "*) printf installed ;;
    *) exit 1 ;;
esac
''')
        self.fake('apt-get', '''#!/bin/bash
printf '%s' "$1" >> "$FAKE_APT_LOG"
if [[ $# -gt 1 ]]; then printf ' %s' "${@:2}" >> "$FAKE_APT_LOG"; fi
printf '\\n' >> "$FAKE_APT_LOG"
case "$1" in
    update) exit "${FAKE_UPDATE_STATUS:-0}" ;;
    install) exit "${FAKE_INSTALL_STATUS:-0}" ;;
    *) exit 99 ;;
esac
''')
        for tool in ('python3', 'openssl', 'curl', 'tar'):
            self.fake(tool, '#!/bin/bash\nexit 0\n')

    def tearDown(self):
        self.temporary.cleanup()

    def fake(self, name, content):
        path = self.bin / name
        path.write_text(content)
        path.chmod(0o755)

    def dependencies(self, installed=REQUIRED, **overrides):
        env = dict(os.environ, PATH=str(self.bin) + ':/usr/bin:/bin',
                   FAKE_APT_LOG=str(self.log), FAKE_INSTALLED=installed, **overrides)
        return subprocess.run(['/bin/bash', '-c', 'source "$1"; install_dependencies',
                               'dependency-test', str(BOOTSTRAP)], env=env,
                              capture_output=True, text=True, timeout=10)

    def commands(self):
        return self.log.read_text().splitlines() if self.log.exists() else []

    def test_installed_dependencies_do_not_trigger_the_broken_ca_upgrade(self):
        # This is the reported Debian 11 case: the tools are installed, but
        # APT would fail if it tried to fetch an unrelated CA package upgrade.
        result = self.dependencies(FAKE_INSTALL_STATUS='100')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.commands(), [])

    def test_only_missing_packages_are_installed_without_upgrading_others(self):
        result = self.dependencies(installed='openssl ca-certificates tar')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.commands(), ['update', 'install -y --no-upgrade python3 curl'])

    def test_missing_ca_package_is_still_required(self):
        result = self.dependencies(installed='python3 openssl curl tar')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.commands(), ['update', 'install -y --no-upgrade ca-certificates'])

    def test_unconfigured_package_is_not_treated_as_installed(self):
        result = self.dependencies(FAKE_PARTIAL='ca-certificates')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.commands(), ['update', 'install -y --no-upgrade ca-certificates'])

    def test_index_failure_stops_before_package_installation(self):
        result = self.dependencies(installed='', FAKE_UPDATE_STATUS='100')
        self.assertEqual(result.returncode, 100)
        self.assertEqual(self.commands(), ['update'])

    def test_missing_package_install_failure_is_not_ignored(self):
        result = self.dependencies(installed='', FAKE_INSTALL_STATUS='100')
        self.assertEqual(result.returncode, 100)
        self.assertEqual(len(self.commands()), 2)


if __name__ == '__main__':
    unittest.main()
