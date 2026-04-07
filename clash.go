package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

const ClashYamlFile = "/usr/local/etc/sing-box/clash.yaml"

// InitClashYaml 初始化基础的 Clash 配置模板 (替代原版 _write_default_clash)
func InitClashYaml() error {
	if _, err := os.Stat(ClashYamlFile); err == nil {
		return nil // 文件已存在则跳过
	}

	defaultTemplate := `port: 7890
socks-port: 7891
allow-lan: false
mode: rule
log-level: info
external-controller: 127.0.0.1:9090
proxies: []
proxy-groups:
  - name: PROXY
    type: select
    proxies: []
rules:
  - MATCH,PROXY
`
	return os.WriteFile(ClashYamlFile, []byte(defaultTemplate), 0644)
}

// AddNodeToYaml 添加节点到 clash.yaml (替代原版 _add_node_to_yaml)
func AddNodeToYaml(proxy map[string]interface{}) {
	InitClashYaml() // 确保文件存在

	data, err := os.ReadFile(ClashYamlFile)
	if err != nil {
		LogError("读取 clash.yaml 失败: %v", err)
		return
	}

	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		LogError("解析 clash.yaml 失败: %v", err)
		return
	}

	// 1. 将节点添加到 proxies 数组
	proxies, _ := root["proxies"].([]interface{})
	root["proxies"] = append(proxies, proxy)

	// 2. 将节点名称添加到 PROXY 策略组
	if proxyGroups, ok := root["proxy-groups"].([]interface{}); ok {
		for i, g := range proxyGroups {
			if group, isMap := g.(map[string]interface{}); isMap {
				if group["name"] == "PROXY" {
					groupProxies, _ := group["proxies"].([]interface{})
					group["proxies"] = append(groupProxies, proxy["name"])
					proxyGroups[i] = group
					break
				}
			}
		}
		root["proxy-groups"] = proxyGroups
	}

	// 3. 写回文件
	out, _ := yaml.Marshal(root)
	os.WriteFile(ClashYamlFile, out, 0644)
}

// RemoveNodeFromYaml 从 clash.yaml 中彻底删除节点 (替代原版 _remove_node_from_yaml)
func RemoveNodeFromYaml(nodeName string) {
	data, err := os.ReadFile(ClashYamlFile)
	if err != nil {
		return
	}

	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return
	}

	// 1. 从 proxies 中删除
	if proxies, ok := root["proxies"].([]interface{}); ok {
		var newProxies []interface{}
		for _, p := range proxies {
			if proxy, isMap := p.(map[string]interface{}); isMap {
				if proxy["name"] != nodeName {
					newProxies = append(newProxies, p)
				}
			}
		}
		root["proxies"] = newProxies
	}

	// 2. 从 PROXY 策略组中删除
	if proxyGroups, ok := root["proxy-groups"].([]interface{}); ok {
		for i, g := range proxyGroups {
			if group, isMap := g.(map[string]interface{}); isMap {
				if group["name"] == "PROXY" {
					if groupProxies, ok := group["proxies"].([]interface{}); ok {
						var newGroupProxies []interface{}
						for _, name := range groupProxies {
							if name != nodeName {
								newGroupProxies = append(newGroupProxies, name)
							}
						}
						group["proxies"] = newGroupProxies
						proxyGroups[i] = group
					}
				}
			}
		}
		root["proxy-groups"] = proxyGroups
	}

	out, _ := yaml.Marshal(root)
	os.WriteFile(ClashYamlFile, out, 0644)
}
