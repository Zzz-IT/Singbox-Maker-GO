package main

import (
	"encoding/json"
	"os"
)

const ConfigFile = "/usr/local/etc/sing-box/config.json"
const MetadataFile = "/usr/local/etc/sing-box/metadata.json"

// AppendInbound 替代 _atomic_modify_json ".inbounds += [$inbound]"
func AppendInbound(newInbound map[string]interface{}) error {
	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		// 如果文件不存在，初始化一个基础结构
		data = []byte(`{"inbounds":[],"outbounds":[{"type":"direct","tag":"direct"}],"route":{"rules":[],"final":"direct"}}`)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}

	// 安全获取或初始化 inbounds 数组
	inbounds, ok := root["inbounds"].([]interface{})
	if !ok {
		inbounds = make([]interface{}, 0)
	}

	// 查重 (简单的按 tag 查重)
	newTag := newInbound["tag"].(string)
	for _, v := range inbounds {
		if existInbound, isMap := v.(map[string]interface{}); isMap {
			if existInbound["tag"] == newTag {
				LogError("Tag '%s' 已存在，请使用其他端口或名称。", newTag)
				return nil
			}
		}
	}

	root["inbounds"] = append(inbounds, newInbound)

	// 写回文件
	out, _ := json.MarshalIndent(root, "", "  ")
	return os.WriteFile(ConfigFile, out, 0600)
}

// SaveMetadata 保存元数据，用于后续删除或查看节点
func SaveMetadata(tag string, meta map[string]interface{}) {
	data, _ := os.ReadFile(MetadataFile)
	var root map[string]interface{}
	if len(data) > 0 {
		json.Unmarshal(data, &root)
	} else {
		root = make(map[string]interface{})
	}

	root[tag] = meta
	out, err := json.MarshalIndent(root, "", "  ")
	if err == nil {
		if writeErr := os.WriteFile(MetadataFile, out, 0600); writeErr != nil {
			LogError("保存元数据文件失败，部分显示信息可能会丢失: %v", writeErr)
		}
	} else {
		LogError("元数据 JSON 编码失败: %v", err)
	}
}
