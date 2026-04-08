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

// 定义全局的 HTTP 客户端，设置 15 秒超时，防止无限挂起
var httpClient = &http.Client{Timeout: 15 * time.Second}

// GithubRelease 用于解析 GitHub API 返回的 JSON
type GithubRelease struct {
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

// UpdatePanel 更新脚本自身 (自更新逻辑)
func UpdatePanel() {
	LogInfo("准备更新面板核心程序...")

	// runtime.GOARCH 会自动返回当前程序编译时的架构 (如 "amd64" 或 "arm64")
	arch := runtime.GOARCH
	url := fmt.Sprintf("https://raw.githubusercontent.com/Zzz-IT/singbox-maker-go/main/sbgo-%s", arch)

	tmpPath := "/usr/local/bin/sb.tmp"

	// 1. 下载新版本到临时文件
	LogInfo("正在获取最新版本 (架构: %s)...", arch)
	if err := downloadFile(tmpPath, url); err != nil {
		LogError("面板下载失败: %v", err)
		return
	}

	// 2. 赋予执行权限
	os.Chmod(tmpPath, 0755)

	// 3. 原子级覆盖自身
	if err := os.Rename(tmpPath, "/usr/local/bin/sb"); err != nil {
		LogError("覆盖旧文件失败: %v", err)
		return
	}

	LogSuccess("面板更新完成！")
	LogInfo("程序即将自动退出，请重新输入 'sb' 进入最新版面板。")
	os.Exit(0)
}

// UpdateCore 零依赖提取并更新 Sing-box 核心
func UpdateCore() {
	LogInfo("准备更新 Sing-box 核心程序...")

	// 1. 通过 GitHub API 获取最新版本号
	resp, err := httpClient.Get("https://api.github.com/repos/SagerNet/sing-box/releases/latest")
	if err != nil {
		LogError("获取最新版本号失败: %v", err)
		return
	}
	defer resp.Body.Close()

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		LogError("解析版本号失败")
		return
	}

	version := strings.TrimPrefix(release.TagName, "v")
	LogInfo("检测到 sing-box 最新版本: %s", version)

	// 2. 拼接下载 URL
	arch := runtime.GOARCH
	// Sing-box 官方包名采用标准 GOARCH，例如 sing-box-1.8.0-linux-amd64.tar.gz
	url := fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/v%s/sing-box-%s-linux-%s.tar.gz", version, version, arch)

	// 3. 停止当前服务以释放内存和解除文件占用
	ManageService("stop")

	LogInfo("正在纯内存解压核心文件，请稍候...")

	// 【新增修复】先下载到临时路径
	tmpPath := "/usr/local/bin/sing-box.tmp"
	if err := downloadAndExtractGz(url, tmpPath, "sing-box"); err != nil {
		LogError("核心更新失败，网络中断或压缩包损坏: %v", err)
		os.Remove(tmpPath)     // 清理可能残缺的临时文件
		ManageService("start") // 恢复旧版本核心运行
		return
	}

	// 【新增修复】下载成功后，赋予权限并替换原文件
	os.Chmod(tmpPath, 0755)
	if err := os.Rename(tmpPath, "/usr/local/bin/sing-box"); err != nil {
		LogError("替换核心文件失败: %v", err)
	} else {
		LogSuccess("核心更新完成")
	}

	ManageService("start")
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

	// 删除自己
	os.Remove("/usr/local/bin/sb")

	LogSuccess("卸载完成，感谢使用！")
	os.Exit(0)
}

// =====================================
// 底层辅助函数: 零依赖网络与解压流处理
// =====================================

// downloadFile 用于简单的单一文件下载
func downloadFile(filepath string, url string) error {
	resp, err := httpClient.Get(url)
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

// downloadAndExtractGz 直接从流中解压提取单个目标文件，极其节省内存
func downloadAndExtractGz(url, destPath, targetFilename string) error {
	resp, err := httpClient.Get(url)
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
			break // 读到压缩包末尾
		}
		if err != nil {
			return err
		}

		// 匹配特定的文件名 (跳过外层文件夹结构)
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
