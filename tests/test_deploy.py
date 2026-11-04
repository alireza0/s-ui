"""Installer checks use temporary files; no real service or host config is changed."""
import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import sqlite3
import socket
import ssl
import subprocess
import tempfile
import unittest
import time
import urllib.error
import urllib.parse
import urllib.request
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location('deploy', Path(__file__).parents[1] / 'scripts/deploy.py')
deploy = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(deploy)


class CertificateTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        cls.root = Path(cls.temp.name)
        for name in ('first', 'other'):
            directory = cls.root / name
            directory.mkdir()
            deploy.run('openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes',
                       '-keyout', directory / 'private.key', '-out', directory / 'fullchain.pem',
                       '-days', '2', '-subj', '/CN=panel.example.com',
                       '-addext', 'subjectAltName=DNS:panel.example.com,IP:127.0.0.1,IP:::1')
        cls.cert = cls.root / 'first/fullchain.pem'
        cls.key = cls.root / 'first/private.key'

    @classmethod
    def tearDownClass(cls):
        cls.temp.cleanup()

    def test_pair_and_address_validation(self):
        for host in ('panel.example.com', '127.0.0.1', '::1'):
            self.assertEqual(deploy.validate_pair(self.cert, self.key, host), (self.cert, self.key))
        with self.assertRaises(RuntimeError):
            deploy.validate_pair(self.cert, self.key, 'wrong.example.com')
        with self.assertRaises(ValueError):
            deploy.validate_pair(self.cert, self.root / 'other/private.key', 'panel.example.com')

    def test_expired_certificate_is_rejected(self):
        with patch.object(deploy.time, 'time', return_value=4102444800):
            with self.assertRaises(ValueError):
                deploy.validate_pair(self.cert, self.key, 'panel.example.com')

    def test_discovery_keeps_renewal_symlinks(self):
        with tempfile.TemporaryDirectory() as temporary:
            live = Path(temporary)
            (live / 'fullchain.pem').symlink_to(self.cert)
            (live / 'privkey.pem').symlink_to(self.key)
            pairs = deploy.discover_pairs('panel.example.com', [(str(live), 1)])
            self.assertEqual(pairs, [(live / 'fullchain.pem', live / 'privkey.pem')])

    def test_wrong_host_and_shallow_root_not_selected(self):
        self.assertEqual(deploy.discover_pairs('wrong.example.com', [(str(self.root), 3)]), [])
        self.assertEqual(deploy.discover_pairs('panel.example.com', [(str(self.root), 1)]), [])

    def test_paste_is_validated_before_returning(self):
        lines = ['2', *self.cert.read_text().splitlines(), 'END',
                 *self.key.read_text().splitlines(), 'END']
        with tempfile.TemporaryDirectory() as temporary, \
                patch.object(deploy, 'discover_pairs', return_value=[]), \
                patch('builtins.input', side_effect=lines), contextlib.redirect_stdout(io.StringIO()):
            cert, key = deploy.choose_certificate('panel.example.com', Path(temporary))
            deploy.validate_pair(cert, key, 'panel.example.com')
            self.assertEqual(key.stat().st_mode & 0o777, 0o600)


class ConfigurationTests(unittest.TestCase):
    def test_host_inputs(self):
        self.assertEqual(deploy.parse_host(' PANEL.example.com. ', '1'), 'panel.example.com')
        self.assertEqual(deploy.parse_host('[2001:db8::1]', '2'), '2001:db8::1')
        for value in ('https://example.com', 'example.com/path', 'example.com:443',
                      '-bad.example.com', '127.0.0.1', ''):
            with self.assertRaises(ValueError):
                deploy.parse_host(value, '1')

    def test_domain_tls_and_ip_http_settings(self):
        settings, scheme, host = deploy.settings_for('panel.example.com', (Path('/a'), Path('/b')))
        self.assertEqual(settings['webCertFile'], settings['subCertFile'])
        self.assertEqual(settings['webKeyFile'], settings['subKeyFile'])
        self.assertEqual((settings['webDomain'], scheme), ('panel.example.com', 'https'))
        settings, scheme, host = deploy.settings_for('2001:db8::1', None)
        self.assertEqual((scheme, host), ('http', '[2001:db8::1]'))
        for key in ('webDomain', 'subDomain', 'webCertFile', 'webKeyFile', 'subCertFile', 'subKeyFile'):
            self.assertEqual(settings[key], '')

    def test_schema_change_does_not_partially_write(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / 'test.db'
            with contextlib.closing(sqlite3.connect(path)) as db, db:
                db.execute('CREATE TABLE settings (key TEXT, value TEXT)')
                db.execute("INSERT INTO settings VALUES ('webDomain', 'old.example.com')")
            with self.assertRaises(RuntimeError):
                deploy.write_settings(path, {'webDomain': 'new.example.com', 'missing': ''})
            with contextlib.closing(sqlite3.connect(path)) as db:
                self.assertEqual(db.execute('SELECT value FROM settings').fetchone()[0], 'old.example.com')

    def test_corrupt_or_unlisted_release_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            archive, sums = Path(temporary) / 'release.tgz', Path(temporary) / 'SHA256SUMS'
            archive.write_bytes(b'archive')
            sums.write_text(hashlib.sha256(b'archive').hexdigest() + '  release.tgz\n')
            deploy.verify_checksum(archive, sums)
            archive.write_bytes(b'corrupt')
            with self.assertRaises(ValueError):
                deploy.verify_checksum(archive, sums)
            sums.write_text('')
            with self.assertRaises(ValueError):
                deploy.verify_checksum(archive, sums)


@unittest.skipUnless(os.environ.get('SUI_TEST_BINARY'), 'Set SUI_TEST_BINARY to an official release sui')
class OfficialBinaryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.install = self.root / 'installation'
        self.stage = self.root / 'stage'
        self.stage.mkdir()
        import shutil
        shutil.copy2(os.environ['SUI_TEST_BINARY'], self.stage / 'sui')
        (self.stage / 's-ui.service').write_text('test service')
        (self.stage / 's-ui.sh').write_text('#!/bin/bash\n')
        self.patches = [patch.object(deploy, 'INSTALL', self.install),
                        patch.object(deploy, 'UNIT', self.root / 'unit'),
                        patch.object(deploy, 'MENU', self.root / 'menu'),
                        patch.object(deploy, 'BACKUP_ROOT', self.root / 'backups')]
        for p in self.patches:
            p.start()
        self.real_run = deploy.run
        self.real_subprocess_run = subprocess.run

    def tearDown(self):
        for p in reversed(self.patches):
            p.stop()
        self.temp.cleanup()

    def fake_subprocess(self, args, **kwargs):
        if str(args[0]) == 'systemctl':
            return subprocess.CompletedProcess(args, 1 if args[1].startswith('is-') else 0, b'', b'')
        return self.real_subprocess_run(args, **kwargs)

    def cli(self, *args, **kwargs):
        if str(args[0]) == 'systemctl' or args[1] == 'healthcheck':
            return b''
        return self.real_run(*args, **kwargs)

    def install_once(self, host='panel.example.com', cli=None):
        with patch.object(deploy, 'run', side_effect=cli or self.cli), \
                patch.object(deploy.subprocess, 'run', side_effect=self.fake_subprocess), \
                patch.object(deploy, 'check_endpoints'), \
                contextlib.redirect_stdout(io.StringIO()):
            deploy.deploy(self.stage, host, None)

    def read_db(self):
        with contextlib.closing(sqlite3.connect(self.install / 'db/s-ui.db')) as db:
            return dict(db.execute('SELECT key, value FROM settings')), \
                db.execute('SELECT username, password FROM users ORDER BY id LIMIT 1').fetchone()

    def test_fresh_install_and_upgrade_preserve_credentials(self):
        self.install_once()
        settings, admin = self.read_db()
        self.assertEqual(admin[0], 'master')
        self.assertNotEqual(admin[1], 'Change@me2025')
        self.assertEqual(settings['subURI'], 'http://panel.example.com:2096/sub/')
        env = dict(os.environ, SUI_DB_FOLDER=str(self.install / 'db'))
        self.real_run(self.install / 'sui', 'admin', '-username', 'custom', '-password', 'a new password', env=env)
        _, before = self.read_db()
        self.install_once('2001:db8::1')
        settings, after = self.read_db()
        self.assertEqual(before, after)
        self.assertEqual(settings['webDomain'], '')
        self.assertEqual(settings['subURI'], 'http://[2001:db8::1]:2096/sub/')

    def test_start_failure_rolls_back_database_and_files(self):
        self.install_once()
        before = self.read_db()
        (self.install / 'user-marker').write_text('keep me')

        def fail_start(*args, **kwargs):
            if str(args[0]) == 'systemctl' and args[1] == 'restart':
                raise RuntimeError('simulated start failure')
            return self.cli(*args, **kwargs)

        with self.assertRaisesRegex(RuntimeError, 'simulated start failure'):
            self.install_once('127.0.0.1', cli=fail_start)
        self.assertEqual(self.read_db(), before)
        self.assertEqual((self.install / 'user-marker').read_text(), 'keep me')

    def test_real_panel_login_and_subscription_over_tls(self):
        self.install_once('127.0.0.1')
        cert, key = self.root / 'fullchain.pem', self.root / 'private.key'
        deploy.run('openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes',
                   '-keyout', key, '-out', cert, '-days', '1', '-subj', '/CN=localhost',
                   '-addext', 'subjectAltName=IP:127.0.0.1')
        # Reserve two distinct ports, then give them to the temporary panel.
        with socket.socket() as first, socket.socket() as second:
            first.bind(('127.0.0.1', 0))
            second.bind(('127.0.0.1', 0))
            panel_port, sub_port = first.getsockname()[1], second.getsockname()[1]
        settings, _, _ = deploy.settings_for('127.0.0.1', (cert, key))
        settings.update(webListen='127.0.0.1', subListen='127.0.0.1',
                        webPort=str(panel_port), subPort=str(sub_port))
        deploy.write_settings(self.install / 'db/s-ui.db', settings)
        env = dict(os.environ, SUI_DB_FOLDER=str(self.install / 'db'),
                   SUI_BIN_FOLDER=str(self.root / 'bin'))
        context = ssl.create_default_context(cafile=str(cert))
        # Ignore proxy variables for this explicitly local integration test.
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}),
                                             urllib.request.HTTPSHandler(context=context))
        process = subprocess.Popen([str(self.install / 'sui')], cwd=self.root, env=env,
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            login_url = f'https://127.0.0.1:{panel_port}/app/api/login'
            for _ in range(100):
                self.assertIsNone(process.poll(), 'temporary panel exited during startup')
                try:
                    opener.open(f'https://127.0.0.1:{panel_port}/app/login', timeout=1).close()
                    break
                except (OSError, urllib.error.URLError):
                    time.sleep(0.1)
            else:
                self.fail('temporary HTTPS panel did not become ready')
            form = urllib.parse.urlencode({'user': 'master', 'pass': 'Change@me2025'}).encode()
            request = urllib.request.Request(login_url, data=form,
                                             headers={'Origin': f'https://127.0.0.1:{panel_port}'})
            with opener.open(request, timeout=5) as response:
                self.assertTrue(json.load(response)['success'])
            with contextlib.closing(sqlite3.connect(self.install / 'db/s-ui.db')) as db:
                actual = dict(db.execute('SELECT key, value FROM settings'))
            deploy.check_endpoints(actual, '127.0.0.1', (cert, key))
            # An invalid subscription ID returns 400, after a verified TLS handshake.
            try:
                with opener.open(f'https://127.0.0.1:{sub_port}/sub/nonexistent', timeout=5) as response:
                    self.assertLess(response.status, 500)
            except urllib.error.HTTPError as error:
                self.assertEqual(error.code, 400)
        finally:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)


if __name__ == '__main__':
    unittest.main()
