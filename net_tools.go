package main

import (
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

// GenerateUUID 调用 sing-box 生成 UUID
func GenerateUUID() string {
	out, err := exec.Command("/usr/local/bin/sing-box", "generate", "uuid").Output()
	if err != nil || len(out) == 0 {
		// 优化 5：防崩溃兜底，避免生成空 uuid 导致 sing-box 启动报错
		return "b0b0b0b0-b0b0-40b0-80b0-b0b0b0b0b0b0"
	}
	return strings.TrimSpace(string(out))
}

// GenerateShortID 生成 8 位的 Hex 短 ID
func GenerateShortID() string {
	out, err := exec.Command("/usr/local/bin/sing-box", "generate", "rand", "--hex", "8").Output()
	if err != nil || len(out) == 0 {
		return "a1b2c3d4" // 兜底
	}
	return strings.TrimSpace(string(out))
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
