# S-UI 自动化部署架构文档

## 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                        本地控制机                                │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │ quick_start  │  │    deploy    │  │ generate_    │         │
│  │    .sh       │─>│     .sh      │<─│  configs.sh  │         │
│  └──────────────┘  └──────┬───────┘  └──────────────┘         │
│                            │                                     │
│                            │ 使用                                │
│                            ↓                                     │
│                   ┌────────────────┐                            │
│                   │  api_helper.sh │                            │
│                   │  (API 函数库)   │                            │
│                   └────────┬───────┘                            │
│                            │                                     │
│  ┌─────────────────────────┼────────────────────────┐          │
│  │         配置文件         │                        │          │
│  │  ┌──────────────────┐  │  ┌──────────────────┐  │          │
│  │  │ tls_config.json  │  │  │inbound_config.json│ │          │
│  │  └──────────────────┘  │  └──────────────────┘  │          │
│  │  ┌──────────────────┐  │                        │          │
│  │  │  servers.txt     │  │                        │          │
│  │  └──────────────────┘  │                        │          │
│  └─────────────────────────┴────────────────────────┘          │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             │ SSH (安装) / HTTP (配置)
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ↓                    ↓                    ↓
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│   Server 1    │    │   Server 2    │    │   Server N    │
│               │    │               │    │               │
│  ┌─────────┐  │    │  ┌─────────┐  │    │  ┌─────────┐  │
│  │  S-UI   │  │    │  │  S-UI   │  │    │  │  S-UI   │  │
│  │  Panel  │  │    │  │  Panel  │  │    │  │  Panel  │  │
│  └────┬────┘  │    │  └────┬────┘  │    │  └────┬────┘  │
│       │       │    │       │       │    │       │       │
│  ┌────▼────┐  │    │  ┌────▼────┐  │    │  ┌────▼────┐  │
│  │sing-box │  │    │  │sing-box │  │    │  │sing-box │  │
│  │  Core   │  │    │  │  Core   │  │    │  │  Core   │  │
│  └─────────┘  │    │  └─────────┘  │    │  └─────────┘  │
└───────────────┘    └───────────────┘    └───────────────┘
        │                    │                    │
        └────────────────────┼────────────────────┘
                             │
                             ↓
                    ┌────────────────┐
                    │   客户端连接    │
                    │ (VLESS/Trojan) │
                    └────────────────┘
```

## 工作流程

### 1. 配置生成阶段

```
用户输入
   │
   ↓
generate_configs.sh
   │
   ├─> 生成 Reality 密钥对
   │   (Private Key / Public Key)
   │
   ├─> 生成 UUID
   │   (客户端唯一标识)
   │
   ├─> 生成 Short ID
   │   (Reality 短 ID)
   │
   ├─> 生成配置文件
   │   ├─> tls_config.json
   │   └─> inbound_config.json
   │
   └─> 输出示例订阅链接
```

### 2. 部署执行阶段

```
deploy.sh 启动
   │
   ├─> 检查前置条件
   │   ├─> curl 已安装?
   │   ├─> jq 已安装?
   │   └─> 配置文件存在?
   │
   ├─> 读取服务器列表
   │   (servers.txt)
   │
   └─> 对每台服务器执行:
       │
       ├─> [可选] SSH 远程安装 S-UI
       │   └─> bash <(curl ...) install.sh
       │
       ├─> 登录 S-UI API
       │   └─> POST /api/login
       │
       ├─> [可选] 修改管理员密码
       │   └─> POST /api/changePass
       │
       ├─> 应用 TLS 配置
       │   └─> POST /api/save (obj=tls)
       │
       ├─> 应用入站配置
       │   └─> POST /api/save (obj=inbounds)
       │
       ├─> 获取客户端信息
       │   └─> GET /api/clients
       │
       ├─> 提取订阅链接
       │   └─> 保存到 subscription_links.txt
       │
       └─> 清理临时文件
```

### 3. API 交互流程

```
┌─────────────┐
│ 客户端脚本   │
└──────┬──────┘
       │
       │ 1. POST /api/login
       │    {username, password}
       ↓
┌─────────────────┐
│   S-UI Server   │─────> Set-Cookie: session_id=xxx
└──────┬──────────┘
       │
       │ 2. POST /api/save
       │    Cookie: session_id=xxx
       │    {obj: "tls", act: "add", data: {...}}
       ↓
┌─────────────────┐
│  保存 TLS 配置   │
└──────┬──────────┘
       │
       │ 3. POST /api/save
       │    Cookie: session_id=xxx
       │    {obj: "inbounds", act: "add", data: {...}}
       ↓
┌─────────────────┐
│ 保存入站配置     │
│ 重启 sing-box   │
└──────┬──────────┘
       │
       │ 4. GET /api/clients
       │    Cookie: session_id=xxx
       ↓
┌─────────────────┐
│ 返回客户端信息   │
│ (包含订阅链接)   │
└─────────────────┘
```

## 数据流图

### 配置数据流

```
用户输入参数
   │
   ↓
[generate_configs.sh]
   │
   ├─> Reality Keys ──┐
   ├─> UUID          ├─> tls_config.json
   ├─> Short ID      │
   └─> Port          ├─> inbound_config.json
                     │
                     ↓
               [deploy.sh]
                     │
                     ├─> 替换 {{SERVER_IP}}
                     │
                     ↓
              [S-UI API]
                     │
                     ├─> 验证配置
                     ├─> 保存到数据库
                     └─> 应用到 sing-box
                     │
                     ↓
              [sing-box Core]
                     │
                     └─> 监听端口
                         接受客户端连接
```

### 订阅链接生成流程

```
[S-UI Database]
   │
   ├─> Inbound Config
   ├─> TLS Config
   └─> Client UUID
   │
   ↓
[S-UI Backend]
   │
   ├─> 构建 VLESS URI
   │   vless://[UUID]@[IP]:[PORT]?
   │   security=reality&
   │   pbk=[PUBLIC_KEY]&
   │   sid=[SHORT_ID]&
   │   fp=chrome&
   │   sni=[SNI]&
   │   flow=xtls-rprx-vision&
   │   type=tcp#[TAG]
   │
   ↓
[API Response]
   │
   └─> {"success": true, "obj": [...links...]}
   │
   ↓
[deploy.sh]
   │
   └─> 保存到 subscription_links.txt
```

## 组件说明

### 核心脚本

#### 1. quick_start.sh
- **功能**: 交互式部署向导
- **职责**:
  - 检查环境依赖
  - 引导用户完成配置
  - 调用其他脚本
  - 展示部署结果

#### 2. deploy.sh
- **功能**: 主部署脚本
- **职责**:
  - 批量服务器部署
  - SSH 远程安装
  - API 配置应用
  - 订阅链接收集
  - 日志记录

#### 3. generate_configs.sh
- **功能**: 配置生成向导
- **职责**:
  - 生成密钥对
  - 生成 UUID/Short ID
  - 创建配置模板
  - 输出示例链接

#### 4. api_helper.sh
- **功能**: API 函数库
- **职责**:
  - 封装所有 S-UI API
  - 处理认证/会话
  - 提供便捷接口

### 配置文件

#### 1. tls_config.json
```json
{
  "server_name": "SNI 域名",
  "reality": {
    "enabled": true,
    "private_key": "Reality 私钥",
    "public_key": "Reality 公钥",
    "short_id": ["短 ID"]
  }
}
```

#### 2. inbound_config.json
```json
{
  "type": "vless",
  "tag": "标签",
  "listen_port": 端口,
  "tls": { Reality 配置 },
  "users": [
    {
      "uuid": "用户 UUID",
      "flow": "xtls-rprx-vision"
    }
  ]
}
```

#### 3. servers.txt
```
IP,SSH_USER,SSH_KEY,NEW_PASSWORD
```

## API 端点映射

| 功能 | 方法 | 端点 | 脚本函数 |
|------|------|------|----------|
| 登录 | POST | /api/login | `sui_login` |
| 登出 | GET | /api/logout | `sui_logout` |
| 获取入站 | GET | /api/inbounds | `sui_get_inbounds` |
| 保存配置 | POST | /api/save | `sui_save` |
| 添加入站 | POST | /api/save | `sui_add_inbound` |
| 获取客户端 | GET | /api/clients | `sui_get_clients` |
| 修改密码 | POST | /api/changePass | `sui_change_password` |
| 生成密钥对 | GET | /api/keypairs | `sui_generate_keypair` |
| 重启服务 | POST | /api/restartSb | `sui_restart_singbox` |
| 系统状态 | GET | /api/status | `sui_get_status` |
| 在线客户端 | GET | /api/onlines | `sui_get_onlines` |
| 流量统计 | GET | /api/stats | `sui_get_stats` |

## 安全考虑

### 认证流程

```
1. 用户凭据
   └─> 明文传输 (建议使用 HTTPS)

2. Session Cookie
   ├─> 存储在 /tmp/sui_cookies_*.txt
   ├─> 每个服务器独立 cookie
   └─> 部署完成后自动删除

3. 密码修改
   └─> 首次登录后立即修改默认密码
```

### 敏感数据保护

```
servers.txt
   ├─> 包含 SSH 密钥路径
   ├─> 包含管理员密码
   └─> 建议权限: chmod 600

tls_config.json
   ├─> 包含 Reality 私钥
   └─> 建议权限: chmod 600

临时 Cookie 文件
   ├─> /tmp/sui_cookies_*.txt
   └─> 部署后自动删除
```

## 性能优化

### 并发处理

当前实现: **顺序处理**
```
Server 1 → Server 2 → Server 3 → ... → Server N
```

可优化为: **并行处理**
```
┌─> Server 1
├─> Server 2
├─> Server 3  (同时执行)
└─> Server N
```

实现方式:
```bash
for server in "${servers[@]}"; do
    deploy_to_server "$server" &
done
wait
```

### 缓存机制

- **密钥对缓存**: 一次生成，多次使用
- **配置模板**: 预生成，动态替换 IP
- **API 响应**: 可缓存非敏感数据

## 扩展性设计

### 支持新协议

1. 创建新的配置模板: `configs/trojan_config.json`
2. 在 `deploy.sh` 中添加处理逻辑
3. 更新 `generate_configs.sh` 支持新参数

### 支持配置导入

```bash
# 从现有服务器导出配置
sui_export_db "source_server" "config.db"

# 导入到新服务器
sui_import_db "target_server" "config.db"
```

### 支持批量用户管理

```bash
# 批量添加用户
for user in "${users[@]}"; do
    sui_add_client "$server" "$user_config"
done
```

## 故障恢复

### 断点续传

```bash
# 记录已成功部署的服务器
echo "$server_ip" >> .deployed_servers

# 跳过已部署的服务器
if grep -q "$server_ip" .deployed_servers; then
    log "Server $server_ip already deployed, skipping..."
    continue
fi
```

### 回滚机制

```bash
# 部署前备份
sui_export_db "$server_ip" "backup_${server_ip}.db"

# 失败时回滚
if [ $? -ne 0 ]; then
    sui_import_db "$server_ip" "backup_${server_ip}.db"
fi
```

## 监控和日志

### 日志级别

- **INFO**: 正常操作 (绿色)
- **WARNING**: 警告信息 (黄色)
- **ERROR**: 错误信息 (红色)

### 日志格式

```
[YYYY-MM-DD HH:MM:SS] [LEVEL] Message
```

### 日志文件

- `deploy_YYYYMMDD_HHMMSS.log`: 详细部署日志
- `deployment_summary.txt`: 部署摘要
- `subscription_links.txt`: 订阅链接

## 测试策略

### 单元测试

```bash
# 测试 API 函数
test_api_login() {
    local result=$(sui_login "test_server" "admin" "admin")
    assert_equals "$(echo $result | jq -r '.success')" "true"
}
```

### 集成测试

```bash
# 测试完整部署流程
test_full_deployment() {
    # 1. 生成配置
    bash generate_configs.sh <<< "test_inputs"

    # 2. 模拟部署
    echo "test_server,,,test_pass" > configs/servers.txt
    bash deploy.sh

    # 3. 验证结果
    assert_file_exists "subscription_links.txt"
}
```

## 版本兼容性

| S-UI 版本 | 脚本版本 | 兼容性 |
|-----------|----------|--------|
| v1.0.x - v1.2.x | v1.0.0 | ⚠️ 部分兼容 |
| v1.3.x | v1.0.0 | ✅ 完全兼容 |
| v1.4.x+ | v1.0.0 | ✅ 完全兼容 |

## 未来改进

1. **Web 界面**: 开发基于 Web 的部署控制台
2. **配置验证**: 增强配置文件的验证功能
3. **健康检查**: 部署后自动检查服务健康状态
4. **批量升级**: 支持批量升级已部署的 S-UI
5. **配置同步**: 自动同步配置变更到所有服务器
6. **性能监控**: 集成流量和性能监控
7. **告警通知**: 支持邮件/Webhook 告警

---

**文档版本**: v1.0.0
**更新日期**: 2025-01-07
