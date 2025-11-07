#!/bin/bash

################################################################################
# S-UI API 辅助脚本
# 用途：提供便捷的 API 调用函数
################################################################################

# S-UI API 封装函数

# 配置
SUI_PORT=${SUI_PORT:-2095}
SUI_PATH=${SUI_PATH:-"/app/"}
COOKIE_FILE="/tmp/sui_cookie.txt"

# 登录
sui_login() {
    local server_ip=$1
    local username=$2
    local password=$3

    curl -s -c "${COOKIE_FILE}" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "username=${username}&password=${password}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/login"
}

# 登出
sui_logout() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/logout"

    rm -f "${COOKIE_FILE}"
}

# 获取所有数据
sui_load_all() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/load"
}

# 获取入站配置
sui_get_inbounds() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/inbounds"
}

# 获取出站配置
sui_get_outbounds() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/outbounds"
}

# 获取 TLS 配置
sui_get_tls() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/tls"
}

# 获取客户端列表
sui_get_clients() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/clients"
}

# 保存配置（通用）
sui_save() {
    local server_ip=$1
    local obj=$2        # inbounds, outbounds, tls, clients, etc.
    local act=$3        # add, edit, delete
    local data=$4       # JSON data

    local request_data=$(jq -n \
        --arg obj "$obj" \
        --arg act "$act" \
        --argjson data "$data" \
        '{obj: $obj, act: $act, data: $data}')

    curl -s -b "${COOKIE_FILE}" \
        -H "Content-Type: application/json" \
        -d "$request_data" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/save"
}

# 添加入站
sui_add_inbound() {
    local server_ip=$1
    local inbound_json=$2

    sui_save "$server_ip" "inbounds" "add" "$inbound_json"
}

# 编辑入站
sui_edit_inbound() {
    local server_ip=$1
    local inbound_json=$2

    sui_save "$server_ip" "inbounds" "edit" "$inbound_json"
}

# 删除入站
sui_delete_inbound() {
    local server_ip=$1
    local inbound_id=$2

    local data=$(jq -n --arg id "$inbound_id" '{id: $id}')
    sui_save "$server_ip" "inbounds" "delete" "$data"
}

# 添加客户端
sui_add_client() {
    local server_ip=$1
    local client_json=$2

    sui_save "$server_ip" "clients" "add" "$client_json"
}

# 重启 sing-box 核心
sui_restart_singbox() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        -X POST \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/restartSb"
}

# 重启应用
sui_restart_app() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        -X POST \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/restartApp"
}

# 获取系统状态
sui_get_status() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/status"
}

# 获取在线客户端
sui_get_onlines() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/onlines"
}

# 获取流量统计
sui_get_stats() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/stats"
}

# 修改密码
sui_change_password() {
    local server_ip=$1
    local old_password=$2
    local new_username=$3
    local new_password=$4

    curl -s -b "${COOKIE_FILE}" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "oldPassword=${old_password}&newUsername=${new_username}&newPassword=${new_password}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/changePass"
}

# 生成密钥对（用于 Reality）
sui_generate_keypair() {
    local server_ip=$1

    curl -s -b "${COOKIE_FILE}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/keypairs"
}

# 导出数据库
sui_export_db() {
    local server_ip=$1
    local output_file=$2

    curl -s -b "${COOKIE_FILE}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/getdb" \
        -o "${output_file}"
}

# 导入数据库
sui_import_db() {
    local server_ip=$1
    local db_file=$2

    curl -s -b "${COOKIE_FILE}" \
        -F "db=@${db_file}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/importdb"
}

# 使用示例函数
example_usage() {
    local SERVER_IP="216.167.29.233"
    local USERNAME="admin"
    local PASSWORD="admin"

    echo "示例 1: 登录并获取所有入站配置"
    sui_login "$SERVER_IP" "$USERNAME" "$PASSWORD"
    sui_get_inbounds "$SERVER_IP" | jq '.'
    sui_logout "$SERVER_IP"

    echo ""
    echo "示例 2: 添加新的 VLESS 入站"
    sui_login "$SERVER_IP" "$USERNAME" "$PASSWORD"

    local inbound_config=$(cat << 'EOF'
{
  "type": "vless",
  "tag": "vless-test",
  "listen": "0.0.0.0",
  "listen_port": 12345,
  "users": [
    {
      "name": "testuser",
      "uuid": "$(uuidgen)",
      "flow": "xtls-rprx-vision"
    }
  ]
}
EOF
)

    sui_add_inbound "$SERVER_IP" "$inbound_config" | jq '.'
    sui_logout "$SERVER_IP"

    echo ""
    echo "示例 3: 生成 Reality 密钥对"
    sui_login "$SERVER_IP" "$USERNAME" "$PASSWORD"
    sui_generate_keypair "$SERVER_IP" | jq '.'
    sui_logout "$SERVER_IP"
}

# 如果直接运行此脚本，显示帮助
if [ "${BASH_SOURCE[0]}" == "${0}" ]; then
    cat << EOF
S-UI API 辅助脚本

用法:
    source api_helper.sh
    sui_login <server_ip> <username> <password>
    sui_get_inbounds <server_ip>
    sui_logout <server_ip>

可用函数:
    sui_login               - 登录
    sui_logout              - 登出
    sui_load_all            - 获取所有数据
    sui_get_inbounds        - 获取入站配置
    sui_get_outbounds       - 获取出站配置
    sui_get_tls             - 获取 TLS 配置
    sui_get_clients         - 获取客户端列表
    sui_save                - 保存配置（通用）
    sui_add_inbound         - 添加入站
    sui_edit_inbound        - 编辑入站
    sui_delete_inbound      - 删除入站
    sui_add_client          - 添加客户端
    sui_restart_singbox     - 重启 sing-box
    sui_restart_app         - 重启应用
    sui_get_status          - 获取系统状态
    sui_get_onlines         - 获取在线客户端
    sui_get_stats           - 获取流量统计
    sui_change_password     - 修改密码
    sui_generate_keypair    - 生成密钥对
    sui_export_db           - 导出数据库
    sui_import_db           - 导入数据库

查看示例:
    bash api_helper.sh example

EOF

    if [ "$1" == "example" ]; then
        example_usage
    fi
fi
