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

		// === 同步至 clash.yaml ===
		clashProxy := map[string]interface{}{
			"name":               name,
			"type":               "vless",
			"server":             serverIP,
			"port":               port,
			"uuid":               uuid,
			"tls":                true,
			"network":            "tcp",
			"flow":               "xtls-rprx-vision",
			"servername":         serverName,
			"client-fingerprint": "chrome",
			"reality-opts": map[string]interface{}{
				"public-key": publicKey,
				"short-id":   shortID,
			},
		}
		AddNodeToYaml(clashProxy)

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
		LogError("伪装域名不能为空")
		return
	}
	port := getValidPort()
	name := ReadInput("名称 (默认 VLESS-WS): ")
	if name == "" {
		name = "VLESS-WS"
	}

	wsPath := ReadInput("WS路径 (回车随机): ")
	if wsPath == "" {
		wsPath = "/" + helperRandHex(4)
	}
	if !strings.HasPrefix(wsPath, "/") {
		wsPath = "/" + wsPath
	}

	tag := fmt.Sprintf("%s_%d", strings.ReplaceAll(name, " ", "_"), port)
	certPath := fmt.Sprintf("%s/%s.pem", CertDir, tag)
	keyPath := fmt.Sprintf("%s/%s.key", CertDir, tag)

	if err := GenerateSelfSignedCert(serverName, certPath, keyPath); err != nil {
		LogError("证书生成失败")
		return
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

		// === 同步至 clash.yaml ===
		clashProxy := map[string]interface{}{
			"name":             name,
			"type":             "vless",
			"server":           serverIP,
			"port":             port,
			"uuid":             uuid,
			"tls":              true,
			"udp":              true,
			"skip-cert-verify": true, // 自签证书默认跳过验证
			"network":          "ws",
			"servername":       serverName,
			"ws-opts": map[string]interface{}{
				"path": wsPath,
				"headers": map[string]interface{}{
					"Host": serverName,
				},
			},
		}
		AddNodeToYaml(clashProxy)

		link := fmt.Sprintf("vless://%s@%s:%d?security=tls&encryption=none&type=ws&host=%s&path=%s&sni=%s&insecure=1#%s",
			uuid, serverIP, port, serverName, url.QueryEscape(wsPath), serverName, url.QueryEscape(name))
		LogSuccess("节点 [%s] 添加成功\n链接: %s\n%s", name, ColorCyan, link)
	}
}

// 3. Trojan-WS-TLS 部署逻辑
func AddTrojanWSTLS() {
	LogInfo(" 创建 Trojan-WS-TLS 节点 ")
	serverName := ReadInput("伪装域名 (SNI): ")
	if serverName == "" {
		LogError("伪装域名不能为空")
		return
	}
	port := getValidPort()
	name := ReadInput("名称 (默认 Trojan-WS): ")
	if name == "" {
		name = "Trojan-WS"
	}

	wsPath := ReadInput("WS路径 (回车随机): ")
	if wsPath == "" {
		wsPath = "/" + helperRandHex(4)
	}
	if !strings.HasPrefix(wsPath, "/") {
		wsPath = "/" + wsPath
	}

	tag := fmt.Sprintf("%s_%d", strings.ReplaceAll(name, " ", "_"), port)
	certPath := fmt.Sprintf("%s/%s.pem", CertDir, tag)
	keyPath := fmt.Sprintf("%s/%s.key", CertDir, tag)
	GenerateSelfSignedCert(serverName, certPath, keyPath)

	password := ReadInput("密码(回车随机): ")
	if password == "" {
		password = helperRandHex(8)
	}

	inbound := map[string]interface{}{
		"type":        "trojan",
		"tag":         tag,
		"listen":      "::",
		"listen_port": port,
		"users":       []map[string]interface{}{{"password": password}},
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

		// === 同步至 clash.yaml ===
		clashProxy := map[string]interface{}{
			"name":             name,
			"type":             "trojan",
			"server":           serverIP,
			"port":             port,
			"password":         password,
			"udp":              true,
			"skip-cert-verify": true,
			"network":          "ws",
			"sni":              serverName,
			"ws-opts": map[string]interface{}{
				"path": wsPath,
				"headers": map[string]interface{}{
					"Host": serverName,
				},
			},
		}
		AddNodeToYaml(clashProxy)

		link := fmt.Sprintf("trojan://%s@%s:%d?security=tls&type=ws&host=%s&path=%s&sni=%s&allowInsecure=1#%s",
			password, serverIP, port, serverName, url.QueryEscape(wsPath), serverName, url.QueryEscape(name))
		LogSuccess("节点 [%s] 添加成功\n链接: %s\n%s", name, ColorCyan, link)
	}
}

// 4. AnyTLS 部署逻辑
func AddAnyTLS() {
	LogInfo(" 创建 AnyTLS 节点 ")
	port := getValidPort()
	serverName := ReadInput("SNI (默认 www.apple.com): ")
	if serverName == "" {
		serverName = "www.apple.com"
	}
	name := ReadInput("名称 (默认 AnyTLS): ")
	if name == "" {
		name = "AnyTLS"
	}

	tag := fmt.Sprintf("%s_%d", strings.ReplaceAll(name, " ", "_"), port)
	certPath := fmt.Sprintf("%s/%s.pem", CertDir, tag)
	keyPath := fmt.Sprintf("%s/%s.key", CertDir, tag)
	GenerateSelfSignedCert(serverName, certPath, keyPath)

	password := ReadInput("密码/UUID(回车随机): ")
	if password == "" {
		password = GenerateUUID()
	}

	inbound := map[string]interface{}{
		"type":        "anytls",
		"tag":         tag,
		"listen":      "::",
		"listen_port": port,
		"users":       []map[string]interface{}{{"name": "default", "password": password}},
		"padding_scheme": []string{"stop=2", "0=100-200", "1=100-200"},
		"tls": map[string]interface{}{
			"enabled":          true,
			"server_name":      serverName,
			"certificate_path": certPath,
			"key_path":         keyPath,
		},
	}

	if AppendInbound(inbound) == nil {
		SaveMetadata(tag, map[string]interface{}{"name": name, "server_name": serverName})
		serverIP := GetPublicIP()

		// === 同步至 clash.yaml ===
		clashProxy := map[string]interface{}{
			"name":                        name,
			"type":                        "anytls", // Note: 某些内核可能写作 vless+anytls，这里遵循原版写法
			"server":                      serverIP,
			"port":                        port,
			"password":                    password,
			"client-fingerprint":          "chrome",
			"udp":                         true,
			"idle-session-check-interval": 30,
			"idle-session-timeout":        30,
			"min-idle-session":            0,
			"sni":                         serverName,
			"alpn":                        []string{"h2", "http/1.1"},
			"skip-cert-verify":            true,
		}
		AddNodeToYaml(clashProxy)

		link := fmt.Sprintf("anytls://%s@%s:%d?security=tls&sni=%s&insecure=1&allowInsecure=1&type=tcp#%s",
			password, serverIP, port, serverName, url.QueryEscape(name))
		LogSuccess("节点 [%s] 添加成功\n链接: %s\n%s", name, ColorCyan, link)
	}
}

// 5. Hysteria2 部署逻辑
func AddHysteria2() {
	LogInfo(" 创建 Hysteria2 节点 ")
	port := getValidPort()
	serverName := ReadInput("伪装域名 (默认 www.apple.com): ")
	if serverName == "" {
		serverName = "www.apple.com"
	}
	name := ReadInput("名称 (默认 Hysteria2): ")
	if name == "" {
		name = "Hysteria2"
	}

	password := ReadInput("密码(回车随机): ")
	if password == "" {
		password = helperRandHex(8)
	}

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
	if obfsPassword != "" {
		inbound["obfs"] = map[string]interface{}{"type": "salamander", "password": obfsPassword}
	}

	if AppendInbound(inbound) == nil {
		SaveMetadata(tag, map[string]interface{}{"name": name, "server_name": serverName})
		serverIP := GetPublicIP()

		// === 同步至 clash.yaml ===
		clashProxy := map[string]interface{}{
			"name":             name,
			"type":             "hysteria2",
			"server":           serverIP,
			"port":             port,
			"password":         password,
			"sni":              serverName,
			"skip-cert-verify": true,
			"alpn":             []string{"h3"},
			"up":               "500 Mbps",
			"down":             "500 Mbps",
		}
		if obfsPassword != "" {
			clashProxy["obfs"] = "salamander"
			clashProxy["obfs-password"] = obfsPassword
		}
		AddNodeToYaml(clashProxy)

		oparam := ""
		if obfsPassword != "" {
			oparam = "&obfs=salamander&obfs-password=" + obfsPassword
		}
		link := fmt.Sprintf("hysteria2://%s@%s:%d?sni=%s&insecure=1%s#%s",
			password, serverIP, port, serverName, oparam, url.QueryEscape(name))
		LogSuccess("节点 [%s] 添加成功\n链接: %s\n%s", name, ColorCyan, link)
	}
}

// 6. TUICv5 部署逻辑
func AddTUIC() {
	LogInfo(" 创建 TUICv5 节点 ")
	port := getValidPort()
	serverName := ReadInput("SNI (默认 www.apple.com): ")
	if serverName == "" {
		serverName = "www.apple.com"
	}
	name := ReadInput("名称 (默认 TUICv5): ")
	if name == "" {
		name = "TUICv5"
	}

	tag := fmt.Sprintf("%s_%d", strings.ReplaceAll(name, " ", "_"), port)
	certPath := fmt.Sprintf("%s/%s.pem", CertDir, tag)
	keyPath := fmt.Sprintf("%s/%s.key", CertDir, tag)
	GenerateSelfSignedCert(serverName, certPath, keyPath)

	uuid := GenerateUUID()
	password := helperRandHex(8)

	inbound := map[string]interface{}{
		"type":        "tuic",
		"tag":         tag,
		"listen":      "::",
		"listen_port": port,
		"users":       []map[string]interface{}{{"uuid": uuid, "password": password}},
		"congestion_control": "bbr",
		"tls": map[string]interface{}{
			"enabled":          true,
			"alpn":             []string{"h3"},
			"certificate_path": certPath,
			"key_path":         keyPath,
		},
	}

	if AppendInbound(inbound) == nil {
		SaveMetadata(tag, map[string]interface{}{"name": name})
		serverIP := GetPublicIP()

		// === 同步至 clash.yaml ===
		clashProxy := map[string]interface{}{
			"name":                  name,
			"type":                  "tuic",
			"server":                serverIP,
			"port":                  port,
			"uuid":                  uuid,
			"password":              password,
			"sni":                   serverName,
			"skip-cert-verify":      true,
			"alpn":                  []string{"h3"},
			"udp-relay-mode":        "native",
			"congestion-controller": "bbr",
		}
		AddNodeToYaml(clashProxy)

		link := fmt.Sprintf("tuic://%s:%s@%s:%d?sni=%s&alpn=h3&congestion_control=bbr&udp_relay_mode=native&allow_insecure=1#%s",
			uuid, password, serverIP, port, serverName, url.QueryEscape(name))
		LogSuccess("节点 [%s] 添加成功\n链接: %s\n%s", name, ColorCyan, link)
	}
}

// 7. Shadowsocks 部署逻辑 (纯净 TCP)
func AddShadowsocks() {
	LogInfo(" 创建 Shadowsocks 节点 ")
	fmt.Println("1) aes-256-gcm  2) ss-2022")
	choice := ReadInput("选择方法 (默认 1): ")

	method := "aes-256-gcm"
	password := helperRandHex(8)
	namePrefix := "SS-aes-256-gcm"

	if choice == "2" {
		method = "2022-blake3-aes-128-gcm"
		password = GenerateShortID() + GenerateShortID()
		namePrefix = "SS-2022"
	}

	port := getValidPort()
	name := ReadInput(fmt.Sprintf("名称 (默认 %s): ", namePrefix))
	if name == "" {
		name = namePrefix
	}
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

		// === 同步至 clash.yaml ===
		clashProxy := map[string]interface{}{
			"name":     name,
			"type":     "ss",
			"server":   serverIP,
			"port":     port,
			"cipher":   method,
			"password": password,
		}
		AddNodeToYaml(clashProxy)

		link := fmt.Sprintf("ss://%s@%s:%d#%s",
			url.QueryEscape(method+":"+password), serverIP, port, url.QueryEscape(name))
		LogSuccess("节点 [%s] 添加成功\n链接: %s\n%s", name, ColorCyan, link)
	}
}

// 8. VLESS-TCP 部署逻辑
func AddVLESSTCP() {
	LogInfo(" 创建 VLESS-TCP 节点 ")
	port := getValidPort()
	name := ReadInput("名称 (默认 VLESS-TCP): ")
	if name == "" {
		name = "VLESS-TCP"
	}

	tag := fmt.Sprintf("%s_%d", strings.ReplaceAll(name, " ", "_"), port)
	uuid := GenerateUUID()

	inbound := map[string]interface{}{
		"type":        "vless",
		"tag":         tag,
		"listen":      "::",
		"listen_port": port,
		"users":       []map[string]interface{}{{"uuid": uuid, "flow": ""}},
		"tls":         map[string]interface{}{"enabled": false},
	}

	if AppendInbound(inbound) == nil {
		SaveMetadata(tag, map[string]interface{}{"name": name})
		serverIP := GetPublicIP()

		// === 同步至 clash.yaml ===
		clashProxy := map[string]interface{}{
			"name":    name,
			"type":    "vless",
			"server":  serverIP,
			"port":    port,
			"uuid":    uuid,
			"tls":     false,
			"network": "tcp",
		}
		AddNodeToYaml(clashProxy)

		link := fmt.Sprintf("vless://%s@%s:%d?encryption=none&type=tcp#%s", uuid, serverIP, port, url.QueryEscape(name))
		LogSuccess("节点 [%s] 添加成功\n链接: %s\n%s", name, ColorCyan, link)
	}
}

// 9. SOCKS5 部署逻辑
func AddSOCKS5() {
	LogInfo(" 创建 SOCKS5 节点 ")
	port := getValidPort()
	username := ReadInput("用户 (回车随机): ")
	if username == "" {
		username = helperRandHex(4)
	}
	password := ReadInput("密码 (回车随机): ")
	if password == "" {
		password = helperRandHex(8)
	}
	name := ReadInput("名称 (默认 SOCKS5): ")
	if name == "" {
		name = "SOCKS5"
	}

	tag := fmt.Sprintf("%s_%d", strings.ReplaceAll(name, " ", "_"), port)

	inbound := map[string]interface{}{
		"type":        "socks",
		"tag":         tag,
		"listen":      "::",
		"listen_port": port,
		"users":       []map[string]interface{}{{"username": username, "password": password}},
	}

	if AppendInbound(inbound) == nil {
		SaveMetadata(tag, map[string]interface{}{"name": name})
		serverIP := GetPublicIP()

		// === 同步至 clash.yaml ===
		clashProxy := map[string]interface{}{
			"name":     name,
			"type":     "socks5",
			"server":   serverIP,
			"port":     port,
			"username": username,
			"password": password,
		}
		AddNodeToYaml(clashProxy)

		LogSuccess("SOCKS5 节点 [%s] 添加成功\n地址: %s:%d\n用户: %s\n密码: %s", name, serverIP, port, username, password)
	}
}
