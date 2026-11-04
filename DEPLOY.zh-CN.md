# 自定义安装与升级

此 Fork 在 `dev` 分支增加 `deploy.sh`，保留原作者代码和原有 `install.sh`。适用于 Debian/Ubuntu、RHEL 系列等使用 systemd 的 Linux 主机；其他系统请使用原作者安装方式。

## 使用

在 VPS 的 **root 终端**中执行一行安装命令：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/wuquanlin/s-ui/dev/deploy.sh)
```

按提示选择域名或 IP，并配置证书即可。

脚本会先检查依赖，只安装缺少的包；已安装的 `ca-certificates` 等依赖会保留，不因部署而触发升级。若缺少的依赖无法安装，部署会停止。

也可以先下载脚本，再以 root 权限运行：

```bash
curl -fL https://raw.githubusercontent.com/wuquanlin/s-ui/dev/deploy.sh -o /tmp/s-ui-deploy.sh
sudo bash /tmp/s-ui-deploy.sh
```

也可以使用 `git clone --branch dev https://github.com/wuquanlin/s-ui.git` 克隆 `dev` 分支后，以 root 运行 `bash deploy.sh --local`。

1. 每次执行都查询并下载 **alireza0/s-ui 最新稳定发行版**，检查官方 SHA256SUMS；下载或校验失败会停止，不使用旧版回退。
2. 显示面板、内置 sing-box、sing-box 上游最新稳定版的版本。新版 S-UI 使用内置 sing-box，遵循兼容性优先策略，使用 S-UI 官方配套内核；不会单独下载一个未被面板使用的内核。这里的“最新”指稳定发行版，不包括预发布和尚未发布的代码。
3. 选择域名或公网 IPv4/IPv6。域名不带 `https://`、端口或路径；IP 模式无需域名。脚本不会替你创建 DNS 记录、配置路由器端口转发或开放防火墙端口。
4. 自动搜索证书：`/` 直属文件，`/root`、`/home` 六层以内，以及 `/etc/letsencrypt`、`/etc/ssl`、`/usr/local/s-ui/cert`。仅扫描 `.pem/.crt/.cer/.key`，排除缓存等目录，不递归扫描整个根文件系统。
5. 自动识别同一目录内配对的 PEM 证书和未加密私钥，校验有效期、所选域名/IP 和公钥。只有一组时自动使用；多组时可选择。找不到时可输入路径、粘贴完整 PEM（证书与私钥分别以独占一行的 `END` 结束），或明确确认使用 HTTP。
6. 给面板和订阅服务设置同一组证书与访问地址，保留既有端口和路径，默认分别为 `2095/app/` 和 `2096/sub/`。输出的订阅地址只是前缀；完整用户订阅链接请在面板中复制。
7. **首次安装**设置账号 `master`、密码 `Change@me2025`。这是公开默认密码，首次登录后请修改。升级保留已有账号、密码、节点和用户。

证书必须适用于所选地址。普通域名证书通常无法用于直接访问 IP；IP 模式需要证书包含该 IP 的 SAN，或显式选择 HTTP。配对和有效期检查不等于浏览器信任验证：请提供浏览器信任的完整证书链。

自动发现和手动指定的证书保留原路径（包括续期软链接）；粘贴的证书保存到 `/usr/local/s-ui/cert/`，私钥权限为 `600`。脚本不申请证书，也不建立续期任务；证书续期后需按证书工具的部署钩子重启 S-UI。

## 备份与验证

升级前停止 S-UI，并将安装目录、数据库、服务文件和菜单脚本备份到 `/var/backups/s-ui/` 下的独立目录。目录仅 root 可访问。安装失败或启动检查失败时恢复原有文件和数据库；已有服务原本运行时会重新启动。备份会保留，需按自己的保留周期清理。

脚本会短暂停止现有 S-UI 服务。只支持官方 S-UI 的安装目录 `/usr/local/s-ui` 和服务名 `s-ui`；旧版独立 sing-box 服务或自定义 systemd 覆盖配置需要先单独迁移。当前版本通过临时目录验证了证书逻辑、配置写入、首次账号、升级保留账号和失败回滚。真实 systemd、DNS、防火墙与外网访问仍需在目标主机验收。

```bash
bash -n deploy.sh
python3 -m unittest discover -s tests -p 'test_deploy.py' -v
# 增加官方发行二进制的隔离配置/回滚测试（不启动 systemd 服务）：
SUI_TEST_BINARY=/absolute/path/to/sui python3 -m unittest discover -s tests -p 'test_deploy.py' -v
```

## 同步上游

自定义部署文件维护在 `dev` 分支，`main` 保留为上游同步分支。请使用本说明中的 `dev` 安装地址。

给 `dev` 同步原作者更新时，应合并上游提交并保留自定义修改；将 `dev` 强制重置为上游分支会移除这些文件，导致安装说明和一行安装命令返回 404。
