# S-UI

**Custom deployment (`dev`):** This branch provides the custom installer described below. `main` follows upstream. [中文安装说明](DEPLOY.zh-CN.md).

**An Advanced Web Panel • Built on SagerNet/Sing-Box**

![](https://img.shields.io/github/v/release/alireza0/s-ui.svg)
![S-UI Docker pull](https://img.shields.io/docker/pulls/alireza7/s-ui.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/alireza0/s-ui)](https://goreportcard.com/report/github.com/alireza0/s-ui)
[![Downloads](https://img.shields.io/github/downloads/alireza0/s-ui/total.svg)](https://img.shields.io/github/downloads/alireza0/s-ui/total.svg)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

> **Disclaimer:** This project is only for personal learning and communication, please do not use it for illegal purposes, please do not use it in a production environment

**If you think this project is helpful to you, you may wish to give a**:star2:

**Want to contribute?** See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding conventions, testing, and the pull request process.

[!["Buy Me A Coffee"](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://www.buymeacoffee.com/alireza7)

<a href="https://nowpayments.io/donation/alireza7" target="_blank" rel="noreferrer noopener">
   <img src="https://nowpayments.io/images/embeds/donation-button-white.svg" alt="Crypto donation button by NOWPayments">
</a>

## Quick Overview
| Features                               |      Enable?       |
| -------------------------------------- | :----------------: |
| Multi-Protocol                         | :heavy_check_mark: |
| Multi-Language                         | :heavy_check_mark: |
| Multi-Client/Inbound                   | :heavy_check_mark: |
| Advanced Traffic Routing Interface     | :heavy_check_mark: |
| Client & Traffic & System Status       | :heavy_check_mark: |
| Subscription Link (link/json/clash + info)| :heavy_check_mark: |
| Dark/Light Theme                       | :heavy_check_mark: |
| API Interface                          | :heavy_check_mark: |

## Supported Platforms

The table below describes upstream S-UI support. This fork's custom installer requires **Linux with systemd**.

| Platform | Architecture | Status |
|----------|--------------|---------|
| Linux    | amd64, arm64, armv7, armv6, armv5, 386, s390x | ✅ Supported |
| Windows  | amd64, 386, arm64 | ✅ Supported |
| macOS    | amd64, arm64 | 🚧 Experimental |

## Screenshots

!["Main"](https://github.com/alireza0/s-ui-frontend/raw/main/media/main.png)

[Other UI Screenshots](https://github.com/alireza0/s-ui-frontend/blob/main/screenshots.md)

## Documentation

Full documentation lives in the [Wiki](https://github.com/alireza0/s-ui/wiki):

| Page | Contents |
|------|----------|
| [Subscription Service](https://github.com/alireza0/s-ui/wiki/Subscription-Service) | Subscription URLs, the three formats, response headers |
| [Subscription JSON Template](https://github.com/alireza0/s-ui/wiki/Subscription-JSON-Template) | Structure and supported keys of the sing-box subscription template |
| [Subscription Clash Template](https://github.com/alireza0/s-ui/wiki/Subscription-Clash-Template) | The Clash.Meta template, proxy groups and filters |
| [API Documentation](https://github.com/alireza0/s-ui/wiki/API-Documentation) | The token-authenticated REST API (`/apiv2`) |
| [Configuration Objects](https://github.com/alireza0/s-ui/wiki/Configuration-Objects) | Shape of the objects read and written through the API |
| [Settings Reference](https://github.com/alireza0/s-ui/wiki/Settings-Reference) | Every panel setting and its default |

## Default Installation Information

For a **fresh installation with this fork's `deploy.sh`**:

| Setting | Default |
|---------|---------|
| Panel port | `2095` |
| Panel path | `/app/` |
| Subscription port | `2096` |
| Subscription path | `/sub/` |
| Username | `master` |
| Password | `Change@me2025` |

Change the public default password after your first login. Upgrades keep your existing account, password, ports, paths, users, and nodes.

## Install & Upgrade to Latest Version

### Linux with systemd — custom installer

Run this one-line command in a **root Bash terminal**:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/wuquanlin/s-ui/dev/deploy.sh)
```

The custom installer supports Linux hosts using systemd, including Debian/Ubuntu and RHEL-family distributions. Its interactive prompts are currently in Simplified Chinese.

During installation or upgrade, it:

1. Downloads the latest **stable official S-UI release** from `alireza0/s-ui` and verifies its published SHA256 checksum. It stops if downloading or verification fails.
2. Uses the sing-box core embedded in that official S-UI release for compatibility. It displays both the embedded version and the latest upstream stable sing-box version.
3. Lets you choose a domain name, public IPv4 address, or public IPv6 address.
4. Searches for matching PEM certificates and unencrypted private keys in the top level of `/`, under `/root` and `/home`, and in common certificate directories. It checks the key pair, validity dates, and the chosen domain or IP.
5. Automatically uses a single matching certificate pair, or lets you choose when several match. You can also enter certificate/key paths, paste their PEM contents, or explicitly confirm plain HTTP.
6. Applies the certificate pair and public address to **both the panel and subscription service**.
7. Backs up the installation, database, service file, and menu script before changing them. It restores them if deployment or the startup HTTP/TLS checks fail.

For HTTPS access by IP, the certificate must include that IP in its Subject Alternative Name. Certificate discovery does not issue new certificates or create renewal jobs. Existing certificate paths and renewal symlinks are preserved; pasted certificates are stored under `/usr/local/s-ui/cert/` with the private key restricted to root. Browser trust also requires an appropriate full certificate chain.

Backups are kept under `/var/backups/s-ui/`. The installer does not create DNS records, open firewall ports, or configure port forwarding. Existing S-UI service is briefly stopped during an upgrade.

To run the checked-out installer instead:

```bash
git clone --branch dev https://github.com/wuquanlin/s-ui.git
cd s-ui
bash deploy.sh --local
```

The subscription address printed at the end is a prefix; copy each user's complete subscription URL from the panel. See the [Chinese deployment guide](DEPLOY.zh-CN.md) for directory search limits, certificate input, backups, and verification details.

### Other platforms and upstream installation methods

For macOS, Alpine/OpenRC, Windows, and the upstream installer language selector, use the [original project's installation instructions](https://github.com/alireza0/s-ui#install--upgrade-to-latest-version). Those methods use their own setup flows and defaults. This fork's account and certificate configuration is applied by `deploy.sh`.

## Keeping dev up to date

Keep custom deployment changes on `dev` and merge upstream updates into it. `main` is used for upstream synchronization. Force-resetting `dev` to upstream removes the custom files and breaks their installation and documentation URLs.

## Install legacy Version (upstream)

**Step 1:** To install your desired legacy version, add the version to the end of the installation command. e.g., ver `v1.5.0`:

```sh
VERSION=v1.5.0 && bash <(curl -Ls https://raw.githubusercontent.com/alireza0/s-ui/$VERSION/install.sh) $VERSION
```

## Manual installation (upstream)

The steps below install official release files directly. Use the custom installer above to apply this fork's account, certificate, and address setup.

### Linux/macOS
1. Get the latest version of S-UI based on your OS/Architecture from GitHub: [https://github.com/alireza0/s-ui/releases/latest](https://github.com/alireza0/s-ui/releases/latest)
2. **OPTIONAL** Get the latest version of `s-ui.sh` [https://raw.githubusercontent.com/alireza0/s-ui/master/s-ui.sh](https://raw.githubusercontent.com/alireza0/s-ui/master/s-ui.sh)
3. **OPTIONAL** Copy `s-ui.sh` to /usr/bin/ and run `chmod +x /usr/bin/s-ui`.
4. Extract s-ui tar.gz file to a directory of your choice and navigate to the directory where you extracted the tar.gz file.
5. Copy *.service files to /etc/systemd/system/ and run `systemctl daemon-reload`.
6. Enable autostart and start S-UI service using `systemctl enable s-ui --now`

### Windows
1. Get the latest Windows version from GitHub: [https://github.com/alireza0/s-ui/releases/latest](https://github.com/alireza0/s-ui/releases/latest)
2. Download the appropriate Windows package (e.g., `s-ui-windows-amd64.zip`)
3. Extract the ZIP file to a directory of your choice
4. Run `install-windows.bat` as Administrator
5. Follow the installation wizard
6. Access the panel at http://localhost:2095/app

## Uninstall S-UI

### systemd
```sh
sudo -i

systemctl disable s-ui  --now

rm -f /etc/systemd/system/sing-box.service
systemctl daemon-reload

rm -fr /usr/local/s-ui
rm /usr/bin/s-ui
```

### Alpine (OpenRC)
```sh
rc-service s-ui stop
rc-update del s-ui default
rm -f /etc/init.d/s-ui

rm -fr /usr/local/s-ui
rm /usr/bin/s-ui
```

## Install using Docker (upstream)

The Docker instructions use the upstream image and its defaults. The custom `deploy.sh` flow runs on systemd hosts.

<details>
   <summary>Click for details</summary>

### Usage

**Step 1:** Install Docker

```shell
curl -fsSL https://get.docker.com | sh
```

**Step 2:** Install S-UI

> Docker compose method

```shell
mkdir s-ui && cd s-ui
wget -q https://raw.githubusercontent.com/alireza0/s-ui/master/docker-compose.yml
docker compose up -d
```

> Use docker

```shell
mkdir s-ui && cd s-ui
docker run -itd \
    -p 2095:2095 -p 2096:2096 -p 443:443 -p 80:80 \
    -v $PWD/db/:/app/db/ \
    -v $PWD/cert/:/root/cert/ \
    --name s-ui --restart=unless-stopped \
    alireza7/s-ui:latest
```

> Build your own image

```shell
git clone https://github.com/alireza0/s-ui
git submodule update --init --recursive
docker build -t s-ui .
```

</details>

## Manual run ( contribution )

<details>
   <summary>Click for details</summary>

### Build and run whole project
```shell
./runSUI.sh
```

### Clone the repository
```shell
# clone repository
git clone https://github.com/alireza0/s-ui
# clone submodules
git submodule update --init --recursive
```


### - Frontend

Visit [s-ui-frontend](https://github.com/alireza0/s-ui-frontend) for frontend code

### - Backend
> Please build frontend once before!

To build backend:
```shell
# remove old frontend compiled files
rm -fr web/html/*
# apply new frontend compiled files
cp -R frontend/dist/ web/html/
# build
go build -o sui main.go
```

To run backend (from root folder of repository):
```shell
./sui
```

</details>

## Languages

- English
- Farsi
- Vietnamese
- Chinese (Simplified)
- Chinese (Traditional)
- Russian

## Features

- Supported protocols:
  - General:  Mixed, SOCKS, HTTP, HTTPS, Direct, Redirect, TProxy
  - V2Ray based: VLESS, VMess, Trojan, Shadowsocks
  - Other protocols: ShadowTLS, Hysteria, Hysteria2, Naive, TUIC
- Supports XTLS protocols
- An advanced interface for routing traffic, incorporating PROXY Protocol, External, and Transparent Proxy, SSL Certificate, and Port
- An advanced interface for inbound and outbound configuration
- Clients’ traffic cap and expiration date
- Displays online clients, inbounds and outbounds with traffic statistics, and system status monitoring
- Subscription service with ability to add external links and subscription
- HTTPS for secure access to the web panel and subscription service (self-provided domain + SSL certificate)
- Dark/Light theme

## Environment Variables

<details>
  <summary>Click for details</summary>

### Usage

| Variable       |                      Type                      | Default       |
| -------------- | :--------------------------------------------: | :------------ |
| SUI_LOG_LEVEL  | `"debug"` \| `"info"` \| `"warn"` \| `"error"` | `"info"`      |
| SUI_DEBUG      |                   `boolean`                    | `false`       |
| SUI_BIN_FOLDER |                    `string`                    | `"bin"`       |
| SUI_DB_FOLDER  |                    `string`                    | `"db"`        |
| SINGBOX_API    |                    `string`                    | -             |

</details>

## SSL Certificate

The custom installer can discover an existing certificate pair, accept file paths, or accept pasted PEM contents, and configure both the panel and subscription service. The optional Certbot example below is an upstream certificate issuance method.

<details>
  <summary>Click for details</summary>

### Certbot

```bash
snap install core; snap refresh core
snap install --classic certbot
ln -s /snap/bin/certbot /usr/bin/certbot

certbot certonly --standalone --register-unsafely-without-email --non-interactive --agree-tos -d <Your Domain Name>
```

</details>

## Third-party Projects

Community-made projects built around S-UI. These are not affiliated with or maintained by S-UI — use them at your own discretion:

- [itning/reset-s-ui-traffic](https://github.com/itning/reset-s-ui-traffic) — periodic traffic reset for all users
- [zqh2333/s-ui-traffic-reset](https://github.com/zqh2333/s-ui-traffic-reset) — traffic reset tool
- [Sownix21/SUI-Bot](https://github.com/Sownix21/SUI-Bot) - telegram bot
- [Android App](https://github.com/Sownix21/SUI-Node) - multi panel controller

> Building something on top of S-UI (a Telegram bot, monitoring, automation, ...)? Open an issue/PR to get it listed here.

## Star History

<a href="https://www.star-history.com/?repos=alireza0%2Fs-ui&type=date&releases=&legend=bottom-right">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=alireza0/s-ui&type=date&theme=dark&legend=bottom-right" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=alireza0/s-ui&type=date&legend=bottom-right" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=alireza0/s-ui&type=date&legend=bottom-right" />
 </picture>
</a>