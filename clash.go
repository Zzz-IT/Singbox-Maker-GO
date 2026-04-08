package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

const ClashYamlFile = "/usr/local/etc/sing-box/clash.yaml"

// 定义结构体，严格锁定 clash.yaml 根节点的顺序
type ClashConfig struct {
	Port               int                      `yaml:"port"`
	SocksPort          int                      `yaml:"socks-port"`
	AllowLan           bool                     `yaml:"allow-lan"`
	Mode               string                   `yaml:"mode"`
	LogLevel           string                   `yaml:"log-level"`
	ExternalController string                   `yaml:"external-controller"`
	Proxies            []map[string]interface{} `yaml:"proxies"`
	ProxyGroups        []map[string]interface{} `yaml:"proxy-groups"`
	Rules              []string                 `yaml:"rules"`
}

func InitClashYaml() error {
	if _, err := os.Stat(ClashYamlFile); err == nil {
		return nil
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
	return AtomicWriteFile(ClashYamlFile, []byte(defaultTemplate), 0644)
}

// AddNodeToYaml 添加节点
func AddNodeToYaml(proxy map[string]interface{}) {
	InitClashYaml()

	data, err := os.ReadFile(ClashYamlFile)
	if err != nil {
		LogError("读取 clash.yaml 失败: %v", err)
		return
	}

	// 使用结构体解析
	var root ClashConfig
	if err := yaml.Unmarshal(data, &root); err != nil {
		LogError("解析 clash.yaml 失败: %v", err)
		return
	}

	// 1. 添加到 proxies
	root.Proxies = append(root.Proxies, proxy)

	// 2. 添加到 PROXY 策略组
	for i, group := range root.ProxyGroups {
		if group["name"] == "PROXY" {
			if groupProxies, ok := group["proxies"].([]interface{}); ok {
				group["proxies"] = append(groupProxies, proxy["name"])
				root.ProxyGroups[i] = group
			}
			break
		}
	}

	// 3. 写回文件
	out, _ := yaml.Marshal(&root)
	AtomicWriteFile(ClashYamlFile, out, 0644)
}

// RemoveNodeFromYaml 删除节点
func RemoveNodeFromYaml(nodeName string) {
	data, err := os.ReadFile(ClashYamlFile)
	if err != nil {
		return
	}

	var root ClashConfig
	if err := yaml.Unmarshal(data, &root); err != nil {
		return
	}

	// 1. 从 proxies 中删除
	var newProxies []map[string]interface{}
	for _, proxy := range root.Proxies {
		if proxy["name"] != nodeName {
			newProxies = append(newProxies, proxy)
		}
	}
	root.Proxies = newProxies

	// 2. 从 PROXY 策略组中删除
	for i, group := range root.ProxyGroups {
		if group["name"] == "PROXY" {
			if groupProxies, ok := group["proxies"].([]interface{}); ok {
				var newGroupProxies []interface{}
				for _, name := range groupProxies {
					if name != nodeName {
						newGroupProxies = append(newGroupProxies, name)
					}
				}
				group["proxies"] = newGroupProxies
				root.ProxyGroups[i] = group
			}
		}
	}

	out, _ := yaml.Marshal(&root)
	AtomicWriteFile(ClashYamlFile, out, 0644)
}

// UpdateNodePortInYaml 修改端口
func UpdateNodePortInYaml(nodeName string, newPort int) {
	data, err := os.ReadFile(ClashYamlFile)
	if err != nil {
		return
	}

	var root ClashConfig
	if err := yaml.Unmarshal(data, &root); err != nil {
		return
	}

	updated := false
	for i, proxy := range root.Proxies {
		if proxy["name"] == nodeName {
			proxy["port"] = newPort
			root.Proxies[i] = proxy
			updated = true
			break
		}
	}

	if updated {
		out, _ := yaml.Marshal(&root)
		AtomicWriteFile(ClashYamlFile, out, 0644)
	}
}
