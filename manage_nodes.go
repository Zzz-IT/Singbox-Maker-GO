package main

import (
	"encoding/json"
	"fmt"
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
func WriteConfig(root map[string]interface{}) error {
	out, _ := json.MarshalIndent(root, "", "  ")
	return os.WriteFile(ConfigFile, out, 0644)
}

// DeleteNode 替代原版的 _delete_node
func DeleteNode() {
	ClearScreen()
	fmt.Print("\n\n")
	LogInfo(" 节点删除管理 ")

	root, err := ReadConfig()
	if err != nil {
		LogError("读取配置失败"); return
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
	if choice == "0" || choice == "" { return }

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
		LogSuccess("节点 %s 已删除", targetTag)
		ManageService("restart")
	}
}

// ModifyPort 替代原版的 _modify_port
func ModifyPort() {
	ClearScreen()
	fmt.Print("\n\n")
	LogInfo(" 修改节点端口 ")

	root, err := ReadConfig()
	if err != nil {
		LogError("读取配置失败"); return
	}

	inbounds, ok := root["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		LogWarn("当前没有任何节点"); return
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
	if choice == "0" || choice == "" { return }

	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(validInbounds) { return }

	target := validInbounds[idx-1]
	oldPort := int(target["listen_port"].(float64))
	oldTag := target["tag"].(string)

	newPort := getValidPort()
	
	// 在 JSON 中直接修改指针引用
	target["listen_port"] = newPort
	// 简单处理：将 Tag 中的旧端口替换为新端口
	target["tag"] = strings.Replace(oldTag, fmt.Sprintf("_%d", oldPort), fmt.Sprintf("_%d", newPort), 1)

	WriteConfig(root)
	LogSuccess("端口已修改: %d -> %d", oldPort, newPort)
	ManageService("restart")
}
