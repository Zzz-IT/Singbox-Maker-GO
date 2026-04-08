package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var InitSystem string

// DetectInitSystem 替代 _detect_init_system
func DetectInitSystem() {
	if _, err := exec.LookPath("systemctl"); err == nil {
		InitSystem = "systemd"
	} else if _, err := exec.LookPath("rc-service"); err == nil {
		InitSystem = "openrc"
	} else {
		InitSystem = "unknown"
	}
}

// CheckServiceStatus 检查服务运行状态
func CheckServiceStatus(serviceName string) bool {
	if InitSystem == "systemd" {
		cmd := exec.Command("systemctl", "is-active", "--quiet", serviceName)
		return cmd.Run() == nil
	} else if InitSystem == "openrc" {
		cmd := exec.Command("rc-service", serviceName, "status")
		out, _ := cmd.CombinedOutput()
		return strings.Contains(string(out), "started")
	}
	return false
}

// ManageService 替代 _manage_service
// ManageService 替代 _manage_service
func ManageService(action string) {
	var err error
	var out []byte

	if InitSystem == "systemd" {
		out, err = exec.Command("systemctl", action, "sing-box").CombinedOutput()
	} else if InitSystem == "openrc" {
		out, err = exec.Command("rc-service", "sing-box", action).CombinedOutput()
	}

	// 1. 捕获并输出错误信息
	if err != nil {
		LogError("sing-box 服务 %s 失败!\n错误详情: %s", action, strings.TrimSpace(string(out)))
		return
	}

	// 2. 正常运行则输出成功提示，并顺便把动作汉化一下
	if action != "status" {
		actionName := action
		switch action {
		case "restart":
			actionName = "重启"
		case "start":
			actionName = "启动"
		case "stop":
			actionName = "停止"
		}
		LogSuccess("sing-box 服务已%s", actionName)
	}
}

// GetOSName 读取真实操作系统名称 (如 Debian, Ubuntu, Alpine)
func GetOSName() string {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "Linux (Unknown)"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			name := strings.TrimPrefix(line, "PRETTY_NAME=")
			name = strings.Trim(name, `"'`) // 去除引号
			return name
		}
	}
	return "Linux"
}

// CheckArgoStatus 动态检查 Argo 隧道的安装与运行状态
func CheckArgoStatus() string {
	// 1. 检查是否安装
	if _, err := os.Stat("/usr/local/bin/cloudflared"); os.IsNotExist(err) {
		return fmt.Sprintf("%s○ Not Installed%s", ColorGrey, ColorReset)
	}

	// 2. 检查是否有进程在运行
	cmd := exec.Command("pgrep", "-f", "cloudflared")
	if err := cmd.Run(); err == nil {
		return fmt.Sprintf("%s● Running%s", ColorGreen, ColorReset)
	}

	// 安装了但没运行
	return fmt.Sprintf("%s○ Stopped%s", ColorRed, ColorReset)
}
