package shell

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// processAt is the shell package's clock: every probe line is stamped with how
// long the process has been up, so "slow to start" can be read as a timeline
// instead of a stopwatch held outside the window.
var processAt = time.Now()

// diagHook is injected before every page load and records the errors that
// would otherwise be swallowed. A hosted page that paints blank almost always
// failed in the module graph, and that happens before any of our own logging
// is installed.
const diagHook = `window.__diagErrors = [];
window.addEventListener('error', function (e) {
	window.__diagErrors.push(String(e.message) + ' @ ' + String(e.filename) + ':' + e.lineno + ':' + e.colno);
}, true);
window.addEventListener('unhandledrejection', function (e) {
	window.__diagErrors.push('rejection: ' + String(e.reason && (e.reason.stack || e.reason)));
});`

// diagProbe asks the page for its render state. It runs on the UI thread via
// Eval and answers through the regular bridge (method DIAG).

// diagProbe asks the page for its render state. It answers through the
// regular bridge (method DIAG), so the reply lands in reportDiag.
const diagProbe = `(function () {
	var out = {
		ready: document.readyState,
		href: String(location.href),
		appChildren: -1,
		bodyLen: -1,
		errors: (window.__diagErrors || []),
		innerWidth: window.innerWidth,
		innerHeight: window.innerHeight,
		dpr: window.devicePixelRatio,
		scrollW: -1,
		toggles: -1, buttons: -1, textBoxes: -1, rightBoxes: -1, rightEmpty: -1
	};
	try {
		out.bodyLen = document.body ? document.body.innerHTML.length : -1;
		var app = document.querySelector('#app');
		if (app) out.appChildren = app.children.length;
		out.scrollW = document.documentElement ? document.documentElement.scrollWidth : -1;
		// 控件计数:一个面把数据画对了、控件却没出现,靠像素是看不出来的。
		out.toggles = document.querySelectorAll('.toggle').length;
		out.buttons = document.querySelectorAll('.btn').length;
		out.textBoxes = document.querySelectorAll('.text-box').length;
		var rb = document.querySelectorAll('.card-right');
		out.rightBoxes = rb.length;
		out.rightEmpty = 0;
		for (var i = 0; i < rb.length; i++) { if (rb[i].children.length === 0) out.rightEmpty++; }
		// 冷启动的时间都花在抓取上：这里把每个资源的耗时原样报出来，
		// 省得靠"感觉慢"去猜是模块图、样式还是第一次网关往返。
		try {
			var res = performance.getEntriesByType('resource') || [];
			out.resources = res.map(function (e) {
				return (e.name.split('/').pop() || e.name) + '@' + Math.round(e.startTime)
					+ '+' + Math.round(e.duration);
			});
			out.marks = (performance.getEntriesByType('mark') || [])
				.map(function (m) { return m.name + '@' + Math.round(m.startTime); });
			var nav = (performance.getEntriesByType('navigation') || [])[0];
			if (nav) {
				out.nav = [Math.round(nav.requestStart), Math.round(nav.responseEnd),
					Math.round(nav.domContentLoadedEventEnd), Math.round(nav.loadEventEnd)].join('/');
			}
		} catch (e2) { out.resources = ['perf:' + String(e2)]; }
		out.logRows = document.querySelectorAll('.gw-log-row').length;
		var empty = document.querySelector('.gw-log-empty');
		out.logEmpty = empty ? empty.textContent.slice(0, 24) : '';
		// 一个控件在 DOM 里但看不见,只有几何与计算样式能说明原因。
		// 主题令牌是否真的存在:缺失时 var() 整条声明失效,控件会"在但看不见"。
		var rootCS = getComputedStyle(document.documentElement);
		out.themeAttr = document.documentElement.getAttribute('data-theme');
		out.themeStore = String((window.__diagTheme || ''));
		out.textPrimary = rootCS.getPropertyValue('--text-primary').trim();
		out.fillControl = rootCS.getPropertyValue('--fill-control').trim();
		out.cardBg = rootCS.getPropertyValue('--card-bg').trim();
		var tg = document.querySelector('.toggle');
		if (tg) {
			var tr = tg.getBoundingClientRect();
			var cs = getComputedStyle(tg);
			out.toggleRect = [Math.round(tr.x), Math.round(tr.y), Math.round(tr.width), Math.round(tr.height)];
			out.toggleStyle = [cs.display, cs.visibility, cs.opacity, cs.borderTopWidth,
				cs.backgroundColor, cs.borderTopColor, cs.position, cs.zIndex].join('|');
		}
	} catch (err) {
		out.errors.push(String(err));
	}
	window.chrome.webview.postMessage(JSON.stringify({ id: -1, method: 'DIAG', payload: out }));
})();`

// importProbe tries to load each module the shell depends on and reports the
// failure text. A module-graph error does not always surface through window.onerror,
// so this asks the browser directly.
const importProbe = `(async function () {
	var out = { mods: [], error: '' };
	var list = ['./js/util.js', './js/bridge.js', './js/components/index.js',
		'./js/pages/gateway.js', './js/shell.js'];
	for (var i = 0; i < list.length; i++) {
		try {
			await import(list[i]);
			out.mods.push(list[i] + ':ok');
		} catch (err) {
			out.mods.push(list[i] + ':FAIL');
			out.error = list[i] + ' — ' + String(err && (err.stack || err.message || err));
			break;
		}
	}
	window.chrome.webview.postMessage(JSON.stringify({ id: -1, method: 'DIAG', payload: out }));
})();`

// paintProbe reports when the page actually painted, measured inside the page.
// The host's own clock cannot answer this: Navigate returns as soon as the
// request is queued, and the first frame arrives several hundred milliseconds
// later, after the module graph and the first state round-trip.
const paintProbe = `(function(){
  var t0 = performance.now();
  var done = false;
  function report(label) {
    if (done) return;
    done = true;
    window.chrome.webview.postMessage(JSON.stringify({id:-1, method:'DIAG', payload:{
      paint: label, afterCreate: Math.round(performance.now() - t0),
      at: Math.round(performance.now())
    }}));
  }
  // 这个脚本在文档元素还不存在时就执行，所以只能等 DOM 树建好再挂观察器。
  document.addEventListener('DOMContentLoaded', function () {
    if (document.querySelector('.page-body .card')) { report('dom-first-card'); return; }
    var obs = new MutationObserver(function(){
      if (document.querySelector('.page-body .card') || document.querySelector('.gw-log')) report('first-card');
    });
    obs.observe(document.documentElement, {childList:true, subtree:true});
  });
})();`

// startDiag installs the collector and probes the page a few times after load.
// Enabled with ZEN_UI_DIAG=1; output goes to stdout.
func (l *Loop) startDiag() {
	if os.Getenv("ZEN_UI_DIAG") != "1" {
		return
	}
	l.WebView.Init(diagHook)
	go func() {
		for i := 0; i < 8; i++ {
			time.Sleep(1000 * time.Millisecond)
			l.WebView.Eval(diagProbe)
			l.WebView.Eval(navProbe)
			if i == 0 {
				l.WebView.Eval(importProbe)
			}
			if i >= 2 {
				l.WebView.Eval(gwTraceProbe)
			}
		}
	}()
}

func (l *Loop) reportDiag(payload []byte) {
	// A renderCount report is the capture loop's readiness signal, not noise:
	// record it and stay quiet so a screenshot run prints only its own lines.
	var probe struct {
		RenderCount *int64 `json:"renderCount"`
	}
	if err := json.Unmarshal(payload, &probe); err == nil && probe.RenderCount != nil {
		noteRenderCount(*probe.RenderCount)
		return
	}
	fmt.Printf("[diag +%dms] %s\n", time.Since(processAt).Milliseconds(), string(payload))
}

// scaleProbe reports what the window and the page each believe the DPI scale is.
const scaleProbe = `(function(){
  window.chrome.webview.postMessage(JSON.stringify({id:-1, method:'DIAG', payload:{
    host: 'page', innerWidth: window.innerWidth, dpr: window.devicePixelRatio
  }}));
})();`

// gwProbe asks the host for the gateway state through the same bridge the page
// uses, and reports what came back. A blank page means the call answered
// ok:false, and that reason never reaches window.onerror.
const gwProbe = `(function(){
	var msg = JSON.stringify({id: 900001, method: 'Gateway', payload: {path: 'state', body: null}});
	var orig = window.__hostReceive;
	window.__hostReceive = function (raw) {
		var m = null;
		try { m = JSON.parse(raw); } catch (e) { m = null; }
		if (m && m.id === 900001) {
			window.__hostReceive = orig;
			var d = m.data;
			window.chrome.webview.postMessage(JSON.stringify({id:-1, method:'DIAG', payload:{
				ok: m.ok, error: m.error || '',
				dataType: typeof d,
				dataKeys: d && typeof d === 'object' ? Object.keys(d).slice(0, 10) : null,
				models: d && d.models ? d.models.length : -1,
				bodyLen: document.body ? document.body.innerHTML.length : -1
			}}));
			return;
		}
		if (orig) return orig(raw);
	};
	window.chrome.webview.postMessage(msg);
})();`

// gwTraceProbe reports what the 网关 page's own calls returned.
const gwTraceProbe = `(function(){
	var t = window.__gwTrace || [];
	var b = document.querySelector('.page-body');
	window.chrome.webview.postMessage(JSON.stringify({id:-1, method:'DIAG', payload:{
		trace: t.slice(-6),
		bodyLen: b ? b.innerHTML.length : -1,
		firstCards: b ? b.querySelectorAll('.card').length : -1
	}}));
})();`

// navProbe reports which page the shell believes is current.
const navProbe = `(function(){
	var h1 = document.querySelector('.page-head h1');
	var sel = document.querySelector('.nav-item.is-selected .nav-label');
	window.chrome.webview.postMessage(JSON.stringify({id:-1, method:'DIAG', payload:{
		page: h1 ? h1.textContent : '(no h1)',
		navSelected: sel ? sel.textContent : '(none)',
		navCount: document.querySelectorAll('.nav-item').length
	}}));
})();`
