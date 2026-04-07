package main

import (
	"io"
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
	urls := []string{"http://icanhazip.com", "http://ipinfo.io/ip"}

	for _, u := range urls {
		resp, err := client.Get(u)
		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			ip := strings.TrimSpace(string(body))
			if ip != "" {
				GlobalServerIP = ip
				return ip
			}
		}
	}
	return "127.0.0.1" // 兜底
}

// GenerateUUID 调用 sing-box 生成 UUID
func GenerateUUID() string {
	out, _ := exec.Command("/usr/local/bin/sing-box", "generate", "uuid").Output()
	return strings.TrimSpace(string(out))
}

// GenerateShortID 生成 8 位的 Hex 短 ID
func GenerateShortID() string {
	out, _ := exec.Command("/usr/local/bin/sing-box", "generate", "rand", "--hex", "8").Output()
	return strings.TrimSpace(string(out))
}

// GenerateRealityKeyPair 生成 Reality 的公私钥对
// 返回: (privateKey, publicKey)
func GenerateRealityKeyPair() (string, string) {
	out, err := exec.Command("/usr/local/bin/sing-box", "generate", "reality-keypair").Output()
	if err != nil {
		return "", ""
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
