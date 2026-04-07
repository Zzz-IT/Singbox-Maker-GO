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
		fmt.Println("             /____/         [ M A K E R  Z ] ")
		fmt.Print(ColorReset)

		fmt.Printf("      %sN E T W O R K   D A S H B O A R D%s\n", ColorCyan, ColorReset)

		// 2. 状态检查
		serviceStatus := fmt.Sprintf("%s● Stopped%s", ColorRed, ColorReset)
		if CheckServiceStatus("sing-box") {
			serviceStatus = fmt.Sprintf("%s● Running%s", ColorGreen, ColorReset)
		}

		// 这里偷懒简写 Argo 状态，实际你可以去检查进程
		argoStatus := fmt.Sprintf("%s○ Not Installed%s", ColorGrey, ColorReset)

		fmt.Printf("  %s───────────────────────────────────────────%s\n", ColorGrey, ColorReset)
		fmt.Printf("   %sSYSTEM:%s %sLinux (Go Engine)%s\n", ColorCyan, ColorReset, ColorWhite, ColorReset)
		fmt.Printf("   %sCORE  :%s %s      %sARGO  :%s %s\n", ColorCyan, ColorReset, serviceStatus, ColorCyan, ColorReset, argoStatus)
		fmt.Printf("  %s───────────────────────────────────────────%s\n", ColorGrey, ColorReset)
		fmt.Println()

		// 3. 菜单选项区
		fmt.Printf("    %sNODE MANAGER%s\n", ColorCyan, ColorReset)
		fmt.Printf("    %s01.%s 添加节点            %s02.%s Argo 隧道\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
		fmt.Printf("    %s03.%s 查看链接            %s04.%s 删除节点\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
		fmt.Printf("    %s05.%s 修改端口\n\n", ColorWhite, ColorReset)

		fmt.Printf("    %sSERVICE CONTROL%s\n", ColorCyan, ColorReset)
		fmt.Printf("    %s06.%s 重启服务            %s07.%s 停止服务\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
		fmt.Printf("    %s08.%s 运行状态            %s09.%s 实时日志\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
		fmt.Printf("    %s10.%s 定时启停            %s11.%s 高级配置\n\n", ColorWhite, ColorReset, ColorWhite, ColorReset)

		fmt.Printf("    %sMAINTENANCE%s\n", ColorCyan, ColorReset)
		fmt.Printf("    %s12.%s 检查配置            %s13.%s 更新脚本\n", ColorWhite, ColorReset, ColorWhite, ColorReset)
		fmt.Printf("    %s14.%s 更新核心            %s15.%s 卸载程序\n", ColorWhite, ColorReset, ColorRed, ColorReset)

		fmt.Printf("\n  %s───────────────────────────────────────────%s\n", ColorGrey, ColorReset)
		fmt.Printf("    %s00.%s 退出脚本\n\n", ColorWhite, ColorReset)

		// 4. 输入处理
		choice := ReadInput("  请输入选项 > ")

		switch choice {
		case "1", "01":
			ShowAddNodeMenu() // 跳转到子菜单
		case "6", "06":
			ManageService("restart")
			Pause("按回车键返回主菜单...")
		case "7", "07":
			ManageService("stop")
			Pause("按回车键返回主菜单...")
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

	fmt.Printf("\n    %s─────────────────────────────────────────────%s\n", ColorGrey, ColorReset)
	fmt.Printf("     %s00.%s  返回主菜单\n\n", ColorWhite, ColorReset)

	choice := ReadInput("     请选择协议 > ")
	switch choice {
	case "1", "01":
		LogInfo("准备进入 VLESS-Reality 部署流程 (这里你可以开始写 Go 的 JSON 构造了)")
	case "0", "00":
		return
	}
	Pause("操作完成，按回车键继续...")
}
