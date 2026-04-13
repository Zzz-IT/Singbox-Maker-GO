package main

import (
	"os"
	"os/exec"
)

// InitRuntime 全局环境初始化（在 main 函数最开始调用）
func InitRuntime() {
	// 1. 识别初始化系统 (systemd 或 openrc)
	DetectInitSystem()

	// ================= 修改开始 =================
	// 2. 修复 Alpine 环境兼容性 (独立安装证书与兼容层)
	if _, err := os.Stat("/sbin/apk"); err == nil {
		// 独立且强制安装网络请求必要的证书和时区数据，防止 GitHub API 请求因 HTTPS 报错
		LogInfo("检测到 Alpine Linux，正在确保系统根证书已安装...")
		exec.Command("apk", "add", "--no-cache", "ca-certificates", "tzdata").Run()

		// 检查是否已经安装 gcompat，如果没有则静默安装
		if err := exec.Command("apk", "info", "-e", "gcompat").Run(); err != nil {
			LogInfo("正在静默安装 glibc 兼容层 (gcompat)...")
			if err := exec.Command("apk", "add", "--no-cache", "gcompat").Run(); err != nil {
				LogError("gcompat 安装失败，可能导致核心无法运行: %v", err)
			}
		}
	}
	// 3. 确保配置目录和基础配置文件存在
	if err := os.MkdirAll("/usr/local/etc/sing-box", 0755); err != nil {
		LogError("创建配置目录失败，请检查系统权限: %v", err)
	}
	if _, err := os.Stat(ConfigFile); os.IsNotExist(err) {
		os.WriteFile(ConfigFile, []byte(`{"inbounds":[],"outbounds":[{"type":"direct","tag":"direct"}],"route":{"rules":[],"final":"direct"}}`), 0644)
	}
	if _, err := os.Stat(MetadataFile); os.IsNotExist(err) {
		os.WriteFile(MetadataFile, []byte(`{}`), 0644)
	}

	// 4. 检查并生成系统守护服务文件 (Systemd / OpenRC)
	GenerateServiceFiles()

	// 5. 检查 Sing-box 核心是否存在且有效 (防止0字节坏文件欺骗系统)
	fileInfo, err := os。Stat("/usr/local/bin/sing-box")
	// 如果文件不存在、是个被误创的文件夹、或者文件大小异常(小于1MB)，则重新拉取
	if os.IsNotExist(err) || fileInfo.IsDir() || fileInfo.Size() < 1024*1024 {
		LogInfo("检测到核心文件缺失或损坏，正在自动拉取 Sing-box 核心组件...")
		UpdateCore(false) // 传入 false 代表这是后台自动初始化，不需要按回车暂停
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
