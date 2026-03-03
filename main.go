package main

import (
	"fmt"
	"os"
	"embed"
	_ "embed"
	"log"
	"runtime"

	"chatclaw/internal/bootstrap"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/sysicon.png
var sysIconDefault []byte

//go:embed build/appicon.png
var sysIconWindows []byte

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU() / 2)
}

func main() {
	// 1. 定义伪装 UA，并在末尾加上腾讯元宝的白名单标识 "app/tencent_yuanbao"
	uaValue := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 app/tencent_yuanbao"

	// 2. 设置 WebView2 启动参数环境变量
	// %q 会自动为字符串添加双引号并处理内部转义，这是 WebView2 识别参数的标准格式
	os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", fmt.Sprintf("--user-agent=%q", uaValue))

	// On macOS the white template icon works perfectly;
	// on Windows we need the dark-outlined variant for taskbar contrast.
	icon := sysIconDefault
	if runtime.GOOS == "windows" {
		icon = sysIconWindows
	}

	app, cleanup, err := bootstrap.NewApp(bootstrap.Options{
		Assets: assets,
		Icon:   icon,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
