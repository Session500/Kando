// Package welcome is the app's first screen, before anything is paired or downloaded: what to pair with, and the button that opens pairing.
package welcome

import (
	"kando/apps/mobile/locales"
	"kando/apps/mobile/look"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

// Page is the first screen; pair opens the pairing page.
func Page(cx *core.Scope, pair func()) core.Node {
	colors, tints := ui.ColorsOf(cx), ui.TintsOf(cx)
	// flat is a phone on its side, where the screen is too short for the art and the room around it.
	flat := func() bool { return cx.Env().Window().Height < 500 }
	space := func(upright float32) core.Node {
		return core.View{StyleFunc: func() core.Style {
			if flat() {
				return core.Style{FlexGrow: 1}
			}
			return core.Style{FlexGrow: upright}
		}}
	}
	return core.View{TestID: "welcome", Style: core.Style{FlexGrow: 1, PaddingEnv: core.EnvAll(core.EnvSafeArea), Padding: core.Edges{Right: 24, Bottom: 16, Left: 24}}, Children: []core.Node{
		space(1),
		core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 20, MaxWidth: core.Pt(420), AlignSelf: core.AlignCenter}, Children: []core.Node{
			core.View{Style: core.Style{CornerRadius: 22, BorderCurve: core.BorderCurveContinuous, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				StyleFunc: func() core.Style {
					side := core.Pt(88)
					if flat() {
						side = core.Pt(64)
					}
					return core.Style{Width: side, Height: side, Background: tints.Accent()}
				},
				Children: []core.Node{ui.Icon(lucide.MonitorSmartphone, 44, colors.Accent)}},
			core.Text{ContentFunc: func() string { return locales.WelcomeTitle(cx) }, Role: core.RoleHeader,
				Style: look.Display.Merge(core.Style{TextAlign: core.TextAlignCenter}), StyleFunc: ui.Colored(colors.Text)},
		}},
		space(1.4),
		core.View{Style: core.Style{MaxWidth: core.Pt(420), Width: core.Pct(100), AlignSelf: core.AlignCenter}, Children: []core.Node{
			look.Button{Kind: look.Primary, Text: func() string { return locales.WelcomePair(cx) }, TestID: "welcome-pair", OnPress: pair},
		}},
	}}
}
