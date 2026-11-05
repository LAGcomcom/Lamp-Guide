package shell

import (
	"os"
	"sync"
	"unsafe"

	"zenui/internal/native"
)

// Loop owns the UI thread: the window, the WebView and the message pump.
//
// Win32 requires that a window be created and pumped on the thread that owns
// it, and Go's goroutine scheduler can move goroutines between OS threads at
// any safepoint. main therefore calls runtime.LockOSThread before Run, and all
// UI work goes through Dispatch.
type Loop struct {
	Window  *Window
	WebView *WebView

	threadID uintptr

	queueMu sync.Mutex
	queue   []func()

	// Handler answers the page's data calls. main wires it to host.Center,
	// which forwards each one into the gateway's own admin routes.
	Handler Handler

	// quit is closed when the pump exits, so main can wait for teardown.
	quit chan struct{}
}

// NewLoop creates the window and the page host, and wires them together.
func NewLoop(title string, widthDIP, heightDIP int, dataDir, uiDir string, debug bool) (*Loop, error) {
	l := &Loop{quit: make(chan struct{})}

	win, err := New(title, widthDIP, heightDIP)
	if err != nil {
		return nil, err
	}
	l.Window = win

	tid, _, _ := native.Kernel32GetCurrentThreadID.Call()
	l.threadID = tid

	win.OnResize = func(width, height int32) {
		if l.WebView != nil {
			l.WebView.Resize()
		}
	}
	win.OnThemeChanged = func(dark bool) {
		if l.WebView != nil {
			l.WebView.Eval("window.__hostTheme && window.__hostTheme(" + boolLiteral(dark) + ")")
		}
	}
	win.OnClose = func() bool { return true }

	/* 先把窗口摆出来，再去建 WebView2 环境。创建环境和控制器要一次完整的
	   Edge 运行时拉起（实测约 0.9s），这期间窗口如果还藏着，用户看到的就是
	   "双击了、什么都没发生"。窗口自己带主题底色，先亮出来即是底色，页面
	   一好就盖上去。 */
	win.Show()

	wv, err := NewWebView(uintptr(win.Hwnd), dataDir, debug, l.Dispatch)
	if err != nil {
		win.Destroy()
		return nil, err
	}
	l.WebView = wv

	if err := wv.ServeFolder("zen.ui", uiDir); err != nil {
		win.Destroy()
		return nil, err
	}

	win.SetMinTrackDIP(820, 560)

	wv.Bridge = l.Handle
	wv.Diag = l.reportDiag
	l.startDiag()
	return l, nil
}

func boolLiteral(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// Dispatch schedules f on the UI thread. Safe to call from any goroutine.
func (l *Loop) Dispatch(f func()) {
	l.queueMu.Lock()
	l.queue = append(l.queue, f)
	l.queueMu.Unlock()
	native.User32PostThreadMessageW.Call(l.threadID, native.WM_APP, 0, 0)
}

func (l *Loop) drain() {
	l.queueMu.Lock()
	q := append([]func(){}, l.queue...)
	l.queue = l.queue[:0]
	l.queueMu.Unlock()
	for _, f := range q {
		f()
	}
}

// Run shows the window, loads the page and pumps messages until WM_QUIT.
//
// It must be called on a locked OS thread (see main).
func (l *Loop) Run(startURL string) {
	l.Window.Show()
	l.WebView.Resize()
	if os.Getenv("ZEN_UI_TIMING") == "1" {
		// 首帧自报必须赶在导航之前挂上：Init 的脚本只对之后创建的文档生效。
		l.WebView.Init(paintProbe)
	}
	l.WebView.Navigate(startURL)
	l.WebView.Focus()

	l.startCapture()

	var msg native.Msg
	for {
		r, _, _ := native.User32GetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 || int32(r) == -1 {
			break
		}
		if msg.Message == native.WM_APP {
			l.drain()
			continue
		}
		native.User32TranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		native.User32DispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	close(l.quit)
}

// Done is closed when the message pump has exited.
func (l *Loop) Done() <-chan struct{} {
	return l.quit
}

// PostClose asks the pump to shut down: the page's own close button routes
// here through Window.Close.
func (l *Loop) PostClose() {
	native.User32PostMessageW.Call(uintptr(l.Window.Hwnd), native.WM_CLOSE, 0, 0)
}
