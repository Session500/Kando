package look

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// ActionBar is the bar of a page's actions at its foot, the last of its column, where a thumb reaches them: over the software keyboard while it shows, else over iOS's floating tab bar or the safe area.
func ActionBar(cx *core.Scope, children ...core.Node) core.Node {
	return core.View{Style: core.Style{PaddingEnv: core.EnvEdges{Right: core.EnvSafeArea, Left: core.EnvSafeArea}},
		// EnvEdges takes one inset an edge, so the bottom's switches between the keyboard's, the tab bar's and the safe area's.
		StyleFunc: func() core.Style {
			pad := core.Edges{Top: 12, Right: 16, Left: 16}
			env := core.EnvEdges{Right: core.EnvSafeArea, Left: core.EnvSafeArea}
			switch inset := ui.TabBarInset(cx); {
			case cx.Env().KeyboardHeight() > 0:
				pad.Bottom, env.Bottom = 8, core.EnvKeyboard
			case inset > 0:
				pad.Bottom = 12 + inset
			default:
				pad.Bottom, env.Bottom = 12, core.EnvSafeArea
			}
			return core.Style{Padding: pad, PaddingEnv: env}
		},
		Children: []core.Node{core.View{Style: core.Style{MaxWidth: core.Pt(560), Width: core.Pct(100), AlignSelf: core.AlignCenter, Gap: 8}, Children: children}}}
}
