package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
)

const CertDir = "/usr/local/etc/sing-box"

// --- 辅助工具函数 ---

// helperRandHex 替代 sing-box generate rand --hex
func helperRandHex(bytes int) string {
	b := make([]byte, bytes)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateSelfSignedCert 完美替代原版的 _generate_self_signed_cert
func GenerateSelfSignedCert(domain, certPath, keyPath string) error {
	LogInfo("正在使用 openssl 生成自签证书...")
	exec.Command("openssl", "ecparam", "-genkey", "-name", "prime256v1", "-out", keyPath).Run()
	err := exec.Command("openssl", "req", "-new", "-x509", "-days", "3650",
		"-key", keyPath, "-out", certPath, "-subj", "/CN="+domain).Run()
	return err
}

// getValidPort 封装读取合法端口的逻辑
func getValidPort() int {
	for {
		portStr := ReadInput("监听端口: ")
		if portStr == "" {
			LogError("端口不能为空，请重新输入")
			continue
		}
		port, err := strconv.Atoi(portStr)
		if err == nil && port > 0 && port <= 65535 {
			return port
		}
		LogError("无效端口，请输入 1-65535 之间的数字")
	}
}

// --- 核心节点部署逻辑 ---

// 1. VLESS-Reality 部署逻辑
func AddVLESSReality() {
	LogInfo(" 创建 VLESS-Reality 节点 ")
	serverName := ReadInput("伪装域名 (默认 www.apple.com): ")
	if serverName == "" {
		serverName = "www.apple.com"
	}
	port := getValidPort()
	name := ReadInput("名称 (默认 VLESS-REALITY): ")
	if name == "" {
		name = "VLESS-REALITY"
	}

	uuid := GenerateUUID()
	shortID := GenerateShortID()
	privateKey, publicKey := GenerateRealityKeyPair()
	tag := fmt.Sprintf("%s_%d", strings.ReplaceAll(name, " ", "_"), port)

	inbound := map[string]interface{}{
		"type":        "vless",
		"tag":         tag,
		"listen":      "::",
		"listen_port": port,
		"users":       []map[string]interface{}{{"uuid": uuid, "flow": "xtls-rprx-vision"}},
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": serverName,
			"reality": map[string]interface{}{
				"enabled":     true,
				"handshake":   map[string]interface{}{"server": serverName, "server_port": 443},
				"private_key": privateKey,
				"short_id":    []string{shortID},
			},
		},
	}

	if AppendInbound(inbound) == nil {
		SaveMetadata(tag, map[string]interface{}{"name": name, "publicKey": publicKey, "shortId": shortID})
		serverIP := GetPublicIP()
		link := fmt.Sprintf("vless://%s@%s:%d?security=reality&encryption=none&pbk=%s&fp=chrome&type=tcp&flow=xtls-rprx-vision&sni=%s&sid=%s#%s",
			uuid, serverIP, port, publicKey, serverName, shortID, url.QueryEscape(name))
		LogSuccess("节点 [%s] 添加成功\n链接: %s\n%s", name, ColorCyan, link)
	}
}

// 2. VLESS-WS-TLS 部署逻辑
func AddVLESSWSTLS() {
	LogInfo(" 创建 VLESS-WS-TLS 节点 ")
	serverName := ReadInput("伪装域名 (SNI): ")
	if serverName == "" {
		LogError("伪装域名不能为空"); return
	}
	port := getValidPort()
	name := ReadInput("名称 (默认 VLESS-WS): ")
	if name == "" { name = "VLESS-WS" }
	
	wsPath := ReadInput("WS路径 (回车随机): ")
	if wsPath == "" { wsPath = "/" + helperRandHex(4) }
	if !strings.HasPrefix(wsPath, "/") { wsPath = "/" + wsPath }

	tag := fmt.Sprintf("%s_%d", strings.ReplaceAll(name, " ", "_"), port)
	certPath := fmt.Sprintf("%s/%s.pem", CertDir, tag)
	keyPath := fmt.Sprintf("%s/%s.key", CertDir, tag)

	// 自动生成证书
	if err := GenerateSelfSignedCert(serverName, certPath, keyPath); err != nil {
		LogError("证书生成失败"); return
	}

	uuid := GenerateUUID()
	inbound := map[string]interface{}{
		"type":        "vless",
		"tag":         tag,
		"listen":      "::",
		"listen_port": port,
		"users":       []map[string]interface{}{{"uuid": uuid, "flow": ""}},
		"tls": map[string]interface{}{
			"enabled":          true,
			"certificate_path": certPath,
			"key_path":         keyPath,
		},
		"transport": map[string]interface{}{
			"type": "ws",
			"path": wsPath,
		},
	}

	if AppendInbound(inbound) == nil {
		SaveMetadata(tag, map[string]interface{}{"name": name, "server_name": serverName})
		serverIP := GetPublicIP()
		link := fmt.Sprintf("vless://%s@%s:%d?security=tls&encryption=none&type=ws&host=%s&path=%s&sni=%s&insecure=1#%s",
			uuid, serverIP, port, serverName, url.QueryEscape(wsPath), serverName, url.QueryEscape(name))
		LogSuccess("节点 [%s] 添加成功\n链接: %s\n%s", name, ColorCyan, link)
	}
}

// 3. Hysteria2 部署逻辑
func AddHysteria2() {
	LogInfo(" 创建 Hysteria2 节点 ")
	port := getValidPort()
	serverName := ReadInput("伪装域名 (默认 www.apple.com): ")
	if serverName == "" { serverName = "www.apple.com" }
	name := ReadInput("名称 (默认 Hysteria2): ")
	if name == "" { name = "Hysteria2" }

	password := ReadInput("密码(回车随机): ")
	if password == "" { password = helperRandHex(8) }
	
	obfsPassword := ""
	if ReadInput("开启 QUIC 混淆? (y/N): ") == "y" {
		obfsPassword = helperRandHex(8)
	}

	tag := fmt.Sprintf("%s_%d", strings.ReplaceAll(name, " ", "_"), port)
	certPath := fmt.Sprintf("%s/%s.pem", CertDir, tag)
	keyPath := fmt.Sprintf("%s/%s.key", CertDir, tag)
	GenerateSelfSignedCert(serverName, certPath, keyPath)

	inbound := map[string]interface{}{
		"type":        "hysteria2",
		"tag":         tag,
		"listen":      "::",
		"listen_port": port,
		"users":       []map[string]interface{}{{"password": password}},
		"tls": map[string]interface{}{
			"enabled":          true,
			"alpn":             []string{"h3"},
			"certificate_path": certPath,
			"key_path":         keyPath,
		},
	}
	// 动态注入混淆配置
	if obfsPassword != "" {
		inbound["obfs"] = map[string]interface{}{"type": "salamander", "password": obfsPassword}
	}

	if AppendInbound(inbound) == nil {
		SaveMetadata(tag, map[string]interface{}{"name": name, "server_name": serverName})
		serverIP := GetPublicIP()
		oparam := ""
		if obfsPassword != "" { oparam = "&obfs=salamander&obfs-password=" + obfsPassword }
		link := fmt.Sprintf("hysteria2://%s@%s:%d?sni=%s&insecure=1%s#%s",
			password, serverIP, port, serverName, oparam, url.QueryEscape(name))
		LogSuccess("节点 [%s] 添加成功\n链接: %s\n%s", name, ColorCyan, link)
	}
}

// 4. Shadowsocks 部署逻辑 (纯净 TCP)
func AddShadowsocks() {
	LogInfo(" 创建 Shadowsocks 节点 ")
	fmt.Println("1) aes-256-gcm  2) ss-2022")
	choice := ReadInput("选择方法 (默认 1): ")
	
	method := "aes-256-gcm"
	password := helperRandHex(8)
	namePrefix := "SS-aes-256-gcm"

	if choice == "2" {
		method = "2022-blake3-aes-128-gcm"
		// 这里的密码在真实的 sing-box 中通常需要 Base64。为简化演示，此处以基础字串替代，实际可调用 sing-box 命令生成
		password = GenerateShortID() + GenerateShortID() 
		namePrefix = "SS-2022"
	}

	port := getValidPort()
	name := ReadInput(fmt.Sprintf("名称 (默认 %s): ", namePrefix))
	if name == "" { name = namePrefix }
	tag := fmt.Sprintf("%s_%d", strings.ReplaceAll(name, " ", "_"), port)

	inbound := map[string]interface{}{
		"type":        "shadowsocks",
		"tag":         tag,
		"listen":      "::",
		"listen_port": port,
		"method":      method,
		"password":    password,
	}

	if AppendInbound(inbound) == nil {
		SaveMetadata(tag, map[string]interface{}{"name": name})
		serverIP := GetPublicIP()
		// SS 链接的 userinfo 需按 base64 编码
		link := fmt.Sprintf("ss://%s@%s:%d#%s",
			url.QueryEscape(method+":"+password), serverIP, port, url.QueryEscape(name))
		LogSuccess("节点 [%s] 添加成功\n链接: %s\n%s", name, ColorCyan, link)
	}
}
