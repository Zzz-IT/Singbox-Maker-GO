package main

import (
	"fmt"
	"os"
	"os/exec"
)

// ViewLog 查看服务日志
func ViewLog() {
	ClearScreen()
	LogInfo("正在查看 sing-box 实时日志 (按 Ctrl+C 退出)...")
	var cmd *exec.Cmd
	if InitSystem == "systemd" {
		cmd = exec.Command("journalctl", "-u", "sing-box", "-f", "--no-pager")
	} else {
		cmd = exec.Command("tail", "-f", "/var/log/sing-box.log")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// CheckConfig 检查配置文件语法
func CheckConfig() {
	LogInfo("正在验证配置语法...")
	cmd := exec.Command("/usr/local/bin/sing-box", "check", "-c", ConfigFile)
	if err := cmd.Run(); err != nil {
		LogError("配置文件存在语法错误！")
	} else {
		LogSuccess("配置文件语法正确无误。")
	}
}

// UpdateCore 更新 Sing-box 核心
func UpdateCore() {
	LogInfo("准备更新 Sing-box 核心程序...")
	ManageService("stop")
	
	// 这里可以用原版的脚本逻辑或调用 bash 脚本来执行下载解压，为了保持纯 Go，这里做简化演示
	cmd := exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/Zzz-IT/-Singbox-Maker-Z/main/install.sh | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	
	if err != nil {
		LogError("核心更新失败")
	} else {
		LogSuccess("核心更新完成")
	}
	ManageService("start")
}

// Uninstall 卸载程序
func Uninstall() {
	if ReadInput(ColorRed+"警告：此操作将彻底删除配置和核心程序，确认卸载？(y/N): "+ColorReset) != "y" {
		return
	}
	ManageService("stop")
	if InitSystem == "systemd" {
		exec.Command("systemctl", "disable", "sing-box").Run()
	} else if InitSystem == "openrc" {
		exec.Command("rc-update", "del", "sing-box", "default").Run()
	}
	os.RemoveAll("/usr/local/etc/sing-box")
	os.Remove("/usr/local/bin/sing-box")
	os.Remove("/usr/local/bin/cloudflared")
	exec.Command("pkill", "-f", "cloudflared").Run()
	LogSuccess("卸载完成，感谢使用！")
	os.Exit(0)
}
