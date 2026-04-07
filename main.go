package main

import (
	"os"
)

func main() {
	// 环境初始化 (必须先执行，因为计划任务是在后台跑的，也需要知道系统类型)
	DetectInitSystem()

	// 1. 拦截命令行参数 (实现快捷部署与定时任务)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "start":
			ManageService("start")
		case "stop":
			ManageService("stop")
		case "restart":
			ManageService("restart")
		case "scheduled_start": // <--- [新增] 响应 cron 的启动指令
			DoScheduledStart()
		case "scheduled_stop": // <--- [新增] 响应 cron 的停止指令
			DoScheduledStop()
		default:
			LogError("未知的命令参数: %s", os.Args[1])
		}
		return
	}

	// 3. 进入交互式主菜单
	ShowMainMenu()
}
