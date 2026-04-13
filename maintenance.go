package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// 在 maintenance.go 顶部定义一个当前版本号（仅用于本地菜单显示）
const CurrentVersion = "v1.a.b"

// 定义全局的 HTTP 客户端，增加超时时间到 300 秒，确保大文件下载稳定
var httpClient = &http.Client{Timeout: 300 * time.Second}

// GithubRelease 用于静默解析 Sing-box 核心的文件名
输入 GithubRelease struct {
	TagName string `json:"tag_name"`
}

// ViewLog 查看服务日志
func ViewLog() {
	ClearScreen()
	LogInfo("正在查看 sing-box 实时日志 (按 Ctrl+C 退出)...")
	var cmd *exec.Cmd
	if InitSystem == "systemd" {
		cmd = exec.Command("journalctl", "-u", "sing-box", "-f", "--no-pager")
	} else {
		cmd = exec.Command("tail", "-f", "/var/log/sing-box.log")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// CheckConfig 检查配置文件语法
func CheckConfig() {
	LogInfo("正在验证配置语法...")
	cmd := exec.Command("/usr/local/bin/sing-box", "check", "-c", ConfigFile)
	if err := cmd.Run(); err != nil {
		LogError("配置文件存在语法错误！")
	} else {
		LogSuccess("配置文件语法正确无误。")
	}
}

// UpdatePanel 采用固定最新链接更新面板程序，不依赖 API 解析
func UpdatePanel() {
	LogInfo("正在从 GitHub 拉取最新面板程序...")

	arch := runtime.GOARCH
	// 直接拼接 GitHub 固定最新 Release 下载地址
	url := fmt.Sprintf("https://github.com/Zzz-IT/singbox-maker-go/releases/latest/download/sbgo-%s", arch)

	tmpPath := "/usr/local/bin/sb.tmp"

	if err := downloadFile(tmpPath, url); err != nil {
		LogError("面板下载失败: %v", err)
		Pause("按回车键返回...")
		return
	}

	os.Chmod(tmpPath, 0755)

	if err := os.Rename(tmpPath, "/usr/local/bin/sb"); err != nil {
		LogError("覆盖旧文件失败: %v", err)
		Pause("按回车键返回...")
		return
	}

	LogSuccess("面板更新完成！")
	Pause("面板已更新，按回车键退出后请重新输入 'sb' 进入。")
	os.Exit(0)
}

// UpdateCore 零依赖提取并更新 Sing-box 核心（静默处理版本）
func UpdateCore(isInteractive bool) {
	LogInfo("正在静默同步 Sing-box 核心组件...")

	// 1. 内部静默获取最新版本以确定文件名（Sing-box 官方包名包含版本号，必须通过此步骤获取）
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/SagerNet/sing-box/releases/latest", nil)
	req.Header.Set("User-Agent", "Singbox-Maker-GO")
	
	resp, err := httpClient.Do(req)
	if err != nil {
		LogError("获取核心信息失败: %v", err)
		return
	}
	defer resp.Body.Close()

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		LogError("解析核心信息失败")
		return
	}

	version := strings.TrimPrefix(release.TagName, "v")

	// 2. 拼接下载 URL
	arch := runtime.GOARCH
	url := fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/v%s/sing-box-%s-linux-%s.tar.gz", version, version, arch)

	// 3. 停止服务并替换
	ManageService("stop")

	LogInfo("正在纯内存解压核心文件...")
	tmpPath := "/usr/local/bin/sing-box.tmp"
	if err := downloadAndExtractGz(url, tmpPath, "sing-box"); err != nil {
		LogError("核心更新失败: %v", err)
		os.Remove(tmpPath)
		ManageService("start")
		// 只在交互模式下暂停
		if isInteractive {
			Pause("按回车键返回维护菜单...")
		}
		return
	}

	os.Chmod(tmpPath, 0755)
	if err := os.Rename(tmpPath, "/usr/local/bin/sing-box"); err != nil {
		LogError("替换核心文件失败: %v", err)
	} else {
		LogSuccess("核心同步完成")
	}

	ManageService("start")
	
	// 只在手动点击菜单更新时暂停，避免初始化时卡死
	if isInteractive {
		Pause("按回车键返回维护菜单...")
	}
}

// Uninstall 卸载程序
func Uninstall() {
	if ReadInput(ColorRed+"警告：此操作将彻底删除配置和核心程序，确认卸载？(y/N): "+ColorReset) != "y" {
		return
	}
	ManageService("stop")
	if InitSystem == "systemd" {
		exec.Command("systemctl", "disable", "sing-box").Run()
	} else if InitSystem == "openrc" {
		exec.Command("rc-update", "del", "sing-box", "default").Run()
	}
	os.RemoveAll("/usr/local/etc/sing-box")
	os.Remove("/usr/local/bin/sing-box")
	os.Remove("/usr/local/bin/cloudflared")
	exec.Command("pkill", "-f", "cloudflared").Run()

	os.Remove("/usr/local/bin/sb")

	LogSuccess("卸载完成，感谢使用！")
	os.Exit(0)
}

// downloadFile 增加 User-Agent 支持和稳定性优化
func downloadFile(filepath string, url string) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Singbox-Maker-GO")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("服务器返回状态码: %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// downloadAndExtractGz 直接从流中解压提取单个目标文件
func downloadAndExtractGz(url, destPath, targetFilename string) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Singbox-Maker-GO")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("服务器返回状态码: %d", resp.StatusCode)
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag == tar.TypeReg && (strings.HasSuffix(header.Name, "/"+targetFilename) || header.Name == targetFilename) {
			out, err := os.Create(destPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			return err
		}
	}
	return fmt.Errorf("未在压缩包中找到 %s", targetFilename)
}
