#!/bin/bash

################################################################################
# 配置生成脚本
# 用途：生成 Reality 密钥对和完整的配置文件
################################################################################

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_DIR="${SCRIPT_DIR}/configs"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

# 生成 UUID
generate_uuid() {
    if command -v uuidgen &> /dev/null; then
        uuidgen | tr '[:upper:]' '[:lower:]'
    else
        cat /proc/sys/kernel/random/uuid
    fi
}

# 生成 Reality 密钥对（使用 sing-box）
generate_reality_keypair() {
    log "生成 Reality 密钥对..."

    if ! command -v sing-box &> /dev/null; then
        warning "sing-box 未安装，将使用临时服务器生成密钥对"
        return 1
    fi

    # 使用 sing-box 生成密钥对
    local keypair=$(sing-box generate reality-keypair)

    local private_key=$(echo "$keypair" | grep "PrivateKey:" | awk '{print $2}')
    local public_key=$(echo "$keypair" | grep "PublicKey:" | awk '{print $2}')

    echo "$private_key|$public_key"
}

# 从 API 生成密钥对
generate_keypair_from_api() {
    local server_ip=${1:-"127.0.0.1"}
    local port=${2:-2095}

    log "从 S-UI API 生成密钥对..."

    # 需要先登录
    source "${SCRIPT_DIR}/api_helper.sh"

    sui_login "$server_ip" "admin" "admin" > /dev/null 2>&1

    local response=$(sui_generate_keypair "$server_ip")
    local success=$(echo "$response" | jq -r '.success')

    if [ "$success" == "true" ]; then
        local private_key=$(echo "$response" | jq -r '.obj.privateKey')
        local public_key=$(echo "$response" | jq -r '.obj.publicKey')

        sui_logout "$server_ip"

        echo "$private_key|$public_key"
        return 0
    else
        sui_logout "$server_ip"
        return 1
    fi
}

# 生成随机端口
generate_port() {
    local min=10000
    local max=65000
    echo $((min + RANDOM % (max - min)))
}

# 生成 short_id
generate_short_id() {
    openssl rand -hex 2
}

# 生成完整的配置文件
generate_full_config() {
    log "=========================================="
    log "S-UI 配置生成向导"
    log "=========================================="

    # 收集配置信息
    echo ""
    read -p "请输入 Reality 伪装域名 (默认: ocsp2.apple.com): " server_name
    server_name=${server_name:-"ocsp2.apple.com"}

    read -p "请输入监听端口 (默认: 随机生成): " listen_port
    if [ -z "$listen_port" ]; then
        listen_port=$(generate_port)
        log "生成的端口: $listen_port"
    fi

    read -p "请输入入站标签 (默认: vless-${listen_port}): " tag
    tag=${tag:-"vless-${listen_port}"}

    # 生成 UUID
    local uuid=$(generate_uuid)
    log "生成的 UUID: $uuid"

    read -p "请输入客户端用户名 (默认: user1): " username
    username=${username:-"user1"}

    # 生成 short_id
    local short_id=$(generate_short_id)
    log "生成的 Short ID: $short_id"

    # 生成密钥对
    local keypair_result=""
    if generate_reality_keypair > /dev/null 2>&1; then
        keypair_result=$(generate_reality_keypair)
    else
        warning "无法本地生成密钥对"
        read -p "是否从已安装的 S-UI 服务器生成? (y/N): " use_api
        if [[ "$use_api" =~ ^[Yy]$ ]]; then
            read -p "请输入服务器 IP: " api_server
            keypair_result=$(generate_keypair_from_api "$api_server")
        else
            warning "请手动填入密钥对"
            read -p "请输入 Private Key: " private_key
            read -p "请输入 Public Key: " public_key
            keypair_result="${private_key}|${public_key}"
        fi
    fi

    IFS='|' read -r private_key public_key <<< "$keypair_result"

    log "Private Key: $private_key"
    log "Public Key: $public_key"

    # 生成 TLS 配置文件
    cat > "${CONFIG_DIR}/tls_config.json" << EOF
{
  "server_name": "${server_name}",
  "certificate_path": "",
  "key_path": "",
  "reality": {
    "enabled": true,
    "handshake": {
      "server": "${server_name}",
      "server_port": 443
    },
    "private_key": "${private_key}",
    "public_key": "${public_key}",
    "short_id": [
      "${short_id}"
    ]
  },
  "acme": {
    "enabled": false
  }
}
EOF

    log "已生成 TLS 配置文件: ${CONFIG_DIR}/tls_config.json"

    # 生成入站配置文件
    cat > "${CONFIG_DIR}/inbound_config.json" << EOF
{
  "type": "vless",
  "tag": "${tag}",
  "listen": "0.0.0.0",
  "listen_port": ${listen_port},
  "sniff": true,
  "sniff_override_destination": false,
  "domain_strategy": "",
  "tls": {
    "enabled": true,
    "server_name": "${server_name}",
    "reality": {
      "enabled": true,
      "handshake": {
        "server": "${server_name}",
        "server_port": 443
      },
      "short_id": [
        "${short_id}"
      ]
    }
  },
  "users": [
    {
      "name": "${username}",
      "uuid": "${uuid}",
      "flow": "xtls-rprx-vision"
    }
  ],
  "transport": {
    "type": "tcp"
  },
  "multiplex": {
    "enabled": false
  }
}
EOF

    log "已生成入站配置文件: ${CONFIG_DIR}/inbound_config.json"

    # 生成示例订阅链接
    local example_ip="216.167.29.233"
    local vless_link="vless://${uuid}@${example_ip}:${listen_port}?security=reality&pbk=${public_key}&sid=${short_id}&fp=chrome&sni=${server_name}&flow=xtls-rprx-vision&type=tcp#${tag}"

    log ""
    log "=========================================="
    log "配置生成完成！"
    log "=========================================="
    log ""
    log "示例订阅链接格式："
    log "$vless_link"
    log ""
    log "下一步："
    log "1. 编辑 ${CONFIG_DIR}/servers.txt 添加你的服务器列表"
    log "2. 运行: bash deploy.sh"
    log "=========================================="
}

# 主函数
main() {
    mkdir -p "${CONFIG_DIR}"
    generate_full_config
}

main "$@"
