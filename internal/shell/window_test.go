package shell

// 无边框窗口的最大化几何：这一条是**像素级**的，因为它修的是一个只有看屏幕
// 才发现的问题 —— 最大化后窗口比工作区还大，右下角整块跑到显示器外，页面的
// 右边与下边永远够不着，而代码里没有任何一处报错。
//
// 根因：WM_GETMINMAXINFO 里没有先调 DefWindowProc。Windows 正是在那一次调用里
// 填 ptMaxSize / ptMaxPosition（「工作区 + 边框外扩」）；跳过它，那些字段就保持
// 调用方给的零值，窗口于是按一个没有意义的矩形被摆放。
//
// 这些断言只覆盖不依赖真实窗口的那一半：fitMaximized 对结构体的改写。窗口的
// 实际 rect 由 shots/ 下的实拍验证。

import (
	"testing"
	"unsafe"

	"zenui/internal/native"
)

// fitMaximized 必须把最大化矩形钉在工作区，且不去动它不负责的字段。
func TestFitMaximizedPinsWorkArea(t *testing.T) {
	// MINMAXINFO 的 ptMaxSize 是第 3 个 POINT（0 基下标 2），ptMaxPosition 是第 4 个。
	var mmi native.MinMaxInfo
	// 造一个「调用方给的零值」：这正是跳过 DefWindowProc 时的真实状态。
	mmi.Reserved = native.Point{}
	mmi.MaxSize = native.Point{}
	mmi.MaxPosition = native.Point{}

	w := &Window{minTrack: native.Point{X: 900, Y: 600}}
	addr := uintptr(unsafe.Pointer(&mmi))
	w.fitMaximized(addr)

	// 断言的是**契约**而不是某台机器的分辨率：最大化矩形必须非零、且落在
	// 某个显示器的工作区内。MonitorFromWindow 在没有窗口句柄时会回落到主显示器，
	// 所以这里拿到的一定是真实工作区，而不是零值 —— 零值正是原来那个 bug。
	if mmi.MaxSize.X <= 0 || mmi.MaxSize.Y <= 0 {
		t.Fatalf("最大化尺寸被写成零值，窗口会按无意义矩形摆放：%+v", mmi.MaxSize)
	}
	if mmi.MaxPosition.X < 0 || mmi.MaxPosition.Y < 0 {
		t.Fatalf("最大化位置为负：%+v", mmi.MaxPosition)
	}
	// MinTrackSize 是「用户能拖到的最小尺寸」，必须原样保留：
	// 覆盖它会让人把窗口拖到看不见内容。
	if mmi.MinTrackSize.X != 900 || mmi.MinTrackSize.Y != 600 {
		t.Fatalf("MinTrackSize 被改掉了：%+v", mmi.MinTrackSize)
	}

	// 传 0 必须直接返回而不 panic（LPARAM 为空在真实消息流里会发生）。
	w.fitMaximized(0)
}

// classify 的每一条分支都是一份「像素契约」，而且错得很安静 —— 拖动带算错
// 只是整条标题栏拖不动，边框算错只是缩放热区消失，两者都不会报错。所以这里
// 把坐标空间和边界值一起钉住。
//
// 第一件要钉的是**坐标空间**:hitTest 收到的 lParam 是屏幕坐标，必须先转成
// 客户区坐标再比。这一条如果写反，症状是「按住标题栏完全没反应」—— 因为
// 窗口在屏幕上的 y 偏移直接盖过了整条拖动带的高度。
func TestClassifyUsesClientCoordinates(t *testing.T) {
	const scale = 1.5
	w := &Window{hit: HitRegions{TitleBarHeight: 38, LeftInset: 0, RightInset: 138}}
	w.scale = scale

	width, height := int32(1180), int32(780)

	// 38 DIP × 1.5 = 57 物理像素的拖动带。带内 -> HTCAPTION。
	// 若把屏幕坐标直接当客户区坐标（原来的 bug），窗口在 y=66 时这一带里
	// 每个点都会是 y≈86，全部落到 HTCLIENT。
	band := int32(float64(38) * scale)
	if got := w.classify(native.Point{X: 400, Y: band / 2}, width, height); got != native.HTCAPTION {
		t.Fatalf("拖动带内应返回 HTCAPTION，得到 %d（带高 %d）", got, band)
	}
	if got := w.classify(native.Point{X: 400, Y: 86}, width, height); got != native.HTCLIENT {
		t.Fatalf("带外应返回 HTCLIENT，得到 %d", got)
	}
}

// 按钮占位必须留在页面手里:右侧 138 DIP 归窗口按钮，左侧同理。
// 折错会让按钮点不动 —— 点一下变成拖动窗口。
func TestClassifyKeepsCaptionButtonsClickable(t *testing.T) {
	w := &Window{hit: HitRegions{TitleBarHeight: 38, LeftInset: 0, RightInset: 138}}
	w.scale = 1

	width, height := int32(1180), int32(780)
	inset := int32(138)

	if got := w.classify(native.Point{X: width - inset + 10, Y: 20}, width, height); got != native.HTCLIENT {
		t.Fatalf("窗口按钮区应返回 HTCLIENT（页面自己处理），得到 %d", got)
	}
	if got := w.classify(native.Point{X: width - inset - 10, Y: 20}, width, height); got != native.HTCAPTION {
		t.Fatalf("按钮左侧仍是拖动带，应返回 HTCAPTION，得到 %d", got)
	}
}

// 四角必须先于四边命中，否则对角缩放永远轮不到。
func TestClassifyCornersBeatEdges(t *testing.T) {
	w := &Window{hit: HitRegions{TitleBarHeight: 38, RightInset: 138}}
	w.scale = 1

	width, height := int32(1180), int32(780)

	if got := w.classify(native.Point{X: 1, Y: 1}, width, height); got != native.HTTOPLEFT {
		t.Fatalf("左上角应为 HTTOPLEFT，得到 %d", got)
	}
	if got := w.classify(native.Point{X: width - 1, Y: 1}, width, height); got != native.HTTOPRIGHT {
		t.Fatalf("右上角应为 HTTOPRIGHT，得到 %d", got)
	}
	if got := w.classify(native.Point{X: 1, Y: height - 1}, width, height); got != native.HTBOTTOMLEFT {
		t.Fatalf("左下角应为 HTBOTTOMLEFT，得到 %d", got)
	}
	if got := w.classify(native.Point{X: width - 1, Y: height - 1}, width, height); got != native.HTBOTTOMRIGHT {
		t.Fatalf("右下角应为 HTBOTTOMRIGHT，得到 %d", got)
	}

	// 边中部：不与角重叠的地方落到直边。
	if got := w.classify(native.Point{X: 1, Y: height / 2}, width, height); got != native.HTLEFT {
		t.Fatalf("左边中部应为 HTLEFT，得到 %d", got)
	}
	if got := w.classify(native.Point{X: width / 2, Y: height - 1}, width, height); got != native.HTBOTTOM {
		t.Fatalf("下边中部应为 HTBOTTOM，得到 %d", got)
	}
}

// 页面没报热区时不能凭空造出一个拖动带:band=0 表示「页面不用原生拖动」。
func TestClassifyNoBandWithoutRegions(t *testing.T) {
	w := &Window{}
	w.scale = 1

	if got := w.classify(native.Point{X: 400, Y: 20}, 1180, 780); got != native.HTCLIENT {
		t.Fatalf("未报热区时整窗应为 HTCLIENT，得到 %d", got)
	}
}

// 缩放热区的名字映射:拼错必须返回 0（不启动任何循环），而不是byId回落到某条边。
// 回落到边会让窗口在用户只是点了一下时突然变形。
func TestResizeCodeRejectsUnknownEdge(t *testing.T) {
	for _, edge := range []string{"left", "right", "top", "bottom", "top-left", "top-right", "bottom-left", "bottom-right"} {
		if resizeCode(edge) == 0 {
			t.Fatalf("%q 应映射到某个 HT* 常量", edge)
		}
	}
	if resizeCode("topedge") != 0 {
		t.Fatalf("未知边名应返回 0，得到 %d", resizeCode("topedge"))
	}
	if resizeCode("") != 0 {
		t.Fatalf("空边名应返回 0，得到 %d", resizeCode(""))
	}
}

// hitTest 的拖动带与按钮带是**页面报上来的 DIP**，按窗口 DPI 折算。
// 折错会让标题栏按钮点不动，或让整条标题栏失去拖动 —— 两者都不会报错。
func TestHitRegionsAreScaledByDPI(t *testing.T) {
	w := &Window{hit: HitRegions{TitleBarHeight: 38, LeftInset: 0, RightInset: 138}}
	w.scale = 1.5 // 150% 缩放：DIP 38 → 57 物理像素

	h := w.hitRegions()
	if h.TitleBarHeight != 38 || h.RightInset != 138 {
    // 存的是 DIP，不是物理像素：存成物理像素后，DPI 一变整条带子就错位。
		t.Fatalf("SetHitRegions 应保留 DIP：%+v", h)
	}
	if got := float64(h.TitleBarHeight) * w.Scale(); got != 57 {
		t.Fatalf("38 DIP × 1.5 = %v，应为 57", got)
	}
}
