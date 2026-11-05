package shell

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// captureState carries the page's self-report between the render poll and the
// capture goroutine. A page that navigates and then awaits host data paints its
// heading first and its cards a moment later: a screenshot taken in between is
// a blank frame, which is exactly what a visual check must not produce.
var (
	captureRenderCount atomic.Int64
	captureMu          sync.Mutex
	captureDone        atomic.Bool
)

// startCapture writes a screenshot of the window after the page has painted,
// then quits. It exists so an automated check can validate the rendered
// result without depending on the window being foreground: a screen grab
// silently returns whatever is on top (lock screen, notification), which makes
// a visual regression invisible.
func (l *Loop) startCapture() {
	path := os.Getenv("ZEN_UI_CAPTURE")
	if path == "" {
		return
	}
	// ZEN_UI_CAPTURE_PAGE optionally navigates before shooting, so a check
	// can cover every page without driving the UI by hand.
	page := os.Getenv("ZEN_UI_CAPTURE_PAGE")

	go func() {
		time.Sleep(4 * time.Second)
		if page != "" {
			// The nav label is the selector: every page is reachable by the text
			// the sidebar shows, which keeps the check independent of internal keys.
			script := fmt.Sprintf(
				"(function(){var n=document.querySelectorAll('.nav-item');for(var i=0;i<n.length;i++){if(n[i].querySelector('.nav-label') && n[i].querySelector('.nav-label').textContent==='%s'){n[i].click();return;}}})()",
				page)
			l.WebView.Eval(script)

			// Wait for content, not for a clock: poll the page's own card count
			// until it stops changing (data-driven pages settle in one IPC round).
			last := int64(-1)
			stable := 0
			deadline := time.Now().Add(8 * time.Second)
			for time.Now().Before(deadline) {
				l.WebView.Eval(renderProbe)
				time.Sleep(250 * time.Millisecond)
				now := captureRenderCount.Load()
				if now > 0 && now == last {
					stable++
					if stable >= 2 {
						break
					}
				} else {
					stable = 0
				}
				last = now
			}
			time.Sleep(400 * time.Millisecond)
		}
		// ZEN_UI_CAPTURE_TAB optionally selects one in-page tab after arriving
		// (网关页的六个面就是这样一个个验的): the tab label is the selector, same
		// convention as the sidebar.
		if tab := os.Getenv("ZEN_UI_CAPTURE_TAB"); tab != "" {
			script := fmt.Sprintf(
				"(function(){var n=document.querySelectorAll('.gw-tab');for(var i=0;i<n.length;i++){if(n[i].textContent==='%s'){n[i].click();return;}}})()",
				tab)
			l.WebView.Eval(script)
			last := int64(-1)
			stable := 0
			deadline := time.Now().Add(8 * time.Second)
			for time.Now().Before(deadline) {
				l.WebView.Eval(renderProbe)
				time.Sleep(250 * time.Millisecond)
				now := captureRenderCount.Load()
				if now > 0 && now == last {
					stable++
					if stable >= 2 {
						break
					}
				} else {
					stable = 0
				}
				last = now
			}
			time.Sleep(500 * time.Millisecond)
		}
		// ZEN_UI_CAPTURE_SCROLL optionally scrolls the content before the grab,
		// so a long page can be checked below its first screen.
		if y := os.Getenv("ZEN_UI_CAPTURE_SCROLL"); y != "" {
			l.WebView.Eval("(function(){var c=document.querySelector('.content');if(c){c.scrollTop=" + y + ";}})()")
			time.Sleep(400 * time.Millisecond)
		} else {
			// Every page renders into the scrollable .content; resetting it before
			// the grab makes the shot independent of what the last render left.
			l.WebView.Eval("(function(){var c=document.querySelector('.content');if(c){c.scrollTop=0;}var b=document.querySelector('.page-body');if(b&&b.parentNode){b.parentNode.scrollTop=0;}})()")
		}
		// ZEN_UI_CAPTURE_EVAL runs one script right before the grab. It is how
		// a check drives a real control (点一个开关、开一个展开器) and then reads
		// the页面's own verdict through the DIAG channel, instead of only trusting
		// that the click was wired up.
		if script := os.Getenv("ZEN_UI_CAPTURE_EVAL"); script != "" {
			l.WebView.Eval(script)
			time.Sleep(2500 * time.Millisecond)
		}
		time.Sleep(600 * time.Millisecond)
		/* PrintWindow(PW_RENDERFULLCONTENT) 取的是最近一次合成完成的帧。页面刚
		   因为点击重建了一批卡片时，那一帧还没被合成器提交，边抓到的就是点击前
		   的旧图——DOM 已经变了、像素还停在原处（_gw_log.png 与 _gw_l.png 逐字节
		   相同就是这么来的）。这里先让页面做一次可观察的重绘（回写一个布局属性
		   再还原），并在抓图前连等两轮，确保新帧已经上屏。 */
		l.WebView.Eval("(function(){var c=document.querySelector('.content');" +
			"if(!c)return;var y=c.scrollTop;c.style.transform='translateZ(0)';c.scrollTop=y;" +
			"void c.offsetHeight;setTimeout(function(){c.style.transform='';},60);})()")
		time.Sleep(500 * time.Millisecond)
		l.WebView.Eval("(function(){void document.body.offsetHeight;})()")
		time.Sleep(500 * time.Millisecond)
		l.Dispatch(func() {
			width, height, err := l.Window.Capture(path)
			if err != nil {
				fmt.Printf("[capture] %v\n", err)
			} else {
				fmt.Printf("[capture] %s (%dx%d)\n", path, width, height)
			}
			l.PostClose()
		})
	}()
}

// renderProbe reports how many content elements the current page has painted.
//
// It counts the cards/infobars the page actually built, so "done" means what
// the eye sees rather than "the navigation click returned".
const renderProbe = `(function(){
	var b = document.querySelector('.page-body');
	var n = 0;
	if (b) {
		n = b.querySelectorAll('.card,.infobar,.card-expander').length;
		n += b.querySelectorAll('.group').length;
	}
	window.chrome.webview.postMessage(JSON.stringify({id:-1, method:'DIAG', payload:{renderCount:n}}));
})();`

// noteRenderCount is called by the DIAG sink for every renderCount report.
func noteRenderCount(n int64) {
	captureRenderCount.Store(n)
}
