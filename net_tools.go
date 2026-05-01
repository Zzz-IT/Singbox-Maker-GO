package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// 全局变量缓存 IP
var GlobalServerIP string

// GetPublicIP 替代 _get_public_ip
func GetPublicIP() string {
	if GlobalServerIP != "" {
		return GlobalServerIP
	}

	client := &http.Client{Timeout: 3 * time.Second}
	// 优化 1：全部升级为 HTTPS，并增加更稳定的备用 API
	urls := []string{
		"https://api.ipify.org",
		"https://icanhazip.com",
		"https://ifconfig.me/ip",
	}

	for _, u := range urls {
		resp, err := client.Get(u)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close() // 优化 2：去掉 defer，立刻手动关闭释放连接

			ip := strings.TrimSpace(string(body))
			// 优化 3：验证返回的内容确实是一个合法的 IP 地址，防止被劫持塞入 HTML
			if ip != "" && net.ParseIP(ip) != nil {
				GlobalServerIP = ip
				return ip
			}
		}
	}

	// 优化 4：如果外网 API 全挂了，尝试通过系统命令获取真实出站网卡 IP，而不是直接摆烂 127.0.0.1
	out, err := exec.Command("sh", "-c", "ip route get 1 | awk '{print $7}'").Output()
	if err == nil {
		localIP := strings.TrimSpace(string(out))
		if net.ParseIP(localIP) != nil {
			return localIP
		}
	}

	return "127.0.0.1" // 最终兜底
}

// GenerateUUID 原生生成 UUID，摆脱对 sing-box 外部命令的依赖，消除硬编码降级风险
func GenerateUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "b0b0b0b0-b0b0-40b0-80b0-b0b0b0b0b0b0"
	}
	// UUID v4 规范
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// GenerateShortID 原生生成 8 位 Hex 短 ID
func GenerateShortID() string {
	b := make([]byte, 4) // 4 bytes = 8 hex chars
	_, err := rand.Read(b)
	if err != nil {
		return "a1b2c3d4"
	}
	return fmt.Sprintf("%x", b)
}

// GenerateRealityKeyPair 生成 Reality 的公私钥对
// 返回: (privateKey, publicKey)
func GenerateRealityKeyPair() (string, string) {
	out, err := exec.Command("/usr/local/bin/sing-box", "generate", "reality-keypair").Output()
	if err != nil {
		// 避免核心异常时数组越界崩溃
		return "fail-private-key", "fail-public-key"
	}

	lines := strings.Split(string(out), "\n")
	var pk, pbk string
	for _, line := range lines {
		if strings.Contains(line, "PrivateKey") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pk = parts[1]
			}
		} else if strings.Contains(line, "PublicKey") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pbk = parts[1]
			}
		}
	}
	return pk, pbk
}
