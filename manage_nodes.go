package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// ReadConfig 安全读取 config.json
func ReadConfig() (map[string]interface{}, error) {
	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		return nil, err
	}
	var root map[string]interface{}
	err = json.Unmarshal(data, &root)
	return root, err
}

// WriteConfig 安全写入 config.json
// WriteConfig 安全写入 config.json (原子化写入，防止断电/磁盘满导致配置丢失)
func WriteConfig(root map[string]interface{}) error {
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON 编码失败: %v", err)
	}

	tmpFile := ConfigFile + ".tmp"
	// 1. 先写入临时文件
	if err := os.WriteFile(tmpFile, out, 0644); err != nil {
		return fmt.Errorf("写入临时配置失败: %v", err)
	}

	// 2. 写入成功后，原子级重命名覆盖原文件
	if err := os.Rename(tmpFile, ConfigFile); err != nil {
		return fmt.Errorf("替换原配置文件失败: %v", err)
	}
	return nil
}

// DeleteNode 替代原版的 _delete_node
func DeleteNode() {
	ClearScreen()
	fmt.Print("\n\n")
	LogInfo(" 节点删除管理 ")

	root, err := ReadConfig()
	if err != nil {
		LogError("读取配置失败")
		return
	}

	inbounds, ok := root["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		LogWarn("当前没有任何节点")
		return
	}

	// 提取常规节点（过滤掉 Argo 节点）
	var validInbounds []map[string]interface{}
	for _, v := range inbounds {
		if inbound, isMap := v.(map[string]interface{}); isMap {
			tag := inbound["tag"].(string)
			if !strings.HasPrefix(tag, "argo_") { // 简单的 Argo 过滤
				validInbounds = append(validInbounds, inbound)
			}
		}
	}

	if len(validInbounds) == 0 {
		LogWarn("当前没有任何常规节点")
		return
	}

	fmt.Println("─────────────────────────────────────────────")
	for i, inbound := range validInbounds {
		tag := inbound["tag"].(string)
		nodeType := inbound["type"].(string)
		port := int(inbound["listen_port"].(float64))
		fmt.Printf("  %s%d)%s %s (%s) @ 端口 %d\n", ColorCyan, i+1, ColorReset, tag, nodeType, port)
	}
	fmt.Printf("  %s99)%s 删除所有节点\n", ColorRed, ColorReset)
	fmt.Println("─────────────────────────────────────────────")

	choice := ReadInput("请输入要删除的节点编号 (0返回): ")
	if choice == "0" || choice == "" {
		return
	}

	if choice == "99" {
		if ReadInput(ColorYellow+"确定要清空所有常规节点吗？(y/N): "+ColorReset) == "y" {
			// 保留出站和路由，清空入站
			root["inbounds"] = []interface{}{}
			WriteConfig(root)
			os.WriteFile(MetadataFile, []byte("{}"), 0644) // 清空元数据
			LogSuccess("所有节点已清空")
			ManageService("restart")
		}
		return
	}

	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(validInbounds) {
		LogError("无效的编号")
		return
	}

	target := validInbounds[idx-1]
	targetTag := target["tag"].(string)

	if ReadInput(fmt.Sprintf("%s确认删除节点 [%s] 吗？(y/N): %s", ColorYellow, targetTag, ColorReset)) == "y" {
		// 重新构建 inbounds 切片
		var newInbounds []interface{}
		for _, v := range inbounds {
			if inbound, isMap := v.(map[string]interface{}); isMap {
				if inbound["tag"].(string) != targetTag {
					newInbounds = append(newInbounds, inbound)
				}
			}
		}
		root["inbounds"] = newInbounds
		WriteConfig(root)

		// === 同步删除 Clash 节点 ===
		metadata := ReadMetadata()
		targetName := targetTag
		if meta, ok := metadata[targetTag].(map[string]interface{}); ok {
			if n, ok := meta["name"].(string); ok {
				targetName = n
			}
		}
		RemoveNodeFromYaml(targetName)

		// === 【新增】清理配套的证书文件 ===
		certPath := fmt.Sprintf("/usr/local/etc/sing-box/%s.pem", targetTag)
		keyPath := fmt.Sprintf("/usr/local/etc/sing-box/%s.key", targetTag)
		os.Remove(certPath)
		os.Remove(keyPath)

		LogSuccess("节点 %s 及关联文件已清理", targetTag)
		ManageService("restart")
	}
}

// ModifyPort 替代原版的 _modify_port
// ModifyPort 替代原版的 _modify_port
func ModifyPort() {
	ClearScreen()
	fmt.Print("\n\n")
	LogInfo(" 修改节点端口 ")

	root, err := ReadConfig()
	if err != nil {
		LogError("读取配置失败")
		return
	}

	inbounds, ok := root["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		LogWarn("当前没有任何节点")
		return
	}

	var validInbounds []map[string]interface{}
	for _, v := range inbounds {
		if inbound, isMap := v.(map[string]interface{}); isMap {
			if !strings.HasPrefix(inbound["tag"].(string), "argo_") {
				validInbounds = append(validInbounds, inbound)
			}
		}
	}

	for i, inbound := range validInbounds {
		tag := inbound["tag"].(string)
		fmt.Printf("  %s%d)%s %s @ 端口 %.0f\n", ColorCyan, i+1, ColorReset, tag, inbound["listen_port"].(float64))
	}

	choice := ReadInput("请输入要修改的节点编号 (0返回): ")
	if choice == "0" || choice == "" {
		return
	}

	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(validInbounds) {
		return
	}

	target := validInbounds[idx-1]
	oldPort := int(target["listen_port"].(float64))
	oldTag := target["tag"].(string)

	newPort := getValidPort()

	// 在 JSON 中直接修改指针引用
	target["listen_port"] = newPort
	newTag := strings.Replace(oldTag, fmt.Sprintf("_%d", oldPort), fmt.Sprintf("_%d", newPort), 1)
	target["tag"] = newTag

	// ==========================================
	// 【新增修复】自动重命名关联的本地自签证书
	// ==========================================
	if tlsMap, ok := target["tls"].(map[string]interface{}); ok {
		// 构建新旧证书文件路径
		oldCertPath := fmt.Sprintf("/usr/local/etc/sing-box/%s.pem", oldTag)
		oldKeyPath := fmt.Sprintf("/usr/local/etc/sing-box/%s.key", oldTag)
		newCertPath := fmt.Sprintf("/usr/local/etc/sing-box/%s.pem", newTag)
		newKeyPath := fmt.Sprintf("/usr/local/etc/sing-box/%s.key", newTag)

		// 尝试重命名磁盘上的证书文件
		if _, err := os.Stat(oldCertPath); err == nil {
			os.Rename(oldCertPath, newCertPath)
		}
		if _, err := os.Stat(oldKeyPath); err == nil {
			os.Rename(oldKeyPath, newKeyPath)
		}

		// 同步修改 JSON 配置里的路径引用 (仅在当前路径匹配旧路径时才修改，防止误伤用户自定义证书)
		if cPath, ok := tlsMap["certificate_path"].(string); ok && cPath == oldCertPath {
			tlsMap["certificate_path"] = newCertPath
		}
		if kPath, ok := tlsMap["key_path"].(string); ok && kPath == oldKeyPath {
			tlsMap["key_path"] = newKeyPath
		}
	}
	// ==========================================

	WriteConfig(root)

	// --- 同步更新 metadata.json ---
	metadata := ReadMetadata()
	var nodeName string = oldTag // 默认 fallback 为老 tag
	if meta, ok := metadata[oldTag].(map[string]interface{}); ok {
		if n, ok := meta["name"].(string); ok {
			nodeName = n // 获取 Clash 中真实的 nodeName
		}
		metadata[newTag] = meta  // 转移到新 tag
		delete(metadata, oldTag) // 删除旧 tag

		outMeta, _ := json.MarshalIndent(metadata, "", "  ")
		os.WriteFile(MetadataFile, outMeta, 0644)
	}

	// --- 同步更新 clash.yaml ---
	UpdateNodePortInYaml(nodeName, newPort)

	LogSuccess("端口已修改: %d -> %d", oldPort, newPort)
	ManageService("restart")
}

// ReadMetadata 安全读取 metadata.json 助手函数
func ReadMetadata() map[string]interface{} {
	data, err := os.ReadFile(MetadataFile)
	var root map[string]interface{}
	if err == nil && len(data) > 0 {
		json.Unmarshal(data, &root)
	} else {
		root = make(map[string]interface{})
	}
	return root
}

// ViewNodes 打印所有节点信息与分享链接（无 Base64 纯净版）
func ViewNodes() {
	ClearScreen()
	fmt.Print("\n\n")
	LogInfo(" 节点信息与分享链接 ")

	root, err := ReadConfig()
	if err != nil {
		LogError("读取配置失败: %v", err)
		return
	}

	metadata := ReadMetadata()

	inbounds, ok := root["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		LogWarn("当前没有任何节点")
		return
	}

	serverIP := GetPublicIP()
	linkIP := serverIP
	if strings.Contains(serverIP, ":") {
		linkIP = "[" + serverIP + "]" // IPv6 安全包裹
	}

	count := 0
	for _, v := range inbounds {
		inbound, isMap := v.(map[string]interface{})
		if !isMap {
			continue
		}

		tag := inbound["tag"].(string)
		if strings.HasPrefix(tag, "argo_") {
			continue // 过滤掉 Argo 隧道节点
		}

		nodeType := inbound["type"].(string)
		port := int(inbound["listen_port"].(float64))

		// 1. 从 Metadata 获取用户友好的节点名称和 Reality 公钥
		name := tag
		var pbk, sid string
		if meta, ok := metadata[tag].(map[string]interface{}); ok {
			if n, ok := meta["name"].(string); ok {
				name = n
			}
			if p, ok := meta["publicKey"].(string); ok {
				pbk = p
			}
			if s, ok := meta["shortId"].(string); ok {
				sid = s
			}
		}

		// [修复] 将全角的 fmt。Printf 替换为半角的 fmt.Printf
		fmt.Printf("─────────────────────────────────────────────\n")
		fmt.Printf("  节点: %s%s%s (%s) @ 端口 %d\n", ColorCyan, name, ColorReset, nodeType, port)

		// 2. 深入 JSON 提取核心参数
		var uuid, password, method, username string
		if users, ok := inbound["users"].([]interface{}); ok && len(users) > 0 {
			if u, ok := users[0].(map[string]interface{}); ok {
				if val, ok := u["uuid"].(string); ok {
					uuid = val
				}
				if val, ok := u["password"].(string); ok {
					password = val
				}
				if val, ok := u["username"].(string); ok {
					username = val
				}
			}
		}
		if m, ok := inbound["method"].(string); ok {
			method = m
		}

		sni := "www.apple.com"
		path := "/"
		if tls, ok := inbound["tls"].(map[string]interface{}); ok {
			if val, ok := tls["server_name"].(string); ok {
				sni = val
			}
		}
		if transport, ok := inbound["transport"].(map[string]interface{}); ok {
			if val, ok := transport["path"].(string); ok {
				path = val
			}
		}

		safeName := url.QueryEscape(name)
		safePath := url.QueryEscape(path)
		var urlStr string

		// 3. 动态拼接 URL
		switch nodeType {
		case "vless":
			if tls, ok := inbound["tls"].(map[string]interface{}); ok && tls["enabled"] == true {
				if _, isReality := tls["reality"]; isReality {
					urlStr = fmt.Sprintf("vless://%s@%s:%d?security=reality&encryption=none&pbk=%s&fp=chrome&type=tcp&flow=xtls-rprx-vision&sni=%s&sid=%s#%s", uuid, linkIP, port, pbk, sni, sid, safeName)
				} else {
					urlStr = fmt.Sprintf("vless://%s@%s:%d?security=tls&encryption=none&type=ws&host=%s&path=%s&sni=%s&insecure=1#%s", uuid, linkIP, port, sni, safePath, sni, safeName)
				}
			} else {
				urlStr = fmt.Sprintf("vless://%s@%s:%d?encryption=none&type=tcp#%s", uuid, linkIP, port, safeName)
			}
		case "trojan":
			urlStr = fmt.Sprintf("trojan://%s@%s:%d?security=tls&type=ws&host=%s&path=%s&sni=%s&allowInsecure=1#%s", password, linkIP, port, sni, safePath, sni, safeName)
		case "hysteria2":
			urlStr = fmt.Sprintf("hysteria2://%s@%s:%d?sni=%s&insecure=1#%s", password, linkIP, port, sni, safeName)
		case "tuic":
			urlStr = fmt.Sprintf("tuic://%s:%s@%s:%d?sni=%s&alpn=h3&congestion_control=bbr&udp_relay_mode=native&allow_insecure=1#%s", uuid, password, linkIP, port, sni, safeName)
		case "anytls":
			urlStr = fmt.Sprintf("anytls://%s@%s:%d?security=tls&sni=%s&insecure=1&allowInsecure=1&type=tcp#%s", password, linkIP, port, sni, safeName)
		case "shadowsocks":
			userInfo := base64.StdEncoding.EncodeToString([]byte(method + ":" + password))
			urlStr = fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, linkIP, port, safeName)
		case "socks":
			urlStr = fmt.Sprintf("地址: %s:%d | 用户: %s | 密码: %s", linkIP, port, username, password)
		}

		if urlStr != "" {
			fmt.Printf("  链接: %s%s%s\n", ColorYellow, urlStr, ColorReset)
		}
		count++
	}

	if count == 0 {
		LogWarn("未找到常规节点")
	} else {
		fmt.Printf("─────────────────────────────────────────────\n")
	}
}
