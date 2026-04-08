package main

import (
	"os"
)

func main() {
	// 0. 全局环境自愈与初始化 (替代原来的 DetectInitSystem)
	InitRuntime()

	// 【新增这一行】0.5 运行配置自检兜底，防止基础配置为空导致后续设置崩溃
	CheckAndFillDefaults()

	// 1. 拦截命令行参数 (实现快捷部署与定时任务)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "start":
			ManageService("start")
		case "stop":
			ManageService("stop")
		case "restart":
			ManageService("restart")
		case "scheduled_start":
			DoScheduledStart()
		case "scheduled_stop":
			DoScheduledStop()
		default:
			LogError("未知的命令参数: %s", os.Args[1])
		}
		return
	}

	// 3. 进入交互式主菜单
	ShowMainMenu()
}
