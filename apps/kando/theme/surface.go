package theme

import "github.com/nsgo-dev/nsgo/core"

// Glass is a dark glass of the design: Fill over a blur of what is behind it by Blur, and a white hairline at Ring just inside its edge. The player draws its bars, panels, menus and cards on one, and the palette and the cheat sheet theirs over the whole window. Its corners are the surface's own, so Style takes them. The same on both appearances: the player is a dark subtree (Dark), and the sheets over the window are dark glass in either.
type Glass struct {
	Fill core.Color
	Blur core.Points
	Ring float32
}

// The glasses, one for each kind of surface, so the player's surfaces of a kind look alike and change together.
var (
	// GlassBar is a bar or a panel over the picture, which it lets show through: the transport, the side panel, the volume and speed readout.
	GlassBar = Glass{Fill: core.RGBA(0x1C1C20, 0.6), Blur: 28, Ring: 0.2}
	// GlassCard is a card or a menu over the picture that holds words to read: the lookup, the recap, a correction, a stream's or an upload's card, the subtitle mode menu.
	GlassCard = Glass{Fill: core.RGBA(0x1C1C20, 0.84), Blur: 28, Ring: 0.2}
	// GlassSheet is a sheet over the whole window, Scrim under it: the command palette, the shortcut cheat sheet.
	GlassSheet = Glass{Fill: core.RGBA(0x1C1C20, 0.88), Blur: 36, Ring: 0.2}
	// GlassClear is a light glass button on the picture: the title bar's capsules, the big play button.
	GlassClear = Glass{Fill: core.RGBA(0xFFFFFF, 0.15), Blur: 20, Ring: 0.3}
	// GlassTip is a tooltip on the player, nearly opaque and with no blur, as it is small and shown for a moment.
	GlassTip = Glass{Fill: core.RGBA(0x1C1C20, 0.92), Ring: 0.2}
)

// Scrim dims the window under a sheet of GlassSheet.
var Scrim = core.RGBA(0x000000, 0.28)

// Style is the glass as the style of a surface whose corners are radius: its fill, its blur and its hairline.
func (g Glass) Style(radius core.Points) core.Style {
	s := core.Style{CornerRadius: radius, Background: g.Fill, Shadows: []core.BoxShadow{g.Rim()}}
	if g.Blur > 0 {
		s.BackdropFilters = []core.Filter{core.Blur(g.Blur)}
	}
	return s
}

// Floating is Style for a surface that floats over the picture, with ShadowHUD under it.
func (g Glass) Floating(radius core.Points) core.Style {
	s := g.Style(radius)
	s.Shadows = append(s.Shadows, ShadowHUD)
	return s
}

// Rim is the glass's hairline just inside its edge, for a surface that draws the blur and the fill on views of its own, as a sheet's popup and its face.
func (g Glass) Rim() core.BoxShadow {
	return core.BoxShadow{Spread: 0.5, Color: core.RGBA(0xFFFFFF, g.Ring), Inset: true}
}

// The shadows of the design, one for each height a surface stands at over what is under it, so surfaces at the same height cast the same shadow. They are black and the same on both appearances, where a lighter face says the height on the dark page (Colors.Card) and the shadow adds little. An accent's glow under a pressable thing (the drop zone, the start button) is not a height and keeps its own color.
var (
	// ShadowRest is a card resting on the page, just lifted off it: the onboarding's choices.
	ShadowRest = shadow(1, 3, 0.05)
	// ShadowRaised is a pill or a field standing off the page or a bar: the library's top bar pills, the browser's address field.
	ShadowRaised = shadow(2, 8, 0.06)
	// ShadowCard is a card over the page: a poster, the sidebar's card.
	ShadowCard = shadow(4, 12, 0.1)
	// ShadowLifted is a card lifted toward the pointer or standing out from the page: a poster under the pointer, the hero.
	ShadowLifted = shadow(12, 32, 0.18)
	// ShadowDialog is a sheet or a dialog over the window: the onboarding sheet, the command palette, the cheat sheet.
	ShadowDialog = shadow(28, 76, 0.42)
	// ShadowHUD is a surface floating over the picture: the player's transport, panels, menus, cards and tooltips.
	ShadowHUD = shadow(12, 36, 0.35)
)

func shadow(y, blur core.Points, alpha float32) core.BoxShadow {
	return core.BoxShadow{Offset: core.Point{Y: y}, Blur: blur, Color: core.RGBA(0x000000, alpha)}
}
