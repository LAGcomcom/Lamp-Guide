// Command Lamp Guide is the desktop front end for the embedded zen gateway.
//
// The process is a single Go binary: it owns the Win32 window (borderless, Mica
// backdrop), hosts a WebView2 child window at the full client area, serves the
// page from an embedded copy of web/ over a virtual https origin, and runs the
// gateway in-process. It has exactly two layers: internal/shell (the window and
// the page host) and internal/host (the page's data door, which forwards every
// call into the gateway's own admin routes). The window is a client of the
// gateway, exactly like any other API consumer.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"zenui/internal/host"
	"zenui/internal/shell"
	"zenui/internal/zen/core"
	"zenui/internal/zen/notify"
	"zenui/internal/zen/update"
	"zenui/internal/zenbridge"
)

//go:embed all:web
var webFS embed.FS

// bootAt is process start, used by the ZEN_UI_TIMING marks below.
var bootAt = time.Now()

func main() {
	// Win32 requires the window to be created and pumped on the thread that
	// owns it; locking keeps the goroutine from migrating mid-call.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := prepareAppIdentity(); err != nil {
		fatal("准备应用标识失败", err)
	}

	uiDir, err := materializeUI()
	if err != nil {
		fatal("准备界面资源失败", err)
	}

	debugMode := os.Getenv("ZEN_UI_DEBUG") == "1"
	// 启动打点：冷启动到底花在哪一段，靠猜没有意义。ZEN_UI_TIMING=1 才打印。
	mark := func(what string) {
		if os.Getenv("ZEN_UI_TIMING") == "1" {
			fmt.Fprintf(os.Stderr, "[timing] %-18s %d ms\n", what, time.Since(bootAt).Milliseconds())
		}
	}
	mark("ui 资源就绪")

	data := host.New()

	// 一键更新换的是"正在跑的这个 exe"。这个程序是界面 + 网关合体，发布页上的
	// zen-gate.exe 是另一套程序：换上去会把这个窗口整个换掉。所以这里关掉原地
	// 更新，界面上会退回到"发布页"按钮，由用户自己决定装什么。
	update.SelfUpdateSupported = false

	// 网关和界面同一个进程：端口、配置、日志都在网关自己的数据目录里。
	// 起不来只影响界面会如实显示原因，不会假装在跑。
	gw, err := startGateway(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "网关启动失败: %v\n", err)
	}
	if gw != nil {
		defer gw.Stop()
	}

	mark("网关就绪")
	loop, err := shell.NewLoop("Lamp Guide", 1180, 780, dataDir(), uiDir, debugMode)
	if err != nil {
		fatal("创建窗口失败", err)
	}
	loop.Handler = data.Dispatch
	mark("窗口与 WebView2 就绪")

	mark("导航已发出")
	loop.Run("https://zen.ui/index.html")

	// The pump returns on WM_QUIT; give WebView2 a moment to flush its data
	// directory before the process exits, otherwise the next start rewrites it.
	select {
	case <-loop.Done():
	case <-time.After(250 * time.Millisecond):
	}
}

// startGateway boots the embedded model gateway and hands its admin router to
// the UI. The gateway owns its own data dir (%APPDATA%\zen-gate or
// ZEN_GATE_HOME), its own port and its own logs.
func startGateway(center *host.Center) (*core.Core, error) {
	c, err := core.Start(context.Background(), core.Options{
		OpenExternal: openBrowser,
		// 模型被限流、免费车道整轮 429 这类事件都不在用户眼前的窗口里，
		// 所以交给系统通知；开关仍然是网关设置里的那一项。
		OnNotify: notify.Toast,
	})
	if err != nil {
		center.SetGateway(nil)
		return nil, err
	}
	center.SetGateway(zenbridge.New(c.Gateway))
	fmt.Fprintf(os.Stderr, "网关已启动: %s\n", c.BaseURL())
	return c, nil
}

// openBrowser hands a URL to the OS default browser.
func openBrowser(url string) {
	if url == "" {
		return
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

// materializeUI writes the embedded page to a per-user cache directory.
//
// WebView2's virtual-host mapping needs a real directory on disk, so the embed
// is unpacked once per build (keyed by the embedded tree's size) into
// %LOCALAPPDATA%\ZenUI\ui.
func materializeUI() (string, error) {
	src, err := fs.Sub(webFS, "web")
	if err != nil {
		return "", err
	}

	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	target := filepath.Join(base, "ZenUI", "ui")

	marker := filepath.Join(target, ".version")
	stamp := buildStamp(src)
	if current, err := os.ReadFile(marker); err == nil && string(current) == stamp {
		return target, nil
	}

	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	if err := copyTree(src, target); err != nil {
		return "", err
	}
	if err := os.WriteFile(marker, []byte(stamp), 0o644); err != nil {
		return "", err
	}
	return target, nil
}

func buildStamp(src fs.FS) string {
	var total int64
	var names int64
	_ = fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			names++
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return fmt.Sprintf("%d-%d", total, names)
}

func copyTree(src fs.FS, target string) error {
	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(target, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
}

func dataDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "ZenUI", "webview")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func fatal(what string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
	os.Exit(1)
}
