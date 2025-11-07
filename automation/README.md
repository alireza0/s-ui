# S-UI 自动化部署方案

## 概述

这是一套完整的 S-UI 自动化部署解决方案，可以帮助您在多台服务器上批量安装 S-UI 并应用统一的配置，最后自动获取每台服务器的订阅链接。

## 功能特性

✅ **批量自动化安装** - 支持通过 SSH 在多台服务器上自动安装 S-UI
✅ **统一配置管理** - 使用模板配置，一次定义，多处应用
✅ **自动密钥生成** - 自动生成 Reality 密钥对、UUID、端口等
✅ **订阅链接收集** - 自动收集并整理所有服务器的订阅链接
✅ **安全密码管理** - 支持自动修改默认密码
✅ **详细日志记录** - 记录完整的部署过程和结果
✅ **API 封装** - 提供便捷的 API 调用函数库

## 目录结构

```
automation/
├── deploy.sh                    # 主部署脚本
├── generate_configs.sh          # 配置生成向导
├── api_helper.sh                # API 辅助函数库
├── configs/
│   ├── tls_config.json         # TLS/Reality 配置模板
│   ├── inbound_config.json     # 入站配置模板
│   └── servers.txt             # 服务器列表
├── subscription_links.txt       # 生成的订阅链接（运行后）
├── subscription_urls.txt        # 生成的订阅 URL（运行后）
└── deployment_summary.txt       # 部署摘要（运行后）
```

## 快速开始

### 前置要求

1. **本地环境**：
   - Linux/macOS 系统
   - bash shell
   - curl
   - jq（JSON 处理工具）
   - ssh（如需远程安装）

   安装 jq：
   ```bash
   # Ubuntu/Debian
   sudo apt-get install jq

   # CentOS/RHEL
   sudo yum install jq

   # macOS
   brew install jq
   ```

2. **远程服务器**（如需 SSH 安装）：
   - 已配置 SSH 密钥认证
   - Root 或 sudo 权限

### 第一步：生成配置

运行配置生成向导：

```bash
cd automation
bash generate_configs.sh
```

向导会引导您：
1. 输入 Reality 伪装域名（如：ocsp2.apple.com）
2. 设置监听端口（自动生成或手动指定）
3. 生成 Reality 密钥对
4. 生成 UUID 和 Short ID
5. 创建配置文件

**示例输出**：
```
[INFO] S-UI 配置生成向导
请输入 Reality 伪装域名 (默认: ocsp2.apple.com):
请输入监听端口 (默认: 随机生成): 16354
请输入入站标签 (默认: vless-16354):
[INFO] 生成的 UUID: 92af9f1a-8dfd-406a-c14a-ec9c4888f582
请输入客户端用户名 (默认: user1):
[INFO] 生成的 Short ID: 4a03
[INFO] Private Key: xxxxx
[INFO] Public Key: s6iCO4iufTXzKP2uRVbdrUpJ5Rh-2OWGXOR4AsUzsn8

[INFO] 配置生成完成！

示例订阅链接格式：
vless://92af9f1a-8dfd-406a-c14a-ec9c4888f582@216.167.29.233:16354?security=reality&pbk=s6iCO4iufTXzKP2uRVbdrUpJ5Rh-2OWGXOR4AsUzsn8&sid=4a03&fp=chrome&sni=ocsp2.apple.com&flow=xtls-rprx-vision&type=tcp#vless-16354
```

### 第二步：配置服务器列表

编辑 `configs/servers.txt`，添加您的服务器：

```bash
# 格式: IP,SSH_USER,SSH_KEY_PATH,NEW_PASSWORD
216.167.29.233,root,/root/.ssh/id_rsa,MySecurePassword123!
192.168.1.100,root,/root/.ssh/id_rsa,AnotherSecurePass456!

# 如果 S-UI 已安装，只需填写 IP 和密码
10.0.0.50,,,OnlyPasswordNeeded789!
```

**字段说明**：
- `IP`: 服务器 IP 地址（必填）
- `SSH_USER`: SSH 用户名（可选，用于远程安装）
- `SSH_KEY_PATH`: SSH 私钥路径（可选，用于远程安装）
- `NEW_PASSWORD`: 新的管理员密码（可选，但强烈推荐）

### 第三步：运行部署

```bash
bash deploy.sh
```

脚本会自动：
1. 检查前置条件
2. 在每台服务器上安装 S-UI（如提供了 SSH 信息）
3. 登录到 S-UI 面板
4. 修改管理员密码
5. 应用 TLS 配置
6. 应用入站配置
7. 获取订阅链接
8. 生成部署报告

### 第四步：查看结果

部署完成后，查看生成的文件：

1. **订阅链接**：`subscription_links.txt`
   ```
   === 服务器: 216.167.29.233 ===
   vless://92af9f1a-8dfd-406a-c14a-ec9c4888f582@216.167.29.233:16354?security=reality&pbk=xxx&sid=4a03&fp=chrome&sni=ocsp2.apple.com&flow=xtls-rprx-vision&type=tcp#vless-16354

   === 服务器: 192.168.1.100 ===
   vless://92af9f1a-8dfd-406a-c14a-ec9c4888f582@192.168.1.100:16354?security=reality&pbk=xxx&sid=4a03&fp=chrome&sni=ocsp2.apple.com&flow=xtls-rprx-vision&type=tcp#vless-16354
   ```

2. **部署摘要**：`deployment_summary.txt`
   ```
   ✓ 216.167.29.233
   ✓ 192.168.1.100
   ✗ 10.0.0.50
   ```

3. **详细日志**：`deploy_YYYYMMDD_HHMMSS.log`

## 高级用法

### 1. 手动使用 API

如果您想手动管理配置，可以使用 `api_helper.sh`：

```bash
# 加载 API 函数库
source api_helper.sh

# 登录
sui_login "216.167.29.233" "admin" "your_password"

# 获取所有入站配置
sui_get_inbounds "216.167.29.233" | jq '.'

# 获取客户端列表
sui_get_clients "216.167.29.233" | jq '.'

# 添加新的入站
inbound_json='{"type":"vless","tag":"test",...}'
sui_add_inbound "216.167.29.233" "$inbound_json"

# 重启 sing-box
sui_restart_singbox "216.167.29.233"

# 登出
sui_logout "216.167.29.233"
```

### 2. 自定义配置模板

#### TLS 配置 (`configs/tls_config.json`)

```json
{
  "server_name": "ocsp2.apple.com",
  "certificate_path": "",
  "key_path": "",
  "reality": {
    "enabled": true,
    "handshake": {
      "server": "ocsp2.apple.com",
      "server_port": 443
    },
    "private_key": "your_private_key",
    "public_key": "your_public_key",
    "short_id": ["4a03"]
  },
  "acme": {
    "enabled": false
  }
}
```

#### 入站配置 (`configs/inbound_config.json`)

```json
{
  "type": "vless",
  "tag": "vless-16354",
  "listen": "0.0.0.0",
  "listen_port": 16354,
  "sniff": true,
  "tls": {
    "enabled": true,
    "server_name": "ocsp2.apple.com",
    "reality": {
      "enabled": true,
      "handshake": {
        "server": "ocsp2.apple.com",
        "server_port": 443
      },
      "short_id": ["4a03"]
    }
  },
  "users": [
    {
      "name": "user1",
      "uuid": "92af9f1a-8dfd-406a-c14a-ec9c4888f582",
      "flow": "xtls-rprx-vision"
    }
  ],
  "transport": {
    "type": "tcp"
  }
}
```

**注意**：配置中可以使用 `{{SERVER_IP}}` 占位符，部署时会自动替换为实际服务器 IP。

### 3. 只应用配置（不安装）

如果服务器已经安装了 S-UI，只需要应用配置：

1. 在 `servers.txt` 中只填写 IP 和密码：
   ```
   216.167.29.233,,,MyPassword
   ```

2. 脚本会跳过安装步骤，直接应用配置

### 4. 导出/导入数据库

```bash
source api_helper.sh

# 登录
sui_login "216.167.29.233" "admin" "password"

# 导出数据库
sui_export_db "216.167.29.233" "backup.db"

# 导入数据库到另一台服务器
sui_login "192.168.1.100" "admin" "password"
sui_import_db "192.168.1.100" "backup.db"

sui_logout "192.168.1.100"
```

## 配置说明

### 订阅链接格式

生成的订阅链接格式示例：

```
vless://[UUID]@[IP]:[PORT]?security=reality&pbk=[PUBLIC_KEY]&sid=[SHORT_ID]&fp=chrome&sni=[SNI]&flow=xtls-rprx-vision&type=tcp#[TAG]
```

**参数说明**：
- `UUID`: 用户唯一标识符
- `IP`: 服务器 IP 地址
- `PORT`: 监听端口
- `PUBLIC_KEY`: Reality 公钥
- `SHORT_ID`: Reality Short ID
- `SNI`: TLS Server Name
- `TAG`: 入站标签

### 环境变量

您可以通过环境变量自定义配置：

```bash
# S-UI 面板端口（默认: 2095）
export SUI_PORT=2095

# S-UI 面板路径（默认: /app/）
export SUI_PATH="/app/"

# 默认用户名（默认: admin）
export SUI_USERNAME="admin"

# 默认密码（默认: admin）
export SUI_PASSWORD="admin"

# 订阅服务端口（默认: 2096）
export SUB_PORT=2096

# 订阅服务路径（默认: /sub/）
export SUB_PATH="/sub/"

# 运行部署
bash deploy.sh
```

## 故障排除

### 1. 无法连接到服务器

**问题**：`Connection refused` 或 `Connection timeout`

**解决方案**：
- 检查服务器 IP 是否正确
- 检查防火墙是否开放了 2095 端口
- 确认 S-UI 服务是否正在运行：`systemctl status s-ui`

### 2. 登录失败

**问题**：`Invalid login` 或 `wrong user or password`

**解决方案**：
- 检查用户名和密码是否正确
- 如果密码已被修改，更新 `SUI_USERNAME` 和 `SUI_PASSWORD`
- 手动登录 Web 面板确认凭据

### 3. 配置应用失败

**问题**：`TLS 配置应用失败` 或 `入站配置应用失败`

**解决方案**：
- 检查 JSON 配置文件格式是否正确
- 使用 `jq` 验证：`jq '.' configs/tls_config.json`
- 检查日志文件获取详细错误信息

### 4. SSH 连接失败

**问题**：`Permission denied` 或 `Host key verification failed`

**解决方案**：
- 确认 SSH 密钥路径正确
- 确认 SSH 密钥权限：`chmod 600 /path/to/key`
- 手动测试 SSH 连接：`ssh -i /path/to/key user@ip`

### 5. jq 命令未找到

**问题**：`command not found: jq`

**解决方案**：
```bash
# Ubuntu/Debian
sudo apt-get install jq

# CentOS/RHEL
sudo yum install jq

# macOS
brew install jq
```

## 安全建议

⚠️ **重要安全提示**：

1. **修改默认密码**：务必在 `servers.txt` 中指定新密码
2. **保护敏感文件**：
   ```bash
   chmod 600 configs/servers.txt
   chmod 600 /path/to/ssh/key
   ```
3. **使用 HTTPS**：生产环境建议配置 TLS 证书
4. **限制访问**：配置防火墙规则，只允许必要的 IP 访问
5. **定期备份**：定期备份数据库
6. **审计日志**：定期检查部署日志

## API 参考

完整的 API 函数列表：

| 函数 | 说明 |
|------|------|
| `sui_login` | 登录 |
| `sui_logout` | 登出 |
| `sui_load_all` | 获取所有数据 |
| `sui_get_inbounds` | 获取入站配置 |
| `sui_get_outbounds` | 获取出站配置 |
| `sui_get_tls` | 获取 TLS 配置 |
| `sui_get_clients` | 获取客户端列表 |
| `sui_add_inbound` | 添加入站 |
| `sui_edit_inbound` | 编辑入站 |
| `sui_delete_inbound` | 删除入站 |
| `sui_add_client` | 添加客户端 |
| `sui_restart_singbox` | 重启 sing-box |
| `sui_restart_app` | 重启应用 |
| `sui_get_status` | 获取系统状态 |
| `sui_get_onlines` | 获取在线客户端 |
| `sui_get_stats` | 获取流量统计 |
| `sui_change_password` | 修改密码 |
| `sui_generate_keypair` | 生成密钥对 |
| `sui_export_db` | 导出数据库 |
| `sui_import_db` | 导入数据库 |

详细用法请参考 `api_helper.sh` 中的注释。

## 使用场景

### 场景 1：新服务器批量部署

```bash
# 1. 生成配置
bash generate_configs.sh

# 2. 添加服务器（带 SSH 信息）
echo "server1.com,root,/root/.ssh/id_rsa,SecurePass1" >> configs/servers.txt
echo "server2.com,root,/root/.ssh/id_rsa,SecurePass2" >> configs/servers.txt

# 3. 运行部署
bash deploy.sh

# 4. 查看订阅链接
cat subscription_links.txt
```

### 场景 2：已安装服务器配置同步

```bash
# 1. 准备配置文件
# 编辑 configs/tls_config.json 和 configs/inbound_config.json

# 2. 添加已安装的服务器（只需 IP 和密码）
echo "10.0.0.1,,,MyPassword1" >> configs/servers.txt
echo "10.0.0.2,,,MyPassword2" >> configs/servers.txt

# 3. 运行部署（跳过安装步骤）
bash deploy.sh
```

### 场景 3：单台服务器手动配置

```bash
source api_helper.sh

SERVER="216.167.29.233"
USER="admin"
PASS="password"

# 登录
sui_login "$SERVER" "$USER" "$PASS"

# 查看当前配置
sui_get_inbounds "$SERVER" | jq '.'

# 添加新配置
inbound_json=$(cat configs/inbound_config.json)
sui_add_inbound "$SERVER" "$inbound_json"

# 重启服务
sui_restart_singbox "$SERVER"

# 登出
sui_logout "$SERVER"
```

## 常见问题 (FAQ)

**Q: 可以为每台服务器使用不同的配置吗？**

A: 可以。修改 `deploy.sh` 中的 `apply_inbound_config` 函数，根据服务器 IP 加载不同的配置文件。

**Q: 如何批量添加多个用户？**

A: 在 `inbound_config.json` 的 `users` 数组中添加多个用户对象，每个用户有不同的 UUID。

**Q: 支持其他协议吗（如 Trojan、Shadowsocks）？**

A: 支持。修改 `inbound_config.json` 中的 `type` 字段和相应参数即可。

**Q: 如何监控服务器状态？**

A: 使用 API 函数：
```bash
source api_helper.sh
sui_login "server_ip" "user" "pass"
sui_get_status "server_ip"
sui_get_onlines "server_ip"
sui_get_stats "server_ip"
```

**Q: 订阅链接在哪里？**

A: 部署完成后，订阅链接保存在 `subscription_links.txt` 文件中。

## 更新日志

### v1.0.0 (2025-01-07)
- ✅ 初始版本
- ✅ 支持批量自动化安装
- ✅ 支持统一配置管理
- ✅ 支持订阅链接自动收集
- ✅ 提供完整的 API 封装
- ✅ 配置生成向导

## 许可证

本项目基于 S-UI 项目，遵循相同的许可证。

## 免责声明

> 本项目仅供个人学习和交流使用，请勿用于非法用途，请勿在生产环境中使用。

## 支持

如有问题或建议，请提交 Issue 或 Pull Request。

---

**祝您使用愉快！** 🚀
