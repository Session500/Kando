package look

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// Empty is what a page shows in place of a list with nothing in it, or that could not be read: an icon over a line, centered, and an action under them when there is one to take.
type Empty struct {
	Icon   func() *core.SVG
	Title  func() string
	Action core.Node
	TestID string
}

// Render implements core.Node.
func (e Empty) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	children := []core.Node{
		ui.IconOf(e.Icon, 44, colors.Faint),
		core.Text{ContentFunc: e.Title, Style: Title.Merge(core.Style{TextAlign: core.TextAlignCenter}), StyleFunc: ui.Colored(colors.Muted)},
	}
	if e.Action != nil {
		children = append(children, e.Action)
	}
	return core.View{TestID: e.TestID, Style: core.Style{FlexGrow: 1, JustifyContent: core.JustifyCenter}, Children: []core.Node{
		core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 12, Padding: core.All(32), MaxWidth: core.Pt(320), AlignSelf: core.AlignCenter}, Children: children},
	}}
}
