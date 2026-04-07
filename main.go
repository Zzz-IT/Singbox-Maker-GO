package main

import (
	"os"
)

func main() {
	// 1. 拦截命令行参数 (实现快捷部署)
	if len(os。Args) > 1 {
		switch os。Args[1] {
		case "start":
			ManageService("start")
		case "stop":
			ManageService("stop")
		case "restart":
			ManageService("restart")
		default:
			LogError("未知的命令参数: %s", os。Args[1])
		}
		return
	}

	// 2. 环境初始化
	// CheckRoot() // 测试时可以先注释掉这行
	DetectInitSystem()

	// 3. 进入交互式主菜单
	ShowMainMenu()
}
