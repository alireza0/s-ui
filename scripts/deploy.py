#!/usr/bin/env python3
"""Interactive deployment using the newest official S-UI release and its embedded core."""
import fcntl
from contextlib import closing
import hashlib
import http.client
import ipaddress
import json
import os
from pathlib import Path
import platform
import re
import shutil
import sqlite3
import ssl
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request

INSTALL = Path('/usr/local/s-ui')
UNIT = Path('/etc/systemd/system/s-ui.service')
MENU = Path('/usr/bin/s-ui')
BACKUP_ROOT = Path('/var/backups/s-ui')
SEARCH_ROOTS = [('/', 1), ('/root', 6), ('/home', 6),
                ('/etc/letsencrypt', 6), ('/etc/ssl', 4), ('/usr/local/s-ui/cert', 2)]
SUFFIXES = {'.pem', '.crt', '.cer', '.key'}


def run(*args, data=None, env=None):
    # Do not echo arguments: the admin command includes a password.
    result = subprocess.run([str(a) for a in args], input=data, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, env=env, timeout=120)
    if result.returncode:
        raise RuntimeError(f'{Path(str(args[0])).name} 执行失败：'
                           + result.stderr.decode(errors='replace')[-1500:])
    return result.stdout


def download(url, destination):
    request = urllib.request.Request(url, headers={'User-Agent': 's-ui-custom-deploy'})
    for attempt in range(3):
        try:
            with urllib.request.urlopen(request, timeout=60) as response:
                with open(destination, 'wb') as output:
                    shutil.copyfileobj(response, output)
            return
        except (OSError, TimeoutError):
            if attempt == 2:
                raise
            time.sleep(1)


def latest_release(repo):
    request = urllib.request.Request(f'https://api.github.com/repos/{repo}/releases/latest',
                                     headers={'User-Agent': 's-ui-custom-deploy'})
    with urllib.request.urlopen(request, timeout=30) as response:
        release = json.load(response)
    if release.get('draft') or release.get('prerelease') or not release.get('tag_name'):
        raise RuntimeError(f'{repo} 未返回稳定发行版。')
    return release


def architecture(machine=None):
    machine = machine or platform.machine()
    names = {'x86_64': 'amd64', 'amd64': 'amd64', 'aarch64': 'arm64',
             'arm64': 'arm64', 'i386': '386', 'i686': '386', 'armv7l': 'armv7',
             'armv6l': 'armv6', 'armv5tel': 'armv5', 's390x': 's390x'}
    if machine not in names:
        raise ValueError(f'不支持的 CPU 架构：{machine}')
    return names[machine]


def prepare_release(folder):
    release = latest_release('alireza0/s-ui')
    core = latest_release('SagerNet/sing-box')
    name = f's-ui-linux-{architecture()}.tar.gz'
    assets = {a['name']: a['browser_download_url'] for a in release['assets']}
    if name not in assets:
        raise RuntimeError(f'最新 S-UI 没有提供 {name}，已停止。')
    archive = folder / name
    print(f'下载最新稳定版 S-UI：{release["tag_name"]}')
    download(assets[name], archive)
    # Refuse unverified binaries rather than silently using a previous release.
    sums = folder / 'SHA256SUMS'
    download(assets['SHA256SUMS'], sums)
    verify_checksum(archive, sums)
    # Read only the three expected regular files; never extract archive paths/links.
    with tarfile.open(archive) as tar:
        for name in ('sui', 's-ui.sh', 's-ui.service'):
            member = tar.getmember(f's-ui/{name}')
            if not member.isfile():
                raise RuntimeError(f'发行包中的 {name} 不是普通文件。')
            with tar.extractfile(member) as source, open(folder / name, 'wb') as output:
                shutil.copyfileobj(source, output)
    (folder / 'sui').chmod(0o700)
    version = run(folder / 'sui', '-v').decode().strip()
    if 'Sing-Box' not in version or 'S-UI Panel' not in version:
        raise RuntimeError('无法识别面板或内置内核版本，停止安装。')
    print(version)
    print(f'sing-box 上游最新稳定版：{core["tag_name"]}')
    if not re.search(r'Sing-Box\s+' + re.escape(core['tag_name']) + r'(?:\s|$)', version):
        print('提示：S-UI 内置内核与上游最新版本不同；按兼容性优先使用官方配套内核。')
    return release['tag_name']


def verify_checksum(archive, sums):
    expected = None
    for line in sums.read_text().splitlines():
        parts = line.split()
        if len(parts) == 2 and parts[1].lstrip('*') == archive.name:
            expected = parts[0].lower()
    if expected is None or hashlib.sha256(archive.read_bytes()).hexdigest() != expected:
        raise ValueError('发行包 SHA256 校验失败。')


def parse_host(value, mode):
    value = value.strip().strip('[]')
    if mode == '2':
        return str(ipaddress.ip_address(value))
    value = value.rstrip('.').encode('idna').decode('ascii').lower()
    try:
        ipaddress.ip_address(value)
    except ValueError:
        pass
    else:
        raise ValueError('这里需要域名；使用 IP 请选 2。')
    if len(value) > 253 or '.' not in value or any(
            not re.fullmatch(r'[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?', label)
            for label in value.split('.')):
        raise ValueError('请输入纯域名，不带协议、端口或路径。')
    return value


def choose_host():
    while True:
        mode = input('访问方式：1. 域名（默认）  2. IP：').strip() or '1'
        if mode not in ('1', '2'):
            continue
        try:
            return parse_host(input('请输入域名：' if mode == '1' else '请输入公网 IP：'), mode)
        except ValueError as error:
            print(error)


def validate_pair(cert, key, host):
    # Preserve live symlinks (e.g. Let's Encrypt); resolving pins an old renewal.
    cert, key = Path(os.path.abspath(cert)), Path(os.path.abspath(key))
    if not cert.is_file() or not key.is_file():
        raise ValueError('证书或私钥文件不存在。')
    cert_pub = run('openssl', 'x509', '-in', cert, '-pubkey', '-noout')
    # -passin avoids blocking on encrypted private keys during discovery.
    key_pub = run('openssl', 'pkey', '-in', key, '-passin', 'pass:', '-pubout')
    if cert_pub != key_pub:
        raise ValueError('证书与私钥不配对。')
    dates = run('openssl', 'x509', '-in', cert, '-noout', '-dates').decode().splitlines()
    dates = dict(line.split('=', 1) for line in dates)
    now = time.time()
    if not ssl.cert_time_to_seconds(dates['notBefore']) <= now < ssl.cert_time_to_seconds(dates['notAfter']):
        raise ValueError('证书已过期或尚未生效。')
    try:
        ipaddress.ip_address(host)
        flag = '-checkip'
    except ValueError:
        flag = '-checkhost'
    run('openssl', 'x509', '-in', cert, '-noout', flag, host)
    return cert, key


def discover_pairs(host, roots=None):
    groups, seen = {}, set()
    for root, depth in SEARCH_ROOTS if roots is None else roots:
        root = Path(root)
        if not root.is_dir():
            continue
        for directory, dirs, files in os.walk(root, followlinks=False):
            current = Path(directory)
            if len(current.relative_to(root).parts) >= depth - 1:
                dirs[:] = []
            dirs[:] = sorted(d for d in dirs if d not in ('.cache', '.git', 'node_modules'))
            for filename in sorted(files):
                path = current / filename
                if path.suffix.lower() not in SUFFIXES or path in seen:
                    continue
                seen.add(path)
                try:
                    if not 0 < path.stat().st_size <= 2 * 1024 * 1024:
                        continue
                    data = path.read_bytes()
                    certs, keys = groups.setdefault(current, ([], []))
                    if b'-----BEGIN CERTIFICATE-----' in data:
                        certs.append(path)
                    if re.search(rb'-----BEGIN (?:RSA |EC |)PRIVATE KEY-----', data):
                        keys.append(path)
                except OSError:
                    continue
    pairs = []
    unique = set()
    for certs, keys in groups.values():
        # Prefer a chain file to the equivalent leaf-only certificate.
        for cert in sorted(certs, key=lambda p: ('fullchain' not in p.name.lower(), str(p))):
            for key in keys:
                try:
                    pair = validate_pair(cert, key, host)
                    fingerprint = run('openssl', 'x509', '-in', cert, '-noout', '-fingerprint', '-sha256')
                    if fingerprint not in unique:
                        unique.add(fingerprint)
                        pairs.append(pair)
                except (ValueError, RuntimeError, OSError, subprocess.TimeoutExpired):
                    continue
    return pairs


def read_pem(label):
    print(f'粘贴{label} PEM 内容，单独输入 END 结束：')
    lines = []
    while True:
        line = input()
        if line == 'END':
            return ('\n'.join(lines) + '\n').encode()
        lines.append(line)


def choose_certificate(host, folder):
    print('搜索 /、/root、/home 及常用证书目录（/ 仅检查直属文件）……')
    pairs = discover_pairs(host)
    if len(pairs) == 1:
        print(f'自动使用匹配证书：{pairs[0][0]}')
        return pairs[0]
    if pairs:
        for i, (cert, _) in enumerate(pairs, 1):
            print(f'{i}. {cert}')
        while True:
            value = input('选择证书编号，或回车改为手动输入：').strip()
            if not value:
                break
            if value.isdigit() and 1 <= int(value) <= len(pairs):
                return pairs[int(value) - 1]
    while True:
        mode = input('证书设置：1. 输入文件路径  2. 粘贴 PEM  3. 使用 HTTP：').strip()
        try:
            if mode == '1':
                cert = Path(input('证书路径（建议 fullchain.pem）：').strip()).expanduser()
                key = Path(input('私钥路径：').strip()).expanduser()
                return validate_pair(cert, key, host)
            if mode == '2':
                cert, key = folder / 'pasted-cert.pem', folder / 'pasted-key.pem'
                cert.write_bytes(read_pem('证书（可含完整证书链）'))
                key.write_bytes(read_pem('未加密私钥'))
                key.chmod(0o600)
                return validate_pair(cert, key, host)
            if mode == '3':
                if input('HTTP 不加密登录信息，输入 HTTP 确认：').strip() == 'HTTP':
                    return None
        except (ValueError, RuntimeError, OSError) as error:
            print(f'证书不可用：{error}')


def settings_for(host, pair):
    scheme = 'https' if pair else 'http'
    host_url = f'[{host}]' if ':' in host else host
    # Leave domain filters empty in IP mode; binding to a public NAT address can fail.
    try:
        ipaddress.ip_address(host)
        domain = ''
    except ValueError:
        domain = host
    settings = {'webDomain': domain, 'subDomain': domain,
                'webListen': '', 'subListen': '',
                'webCertFile': str(pair[0]) if pair else '',
                'webKeyFile': str(pair[1]) if pair else '',
                'subCertFile': str(pair[0]) if pair else '',
                'subKeyFile': str(pair[1]) if pair else ''}
    return settings, scheme, host_url


def write_settings(db_path, settings):
    with closing(sqlite3.connect(db_path)) as db, db:
        existing = {row[0] for row in db.execute('SELECT key FROM settings')}
        if not settings.keys() <= existing:
            raise RuntimeError('上游设置结构已变化，停止安装。')
        for key, value in settings.items():
            db.execute('UPDATE settings SET value = ? WHERE key = ?', (value, key))


def check_endpoints(settings, host, pair):
    # Certificate identity/validity was checked before changing the host. This
    # probe checks both real listeners, including TLS, without requiring public DNS
    # or a particular CA (the operator may intentionally use a private CA).
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE
    for prefix in ('web', 'sub'):
        for address in ('127.0.0.1', '::1'):
            connection = http.client.HTTPConnection(address, int(settings[prefix + 'Port']), timeout=2)
            try:
                connection.connect()
                if pair:
                    connection.sock = context.wrap_socket(connection.sock, server_hostname=host)
                authority = f'[{host}]' if ':' in host else host
                connection.request('GET', settings[prefix + 'Path'],
                                   headers={'Host': f'{authority}:{settings[prefix + "Port"]}'})
                response = connection.getresponse()
                if response.status >= 500:
                    raise RuntimeError('服务返回错误状态。')
                break
            except (OSError, http.client.HTTPException, RuntimeError):
                if address == '::1':
                    raise RuntimeError(f'{prefix} 服务 HTTP/TLS 检查失败。') from None
            finally:
                connection.close()


def deploy(folder, host, pair):
    fresh = not (INSTALL / 'db/s-ui.db').exists()
    was_active = subprocess.run(['systemctl', 'is-active', '--quiet', 's-ui']).returncode == 0
    was_enabled = subprocess.run(['systemctl', 'is-enabled', '--quiet', 's-ui'],
                                 stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0
    backup_root = BACKUP_ROOT
    backup_root.mkdir(parents=True, exist_ok=True, mode=0o700)
    backup = Path(tempfile.mkdtemp(prefix=time.strftime('%Y%m%d-%H%M%S-'), dir=backup_root))
    print(f'部署备份：{backup}')
    if was_active:
        run('systemctl', 'stop', 's-ui')
    originals = [(INSTALL, backup / 'installation'), (UNIT, backup / 'service'), (MENU, backup / 'menu')]
    try:
        for source, dest in originals:
            if source.is_dir():
                shutil.copytree(source, dest, symlinks=True)
            elif source.exists():
                shutil.copy2(source, dest)
    except BaseException:
        if was_active:
            run('systemctl', 'start', 's-ui')
        raise
    try:
        INSTALL.mkdir(parents=True, exist_ok=True)
        shutil.copy2(folder / 'sui', INSTALL / 'sui')
        (INSTALL / 'sui').chmod(0o755)
        shutil.copy2(folder / 's-ui.sh', MENU)
        MENU.chmod(0o755)
        shutil.copy2(folder / 's-ui.service', UNIT)
        db_path = INSTALL / 'db/s-ui.db'
        # Explicit DB location, independent of the calling shell's environment.
        env = dict(os.environ, SUI_DB_FOLDER=str(INSTALL / 'db'))
        run(INSTALL / 'sui', 'migrate', env=env)
        run(INSTALL / 'sui', 'setting', '-show', env=env)
        if pair:
            if str(pair[0]).startswith(str(folder) + '/'):
                cert_dir = INSTALL / 'cert'
                cert_dir.mkdir(mode=0o700, exist_ok=True)
                saved = cert_dir / 'panel-fullchain.pem', cert_dir / 'panel-key.pem'
                for source, destination in zip(pair, saved):
                    shutil.copy2(source, destination)
                    destination.chmod(0o600)
                pair = saved
            # Discovered/path certificates stay at their original paths for renewal.
        settings, scheme, host_url = settings_for(host, pair)
        with closing(sqlite3.connect(db_path)) as db:
            old = dict(db.execute('SELECT key, value FROM settings'))
        settings['webURI'] = f'{scheme}://{host_url}:{old["webPort"]}{old["webPath"]}'
        settings['subURI'] = f'{scheme}://{host_url}:{old["subPort"]}{old["subPath"]}'
        write_settings(db_path, settings)
        if fresh:
            run(INSTALL / 'sui', 'admin', '-username', 'master', '-password', 'Change@me2025', env=env)
            with closing(sqlite3.connect(db_path)) as db:
                if db.execute('SELECT username FROM users ORDER BY id LIMIT 1').fetchone() != ('master',):
                    raise RuntimeError('默认管理员设置失败。')
        run('systemctl', 'daemon-reload')
        run('systemctl', 'enable', 's-ui')
        run('systemctl', 'restart', 's-ui')
        for attempt in range(30):
            try:
                run(INSTALL / 'sui', 'healthcheck', env=env)
                check_endpoints(dict(old, **settings), host, pair)
                break
            except RuntimeError:
                if attempt == 29:
                    raise RuntimeError('面板启动检查失败。')
                time.sleep(1)
        print(f'面板地址：{settings["webURI"]}')
        print(f'订阅地址前缀：{settings["subURI"]}（完整订阅链接请在面板中复制）')
        print('首次安装账号：master，密码：Change@me2025；登录后请修改。' if fresh else '升级完成，原账号密码已保留。')
    except BaseException:
        print('部署失败，恢复安装前的文件和数据库……', file=sys.stderr)
        subprocess.run(['systemctl', 'stop', 's-ui'], stdout=subprocess.DEVNULL,
                       stderr=subprocess.DEVNULL)
        if not was_enabled:
            # Disable while the new unit still exists; a fresh failed install
            # has no previous unit to restore.
            subprocess.run(['systemctl', 'disable', 's-ui'], stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL)
        for destination, source in originals:
            if destination.is_dir():
                shutil.rmtree(destination)
            elif destination.exists():
                destination.unlink()
            if source.is_dir():
                shutil.copytree(source, destination, symlinks=True)
            elif source.exists():
                shutil.copy2(source, destination)
        run('systemctl', 'daemon-reload')
        if was_active:
            run('systemctl', 'start', 's-ui')
        raise


def main():
    if platform.system() != 'Linux' or os.geteuid() != 0 or not Path('/run/systemd/system').is_dir():
        raise RuntimeError('请在使用 systemd 的 Linux 主机上以 root 运行 deploy.sh。')
    os.umask(0o077)
    with open('/run/s-ui-deploy.lock', 'w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        with tempfile.TemporaryDirectory(prefix='s-ui-deploy-') as temporary:
            folder = Path(temporary)
            prepare_release(folder)
            host = choose_host()
            pair = choose_certificate(host, folder)
            deploy(folder, host, pair)


if __name__ == '__main__':
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        print(f'安装未完成：{error}', file=sys.stderr)
        sys.exit(1)
