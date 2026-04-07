package main

import (
	"fmt"
)

// --- 状态获取辅助函数 ---

func getCurrentLogLevel(root map[string]interface{}) string {
	if log, ok := root["log"].(map[string]interface{}); ok {
		if level, ok := log["level"].(string); ok {
			switch level {
			case "error":
				return "Error (错误)"
			case "warn":
				return "Warn (警告)"
			case "info":
				return "Info (信息)"
			case "debug":
				return "Debug (调试)"
			default:
				return level
			}
		}
	}
	return "Error (错误)"
}

func getCurrentDNS(root map[string]interface{}) string {
	if dns, ok := root["dns"].(map[string]interface{}); ok {
		if servers, ok := dns["servers"].([]interface{}); ok {
			for _, s := range servers {
				if server, isMap := s.(map[string]interface{}); isMap {
					if tag, ok := server["tag"].(string); ok && tag == "bootstrap-cn" {
						return "国内优先"
					}
				}
			}
		}
	}
	return "国外优先"
}

func getCurrentStrategy(root map[string]interface{}) string {
	if route, ok := root["route"].(map[string]interface{}); ok {
		if rules, ok := route["rules"].([]interface{}); ok {
			for _, r := range rules {
				if rule, isMap := r.(map[string]interface{}); isMap {
					if action, ok := rule["action"].(string); ok && action == "resolve" {
						if strategy, ok := rule["strategy"].(string); ok {
							switch strategy {
							case "prefer_ipv6":
								return "优先 IPv6"
							case "prefer_ipv4":
								return "优先 IPv4"
							case "ipv4_only":
								return "仅 IPv4"
							case "ipv6_only":
								return "仅 IPv6"
							default:
								return strategy
							}
						}
					}
				}
			}
		}
	}
	return "优先 IPv6"
}

// --- 高级设置主菜单 ---

func ShowAdvancedMenu() {
	for {
		root, err := ReadConfig()
		if err != nil {
			LogError("读取配置失败，无法进入高级设置: %v", err)
			return
		}

		sLog := getCurrentLogLevel(root)
		sDNS := getCurrentDNS(root)
		sStr := getCurrentStrategy(root)

		ClearScreen()
		fmt.Print("\n\n")
		fmt.Printf("       %sA D V A N C E D   S E T T I N G S%s\n", ColorCyan, ColorReset)
		fmt.Printf("  %s─────────────────────────────────────────────%s\n\n", ColorGrey, ColorReset)

		fmt.Printf("  %s01.%s 日志等级            %s状态: %s%s%s\n", ColorWhite, ColorReset, ColorReset, ColorYellow, sLog, ColorReset)
		fmt.Printf("  %s02.%s DNS 模式            %s状态: %s%s%s\n", ColorWhite, ColorReset, ColorReset, ColorYellow, sDNS, ColorReset)
		fmt.Printf("  %s03.%s IP 策略             %s状态: %s%s%s\n\n", ColorWhite, ColorReset, ColorReset, ColorYellow, sStr, ColorReset)

		fmt.Printf("  %s─────────────────────────────────────────────%s\n", ColorGrey, ColorReset)
		fmt.Printf("  %s00.%s 返回主菜单\n\n", ColorWhite, ColorReset)

		choice := ReadInput("  请输入选项 > ")
		switch choice {
		case "1", "01":
			SettingLog()
		case "2", "02":
			SettingDNS()
		case "3", "03":
			SettingStrategy()
		case "0", "00":
			return
		default:
			fmt.Printf("\n  %s无效输入，请重试...%s\n", ColorGrey, ColorReset)
			Pause("按回车键继续...")
		}
	}
}

// --- 具体设置功能 ---

func SettingLog() {
	fmt.Printf("\n %s   日志配置  %s\n\n", ColorCyan, ColorReset)
	fmt.Printf("  %s01.%s Error (仅错误 - 推荐/默认)\n", ColorWhite, ColorReset)
	fmt.Printf("  %s02.%s Warn  (警告)\n", ColorWhite, ColorReset)
	fmt.Printf("  %s03.%s Info  (信息 - 调试用)\n", ColorWhite, ColorReset)
	fmt.Printf("  %s04.%s Debug (调试 - 极量日志)\n\n", ColorWhite, ColorReset)
	fmt.Printf("  %s00. 返回%s\n\n", ColorGrey, ColorReset)

	choice := ReadInput("请选择 [01-04]: ")
	level := "error"
	switch choice {
	case "1", "01":
		level = "error"
	case "2", "02":
		level = "warn"
	case "3", "03":
		level = "info"
	case "4", "04":
		level = "debug"
	case "0", "00":
		return
	default:
		LogError("无效输入")
		return
	}

	root, _ := ReadConfig()
	root["log"] = map[string]interface{}{"level": level, "timestamp": false}
	WriteConfig(root)
	LogSuccess("日志等级已更新为: %s", level)

	if ReadInput("需要重启服务生效，立即重启? (y/N): ") == "y" {
		ManageService("restart")
	}
}

func SettingDNS() {
	fmt.Printf("\n %s   DNS 策略配置  %s\n\n", ColorCyan, ColorReset)
	fmt.Printf("  %s01.%s 国外优先 (Cloudflare/Google/Quad9) [推荐]\n", ColorWhite, ColorReset)
	fmt.Printf("     %s适合: 境外 VPS，能够访问国际互联网的环境%s\n", ColorGrey, ColorReset)
	fmt.Printf("  %s02.%s 国内优先 (AliDNS/DNSPod)\n", ColorWhite, ColorReset)
	fmt.Printf("     %s适合: 国内服务器或者VPS%s\n\n", ColorGrey, ColorReset)
	fmt.Printf("  %s00. 返回%s\n\n", ColorGrey, ColorReset)

	choice := ReadInput("请选择 [01-02]: ")
	var dnsServers []map[string]interface{}
	switch choice {
	case "1", "01":
		dnsServers = []map[string]interface{}{
			{"type": "udp", "tag": "bootstrap-v4", "server": "1.1.1.1", "server_port": 53},
			{"type": "https", "tag": "dns", "server": "cloudflare-dns.com", "path": "/dns-query", "domain_resolver": "bootstrap-v4"},
			{"type": "https", "tag": "doh-google", "server": "dns.google", "path": "/dns-query", "domain_resolver": "bootstrap-v4"},
			{"type": "https", "tag": "doh-quad9", "server": "dns.quad9.net", "path": "/dns-query", "domain_resolver": "bootstrap-v4"},
		}
		LogInfo("已选择: 国外 DNS 组")
	case "2", "02":
		dnsServers = []map[string]interface{}{
			{"type": "udp", "tag": "bootstrap-cn", "server": "223.5.5.5", "server_port": 53},
			{"type": "https", "tag": "dns", "server": "dns.alidns.com", "path": "/dns-query", "domain_resolver": "bootstrap-cn"},
			{"type": "https", "tag": "doh-tencent", "server": "doh.pub", "path": "/dns-query", "domain_resolver": "bootstrap-cn"},
		}
		LogInfo("已选择: 国内 DNS 组")
	case "0", "00":
		return
	default:
		LogError("无效输入")
		return
	}

	root, _ := ReadConfig()
	root["dns"] = map[string]interface{}{"servers": dnsServers}

	// 保证 route 存在
	if _, ok := root["route"]; !ok {
		root["route"] = map[string]interface{}{"final": "direct", "auto_detect_interface": true}
	}
	if route, ok := root["route"].(map[string]interface{}); ok {
		route["default_domain_resolver"] = "dns"
		root["route"] = route
	}

	WriteConfig(root)
	LogSuccess("DNS 配置已更新")
	if ReadInput("需要重启服务生效，立即重启? (y/N): ") == "y" {
		ManageService("restart")
	}
}

func SettingStrategy() {
	fmt.Printf("\n %s   IP 出站策略   %s\n\n", ColorCyan, ColorReset)
	fmt.Printf("  %s01.%s 优先 IPv6 (prefer_ipv6) [默认]\n", ColorWhite, ColorReset)
	fmt.Printf("  %s02.%s 优先 IPv4 (prefer_ipv4)\n", ColorWhite, ColorReset)
	fmt.Printf("  %s03.%s 仅 IPv4   (ipv4_only)\n", ColorWhite, ColorReset)
	fmt.Printf("  %s04.%s 仅 IPv6   (ipv6_only)\n\n", ColorWhite, ColorReset)
	fmt.Printf("  %s00. 返回%s\n\n", ColorGrey, ColorReset)

	choice := ReadInput("请选择 [01-04]: ")
	strategy := "prefer_ipv6"
	switch choice {
	case "1", "01":
		strategy = "prefer_ipv6"
	case "2", "02":
		strategy = "prefer_ipv4"
	case "3", "03":
		strategy = "ipv4_only"
	case "4", "04":
		strategy = "ipv6_only"
	case "0", "00":
		return
	default:
		LogError("无效输入")
		return
	}

	root, _ := ReadConfig()
	if _, ok := root["route"]; !ok {
		root["route"] = map[string]interface{}{"final": "direct", "auto_detect_interface": true}
	}
	route := root["route"].(map[string]interface{})

	var rules []interface{}
	if r, ok := route["rules"].([]interface{}); ok {
		rules = r
	}

	// 寻找、更新或插入 resolve 规则
	found := false
	for i, r := range rules {
		if rule, isMap := r.(map[string]interface{}); isMap {
			if action, ok := rule["action"].(string); ok && action == "resolve" {
				rule["strategy"] = strategy
				rules[i] = rule
				found = true
				break
			}
		}
	}

	if !found {
		newRule := map[string]interface{}{
			"action":        "resolve",
			"strategy":      strategy,
			"disable_cache": false,
		}
		// 将其置于数组首位
		rules = append([]interface{}{newRule}, rules...)
	}

	route["rules"] = rules
	route["default_domain_resolver"] = "dns"
	root["route"] = route

	WriteConfig(root)
	LogSuccess("出站策略已更新为: %s", strategy)
	if ReadInput("需要重启服务生效，立即重启? (y/N): ") == "y" {
		ManageService("restart")
	}
}
