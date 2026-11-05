package shell

import (
	"encoding/json"
	"fmt"
)

// request is what the page posts through window.chrome.webview.postMessage.
//
// The shape mirrors web/js/bridge.js: one request per id, one reply per id.
type request struct {
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload"`
}

// reply is what the page receives back through window.__hostReceive.
//
// ok=false carries a human-readable error the page shows in an InfoBar; the
// page distinguishes "did not reach the host" (rejected promise) from "host
// refused" (ok:false), so the error text has to be meaningful.
type reply struct {
	ID    int             `json:"id"`
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// Handler answers non-window methods. main wires it to the backend dispatcher.
type Handler func(method string, payload json.RawMessage) (json.RawMessage, error)

// Handle is the single entry point for page → host calls.
//
// Window controls are answered here because they are properties of the shell,
// not of the settings backend; everything else goes to the handler injected by
// main. Unknown methods return ok:false instead of silently succeeding, so a
// typo in the page surfaces as a visible error rather than a dead control.
func (l *Loop) Handle(raw string) string {
	var req request
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return marshal(reply{ID: req.ID, OK: false, Error: "请求格式不合法"})
	}

	if req.Method == "DIAG" {
		if l.WebView != nil && l.WebView.Diag != nil {
			l.WebView.Diag(req.Payload)
		}
		return marshal(reply{ID: req.ID, OK: true})
	}

	if res, handled := l.handleWindow(req); handled {
		res.ID = req.ID
		return marshal(res)
	}

	if l.Handler == nil {
		return marshal(reply{ID: req.ID, OK: false, Error: "后台尚未就绪"})
	}

	data, err := l.Handler(req.Method, req.Payload)
	if err != nil {
		return marshal(reply{ID: req.ID, OK: false, Error: err.Error()})
	}

	// The backend answers with its own envelope ({ok, data} or {ok, error}),
	// because it needs to report refusals the page must roll back. Unwrap it
	// here so the page sees exactly one envelope shape regardless of whether the
	// call was handled by the shell or by the backend.
	var env struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &env); err == nil {
			return marshal(reply{ID: req.ID, OK: env.OK, Data: env.Data, Error: env.Error})
		}
	}
	return marshal(reply{ID: req.ID, OK: true, Data: data})
}

// handleWindow answers the shell-owned methods. The bool reports whether the
// method belonged to this group.
func (l *Loop) handleWindow(req request) (reply, bool) {
	switch req.Method {
	case "Window.Minimize":
		l.Window.ShowMinimized()
		return reply{OK: true}, true

	case "Window.ToggleMaximize":
		l.Window.ToggleMaximize()
		return reply{OK: true}, true

	case "Window.IsMaximized":
		data, _ := json.Marshal(map[string]bool{"isMaximized": l.Window.IsMaximized()})
		return reply{OK: true, Data: data}, true

	case "Window.Close":
		l.PostClose()
		return reply{OK: true}, true

	case "Window.SetHitRegions":
		var hits HitRegions
		if len(req.Payload) > 0 {
			if err := json.Unmarshal(req.Payload, &hits); err != nil {
				return reply{OK: false, Error: "标题栏热区参数不合法"}, true
			}
		}
		l.Window.SetHitRegions(hits)
		return reply{OK: true}, true

	// Both of these hand control to a modal OS loop that blocks until the
	// user releases the button, and both depend on thread-affine state
	// (mouse capture). They cannot run on the bridge goroutine, so they are
	// dispatched and answered immediately — the loop is interactive feedback
	// by itself, and the page has nothing to wait for.
	case "Window.StartDrag":
		l.Dispatch(l.Window.StartDrag)
		return reply{OK: true}, true

	case "Window.StartResize":
		var arg struct {
			Edge string `json:"edge"`
		}
		if len(req.Payload) > 0 {
			_ = json.Unmarshal(req.Payload, &arg)
		}
		edge := arg.Edge
		l.Dispatch(func() { l.Window.StartResize(edge) })
		return reply{OK: true}, true

	case "Window.State":
		scale := l.Window.Scale()
		width, height := l.Window.ClientRect()
		data, _ := json.Marshal(map[string]any{
			"isMaximized":  l.Window.IsMaximized(),
			"scale":        scale,
			"clientWidth":  width,
			"clientHeight": height,
		})
		return reply{OK: true, Data: data}, true
	}
	return reply{}, false
}

func marshal(r reply) string {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf(`{"id":%d,"ok":false,"error":"序列化失败"}`, r.ID)
	}
	return string(b)
}
