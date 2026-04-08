package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	StartKey = "scheduled_start"
	StopKey  = "scheduled_stop"
	CronCmd  = "/usr/local/bin/sb"
	LogFile  = "/var/log/sing-box.log"
)

// ScheduledLifecycleMenu 定时启停菜单
func ScheduledLifecycleMenu() {
	for {
		ClearScreen()
		fmt.Print("\n\n")
		fmt.Printf(" %s   定时启停管理  %s\n", ColorCyan, ColorReset)
		fmt.Println(" 功能说明: 每天指定时间(精确到分)自动启动和停止所有服务")

		// 强制获取东八区时间
		loc, _ := time.LoadLocation("Asia/Shanghai")
		now := time.Now().In(loc)
		fmt.Printf(" 系统时间: %s%s (CST)%s\n\n", ColorYellow, now.Format("2006-01-02 15:04:05"), ColorReset)

		// --- 读取当前 Crontab 状态 ---
		out, _ := exec.Command("crontab", "-l").Output()
		cronConfig := string(out)

		startMatch := regexp.MustCompile(`(\d+)\s+(\d+).*` + StartKey).FindStringSubmatch(cronConfig)
		stopMatch := regexp.MustCompile(`(\d+)\s+(\d+).*` + StopKey).FindStringSubmatch(cronConfig)

		if len(startMatch) > 2 && len(stopMatch) > 2 {
			sM, _ := strconv.Atoi(startMatch[1])
			sH, _ := strconv.Atoi(startMatch[2])
			eM, _ := strconv.Atoi(stopMatch[1])
			eH, _ := strconv.Atoi(stopMatch[2])
			fmt.Printf(" 当前状态: %s已启用%s (启动: %02d:%02d | 停止: %02d:%02d)\n\n", ColorGreen, ColorReset, sH, sM, eH, eM)
		} else {
			fmt.Printf(" 当前状态: %s未启用%s\n\n", ColorRed, ColorReset)
		}

		fmt.Printf(" %s[1]%s 设置/修改 定时计划\n", ColorGreen, ColorReset)
		fmt.Printf(" %s[2]%s 删除 定时计划\n", ColorRed, ColorReset)
		fmt.Printf(" %s[0]%s 返回\n\n", ColorYellow, ColorReset)

		choice := ReadInput("选择: ")
		switch choice {
		case "1":
			SetCronJob()
			Pause("操作完成，按回车键刷新...")
		case "2":
			RemoveCronJob()
			Pause("操作完成，按回车键刷新...")
		case "0":
			return
		default:
			LogError("无效选项")
			Pause("按回车键继续...")
		}
	}
}

// SetCronJob 设置定时任务
func SetCronJob() {
	fmt.Printf("\n%s请输入 24小时制时间 (格式 HH:MM)%s\n", ColorYellow, ColorReset)
	startInput := ReadInput("启动时间 (例如 08:30): ")
	stopInput := ReadInput("停止时间 (例如 23:15): ")

	startInput = strings.ReplaceAll(startInput, " ", "")
	stopInput = strings.ReplaceAll(stopInput, " ", "")

	timeRegex := regexp.MustCompile(`^([01]?[0-9]|2[0-3]):([0-5][0-9])$`)

	if !timeRegex.MatchString(startInput) || !timeRegex.MatchString(stopInput) {
		LogError("时间格式错误! 请严格使用 HH:MM (例如 08:30)")
		return
	}

	sH, sM := parseTimeMatch(timeRegex.FindStringSubmatch(startInput))
	eH, eM := parseTimeMatch(timeRegex.FindStringSubmatch(stopInput))

	LogInfo("正在更新定时任务...")

	// 获取现有 cron 排除当前脚本的任务
	out, _ := exec.Command("crontab", "-l").Output()
	var newCron bytes.Buffer
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		// 【严谨校验】必须同时包含面板路径和关键字，才视为本面板的任务并剔除
		isOurTask := strings.Contains(line, CronCmd) && (strings.Contains(line, StartKey) || strings.Contains(line, StopKey))
		if isOurTask {
			continue
		}
		newCron.WriteString(line + "\n")
	}

	// 写入新的任务
	envPath := "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	newCron.WriteString(fmt.Sprintf("%d %d * * * %s %s %s >> %s 2>&1\n", sM, sH, envPath, CronCmd, StartKey, LogFile))
	newCron.WriteString(fmt.Sprintf("%d %d * * * %s %s %s >> %s 2>&1\n", eM, eH, envPath, CronCmd, StopKey, LogFile))

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = &newCron
	if err := cmd.Run(); err != nil {
		LogError("写入 Crontab 失败，请检查系统权限。")
	} else {
		// 尝试重启 Cron 服务
		if InitSystem == "systemd" {
			exec.Command("systemctl", "restart", "cron").Run()
			exec.Command("systemctl", "restart", "crond").Run()
		} else {
			exec.Command("rc-service", "crond", "restart").Run()
		}
		LogSuccess("定时计划已设置：启动 %02d:%02d | 停止 %02d:%02d", sH, sM, eH, eM)
	}
}

func RemoveCronJob() {
	out, _ := exec.Command("crontab", "-l").Output()
	var newCron bytes.Buffer
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		// 【严谨校验】只移除本面板的任务，绝不误伤系统其他任务
		isOurTask := strings.Contains(line, CronCmd) && (strings.Contains(line, StartKey) || strings.Contains(line, StopKey))
		if isOurTask {
			continue
		}
		newCron.WriteString(line + "\n")
	}

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = &newCron
	cmd.Run()
	LogSuccess("定时计划已移除。")
}

func parseTimeMatch(match []string) (int, int) {
	h, _ := strconv.Atoi(match[1])
	m, _ := strconv.Atoi(match[2])
	return h, m
}

// DoScheduledStart 供 crontab 调用的后台启动方法
func DoScheduledStart() {
	ManageService("start")
	// 你可以在这里补充唤醒 Argo 隧道的逻辑 (类似于之前写在 argo.go 中的函数)
	LogSuccess("[Cron] 执行定时启动任务完成")
}

// DoScheduledStop 供 crontab 调用的后台停止方法
func DoScheduledStop() {
	ManageService("stop")
	exec.Command("pkill", "-f", "cloudflared").Run() // 一并停掉所有的 Argo
	LogSuccess("[Cron] 执行定时停止任务完成")
}
