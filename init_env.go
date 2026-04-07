package main

import (
	"os"
	"os/exec"
)

// InitRuntime 全局环境初始化（在 main 函数最开始调用）
func InitRuntime() {
	// 1. 识别初始化系统 (systemd 或 openrc)
	DetectInitSystem()

	// 2. 修复 Alpine 环境兼容性 (补全 glibc 兼容层)
	if _, err := os.Stat("/sbin/apk"); err == nil {
		// 检查是否已经安装 gcompat，如果没有则静默安装
		if err := exec.Command("apk", "info", "-e", "gcompat").Run(); err != nil {
			LogInfo("检测到 Alpine Linux，正在静默安装兼容性依赖 (gcompat)...")
			exec.Command("apk", "add", "--no-cache", "gcompat", "ca-certificates", "tzdata").Run()
		}
	}

	// 3. 确保配置目录和基础配置文件存在
	os.MkdirAll("/usr/local/etc/sing-box", 0755)
	if _, err := os.Stat(ConfigFile); os.IsNotExist(err) {
		os.WriteFile(ConfigFile, []byte(`{"inbounds":[],"outbounds":[{"type":"direct","tag":"direct"}],"route":{"rules":[],"final":"direct"}}`), 0644)
	}
	if _, err := os.Stat(MetadataFile); os.IsNotExist(err) {
		os.WriteFile(MetadataFile, []byte(`{}`), 0644)
	}

	// 4. 检查并生成系统守护服务文件 (Systemd / OpenRC)
	GenerateServiceFiles()

	// 5. 检查 Sing-box 核心是否存在，不存在则自动下载
	if _, err := os.Stat("/usr/local/bin/sing-box"); os.IsNotExist(err) {
		LogInfo("初次运行，正在自动拉取 Sing-box 核心组件...")
		UpdateCore()
	}
}

// GenerateServiceFiles 生成 Systemd 或 OpenRC 的服务卡片
func GenerateServiceFiles() {
	if InitSystem == "systemd" {
		servicePath := "/etc/systemd/system/sing-box.service"
		if _, err := os.Stat(servicePath); os.IsNotExist(err) {
			content := `[Unit]
Description=sing-box service
After=network.target nss-lookup.target

[Service]
Type=simple
ExecStart=/usr/local/bin/sing-box run -c /usr/local/etc/sing-box/config.json
Restart=on-failure
RestartSec=3s
LimitNOFILE=infinity

# 开启内存记账
MemoryAccounting=yes
# 软限制：当内存达到 80M 时，系统会尽量压榨清理它的缓存
MemoryHigh=80M
# 硬限制：当内存达到 100M 时，直接在沙盒内重启该服务，绝不波及宿主机导致死机
MemoryMax=100M

[Install]
WantedBy=multi-user.target
`
			os.WriteFile(servicePath, []byte(content), 0644)
			exec.Command("systemctl", "daemon-reload").Run()
			exec.Command("systemctl", "enable", "sing-box").Run()
			LogSuccess("Systemd 服务守护文件已生成")
		}
	} else if InitSystem == "openrc" {
		servicePath := "/etc/init.d/sing-box"
		if _, err := os.Stat(servicePath); os.IsNotExist(err) {
			content := `#!/sbin/openrc-run
description="sing-box service"
command="/usr/local/bin/sing-box"
command_args="run -c /usr/local/etc/sing-box/config.json"
supervisor="supervise-daemon"
respawn_delay=3
respawn_max=0
pidfile="/var/run/sing-box.pid"
output_log="/var/log/sing-box.log"
error_log="/var/log/sing-box.log"

depend() {
    need net
    after firewall
    use dns logger
}
`
			os.WriteFile(servicePath, []byte(content), 0755)
			exec.Command("rc-update", "add", "sing-box", "default").Run()
			LogSuccess("OpenRC 服务守护文件已生成")
		}
	}
}
