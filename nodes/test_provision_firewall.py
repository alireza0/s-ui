import json
import shlex
import stat
import subprocess
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import provision_firewall as firewall


class Backend:
    def __init__(self, kind):
        self.kind = kind
        self.active = True
        self.zones = ['public', 'internal']
        self.ports = set()
        self.ufw = {}
        self.calls = []
        self.fail_after = None

    def which(self, command):
        if self.kind == 'ufw':
            return command if command == 'ufw' else None
        return command if command in ('firewall-cmd', 'firewall-offline-cmd') else None

    def run(self, arguments, check=True):
        self.calls.append(arguments)
        if arguments[0] == 'ufw':
            return self.run_ufw(arguments)
        if '--state' in arguments:
            return SimpleNamespace(returncode=0 if self.active else 252, stdout='running')
        if '--get-active-zones' in arguments:
            return SimpleNamespace(returncode=0, stdout='\n'.join(zone + '\n  interfaces: test' for zone in self.zones))
        zone = next(value.split('=', 1)[1] for value in arguments if value.startswith('--zone='))
        key = ('--permanent' in arguments or arguments[0] == 'firewall-offline-cmd', zone, arguments[-1].split('=', 1)[1])
        if arguments[-1].startswith('--query-port='):
            return SimpleNamespace(returncode=0 if key in self.ports else 1, stdout='')
        if arguments[-1].startswith('--add-port='):
            self.ports.add(key)
            self.maybe_fail('add')
        elif arguments[-1].startswith('--remove-port='):
            self.ports.remove(key)
            self.maybe_fail('remove')
        else:
            raise AssertionError(arguments)
        return SimpleNamespace(returncode=0, stdout='')

    def run_ufw(self, arguments):
        if arguments[1:] == ['status']:
            return SimpleNamespace(returncode=0, stdout='Status: active' if self.active else 'Status: inactive')
        if arguments[1:] == ['show', 'added']:
            return SimpleNamespace(returncode=0, stdout='\n'.join('ufw allow ' + port + ' comment ' + shlex.quote(comment) for port, comment in self.ufw.items()))
        if arguments[1] == 'allow':
            self.ufw[arguments[2]] = arguments[-1]
            self.maybe_fail('add')
        elif arguments[1:4] == ['--force', 'delete', 'allow']:
            del self.ufw[arguments[4]]
            self.maybe_fail('remove')
        else:
            raise AssertionError(arguments)
        return SimpleNamespace(returncode=0, stdout='')

    def maybe_fail(self, action):
        if action == self.fail_after:
            self.fail_after = None
            raise subprocess.CalledProcessError(1, ['interrupted-' + action])


class OwnershipChecks(unittest.TestCase):
    def test_corrupt_persisted_rules_are_rejected_before_commands(self):
        self.apply([(443, 'tcp')])
        path = self.directory / 'firewall-state.json'
        original = json.loads(path.read_text())
        cases = [('port', '--malformed'), ('network', 'tcp;invalid'), ('zone', 'public --invalid')]
        for field, value in cases:
            with self.subTest(field=field):
                state = json.loads(json.dumps(original))
                next(iter(state['rules'].values()))[field] = value
                path.write_text(json.dumps(state))
                calls = len(self.backend.calls)
                with self.assertRaises(ValueError):
                    self.apply([])
                self.assertEqual(len(self.backend.calls), calls)

    def test_firewall_owner_requires_ascii_digits(self):
        with self.assertRaises(ValueError):
            self.apply([], owner='s-ui-node-1\u0660')
        self.assertEqual(self.backend.calls, [])

    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.directory = Path(temporary.name)
        self.backend = Backend('firewalld')
        directory = patch.object(firewall, 'FIREWALL_STATE_DIR', self.directory)
        commands = patch.object(firewall, '_firewall_run', side_effect=lambda *args, **kwargs: self.backend.run(*args, **kwargs))
        lookup = patch.object(firewall.shutil, 'which', side_effect=lambda command: self.backend.which(command))
        for item in (directory, commands, lookup):
            item.start()
            self.addCleanup(item.stop)

    def apply(self, ports, owner='s-ui-node-10', confirmed=True, revision=1, **options):
        listeners = [{'port': port, 'network': network} for port, network in ports]
        version = {'revision': revision, 'policy': options.pop('policy', 0), 'generation': options.pop('generation', 0), 'operation': options.pop('operation', '')}
        firewall.configure_node_firewall(listeners, owner, confirmed, version=version, **options)

    def state(self):
        return json.loads((self.directory / 'firewall-state.json').read_text())

    def test_shared_ports_survive_until_last_owner_releases(self):
        ports = [(443, 'tcp'), (443, 'udp')]
        self.apply(ports)
        self.apply(ports, owner='s-ui-node-20')
        self.assertEqual(len(self.backend.ports), 8)
        self.apply([], revision=2)
        self.assertEqual(len(self.backend.ports), 8)
        self.apply([], owner='s-ui-node-20', release=True)
        self.assertEqual(self.backend.ports, set())
        self.assertFalse(any('--reload' in call for call in self.backend.calls))

    def test_old_ports_close_only_after_acknowledgement(self):
        self.apply([(443, 'tcp')])
        self.apply([(8443, 'tcp')], confirmed=False, revision=2)
        self.assertEqual(len(self.backend.ports), 8)
        self.apply([(8443, 'tcp')], revision=2)
        self.assertEqual({port for _, _, port in self.backend.ports}, {'8443/tcp'})
        self.apply([(443, 'tcp')], revision=1)
        self.assertEqual({port for _, _, port in self.backend.ports}, {'8443/tcp'})

    def test_rejected_configuration_releases_pending_ports(self):
        self.apply([(443, 'tcp')])
        self.apply([(8443, 'tcp')], confirmed=False, revision=2)
        self.apply([(443, 'tcp')])
        self.assertEqual({port for _, _, port in self.backend.ports}, {'443/tcp'})

    def test_existing_permissions_are_preserved(self):
        original = {(False, 'public', '443/tcp'), (True, 'public', '443/tcp')}
        self.backend.ports.update(original)
        self.apply([(443, 'tcp')])
        self.apply([], revision=2)
        self.assertEqual(self.backend.ports, original)

    def test_interrupted_add_is_still_owned(self):
        self.backend.fail_after = 'add'
        with self.assertRaises(subprocess.CalledProcessError):
            self.apply([(443, 'tcp')], confirmed=False)
        self.apply([], revision=2)
        self.assertEqual(self.backend.ports, set())
        self.assertEqual(self.state()['rules'], {})

    def test_interrupted_removal_can_be_retried(self):
        self.apply([(443, 'tcp')])
        self.backend.fail_after = 'remove'
        with self.assertRaises(subprocess.CalledProcessError):
            self.apply([], revision=2)
        self.apply([], revision=2)
        self.assertEqual(self.backend.ports, set())

    def test_cleanup_when_firewalld_is_stopped(self):
        self.apply([(443, 'tcp')])
        self.backend.active = False
        self.backend.ports = {rule for rule in self.backend.ports if rule[0]}
        self.apply([], revision=2)
        self.assertEqual(self.backend.ports, set())
        self.assertTrue(any(call[0] == 'firewall-offline-cmd' for call in self.backend.calls))

    def test_no_permissions_added_to_inactive_firewall(self):
        self.backend.active = False
        self.apply([(443, 'tcp')])
        self.assertEqual(self.backend.ports, set())
        self.assertEqual(self.state()['rules'], {})

    def test_recovery_fences_previous_installation(self):
        self.apply([(443, 'tcp')], generation=10, operation='previous')
        self.apply([], release=True)
        with self.assertRaisesRegex(ValueError, 'stale firewall installation'):
            self.apply([(443, 'tcp')], generation=10, operation='previous')
        self.apply([(8443, 'tcp')], generation=1, operation='restored', initialize=True)
        with self.assertRaisesRegex(ValueError, 'stale firewall installation'):
            self.apply([(443, 'tcp')], generation=10, operation='previous')
        self.assertEqual({port for _, _, port in self.backend.ports}, {'8443/tcp'})

    def test_ufw_preserves_external_and_shared_permissions(self):
        self.backend = Backend('ufw')
        self.backend.ufw['22/tcp'] = 'administrator'
        self.apply([(443, 'tcp')])
        self.apply([(443, 'tcp')], owner='s-ui-node-20')
        self.apply([], revision=2)
        self.assertIn('443/tcp', self.backend.ufw)
        self.apply([], owner='s-ui-node-20', revision=2)
        self.assertEqual(self.backend.ufw, {'22/tcp': 'administrator'})

    def test_ufw_existing_all_protocol_permission_is_external(self):
        self.backend = Backend('ufw')
        self.backend.ufw['443'] = 'administrator'
        self.apply([(443, 'tcp'), (443, 'udp')])
        self.apply([], revision=2)
        self.assertEqual(self.backend.ufw, {'443': 'administrator'})

    def test_ufw_administrator_can_take_over_a_managed_rule(self):
        self.backend = Backend('ufw')
        self.apply([(443, 'tcp')])
        self.backend.ufw['443/tcp'] = 'administrator'
        self.apply([], revision=2)
        self.assertEqual(self.backend.ufw, {'443/tcp': 'administrator'})

    def test_ownership_file_is_private(self):
        self.apply([(443, 'tcp')])
        self.assertEqual(stat.S_IMODE((self.directory / 'firewall-state.json').stat().st_mode), 0o600)

    def test_http01_survives_protocol_apply_and_has_independent_revision(self):
        self.apply([(443, 'tcp')], revision=10)
        self.apply([(80, 'tcp')], owner='s-ui-node-10-http01', revision=1)
        self.apply([(8443, 'tcp')], revision=11)
        self.assertEqual({port for _, _, port in self.backend.ports}, {'80/tcp', '8443/tcp'})
        self.apply([], revision=12)
        self.assertEqual({port for _, _, port in self.backend.ports}, {'80/tcp'})
        self.apply([], owner='s-ui-node-10-http01', revision=2)
        self.apply([(80, 'tcp')], owner='s-ui-node-10-http01', revision=1)
        self.assertEqual(self.backend.ports, set())

    def test_http01_shares_port_with_a_proxy_and_preserves_external_rules(self):
        self.apply([(80, 'tcp')])
        self.apply([(80, 'tcp')], owner='s-ui-node-10-http01')
        self.apply([], owner='s-ui-node-10-http01', revision=2)
        self.assertEqual({port for _, _, port in self.backend.ports}, {'80/tcp'})
        self.apply([], revision=2)
        for permanent in (False, True):
            for zone in self.backend.zones:
                self.backend.ports.add((permanent, zone, '80/tcp'))
        self.apply([(80, 'tcp')], owner='s-ui-node-10-http01', revision=3)
        self.apply([], owner='s-ui-node-10-http01', revision=4)
        self.assertEqual(len(self.backend.ports), 4)

    def test_http01_reenable_fences_late_disable_with_same_certificate(self):
        owner = 's-ui-node-10-http01'
        self.apply([(80, 'tcp')], owner=owner, policy=1)
        self.apply([], owner=owner, policy=2)
        self.apply([(80, 'tcp')], owner=owner, policy=3)
        self.apply([], owner=owner, policy=2)
        self.assertEqual({port for _, _, port in self.backend.ports}, {'80/tcp'})
        self.apply([], owner=owner, policy=4)
        self.apply([(80, 'tcp')], owner=owner, policy=3)
        self.assertEqual(self.backend.ports, set())

    def test_fencing_releases_http01_and_proxy_owners(self):
        self.apply([(443, 'tcp')])
        self.apply([(80, 'tcp')], owner='s-ui-node-10-http01')
        self.apply([(80, 'tcp')], owner='s-ui-node-20-http01')
        firewall.release_node_firewall('s-ui-node-10')
        self.assertEqual({port for _, _, port in self.backend.ports}, {'80/tcp'})
        firewall.release_node_firewall('s-ui-node-20')
        self.assertEqual(self.backend.ports, set())

    def test_http01_reinstallation_fences_old_bootstrap(self):
        owner = 's-ui-node-10-0123456789ab-http01'
        self.apply([(80, 'tcp')], owner=owner, generation=1, operation='old', revision=5)
        self.apply([], owner=owner, initialize=True, generation=2, operation='new', revision=0)
        with self.assertRaises(ValueError):
            self.apply([(80, 'tcp')], owner=owner, generation=1, operation='old', revision=6)
        self.assertEqual(self.backend.ports, set())
        self.apply([(80, 'tcp')], owner=owner, generation=2, operation='new', revision=5)
        self.assertEqual({port for _, _, port in self.backend.ports}, {'80/tcp'})


if __name__ == '__main__':
    unittest.main()
