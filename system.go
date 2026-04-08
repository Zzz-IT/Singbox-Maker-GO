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

// ManageService 具备进程级自愈能力的服务管理器
func ManageService(action string) {
	var err error
	var out []byte

	// 首次尝试执行服务命令
	if InitSystem == "systemd" {
		out, err = exec.Command("systemctl", action, "sing-box").CombinedOutput()
	} else if InitSystem == "openrc" {
		out, err = exec.Command("rc-service", "sing-box", action).CombinedOutput()
	}

	// ==========================================
	// 拦截错误并执行自动化自愈 (Self-Healing)
	// ==========================================
	if err != nil {
		// 如果是启动或重启失败，极大概率是僵尸进程/锁文件导致
		if action == "start" || action == "restart" {
			LogWarn("检测到系统进程管理异常，面板正在触发自动自愈清理机制...")

			// 1. 暴力清理所有残留进程
			exec.Command("pkill", "-9", "-f", "sing-box").Run()

			// 2. 清理可能残留的 PID 锁文件
			os.Remove("/var/run/sing-box.pid")

			// 3. 重置系统服务管理器的状态
			if InitSystem == "openrc" {
				exec.Command("rc-service", "sing-box", "zap").Run()
			} else if InitSystem == "systemd" {
				exec.Command("systemctl", "reset-failed", "sing-box").Run()
			}

			LogInfo("残留环境清理完成，正在重新尝试 %s...", action)

			// 4. 第二次尝试执行服务命令
			if InitSystem == "systemd" {
				out, err = exec.Command("systemctl", action, "sing-box").CombinedOutput()
			} else if InitSystem == "openrc" {
				out, err = exec.Command("rc-service", "sing-box", action).CombinedOutput()
			}
		}

		// 如果抢救之后依然失败，再向用户报错
		if err != nil {
			LogError("sing-box 服务 %s 彻底失败!\n错误详情: %s", action, strings.TrimSpace(string(out)))
			return
		}
	}

	// 正常运行则输出成功提示，并顺便把动作汉化一下
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
