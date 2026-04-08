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
		if writeErr := AtomicWriteFile(MetadataFile, out, 0600); writeErr != nil {
			LogError("保存元数据文件失败，部分显示信息可能会丢失: %v", writeErr)
		}
	} else {
		LogError("元数据 JSON 编码失败: %v", err)
	}
}

// CheckAndFillDefaults 完美复刻并增强 _check_and_fill_defaults
// 该函数在主程序启动时调用，用于修复缺失的关键配置
func CheckAndFillDefaults() {
	// ==========================================
	// 第一部分：处理基础设置 config.json (日志、DNS、路由)
	// ==========================================
	root, err := ReadConfig()
	if err == nil {
		modified := false

		// 1. 检查 Log 配置
		if _, ok := root["log"]; !ok {
			root["log"] = map[string]interface{}{"level": "error", "timestamp": false}
			LogInfo("已自动应用默认日志设置: Error")
			modified = true
		}

		// 2. 检查 DNS 配置
		if _, ok := root["dns"]; !ok {
			root["dns"] = map[string]interface{}{
				"servers": []map[string]interface{}{
					{"type": "udp", "tag": "bootstrap-v4", "server": "1.1.1.1", "server_port": 53},
					{"type": "https", "tag": "dns", "server": "cloudflare-dns.com", "path": "/dns-query", "domain_resolver": "bootstrap-v4"},
					{"type": "https", "tag": "doh-google", "server": "dns.google", "path": "/dns-query", "domain_resolver": "bootstrap-v4"},
					{"type": "https", "tag": "doh-quad9", "server": "dns.quad9.net", "path": "/dns-query", "domain_resolver": "bootstrap-v4"},
				},
			}
			LogInfo("已自动应用默认 DNS 设置: 国外优先")
			modified = true
		}

		// 3. 检查 Route 基础结构
		route, routeOk := root["route"].(map[string]interface{})
		if !routeOk {
			route = map[string]interface{}{
				"final":                 "direct",
				"auto_detect_interface": true,
			}
			root["route"] = route
			modified = true
		}

		// 4. 检查 Route 策略并置顶 default_strategy
		hasResolve := false
		if rules, ok := route["rules"].([]interface{}); ok {
			for _, r := range rules {
				if rule, isMap := r.(map[string]interface{}); isMap {
					if action, _ := rule["action"].(string); action == "resolve" {
						hasResolve = true
						break
					}
				}
			}
		}

		if !hasResolve {
			defaultRule := map[string]interface{}{
				"action":        "resolve",
				"strategy":      "prefer_ipv6",
				"disable_cache": false,
			}
			if rules, ok := route["rules"].([]interface{}); ok {
				route["rules"] = append([]interface{}{defaultRule}, rules...)
			} else {
				route["rules"] = []interface{}{defaultRule}
			}
			route["default_domain_resolver"] = "dns"
			root["route"] = route
			LogInfo("已自动应用默认路由策略: 优先 IPv6")
			modified = true
		}

		if modified {
			WriteConfig(root)
			LogSuccess("核心配置自检完成: 已补全缺失的默认设置")
		}
	}

	// ==========================================
	// 第二部分：处理高级功能 metadata.json (ECH 默认开启)
	// ==========================================
	metaData, err := os.ReadFile(MetadataFile)
	var metaRoot map[string]interface{}
	metaModified := false

	if err != nil || len(metaData) == 0 {
		metaRoot = make(map[string]interface{})
	} else {
		json.Unmarshal(metaData, &metaRoot)
	}

	// 如果没有 ECH 字段，强行写入 true
	if _, ok := metaRoot["client_ech_enabled"]; !ok {
		metaRoot["client_ech_enabled"] = true
		metaModified = true
		LogInfo("已自动应用默认高级设置: 客户端 ECH 注入 (开启)")
	}

	if metaModified {
		out, _ := json.MarshalIndent(metaRoot, "", "  ")
		AtomicWriteFile(MetadataFile, out, 0600)
	}
}
