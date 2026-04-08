package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const ArgoMetadataFile = "/usr/local/etc/sing-box/argo_metadata.json"
const CloudflaredBin = "/usr/local/bin/cloudflared"

// --- 底层支持函数 ---

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
	urlStr := fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-%s", arch)
	cmd := exec.Command("wget", "-qO", CloudflaredBin, urlStr)
	if err := cmd.Run(); err != nil {
		LogError("cloudflared 下载失败")
		return false
	}
	os.Chmod(CloudflaredBin, 0755)
	LogSuccess("cloudflared 安装成功")
	return true
}

// StartArgoTunnel 依赖 Systemd/OpenRC 启动隧道并提取域名，提供极致稳定性
func StartArgoTunnel(port int, tunnelType string, token string) (string, error) {
	LogInfo("正在配置 Argo 隧道系统守护服务 (端口: %d)...", port)

	logFile := fmt.Sprintf("/usr/local/etc/sing-box/argo_%d.log", port)
	os.Remove(logFile) // 清理旧日志，确保抓取最新域名

	// 预创建日志文件防止权限报错
	if f, err := os.Create(logFile); err == nil {
		f.Close()
		os.Chmod(logFile, 0666)
	}

	// 1. 生成并启动系统服务
	err := createAndStartArgoService(port, tunnelType, token, logFile)
	if err != nil {
		return "", err
	}

	if tunnelType == "fixed" {
		time.Sleep(2 * time.Second)
		return "", nil // 固定隧道直接返回即可
	}

	// 2. 轮询抓取 trycloudflare 临时免费域名
	LogInfo("正在等待 Cloudflare 分配域名...")
	re := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		content, err := os.ReadFile(logFile)
		if err == nil {
			if match := re.FindString(string(content)); match != "" {
				return strings.TrimPrefix(match, "https://"), nil
			}
		}
	}
	return "", fmt.Errorf("获取临时域名超时，请检查网络或 Cloudflare 限制")
}

// SaveArgoMetadata 独立保存 Argo 元数据
func SaveArgoMetadata(tag string, meta map[string]interface{}) {
	data, _ := os.ReadFile(ArgoMetadataFile)
	var root map[string]interface{}
	if len(data) > 0 {
		json.Unmarshal(data, &root)
	} else {
		root = make(map[string]interface{})
	}

	root[tag] = meta
	out, _ := json.MarshalIndent(root, "", "  ")
	os.WriteFile(ArgoMetadataFile, out, 0644)
}

// extractArgoToken 智能截取 Token，支持过滤整条 cloudflared 安装命令
func extractArgoToken(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	parts := strings.Fields(input) // 按空格分割

	// 1. 精准匹配：Cloudflare Tunnel Token 永远是 Base64 编码的 JSON，必然以 "eyJ" 开头
	for _, part := range parts {
		if strings.HasPrefix(part, "eyJ") {
			return part // 精准抓取，忽略其他所有无用字符
		}
	}

	// 2. 备用逻辑：如果用户贴了什么奇怪的命令，但包含了 cloudflared，默认最后一个词是 Token
	if strings.Contains(input, "cloudflared") || strings.Contains(input, "tunnel") {
		return parts[len(parts)-1]
	}

	// 3. 兜底逻辑：如果什么都没匹配到，假设用户直接输入的纯 Token
	return input
}

// --- 节点部署逻辑 ---

func deployArgoNode(nodeType string) {
	LogInfo(" 创建 Argo + %s 节点 ", strings.ToUpper(nodeType))

	// 1. 获取基础参数
	port := getValidPort()
	name := ReadInput(fmt.Sprintf("名称 (默认 Argo-%s): ", strings.ToUpper(nodeType)))
	if name == "" {
		name = fmt.Sprintf("Argo-%s", strings.ToUpper(nodeType))
	}
	wsPath := ReadInput("WS路径 (回车随机): ")
	if wsPath == "" {
		wsPath = "/" + helperRandHex(4)
	}
	if !strings.HasPrefix(wsPath, "/") {
		wsPath = "/" + wsPath
	}

	// 2. 选择隧道模式
	fmt.Println("\n  1) 临时隧道 (由 CF 自动分配 trycloudflare 免费域名)")
	fmt.Println("  2) 固定隧道 (需要提供 Cloudflare Tunnel Token 及绑定的域名)")
	tChoice := ReadInput("请选择模式 (默认 1): ")

	tunnelType := "temp"
	token := ""
	fixedDomain := ""

	if tChoice == "2" {
		tunnelType = "fixed"

		// 提示用户可以直接粘贴长命令
		rawTokenInput := ReadInput("请输入 Cloudflare Tunnel Token (支持直接粘贴完整安装命令): ")

		// 调用智能截取
		token = extractArgoToken(rawTokenInput)

		if token != "" && token != rawTokenInput {
			LogSuccess("已自动提取出纯 Token！")
		}

		fixedDomain = ReadInput("请输入绑定的域名 (如 argo.xxx.com): ")
		if token == "" || fixedDomain == "" {
			LogError("固定隧道必须提供 Token 和域名")
			return
		}
		LogWarn("注意: 固定隧道需在 Cloudflare 仪表盘中将路由指向 http://127.0.0.1:%d", port)
	}

	// 3. 构建本地入站配置 (绑定 127.0.0.1，不暴露公网，无需本地 TLS)
	tag := fmt.Sprintf("argo_%s_%d", nodeType, port)
	var inbound map[string]interface{}
	var uuid, password string

	if nodeType == "vless" {
		uuid = GenerateUUID()
		inbound = map[string]interface{}{
			"type":        "vless",
			"tag":         tag,
			"listen":      "127.0.0.1",
			"listen_port": port,
			"users":       []map[string]interface{}{{"uuid": uuid, "flow": ""}},
			"transport":   map[string]interface{}{"type": "ws", "path": wsPath},
		}
	} else {
		password = helperRandHex(8)
		inbound = map[string]interface{}{
			"type":        "trojan",
			"tag":         tag,
			"listen":      "127.0.0.1",
			"listen_port": port,
			"users":       []map[string]interface{}{{"password": password}},
			"transport":   map[string]interface{}{"type": "ws", "path": wsPath},
		}
	}

	if AppendInbound(inbound) != nil {
		LogError("写入配置文件失败")
		return
	}

	// 4. 启动穿透隧道
	if !InstallCloudflared() {
		return
	}
	domain, err := StartArgoTunnel(port, tunnelType, token)
	if err != nil {
		LogError("隧道启动失败: %v", err)
		return
	}

	if tunnelType == "fixed" {
		domain = fixedDomain
	}

	// 5. 存储元数据以便后续查看和删除
	meta := map[string]interface{}{
		"name": name, "type": nodeType, "local_port": port, "tunnel_type": tunnelType,
		"domain": domain, "token": token, "uuid": uuid, "password": password, "ws_path": wsPath,
	}
	SaveArgoMetadata(tag, meta)

	// 6. 生成并输出分享链接 (Cloudflare 默认提供 443 端口的 TLS)
	var link string
	if nodeType == "vless" {
		link = fmt.Sprintf("vless://%s@%s:443?encryption=none&security=tls&type=ws&host=%s&path=%s#%s",
			uuid, domain, domain, url.QueryEscape(wsPath), url.QueryEscape(name))
	} else {
		link = fmt.Sprintf("trojan://%s@%s:443?security=tls&type=ws&host=%s&path=%s#%s",
			password, domain, domain, url.QueryEscape(wsPath), url.QueryEscape(name))
	}

	// ==========================================
	// 6. 生成并同步配置 (整合 ECH 逻辑)
	// ==========================================
	echEnabled := GetClientECH()
	sniParam := domain
	if echEnabled {
		sniParam = "cloudflare-ech.com"
	}

	clashProxy := map[string]interface{}{
		"name":             name,
		"server":           domain,
		"port":             443,
		"tls":              true,
		"network":          "ws",
		"servername":       domain,
		"skip-cert-verify": false,
	}

	if echEnabled {
		clashProxy["client-fingerprint"] = "chrome"
		clashProxy["ech-opts"] = map[string]interface{}{
			"enable":            true,
			"query-server-name": "cloudflare-ech.com",
		}
	}

	if nodeType == "vless" {
		clashProxy["type"] = "vless"
		clashProxy["uuid"] = uuid
		clashProxy["udp"] = true
		clashProxy["ws-opts"] = map[string]interface{}{
			"path":    wsPath,
			"headers": map[string]interface{}{"Host": domain},
		}
	} else {
		clashProxy["type"] = "trojan"
		clashProxy["password"] = password
		clashProxy["udp"] = true
		clashProxy["sni"] = domain
		clashProxy["ws-opts"] = map[string]interface{}{
			"path":    wsPath,
			"headers": map[string]interface{}{"Host": domain},
		}
	}
	AddNodeToYaml(clashProxy)

	// 最终生成分享链接
	if nodeType == "vless" {
		link = fmt.Sprintf("vless://%s@%s:443?encryption=none&security=tls&type=ws&host=%s&path=%s&sni=%s&fp=chrome#%s",
			uuid, domain, domain, url.QueryEscape(wsPath), sniParam, url.QueryEscape(name))
	} else {
		link = fmt.Sprintf("trojan://%s@%s:443?security=tls&type=ws&host=%s&path=%s&sni=%s&fp=chrome#%s",
			password, domain, domain, url.QueryEscape(wsPath), sniParam, url.QueryEscape(name))
	}

	LogSuccess("Argo 节点 [%s] 部署完成！", name)
	if echEnabled {
		fmt.Printf("%s(已开启客户端 ECH 注入保护)%s\n", ColorYellow, ColorReset)
	}
	fmt.Printf("分配域名: %s\n", domain)
	fmt.Printf("\n%s--- 分享链接 ---%s\n%s%s%s\n", ColorYellow, ColorReset, ColorCyan, link, ColorReset)
	ManageService("restart")
}

// --- 菜单与管理模块 ---

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
		fmt.Printf("  %s05.%s  停止所有隧道\n", ColorWhite, ColorReset)
		fmt.Printf("  %s06.%s  重启所有隧道\n\n", ColorWhite, ColorReset)
		fmt.Printf("  %s─────────────────────────────────────────────%s\n", ColorGrey, ColorReset)
		fmt.Printf("  %s00.%s  返回主菜单\n\n", ColorWhite, ColorReset)

		choice := ReadInput("  请输入选项 > ")
		switch choice {
		case "1", "01":
			deployArgoNode("vless")
			Pause("按回车键继续...")
		case "2", "02":
			deployArgoNode("trojan")
			Pause("按回车键继续...")
		case "3", "03":
			ViewArgoNodes()
			Pause("按回车键继续...")
		case "4", "04":
			DeleteArgoNode()
			Pause("按回车键继续...")
		case "5", "05":
			StopAllArgoTunnels()
			Pause("按回车键继续...")
		case "6", "06":
			RestartAllArgoTunnels()
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

	if len(root) == 0 {
		LogWarn("没有配置的 Argo 隧道")
		return
	}

	for _, v := range root {
		meta := v.(map[string]interface{})
		tType := "临时免费"
		if meta["tunnel_type"] == "fixed" {
			tType = "Token绑定"
		}

		fmt.Printf("─────────────────────────────────────────────\n")
		fmt.Printf("  节点: %s%s%s (Argo %s) | 模式: %s\n", ColorGreen, meta["name"], ColorReset, strings.ToUpper(meta["type"].(string)), tType)
		fmt.Printf("  本地监听: 127.0.0.1:%.0f | WS路径: %s\n", meta["local_port"], meta["ws_path"])
		if domain, ok := meta["domain"].(string); ok && domain != "" {
			fmt.Printf("  穿透域名: %s%s%s\n", ColorCyan, domain, ColorReset)

			// 还原分享链接
			var link string
			if meta["type"] == "vless" {
				link = fmt.Sprintf("vless://%s@%s:443?encryption=none&security=tls&type=ws&host=%s&path=%s#%s",
					meta["uuid"], domain, domain, url.QueryEscape(meta["ws_path"].(string)), url.QueryEscape(meta["name"].(string)))
			} else {
				link = fmt.Sprintf("trojan://%s@%s:443?security=tls&type=ws&host=%s&path=%s#%s",
					meta["password"], domain, domain, url.QueryEscape(meta["ws_path"].(string)), url.QueryEscape(meta["name"].(string)))
			}
			fmt.Printf("  链接: %s%s%s\n", ColorYellow, link, ColorReset)
		}
	}
}

func DeleteArgoNode() {
	data, err := os.ReadFile(ArgoMetadataFile)
	if err != nil || len(data) == 0 {
		LogWarn("没有配置的 Argo 隧道")
		return
	}
	var metaRoot map[string]interface{}
	json.Unmarshal(data, &metaRoot)

	if len(metaRoot) == 0 {
		LogWarn("没有配置的 Argo 隧道")
		return
	}

	var tags []string
	fmt.Printf("\n─────────────────────────────────────────────\n")
	i := 1
	for tag, v := range metaRoot {
		meta := v.(map[string]interface{})
		fmt.Printf("  %s%d)%s %s (本地端口: %.0f)\n", ColorCyan, i, ColorReset, meta["name"], meta["local_port"])
		tags = append(tags, tag)
		i++
	}

	choice := ReadInput("请输入要删除的编号 (0返回): ")
	if choice == "0" || choice == "" {
		return
	}

	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(tags) {
		LogError("无效的编号")
		return
	}

	targetTag := tags[idx-1]
	targetMeta := metaRoot[targetTag].(map[string]interface{})

	if ReadInput(fmt.Sprintf("%s确认删除 Argo 节点 [%s] 吗？(y/N): %s", ColorYellow, targetMeta["name"], ColorReset)) == "y" {
		// 1. 从 config.json 中删除
		conf, _ := ReadConfig()
		if inbounds, ok := conf["inbounds"].([]interface{}); ok {
			var newInbounds []interface{}
			for _, v := range inbounds {
				if inb, isMap := v.(map[string]interface{}); isMap && inb["tag"].(string) != targetTag {
					newInbounds = append(newInbounds, inb)
				}
			}
			conf["inbounds"] = newInbounds
			WriteConfig(conf)
		}

		// 2. 优雅停止并注销对应的 Argo 系统服务
		serviceName := fmt.Sprintf("argo-%.0f", targetMeta["local_port"])
		if InitSystem == "systemd" {
			exec.Command("systemctl", "stop", serviceName).Run()
			exec.Command("systemctl", "disable", serviceName).Run()
			os.Remove(fmt.Sprintf("/etc/systemd/system/%s.service", serviceName))
			exec.Command("systemctl", "daemon-reload").Run()
		} else if InitSystem == "openrc" {
			exec.Command("rc-service", serviceName, "stop").Run()
			exec.Command("rc-update", "del", serviceName, "default").Run()
			os.Remove(fmt.Sprintf("/etc/init.d/%s", serviceName))
		}

		// 清除隧道日志文件
		os.Remove(fmt.Sprintf("/usr/local/etc/sing-box/argo_%.0f.log", targetMeta["local_port"]))

		// 3. 从元数据中删除
		delete(metaRoot, targetTag)
		out, _ := json.MarshalIndent(metaRoot, "", "  ")
		os.WriteFile(ArgoMetadataFile, out, 0644)

		LogSuccess("节点已彻底删除")
		ManageService("restart")
	}
}

// ==========================================
// 系统服务级隧道管理模块 (守护进程 + GC优化)
// ==========================================

// createAndStartArgoService 动态生成并启动系统服务，包含 GC 内存控制限制
func createAndStartArgoService(port int, tunnelType string, token string, logFile string) error {
	serviceName := fmt.Sprintf("argo-%d", port)
	var cmdStr string

	if tunnelType == "fixed" {
		cmdStr = fmt.Sprintf("%s tunnel run --token %s", CloudflaredBin, token)
	} else {
		cmdStr = fmt.Sprintf("%s tunnel --url http://127.0.0.1:%d", CloudflaredBin, port)
	}

	if InitSystem == "systemd" {
		servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
		serviceContent := fmt.Sprintf(`[Unit]
Description=Argo Tunnel for Port %d
After=network.target

[Service]
Type=simple
Restart=always
RestartSec=5
Environment="GOMEMLIMIT=50MiB"
Environment="GOGC=50"
ExecStart=%s
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=multi-user.target
`, port, cmdStr, logFile, logFile)

		os.WriteFile(servicePath, []byte(serviceContent), 0644)
		exec.Command("systemctl", "daemon-reload").Run()
		exec.Command("systemctl", "enable", serviceName).Run()
		if err := exec.Command("systemctl", "restart", serviceName).Run(); err != nil {
			return fmt.Errorf("systemd 启动失败: %v", err)
		}
	} else if InitSystem == "openrc" {
		servicePath := fmt.Sprintf("/etc/init.d/%s", serviceName)
		serviceContent := fmt.Sprintf(`#!/sbin/openrc-run
description="Argo Tunnel for Port %d"
command="/bin/sh"
command_args="-c 'GOMEMLIMIT=50MiB GOGC=50 exec %s >> %s 2>&1'"
command_background="yes"
pidfile="/run/%s.pid"

depend() {
    need net
}
`, port, cmdStr, logFile, serviceName)

		os.WriteFile(servicePath, []byte(serviceContent), 0755)
		exec.Command("rc-update", "add", serviceName, "default").Run()
		if err := exec.Command("rc-service", serviceName, "restart").Run(); err != nil {
			return fmt.Errorf("openrc 启动失败: %v", err)
		}
	} else {
		return fmt.Errorf("未知的初始化系统，无法创建服务")
	}

	return nil
}

// RestartAllArgoTunnels 重新启动所有已保存的 Argo 隧道系统服务
func RestartAllArgoTunnels() {
	data, _ := os.ReadFile(ArgoMetadataFile)
	var metaRoot map[string]interface{}
	json.Unmarshal(data, &metaRoot)

	if len(metaRoot) == 0 {
		LogWarn("没有找到有效的 Argo 隧道配置。")
		return
	}

	LogInfo("正在通过系统服务重新启动隧道...")
	successCount := 0
	for tag, v := range metaRoot {
		meta := v.(map[string]interface{})
		port := int(meta["local_port"].(float64))
		serviceName := fmt.Sprintf("argo-%d", port)
		tunnelType, _ := meta["tunnel_type"].(string)
		logFile := fmt.Sprintf("/usr/local/etc/sing-box/argo_%d.log", port)

		// 修复：如果是临时隧道，重启前必须清理旧日志，否则会抓取到上一轮的旧域名
		if tunnelType == "temp" {
			os.Remove(logFile)
			if f, err := os.Create(logFile); err == nil {
				f.Close()
				os.Chmod(logFile, 0666)
			}
		}

		var err error
		if InitSystem == "systemd" {
			err = exec.Command("systemctl", "restart", serviceName).Run()
		} else if InitSystem == "openrc" {
			err = exec.Command("rc-service", serviceName, "restart").Run()
		}

		if err == nil {
			successCount++
			// 修复：针对临时隧道，重启后重新轮询抓取新的域名并更新元数据
			if tunnelType == "temp" {
				LogInfo("正在等待 Cloudflare 分配新临时域名...")
				re := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
				newDomain := ""
				for i := 0; i < 30; i++ {
					time.Sleep(500 * time.Millisecond)
					content, err := os.ReadFile(logFile)
					if err == nil {
						if match := re.FindString(string(content)); match != "" {
							newDomain = strings.TrimPrefix(match, "https://")
							break
						}
					}
				}

				if newDomain != "" {
					meta["domain"] = newDomain
					metaRoot[tag] = meta
					LogSuccess("临时隧道服务 [%s] 重启成功！分配新域名: %s", serviceName, newDomain)
				} else {
					LogError("临时隧道服务 [%s] 重启成功，但获取新域名超时！", serviceName)
				}
			} else {
				LogSuccess("固定隧道服务 [%s] 重启成功！", serviceName)
			}
		} else {
			LogError("隧道服务 [%s] 重启失败！", serviceName)
		}
	}

	// 修复：将抓取到的新域名写回配置文件，防止后续“查看节点详情”时依然显示死域名
	out, _ := json.MarshalIndent(metaRoot, "", "  ")
	os.WriteFile(ArgoMetadataFile, out, 0644)

	LogSuccess("操作完成！成功重启 %d 个隧道服务。", successCount)
}

// StopAllArgoTunnels 停止所有已保存的 Argo 隧道系统服务
func StopAllArgoTunnels() {
	data, _ := os.ReadFile(ArgoMetadataFile)
	var metaRoot map[string]interface{}
	json.Unmarshal(data, &metaRoot)

	if len(metaRoot) == 0 {
		LogWarn("没有正在运行的 Argo 隧道。")
		return
	}

	for _, v := range metaRoot {
		meta := v.(map[string]interface{})
		port := int(meta["local_port"].(float64))
		serviceName := fmt.Sprintf("argo-%d", port)

		if InitSystem == "systemd" {
			exec.Command("systemctl", "stop", serviceName).Run()
		} else if InitSystem == "openrc" {
			exec.Command("rc-service", serviceName, "stop").Run()
		}
	}
	LogSuccess("已向所有系统级隧道服务发送停止指令！")
}
