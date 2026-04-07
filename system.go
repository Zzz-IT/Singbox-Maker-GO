package main

import (
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
