#!/usr/bin/env bash
set -euo pipefail

# 颜色定义
RED='\033[1;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

echo -e "${CYAN}[信息]${NC} 正在初始化 Singbox Maker GO..."

# 1. 检查 Root 权限
if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo -e "${RED}[错误]${NC} 请使用 root 权限运行此安装脚本。" >&2
  exit 1
fi

# 2. 识别系统架构
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) GOARCH="amd64" ;;
    aarch64|arm64) GOARCH="arm64" ;;
    *) echo -e "${RED}[错误]${NC} 不支持的架构: $ARCH" >&2; exit 1 ;;
esac

echo -e "${CYAN}[信息]${NC} 匹配到系统架构: ${WHITE}${GOARCH}${NC}"

# 3. 变量定义 (请确保 Github 仓库路径正确)
REPO="Zzz-IT/singbox-maker-go"
BRANCH="main"
BINARY_NAME="sbgo-${GOARCH}"
DOWNLOAD_URL="https://raw.githubusercontent.com/${REPO}/refs/heads/${BRANCH}/${BINARY_NAME}"
BIN_PATH="/usr/local/bin/sb"

# 4. 清理可能存在的旧版 Shell 遗留软链接或目录
if [[ -L "$BIN_PATH" || -f "$BIN_PATH" ]]; then
    echo -e "${CYAN}[信息]${NC} 正在清理旧版本..."
    rm -f "$BIN_PATH"
fi

# 5. 下载核心程序
echo -e "${CYAN}[信息]${NC} 正在拉取核心控制台程序..."

if command -v curl >/dev/null 2>&1; then
    if ! curl -L -f -s -o "$BIN_PATH" "$DOWNLOAD_URL"; then
        echo -e "${RED}[错误]${NC} 下载失败，请检查网络或确认 ${BINARY_NAME} 已上传至 Github。" >&2
        exit 1
    fi
elif command -v wget >/dev/null 2>&1; then
    if ! wget -q -O "$BIN_PATH" "$DOWNLOAD_URL"; then
         echo -e "${RED}[错误]${NC} 下载失败，请检查网络或确认 ${BINARY_NAME} 已上传至 Github。" >&2
         exit 1
    fi
else
    echo -e "${RED}[错误]${NC} 系统缺少 curl 或 wget，无法下载程序。" >&2
    exit 1
fi

# 6. 赋予执行权限
chmod +x "$BIN_PATH"

echo -e "${GREEN}[成功]${NC} Singbox Maker GO 安装完成！"
echo -e "${YELLOW}[提示]${NC} 请在终端直接输入 ${WHITE}sb${NC} ${YELLOW}即可呼出管理菜单。${NC}"
