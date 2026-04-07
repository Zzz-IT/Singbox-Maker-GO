package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const ArgoMetadataFile = "/usr/local/etc/sing-box/argo_metadata.json"
const CloudflaredBin = "/usr/local/bin/cloudflared"

// InstallCloudflared 自动安装 cloudflared
func InstallCloudflared() bool {
	if _, err := os.Stat(CloudflaredBin); err == nil {
		return true
	}
	LogInfo("正在安装 cloudflared...")
	arch := "amd64"
	if out, _ := exec.Command("uname", "-m").Output(); strings.Contains(string(out), "aarch64") {
		arch = "arm64"
	}
	url := fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-%s", arch)
	cmd := exec.Command("wget", "-qO", CloudflaredBin, url)
	if err := cmd.Run(); err != nil {
		LogError("cloudflared 下载失败")
		return false
	}
	os.Chmod(CloudflaredBin, 0755)
	LogSuccess("cloudflared 安装成功")
	return true
}

// StartArgoTunnel 启动隧道并获取域名（完美替代原版的 awk 管道）
func StartArgoTunnel(port int, tunnelType string, token string) (string, error) {
	LogInfo("正在启动 Argo 隧道 (端口: %d)...", port)

	if tunnelType == "fixed" {
		cmd := exec.Command("nohup", CloudflaredBin, "tunnel", "run", "--token", token)
		cmd.Start()
		time.Sleep(3 * time.Second) // 等待启动
		return "", nil
	}

	// 临时隧道逻辑
	urlArg := fmt.Sprintf("http://localhost:%d", port)
	cmd := exec.Command(CloudflaredBin, "tunnel", "--url", urlArg)
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	
	if err := cmd.Start(); err != nil {
		return "", err
	}

	// 利用 Go 的 bufio 安全读取实时日志，提取 trycloudflare.com
	domainChan := make(chan string)
	go func() {
		scanner := bufio.NewScanner(stderr)
		re := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
		for scanner.Scan() {
			line := scanner.Text()
			if match := re.FindString(line); match != "" {
				domainChan <- strings.TrimPrefix(match, "https://")
				return
			}
		}
	}()

	select {
	case domain := <-domainChan:
		return domain, nil
	case <-time.After(15 * time.Second):
		cmd.Process.Kill()
		return "", fmt.Errorf("获取临时域名超时")
	}
}

// ShowArgoMenu Argo 隧道主菜单
func ShowArgoMenu() {
	for {
		ClearScreen()
		fmt.Print("\n\n\n")
		fmt.Printf("      %sA R G O   T U N N E L   M A N A G E R%s\n", ColorCyan, ColorReset)
		fmt.Printf("  %s─────────────────────────────────────────────%s\n\n", ColorGrey, ColorReset)
		fmt.Printf("  %s01.%s  部署 VLESS 隧道\n", ColorWhite, ColorReset)
		fmt.Printf("  %s02.%s  部署 Trojan 隧道\n\n", ColorWhite, ColorReset)
		fmt.Printf("  %s03.%s  查看节点详情\n", ColorWhite, ColorReset)
		fmt.Printf("  %s04.%s  删除配置节点\n\n", ColorWhite, ColorReset)
		fmt.Printf("  %s05.%s  停止所有服务\n", ColorWhite, ColorReset)
		fmt.Printf("  %s06.%s  卸载 Argo 模块\n\n", ColorWhite, ColorReset)
		fmt.Printf("  %s─────────────────────────────────────────────%s\n", ColorGrey, ColorReset)
		fmt.Printf("  %s00.%s  返回主菜单\n\n", ColorWhite, ColorReset)

		choice := ReadInput("  请输入选项 > ")
		switch choice {
		case "1", "01":
			LogInfo("Argo VLESS 部署功能 (调用添加节点逻辑，参考 nodes.go 并将 Listen 绑定 127.0.0.1)")
			Pause("按回车键继续...")
		case "3", "03":
			ViewArgoNodes()
			Pause("按回车键继续...")
		case "5", "05":
			exec.Command("pkill", "-f", "cloudflared").Run()
			LogSuccess("已停止所有 Argo 进程")
			Pause("按回车键继续...")
		case "0", "00":
			return
		}
	}
}

func ViewArgoNodes() {
	data, err := os.ReadFile(ArgoMetadataFile)
	if err != nil || len(data) == 0 {
		LogWarn("没有配置的 Argo 隧道")
		return
	}
	var root map[string]interface{}
	json.Unmarshal(data, &root)
	
	for tag, v := range root {
		meta := v.(map[string]interface{})
		fmt.Printf("─────────────────────────────────────────────\n")
		fmt.Printf("  节点: %s%s%s (Argo Tunnel)\n", ColorGreen, meta["name"], ColorReset)
		fmt.Printf("  端口: %.0f | 类型: %s\n", meta["local_port"], meta["type"])
		if domain, ok := meta["domain"].(string); ok && domain != "" {
			fmt.Printf("  域名: %s%s%s\n", ColorCyan, domain, ColorReset)
		}
	}
}
