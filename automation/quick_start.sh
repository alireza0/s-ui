#!/bin/bash

################################################################################
# S-UI 自动化部署 - 快速开始脚本
################################################################################

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

info() {
    echo -e "${BLUE}[提示]${NC} $*"
}

banner() {
    cat << 'EOF'
╔═══════════════════════════════════════════════════════════╗
║                                                           ║
║     ____        _   _ ___    _         _                 ║
║    / ___|      | | | |_ _|  / \  _   _| |_ ___           ║
║    \___ \ _____| | | || |  / _ \| | | | __/ _ \          ║
║     ___) |_____| |_| || | / ___ \ |_| | || (_) |         ║
║    |____/       \___/|___/_/   \_\__,_|\__\___/          ║
║                                                           ║
║           自动化部署脚本 - 快速开始向导                    ║
║                                                           ║
╚═══════════════════════════════════════════════════════════╝
EOF
    echo ""
}

check_command() {
    if ! command -v "$1" &> /dev/null; then
        error "未找到命令: $1"
        return 1
    fi
    return 0
}

check_prerequisites() {
    log "检查前置条件..."
    echo ""

    local missing=0

    if ! check_command "curl"; then
        error "请安装 curl"
        ((missing++))
    else
        log "✓ curl 已安装"
    fi

    if ! check_command "jq"; then
        error "请安装 jq"
        echo "  Ubuntu/Debian: sudo apt-get install jq"
        echo "  CentOS/RHEL:   sudo yum install jq"
        echo "  macOS:         brew install jq"
        ((missing++))
    else
        log "✓ jq 已安装"
    fi

    echo ""

    if [ $missing -gt 0 ]; then
        error "缺少 $missing 个必需组件，请先安装"
        exit 1
    fi

    log "所有前置条件已满足"
    echo ""
}

interactive_setup() {
    banner

    log "欢迎使用 S-UI 自动化部署工具！"
    echo ""
    info "本工具将帮助您："
    echo "  1. 在多台服务器上批量安装 S-UI"
    echo "  2. 应用统一的配置"
    echo "  3. 自动获取订阅链接"
    echo ""

    read -p "按 Enter 继续，或 Ctrl+C 退出..."
    echo ""

    check_prerequisites

    # 步骤 1: 生成配置
    log "步骤 1/3: 生成配置"
    echo ""
    info "即将运行配置生成向导..."
    read -p "按 Enter 继续..."
    echo ""

    bash "${SCRIPT_DIR}/generate_configs.sh"

    echo ""
    log "配置文件已生成！"
    echo ""

    # 步骤 2: 配置服务器列表
    log "步骤 2/3: 配置服务器列表"
    echo ""
    info "现在需要配置服务器列表"
    echo ""
    echo "服务器列表格式: IP,SSH_USER,SSH_KEY_PATH,NEW_PASSWORD"
    echo ""
    echo "示例 1 - 使用 SSH 自动安装:"
    echo "  216.167.29.233,root,/root/.ssh/id_rsa,MySecurePassword123!"
    echo ""
    echo "示例 2 - 服务器已安装 S-UI:"
    echo "  192.168.1.100,,,MyPassword456"
    echo ""

    read -p "是否现在编辑服务器列表? (y/N): " edit_servers

    if [[ "$edit_servers" =~ ^[Yy]$ ]]; then
        if command -v nano &> /dev/null; then
            nano "${SCRIPT_DIR}/configs/servers.txt"
        elif command -v vim &> /dev/null; then
            vim "${SCRIPT_DIR}/configs/servers.txt"
        elif command -v vi &> /dev/null; then
            vi "${SCRIPT_DIR}/configs/servers.txt"
        else
            warning "未找到文本编辑器，请手动编辑文件:"
            echo "  ${SCRIPT_DIR}/configs/servers.txt"
            read -p "编辑完成后按 Enter 继续..."
        fi
    else
        warning "请手动编辑服务器列表文件:"
        echo "  ${SCRIPT_DIR}/configs/servers.txt"
        echo ""
        read -p "编辑完成后按 Enter 继续..."
    fi

    echo ""

    # 检查服务器列表是否为空
    if ! grep -qE "^[^#]" "${SCRIPT_DIR}/configs/servers.txt"; then
        error "服务器列表为空！请添加至少一台服务器。"
        echo ""
        info "编辑文件: ${SCRIPT_DIR}/configs/servers.txt"
        exit 1
    fi

    local server_count=$(grep -cE "^[^#]" "${SCRIPT_DIR}/configs/servers.txt" || true)
    log "检测到 ${server_count} 台服务器"
    echo ""

    # 步骤 3: 确认并部署
    log "步骤 3/3: 开始部署"
    echo ""
    info "即将在以下服务器上部署 S-UI:"
    echo ""
    grep -E "^[^#]" "${SCRIPT_DIR}/configs/servers.txt" | while read -r line; do
        local ip=$(echo "$line" | cut -d',' -f1)
        echo "  • $ip"
    done
    echo ""

    read -p "确认开始部署? (y/N): " confirm_deploy

    if [[ ! "$confirm_deploy" =~ ^[Yy]$ ]]; then
        warning "部署已取消"
        exit 0
    fi

    echo ""
    log "开始部署..."
    echo ""

    # 运行部署脚本
    bash "${SCRIPT_DIR}/deploy.sh"

    echo ""
    log "=========================================="
    log "部署完成！"
    log "=========================================="
    echo ""
    info "结果文件:"
    echo "  • 订阅链接: ${SCRIPT_DIR}/subscription_links.txt"
    echo "  • 部署摘要: ${SCRIPT_DIR}/deployment_summary.txt"
    echo "  • 详细日志: ${SCRIPT_DIR}/deploy_*.log"
    echo ""

    if [ -f "${SCRIPT_DIR}/subscription_links.txt" ]; then
        log "订阅链接预览:"
        echo ""
        head -20 "${SCRIPT_DIR}/subscription_links.txt"
        echo ""
    fi

    info "下一步:"
    echo "  1. 查看完整订阅链接: cat ${SCRIPT_DIR}/subscription_links.txt"
    echo "  2. 在客户端中导入订阅链接"
    echo "  3. 测试连接"
    echo ""

    log "感谢使用！🚀"
}

# 主函数
main() {
    cd "$SCRIPT_DIR"
    interactive_setup
}

main "$@"
