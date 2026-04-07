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
func ManageService(action string) {
	if InitSystem == "systemd" {
		exec.Command("systemctl", action, "sing-box").Run()
	} else if InitSystem == "openrc" {
		exec.Command("rc-service", "sing-box", action).Run()
	}
	if action != "status" {
		LogSuccess("sing-box 服务已 %s", action)
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
