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

// ManageService 具备彻底清理能力的稳妥服务管理器
func ManageService(action string) {
	var err error
	var out []byte
	originalAction := action

	// 如果动作是 restart 或 stop，第一步都是“优雅停止 + 强制补刀清理”
	if action == "restart" || action == "stop" {
		// 1. 尝试正常停止服务
		if InitSystem == "systemd" {
			exec.Command("systemctl", "stop", "sing-box").Run()
		} else if InitSystem == "openrc" {
			exec.Command("rc-service", "sing-box", "stop").Run()
		}

		// 2. 深度清理：干掉所有残留进程、清掉锁文件、重置系统状态
		exec.Command("pkill", "-9", "-f", "sing-box").Run()
		os.Remove("/var/run/sing-box.pid")
		if InitSystem == "openrc" {
			exec.Command("rc-service", "sing-box", "zap").Run()
		} else if InitSystem == "systemd" {
			exec.Command("systemctl", "reset-failed", "sing-box").Run()
		}

		// 3. 动作分流
		if action == "stop" {
			// 如果用户的目的只是停止，清理完就可以直接宣告成功并返回了
			LogSuccess("sing-box 服务已停止")
			return
		} else {
			// 如果用户的目的是重启，清理完后将接下来的动作替换为启动
			action = "start"
		}
	}

	// 此时只会剩下 start 或 status 动作需要向系统发送指令
	if InitSystem == "systemd" {
		out, err = exec.Command("systemctl", action, "sing-box").CombinedOutput()
	} else if InitSystem == "openrc" {
		out, err = exec.Command("rc-service", "sing-box", action).CombinedOutput()
	}

	// 错误处理与输出
	if err != nil {
		LogError("sing-box 服务 %s 失败!\n错误详情: %s", originalAction, strings.TrimSpace(string(out)))
		return
	}

	// 成功提示汉化
	if originalAction != "status" {
		actionName := originalAction
		if originalAction == "start" {
			actionName = "启动"
		} else if originalAction == "restart" {
			actionName = "重启"
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
