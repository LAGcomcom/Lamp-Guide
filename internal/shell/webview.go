package shell

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/jchv/go-webview2/pkg/edge"
)

// WebView hosts the page inside the shell window.
//
// The edge.Chromium backend is used directly rather than the wrapper New: the
// wrapper always creates its own top-level window, while the shell must own the
// frame, the DPI handling and the hit testing. Chromium.Embed attaches the child
// controller to an HWND we already created.
type WebView struct {
	chromium *edge.Chromium

	// Bridge answers every request posted by the page. It must return the
	// serialised JSON reply (see bridge.go).
	Bridge func(request string) string

	// Diag receives the page self-report when diagnostics are enabled.
	Diag func(payload []byte)

	dispatch func(func())

	postMu sync.Mutex
}

// NewWebView embeds a WebView2 child into an existing HWND and blocks until the
// controller is ready.
func NewWebView(hwnd uintptr, dataDir string, debug bool, dispatch func(func())) (*WebView, error) {
	ch := edge.NewChromium()
	ch.DataPath = dataDir

	wv := &WebView{chromium: ch, dispatch: dispatch}
	ch.MessageCallback = wv.onMessage

	if !ch.Embed(hwnd) {
		return nil, fmt.Errorf("webview2: failed to embed controller into hwnd %#x", hwnd)
	}

	// The controller starts hidden when it was created for an HWND that was not
	// yet visible, and a hidden controller paints nothing — including its
	// default background. Showing it explicitly is the difference between a
	// window full of page and a window full of nothing.
	if err := ch.Show(); err != nil {
		return nil, fmt.Errorf("webview2: show controller: %w", err)
	}

	paintBackground(ch, systemIsDark())
	configureSettings(ch, debug)
	return wv, nil
}

// onMessage receives a postMessage payload from the page and answers it.
func (wv *WebView) onMessage(raw string) {
	if wv.Bridge == nil {
		return
	}
	go func() {
		reply := wv.Bridge(raw)
		if reply == "" {
			return
		}
		wv.PostJSON(reply)
	}()
}

// PostJSON pushes a finished reply into the page.
func (wv *WebView) PostJSON(payload string) {
	expr := "window.__hostReceive && window.__hostReceive(" + jsStringLiteral(payload) + ")"
	wv.dispatch(func() { wv.chromium.Eval(expr) })
}

// Eval runs a script on the UI thread.
func (wv *WebView) Eval(script string) {
	wv.dispatch(func() { wv.chromium.Eval(script) })
}

// Init injects a script that runs before every page load.
//
// It runs on the calling goroutine rather than through Dispatch: every caller
// installs the script during construction, on the UI thread, and a deferred
// dispatch would land *after* Navigate — the document that needs the script
// would already exist, and the injection would silently apply to the next one.
func (wv *WebView) Init(script string) {
	wv.chromium.Init(script)
}

// ServeFolder maps a virtual https origin onto a local directory.
func (wv *WebView) ServeFolder(host, folder string) error {
	abs, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	v := wv.chromium.GetICoreWebView2_3()
	if v == nil {
		return fmt.Errorf("webview2: ICoreWebView2_3 unavailable (runtime too old)")
	}
	return v.SetVirtualHostNameToFolderMapping(host, abs, edge.COREWEBVIEW2_HOST_RESOURCE_ACCESS_KIND_ALLOW)
}

// Navigate loads the given URL.
func (wv *WebView) Navigate(url string) {
	wv.chromium.Navigate(url)
}

// Resize re-fits the child window to the parent client area.
func (wv *WebView) Resize() {
	wv.chromium.Resize()
}

// Focus moves keyboard focus into the page.
func (wv *WebView) Focus() {
	wv.chromium.Focus()
}

func paintBackground(ch *edge.Chromium, dark bool) {
	controller := ch.GetController()
	if controller == nil {
		return
	}
	c2 := controller.GetICoreWebView2Controller2()
	if c2 == nil {
		return
	}
	// The controller paints this colour before the page first frame. Matching
	// the page background keeps the window from flashing white on startup.
	col := edge.COREWEBVIEW2_COLOR{A: 255, R: 243, G: 243, B: 243} // light #F3F3F3
	if dark {
		col = edge.COREWEBVIEW2_COLOR{A: 255, R: 32, G: 32, B: 32}
	}
	_ = c2.PutDefaultBackgroundColor(col)
}

func configureSettings(ch *edge.Chromium, debug bool) {
	settings, err := ch.GetSettings()
	if err != nil || settings == nil {
		return
	}
	// A desktop app should not answer to Ctrl+P / Ctrl+F / Ctrl+scroll: the page
	// owns its own shortcuts, and browser chrome would break the illusion.
	_ = settings.PutAreBrowserAcceleratorKeysEnabled(false)
	_ = settings.PutIsZoomControlEnabled(false)
	_ = settings.PutIsStatusBarEnabled(false)
	_ = settings.PutIsPinchZoomEnabled(false)
	_ = settings.PutIsSwipeNavigationEnabled(false)
	_ = settings.PutAreDevToolsEnabled(debug)
	_ = settings.PutAreDefaultContextMenusEnabled(debug)
}

// jsStringLiteral quotes a string for embedding inside an Eval expression.
func jsStringLiteral(s string) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		case '\t':
			out = append(out, '\\', 't')
		default:
			if c < 0x20 {
				out = append(out, '\\', 'u', '0', '0', hex[c>>4], hex[c&0x0F])
				continue
			}
			out = append(out, c)
		}
	}
	out = append(out, '"')
	return string(out)
}
