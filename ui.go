package main

import (
	"fmt"
	"os"
)

// ShowMainMenu 替代 _main_menu
func ShowMainMenu() {
	for {
		ClearScreen()
		fmt.Print("\n\n")

		// 1. 抬头区域 (完美复刻原版 ASCII)
		fmt.Print(ColorCyan)
		fmt.Println("   _____ _               __                 ")
		fmt.Println("  / ___/(_)___  ____    / /_  ____  _  __   ")
		fmt.Println("  \\__ \\/ / __ \\/ __ \\  / __ \\/ __ \\| |/_/   ")
		fmt.Println(" ___/ / / / / / /_/ / / /_/ / /_/ />  <     ")
		fmt.Println("/____/_/_/ /_/\\__, / /_.___/\\____/_/|_|     ")
		fmt.Println("             /____/         [ M A K E R  G O ] ")
		fmt.Print(ColorReset)

		fmt.Printf("      %sN E T W O R K   D A S H B O A R D%s\n", ColorCyan, ColorReset)

		// 2. 状态检查 (完全动态读取)
		serviceStatus := fmt.Sprintf("%s● Stopped%s", ColorRed, ColorReset)
		if CheckServiceStatus("sing-box") {
			serviceStatus = fmt.Sprintf("%s● Running%s", ColorGreen, ColorReset)
		}

		// 动态获取 Argo 状态
		argoStatus := CheckArgoStatus()

		// 动态获取 系统名称 (如果太长截断以免撑破 UI)
		osName := GetOSName()
		if len(osName) > 28 {
			osName = osName[:25] + "..."
		}

		fmt.Printf("  %s───────────────────────────────────────────%s\n", ColorGrey, ColorReset)
		fmt.Printf("   %sSYSTEM:%s %s%s(Go Engine)%s\n", ColorCyan, ColorReset, ColorWhite, osName, ColorReset)
		fmt.Printf("   %sCORE  :%s %s      %sARGO  :%s %s\n", ColorCyan, ColorReset, serviceStatus, ColorCyan, ColorReset, argoStatus)
		fmt.Printf("  %s───────────────────────────────────────────%s\n", ColorGrey, ColorReset)
		fmt.Println()

		// 3. 菜单选项区
		fmt.Printf("    %sNODE MANAGER%s\n", ColorCyan, ColorReset)
		fmt.Printf("    %s01.%s 添加节点            %s02.%s Argo隧道\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
		fmt.Printf("    %s03.%s 查看链接            %s04.%s 删除节点\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
		fmt.Printf("    %s05.%s 修改端口\n\n", ColorWhite, ColorReset)

		fmt.Printf("    %sSERVICE CONTROL%s\n", ColorCyan, ColorReset)
		fmt.Printf("    %s06.%s 重启服务            %s07.%s 停止服务\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
		fmt.Printf("    %s08.%s 运行状态            %s09.%s 实时日志\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
		fmt.Printf("    %s10.%s 定时启停            %s11.%s 高级设置\n\n", ColorWhite, ColorReset, ColorWhite, ColorReset)

		fmt.Printf("    %sMAINTENANCE%s\n", ColorCyan, ColorReset)
		fmt.Printf("    %s12.%s 检查配置            %s13.%s 更新程序\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
		fmt.Printf("    %s14.%s 更新核心            %s15.%s 卸载所有\n", ColorWhite, ColorReset, ColorRed, ColorReset)

		fmt.Printf("\n  %s───────────────────────────────────────────%s\n", ColorGrey, ColorReset)
		fmt.Printf("    %s00.%s 退出脚本\n\n", ColorWhite, ColorReset)

		// 4. 输入处理
		choice := ReadInput("  请输入选项 > ")

		switch choice {
		case "1", "01":
			ShowAddNodeMenu()
		case "2", "02":
			ShowArgoMenu() // <--- 挂载 Argo 菜单
		case "3", "03":
			ViewNodes()
			Pause("按回车键返回主菜单...")
		case "4", "04":
			DeleteNode()
			Pause("按回车键返回主菜单...")
		case "5", "05":
			ModifyPort()
			Pause("按回车键返回主菜单...")
		case "6", "06":
			ManageService("restart")
			Pause("按回车键返回主菜单...")
		case "7", "07":
			ManageService("stop")
			Pause("按回车键返回主菜单...")
		case "8", "08":
			if CheckServiceStatus("sing-box") {
				LogSuccess("服务正在运行中")
			} else {
				LogError("服务已停止")
			}
			Pause("按回车键返回主菜单...")
		case "9", "09":
			ViewLog() // <--- 挂载查看日志
		case "10":
			ScheduledLifecycleMenu()
		case "11":
			ShowAdvancedMenu() // <--- 完美挂载高级设置
		case "12":
			CheckConfig() // <--- 挂载检查配置
			Pause("按回车键返回主菜单...")
		case "13":
			UpdatePanel() // <--- 更新脚本与核心复用逻辑
		case "14":
			UpdateCore(true)// <--- 挂载更新核心
		case "15":
			Uninstall() // <--- 挂载卸载程序
		case "0", "00":
			os.Exit(0)
		default:
			fmt.Printf("\n  %s无效输入，请重试...%s\n", ColorGrey, ColorReset)
			Pause("按回车键继续...")
		}
	}
}

// ShowAddNodeMenu 替代 _show_add_node_menu
func ShowAddNodeMenu() {
	ClearScreen()
	fmt.Print("\n\n\n")
	fmt.Printf("              %sA D D   N O D E   M E N U%s\n", ColorCyan, ColorReset)
	fmt.Printf("    %s─────────────────────────────────────────────%s\n\n", ColorGrey, ColorReset)

	fmt.Printf("     %s01.%s  VLESS-Reality       %s02.%s  VLESS-WS-TLS\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
	fmt.Printf("     %s03.%s  Trojan-WS-TLS       %s04.%s  AnyTLS\n\n", ColorWhite, ColorReset, ColorWhite, ColorReset)

	fmt.Printf("     %s05.%s  Hysteria2           %s06.%s  TUICv5\n", ColorWhite, ColorReset, ColorWhite, ColorReset)

	// [修复] 补齐被隐藏的 3 个常规节点选项
	fmt.Printf("     %s07.%s  Shadowsocks         %s08.%s  VLESS-TCP\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
	fmt.Printf("     %s09.%s  SOCKS5\n", ColorWhite, ColorReset)

	fmt.Printf("\n    %s─────────────────────────────────────────────%s\n", ColorGrey, ColorReset)
	fmt.Printf("     %s00.%s  返回主菜单\n\n", ColorWhite, ColorReset)

	choice := ReadInput("     请选择协议 > ")
	switch choice {
	case "1", "01":
		AddVLESSReality()
	case "2", "02":
		AddVLESSWSTLS()
	case "3", "03":
		AddTrojanWSTLS()
	case "4", "04":
		AddAnyTLS()
	case "5", "05":
		AddHysteria2()
	case "6", "06":
		AddTUIC()
	case "7", "07":
		AddShadowsocks()
	case "8", "08":
		AddVLESSTCP()
	case "9", "09":
		AddSOCKS5()
	case "0", "00":
		return
	default:
		LogWarn("无效选项，取消操作...")
		return
	}

	// 操作完成后自动重启服务应用配置
	ManageService("restart")
	Pause("操作完成，按回车键继续...")
}
