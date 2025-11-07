#!/bin/bash

################################################################################
# S-UI 自动化部署脚本
# 用途：在多台服务器上自动安装 s-ui 并应用统一配置
################################################################################

set -e

# 配置变量
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_DIR="${SCRIPT_DIR}/configs"
LOG_FILE="${SCRIPT_DIR}/deploy_$(date +%Y%m%d_%H%M%S).log"

# S-UI 默认配置
SUI_PORT=${SUI_PORT:-2095}
SUI_PATH=${SUI_PATH:-"/app/"}
SUI_USERNAME=${SUI_USERNAME:-"admin"}
SUI_PASSWORD=${SUI_PASSWORD:-"admin"}

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $*" | tee -a "${LOG_FILE}"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*" | tee -a "${LOG_FILE}"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*" | tee -a "${LOG_FILE}"
}

# 检查必要文件
check_prerequisites() {
    log "检查前置条件..."

    if [ ! -f "${CONFIG_DIR}/tls_config.json" ]; then
        error "找不到 TLS 配置文件: ${CONFIG_DIR}/tls_config.json"
        exit 1
    fi

    if [ ! -f "${CONFIG_DIR}/inbound_config.json" ]; then
        error "找不到入站配置文件: ${CONFIG_DIR}/inbound_config.json"
        exit 1
    fi

    if [ ! -f "${CONFIG_DIR}/servers.txt" ]; then
        error "找不到服务器列表文件: ${CONFIG_DIR}/servers.txt"
        exit 1
    fi

    if ! command -v curl &> /dev/null; then
        error "需要安装 curl"
        exit 1
    fi

    if ! command -v jq &> /dev/null; then
        error "需要安装 jq (用于 JSON 处理)"
        exit 1
    fi

    log "前置条件检查通过"
}

# 在远程服务器上安装 s-ui
install_sui_remote() {
    local server_ip=$1
    local ssh_user=$2
    local ssh_key=$3

    log "在服务器 ${server_ip} 上安装 s-ui..."

    ssh -i "${ssh_key}" -o StrictHostKeyChecking=no "${ssh_user}@${server_ip}" << 'ENDSSH'
        # 安装 s-ui
        bash <(curl -Ls https://raw.githubusercontent.com/alireza0/s-ui/master/install.sh)

        # 等待服务启动
        sleep 10

        # 检查服务状态
        systemctl status s-ui --no-pager
ENDSSH

    if [ $? -eq 0 ]; then
        log "服务器 ${server_ip} 上的 s-ui 安装成功"
        return 0
    else
        error "服务器 ${server_ip} 上的 s-ui 安装失败"
        return 1
    fi
}

# 登录并获取 session cookie
login_sui() {
    local server_ip=$1
    local username=$2
    local password=$3

    log "登录到 ${server_ip}..."

    local response=$(curl -s -c /tmp/sui_cookies_${server_ip}.txt \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "username=${username}&password=${password}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/login")

    local success=$(echo "$response" | jq -r '.success')

    if [ "$success" == "true" ]; then
        log "登录成功: ${server_ip}"
        return 0
    else
        error "登录失败: ${server_ip}"
        echo "$response" | jq '.'
        return 1
    fi
}

# 修改管理员密码
change_password() {
    local server_ip=$1
    local old_password=$2
    local new_username=$3
    local new_password=$4

    log "修改服务器 ${server_ip} 的管理员密码..."

    local response=$(curl -s -b /tmp/sui_cookies_${server_ip}.txt \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "oldPassword=${old_password}&newUsername=${new_username}&newPassword=${new_password}" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/changePass")

    local success=$(echo "$response" | jq -r '.success')

    if [ "$success" == "true" ]; then
        log "密码修改成功: ${server_ip}"
        return 0
    else
        warning "密码修改失败: ${server_ip} (可能已经修改过)"
        return 0
    fi
}

# 应用 TLS 配置
apply_tls_config() {
    local server_ip=$1
    local config_file=$2

    log "应用 TLS 配置到 ${server_ip}..."

    # 读取配置文件
    local tls_config=$(cat "${config_file}")

    # 构建请求数据
    local request_data=$(jq -n \
        --arg obj "tls" \
        --arg act "add" \
        --argjson data "$tls_config" \
        '{obj: $obj, act: $act, data: $data}')

    local response=$(curl -s -b /tmp/sui_cookies_${server_ip}.txt \
        -H "Content-Type: application/json" \
        -d "$request_data" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/save")

    local success=$(echo "$response" | jq -r '.success')

    if [ "$success" == "true" ]; then
        log "TLS 配置应用成功: ${server_ip}"
        return 0
    else
        error "TLS 配置应用失败: ${server_ip}"
        echo "$response" | jq '.'
        return 1
    fi
}

# 应用入站配置 (替换 IP 地址)
apply_inbound_config() {
    local server_ip=$1
    local config_file=$2

    log "应用入站配置到 ${server_ip}..."

    # 读取配置并替换 IP 地址占位符
    local inbound_config=$(cat "${config_file}" | sed "s/{{SERVER_IP}}/${server_ip}/g")

    # 构建请求数据
    local request_data=$(jq -n \
        --arg obj "inbounds" \
        --arg act "add" \
        --argjson data "$inbound_config" \
        '{obj: $obj, act: $act, data: $data}')

    local response=$(curl -s -b /tmp/sui_cookies_${server_ip}.txt \
        -H "Content-Type: application/json" \
        -d "$request_data" \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/save")

    local success=$(echo "$response" | jq -r '.success')

    if [ "$success" == "true" ]; then
        log "入站配置应用成功: ${server_ip}"

        # 获取创建的 inbound ID (从返回的数据中提取)
        local inbound_id=$(echo "$response" | jq -r '.obj // empty')
        echo "${inbound_id}" > "/tmp/sui_inbound_id_${server_ip}.txt"

        return 0
    else
        error "入站配置应用失败: ${server_ip}"
        echo "$response" | jq '.'
        return 1
    fi
}

# 获取客户端/订阅链接
get_subscription_link() {
    local server_ip=$1
    local inbound_tag=$2

    log "获取服务器 ${server_ip} 的订阅链接..."

    # 首先获取所有客户端信息
    local response=$(curl -s -b /tmp/sui_cookies_${server_ip}.txt \
        "http://${server_ip}:${SUI_PORT}${SUI_PATH}api/clients")

    local success=$(echo "$response" | jq -r '.success')

    if [ "$success" != "true" ]; then
        error "获取客户端信息失败: ${server_ip}"
        return 1
    fi

    # 提取客户端信息并生成订阅链接
    local clients=$(echo "$response" | jq -r '.obj')

    # 保存到结果文件
    echo "=== 服务器: ${server_ip} ===" >> "${SCRIPT_DIR}/subscription_links.txt"
    echo "$clients" | jq -r '.[] | select(.inbound != null) | .link' >> "${SCRIPT_DIR}/subscription_links.txt"
    echo "" >> "${SCRIPT_DIR}/subscription_links.txt"

    log "订阅链接已保存到 subscription_links.txt"
}

# 获取订阅链接（通过订阅服务）
get_subscription_url() {
    local server_ip=$1
    local client_id=$2
    local sub_port=${SUB_PORT:-2096}
    local sub_path=${SUB_PATH:-"/sub/"}

    # 生成订阅 URL
    local sub_url="http://${server_ip}:${sub_port}${sub_path}${client_id}"

    log "订阅 URL: ${sub_url}"
    echo "${sub_url}" >> "${SCRIPT_DIR}/subscription_urls.txt"
}

# 部署到单个服务器
deploy_to_server() {
    local server_line=$1

    # 解析服务器信息: IP,SSH_USER,SSH_KEY,NEW_PASSWORD
    IFS=',' read -r server_ip ssh_user ssh_key new_password <<< "$server_line"

    log "=========================================="
    log "开始部署到服务器: ${server_ip}"
    log "=========================================="

    # 1. 安装 s-ui (如果使用 SSH)
    if [ -n "${ssh_key}" ] && [ -f "${ssh_key}" ]; then
        install_sui_remote "${server_ip}" "${ssh_user}" "${ssh_key}" || return 1
    else
        warning "跳过远程安装，假设 s-ui 已安装在 ${server_ip}"
    fi

    # 等待服务启动
    sleep 5

    # 2. 登录
    login_sui "${server_ip}" "${SUI_USERNAME}" "${SUI_PASSWORD}" || return 1

    # 3. 修改密码（如果提供了新密码）
    if [ -n "${new_password}" ]; then
        change_password "${server_ip}" "${SUI_PASSWORD}" "${SUI_USERNAME}" "${new_password}" || true
        # 使用新密码重新登录
        login_sui "${server_ip}" "${SUI_USERNAME}" "${new_password}" || return 1
    fi

    # 4. 应用 TLS 配置
    apply_tls_config "${server_ip}" "${CONFIG_DIR}/tls_config.json" || return 1

    # 5. 应用入站配置
    apply_inbound_config "${server_ip}" "${CONFIG_DIR}/inbound_config.json" || return 1

    # 6. 获取订阅链接
    get_subscription_link "${server_ip}" || true

    log "服务器 ${server_ip} 部署完成"
    log "=========================================="
    echo ""

    # 清理临时文件
    rm -f /tmp/sui_cookies_${server_ip}.txt
}

# 主函数
main() {
    log "=========================================="
    log "S-UI 自动化部署脚本启动"
    log "=========================================="

    check_prerequisites

    # 清空之前的结果文件
    > "${SCRIPT_DIR}/subscription_links.txt"
    > "${SCRIPT_DIR}/subscription_urls.txt"
    > "${SCRIPT_DIR}/deployment_summary.txt"

    local success_count=0
    local fail_count=0

    # 读取服务器列表并部署
    while IFS= read -r server_line; do
        # 跳过空行和注释
        [[ -z "$server_line" || "$server_line" =~ ^[[:space:]]*# ]] && continue

        if deploy_to_server "$server_line"; then
            ((success_count++))
            echo "✓ ${server_line%%,*}" >> "${SCRIPT_DIR}/deployment_summary.txt"
        else
            ((fail_count++))
            echo "✗ ${server_line%%,*}" >> "${SCRIPT_DIR}/deployment_summary.txt"
        fi
    done < "${CONFIG_DIR}/servers.txt"

    log "=========================================="
    log "部署完成！"
    log "成功: ${success_count} 台"
    log "失败: ${fail_count} 台"
    log "详细日志: ${LOG_FILE}"
    log "订阅链接: ${SCRIPT_DIR}/subscription_links.txt"
    log "=========================================="
}

# 运行主函数
main "$@"
