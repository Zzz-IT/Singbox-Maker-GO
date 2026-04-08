package main

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strings"
)

// --- 颜色定义 (对标 lib/log.sh) ---
const (
	ColorRed    = "\033[1;31m"
	ColorGreen  = "\033[0;32m"
	ColorYellow = "\033[0;33m"
	ColorCyan   = "\033[0;36m"
	ColorWhite  = "\033[1;37m"
	ColorGrey   = "\033[0;37m"
	ColorReset  = "\033[0m"
)

// --- 日志打印 (对标 lib/log.sh) ---
func LogInfo(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s[信息] %s%s\n", ColorCyan, msg, ColorReset)
}

func LogSuccess(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s[成功] %s%s\n", ColorGreen, msg, ColorReset)
}

func LogWarn(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s[注意] %s%s\n", ColorYellow, msg, ColorReset)
}

func LogError(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s[错误] %s%s\n", ColorRed, msg, ColorReset)
}

// --- 辅助输入功能 ---
// ReadInput 替代 read -p
func ReadInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// Pause 替代 read -n 1 -p "按任意键继续..."
func Pause(prompt string) {
	fmt.Printf("\n  %s%s%s\n", ColorGrey, prompt, ColorReset)
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

// ClearScreen 清屏
func ClearScreen() {
	fmt.Print("\033c")
}

// CheckRoot 替代 _check_root
func CheckRoot() {
	currentUser, err := user.Current()
	if err != nil || currentUser.Uid != "0" {
		LogError("此脚本必须以 root 权限运行。")
		os.Exit(1)
	}
}

// FormatIPForURI 确保纯 IPv6 地址在 URI 中被方括号包裹
func FormatIPForURI(ip string) string {
	if strings.Contains(ip, ":") && !strings.HasPrefix(ip, "[") {
		return "[" + ip + "]"
	}
	return ip
}

// AtomicWriteFile 原子化写入文件，防止断电或磁盘满导致文件损坏
func AtomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	tmpFile := filename + ".tmp"
	// 1. 先写入临时文件
	if err := os.WriteFile(tmpFile, data, perm); err != nil {
		return err
	}
	// 2. 写入成功后，原子级重命名覆盖原文件
	return os.Rename(tmpFile, filename)
}
