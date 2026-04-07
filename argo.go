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

// StartArgoTunnel 后台启动隧道并提取域名 (不受 Go 进程退出影响)
func StartArgoTunnel(port int, tunnelType string, token string) (string, error) {
	LogInfo("正在启动 Argo 隧道 (目标端口: %d)...", port)

	if tunnelType == "fixed" {
		// 【修复】直接使用 exec.Command 传参，避免 sh -c 导致的 Shell 注入风险
		cmd := exec.Command(CloudflaredBin, "tunnel", "run", "--token", token)
		// 脱离终端标准输出和错误输出，实现静默后台运行
		cmd.Stdout = nil
		cmd.Stderr = nil

		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("启动固定隧道失败: %v", err)
		}
		time.Sleep(2 * time.Second)
		return "", nil
	}

	// 临时隧道逻辑：
	// 【修复】将日志存放路径从危险的 /tmp 移动到你的专属配置目录下，防止符号链接攻击
	logFile := fmt.Sprintf("/usr/local/etc/sing-box/argo_%d.log", port)
	os.Remove(logFile)

	// 安全创建并打开日志文件，用于重定向子进程的输出
	outFile, err := os.Create(logFile)
	if err != nil {
		return "", fmt.Errorf("创建临时日志文件失败: %v", err)
	}

	// 【修复】不使用 sh -c，直接传递参数防止注入
	urlParam := fmt.Sprintf("http://127.0.0.1:%d", port)
	cmd := exec.Command(CloudflaredBin, "tunnel", "--url", urlParam)
	cmd.Stdout = outFile
	cmd.Stderr = outFile

	if err := cmd.Start(); err != nil {
		outFile.Close()
		return "", fmt.Errorf("启动临时隧道失败: %v", err)
	}
	// 父进程关闭文件句柄，子进程会继续持有并写入
	outFile.Close()

	// 轮询抓取 trycloudflare 域名 (最长等待 15 秒)
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
			LogSuccess("已自动为您提取出纯 Token！")
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
			exec.Command("pkill", "-f", "cloudflared").Run()
			LogSuccess("已向所有 cloudflared 进程发送停止信号")
			Pause("按回车键继续...")
		case "6", "06":
			// 简单的重启逻辑：杀掉全部进程，然后系统重启或者用户手动进入节点详情触发重连
			LogWarn("暂不支持一键热重启，若遇到断连请直接重启 sing-box 服务。")
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

		// 2. 杀掉对应的 cloudflared 进程
		if targetMeta["tunnel_type"] == "fixed" {
			exec.Command("pkill", "-f", targetMeta["token"].(string)).Run()
		} else {
			exec.Command("pkill", "-f", fmt.Sprintf("127.0.0.1:%.0f", targetMeta["local_port"])).Run()
		}

		// 3. 从元数据中删除
		delete(metaRoot, targetTag)
		out, _ := json.MarshalIndent(metaRoot, "", "  ")
		os.WriteFile(ArgoMetadataFile, out, 0644)

		LogSuccess("节点已彻底删除")
		ManageService("restart")
	}
}
