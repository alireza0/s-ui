"""Manage only firewall permissions created by S-UI node provisioning."""

import fcntl
import json
import os
import re
import shlex
import shutil
import subprocess
from pathlib import Path


FIREWALL_STATE_DIR = Path('/etc/s-ui-nodes')
FIREWALL_COMMENT = 's-ui-managed-nodes'


def _firewall_run(arguments, check=True):
    return subprocess.run(arguments, check=check, capture_output=True, text=True,
                          timeout=15, env=dict(os.environ, LC_ALL='C'))


def _firewall_save(state):
    path = FIREWALL_STATE_DIR / 'firewall-state.json'
    temporary = path.with_suffix('.json.new')
    descriptor = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW, 0o600)
    os.fchmod(descriptor, 0o600)
    with os.fdopen(descriptor, 'w') as stream:
        json.dump(state, stream, sort_keys=True)
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(temporary, path)
    descriptor = os.open(FIREWALL_STATE_DIR, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def _firewall_load():
    path = FIREWALL_STATE_DIR / 'firewall-state.json'
    try:
        descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    except FileNotFoundError:
        return {'version': 1, 'owners': {}, 'rules': {}}
    with os.fdopen(descriptor) as stream:
        state = json.load(stream)
    if state.get('version') != 1 or not isinstance(state.get('owners'), dict) or not isinstance(state.get('rules'), dict):
        raise ValueError('invalid firewall ownership state')
    for key, rule in state['rules'].items():
        _validate_firewall_rule(key, rule)
    return state


def _validate_firewall_rule(key, rule):
    if not isinstance(rule, dict) or not isinstance(rule.get('port'), int) or isinstance(rule['port'], bool):
        raise ValueError('invalid tracked firewall port')
    if not isinstance(rule.get('permanent'), bool) or not isinstance(rule.get('owned'), bool):
        raise ValueError('invalid tracked firewall ownership')
    if key != _firewall_key(rule):
        raise ValueError('invalid tracked firewall identity')
    zone = rule.get('zone')
    if not isinstance(zone, str) or (rule['backend'] == 'ufw' and zone != ''):
        raise ValueError('invalid tracked firewall zone')
    if rule['backend'] == 'firewalld':
        _firewall_zone(zone)


def _firewall_backends():
    ufw = shutil.which('ufw')
    firewall = shutil.which('firewall-cmd')
    ufw_active = bool(ufw and 'Status: active' in _firewall_run([ufw, 'status']).stdout)
    firewalld_active = bool(firewall and _firewall_run([firewall, '--state'], check=False).returncode == 0)
    zones = []
    if firewalld_active:
        zones = [line.split()[0] for line in _firewall_run([firewall, '--get-active-zones']).stdout.splitlines()
                 if line and not line[0].isspace()]
        if not zones:
            zones = [_firewall_run([firewall, '--get-default-zone']).stdout.strip()]
    if any(not re.fullmatch(r'[A-Za-z0-9_.-]+', zone) for zone in zones):
        raise ValueError('invalid firewall zone')
    return {'ufw': ufw, 'firewall': firewall, 'offline': shutil.which('firewall-offline-cmd'),
            'ufw_active': ufw_active, 'firewalld_active': firewalld_active, 'zones': zones}


def _firewall_key(rule):
    if rule['backend'] not in ('ufw', 'firewalld') or rule['network'] not in ('tcp', 'udp') or not 1 <= rule['port'] <= 65535:
        raise ValueError('invalid tracked firewall rule')
    return '/'.join((rule['backend'], rule['zone'], str(int(rule['permanent'])),
                     str(rule['port']), rule['network']))


def _ufw_permission(rule, command):
    endpoint = _firewall_endpoint(rule)
    for line in _firewall_run([command, 'show', 'added']).stdout.splitlines():
        parts = shlex.split(line)
        if parts[:2] != ['ufw', 'allow']:
            continue
        parts = parts[2:]
        if parts[:1] == ['in']:
            parts = parts[1:]
        if parts and parts[0] in (endpoint, str(rule['port'])):
            comment = parts[parts.index('comment') + 1] if 'comment' in parts else ''
            return True, comment == FIREWALL_COMMENT
    return False, False


def _firewalld_prefix(rule, backends):
    zone = _firewall_zone(rule['zone'])
    if backends['firewalld_active']:
        return [backends['firewall'], *(['--permanent'] if rule['permanent'] else []), '--zone=' + zone]
    if rule['permanent'] and backends['offline']:
        return [backends['offline'], '--zone=' + zone]
    if rule['permanent']:
        raise RuntimeError('permanent firewall cleanup unavailable')
    return None


def _firewall_permission(rule, backends):
    if rule['backend'] == 'ufw':
        if not backends['ufw']:
            raise RuntimeError('UFW cleanup unavailable')
        return _ufw_permission(rule, backends['ufw'])
    prefix = _firewalld_prefix(rule, backends)
    if prefix is None:
        return False, True
    endpoint = _firewall_endpoint(rule)
    query = _firewall_run([*prefix, '--query-port=' + endpoint], check=False)
    if query.returncode not in (0, 1):
        raise RuntimeError('firewall port query failed')
    return query.returncode == 0, True


def _firewall_change(rule, backends, add):
    endpoint = _firewall_endpoint(rule)
    if rule['backend'] == 'ufw':
        if add:
            arguments = [backends['ufw'], 'allow', endpoint, 'comment', FIREWALL_COMMENT]
        else:
            arguments = [backends['ufw'], '--force', 'delete', 'allow', endpoint]
        _firewall_run(arguments)
        return
    prefix = _firewalld_prefix(rule, backends)
    if prefix is not None:
        action = '--add-port=' if add else '--remove-port='
        _firewall_run([*prefix, action + endpoint])


def _firewall_zone(zone):
    if not isinstance(zone, str) or not re.fullmatch(r'[A-Za-z0-9_.-]+', zone):
        raise ValueError('invalid firewall zone')
    return zone


def _firewall_endpoint(rule):
    port = int(rule['port'])
    if not 1 <= port <= 65535 or rule['network'] not in ('tcp', 'udp'):
        raise ValueError('invalid firewall endpoint')
    network = 'tcp' if rule['network'] == 'tcp' else 'udp'
    return str(port) + '/' + network


def _active_firewall_candidates(endpoints, backends):
    candidates = {}
    for port, network in endpoints:
        if backends['ufw_active']:
            rule = {'backend': 'ufw', 'zone': '', 'permanent': True, 'port': port, 'network': network}
            candidates[_firewall_key(rule)] = rule
        if backends['firewalld_active']:
            for zone in backends['zones']:
                for permanent in (False, True):
                    rule = {'backend': 'firewalld', 'zone': zone, 'permanent': permanent, 'port': port, 'network': network}
                    candidates[_firewall_key(rule)] = rule
    return candidates


def _firewall_candidates(endpoints, backends, previous, rules):
    candidates = _active_firewall_candidates(endpoints, backends)
    for key in previous:
        rule = rules[key]
        active = backends['ufw_active'] if rule['backend'] == 'ufw' else backends['firewalld_active']
        if not active and (rule['port'], rule['network']) in endpoints:
            candidates[key] = rule
    return candidates


def _firewall_collect(state, backends):
    referenced = {key for owner in state['owners'].values() for key in owner['rules'] + owner['pending']}
    # Snapshot keys because each successfully removed rule is deleted and saved.
    for key in tuple(state['rules']):
        rule = state['rules'][key]
        if key in referenced:
            continue
        if rule['owned']:
            exists, managed = _firewall_permission(rule, backends)
            if exists and managed:
                _firewall_change(rule, backends, False)
        del state['rules'][key]
        _firewall_save(state)


def configure_node_firewall(listeners, name, confirmed=False, version=None, release=False, initialize=False):
    if not re.fullmatch(r's-ui-node-[1-9]\d*(?:-[a-f0-9]{12})?(?:-http01)?', name, flags=re.ASCII):
        raise ValueError('invalid firewall owner')
    endpoints = {(int(row['port']), row['network']) for row in listeners}
    if any(not 1 <= port <= 65535 or network not in ('tcp', 'udp') for port, network in endpoints):
        raise ValueError('invalid firewall listener')
    if FIREWALL_STATE_DIR.is_symlink():
        raise ValueError('unsafe firewall state directory')
    FIREWALL_STATE_DIR.mkdir(mode=0o755, parents=True, exist_ok=True)
    descriptor = os.open(FIREWALL_STATE_DIR / '.firewall.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, 'r+') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        _configure_firewall_locked(endpoints, name, confirmed, version or {}, release, initialize)


def release_node_firewall(name):
    configure_node_firewall([], name + '-http01', release=True)
    configure_node_firewall([], name, release=True)


def _firewall_owner(state, name, version, release, initialize):
    generation = version.get('generation', 0)
    revision = version.get('revision', 0)
    policy = version.get('policy', 0)
    operation = version.get('operation', '')
    owner = state['owners'].get(name, {'generation': generation, 'revision': 0,
                                     'operation': operation, 'rules': [], 'pending': []})
    if initialize:
        owner.update(generation=generation, revision=0, policy=0, operation=operation, released=False)
    if not release and (owner.get('released') or owner['operation'] != operation):
        raise ValueError('stale firewall installation')
    if not release and generation < owner['generation']:
        raise ValueError('stale firewall generation')
    if generation > owner['generation']:
        owner['revision'] = 0
        owner['policy'] = 0
    if not release and (revision < owner['revision'] or policy < owner.get('policy', 0)):
        return None
    owner['generation'] = generation
    return owner


def _configure_firewall_locked(endpoints, name, confirmed, version, release, initialize):
    state = _firewall_load()
    owner = _firewall_owner(state, name, version, release, initialize)
    if owner is None:
        return
    backends = _firewall_backends()
    candidates = {} if release else _firewall_candidates(endpoints, backends, owner['rules'] + owner['pending'], state['rules'])
    owner['pending'] = sorted(set(owner['pending']) | set(candidates))
    state['owners'][name] = owner
    additions = []
    for key, rule in candidates.items():
        exists, managed = _firewall_permission(rule, backends)
        tracked = state['rules'].get(key)
        owned = not exists or (tracked is not None and tracked['owned'] and managed)
        state['rules'][key] = {**rule, 'owned': owned}
        if not exists:
            additions.append(rule)
    # Persist all references before mutations, including commands which may
    # complete just before the SSH connection or the process is interrupted.
    _firewall_save(state)
    for rule in additions:
        _firewall_change(rule, backends, True)
    if confirmed or release:
        owner['rules'] = sorted(candidates)
        owner['pending'] = []
        owner['revision'] = version.get('revision', 0)
        owner['policy'] = version.get('policy', 0)
    if release:
        owner['released'] = True
    _firewall_save(state)
    _firewall_collect(state, backends)
