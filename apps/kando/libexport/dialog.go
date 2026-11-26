package libexport

import (
	"strings"

	"kando/apps/kando/live"
	"kando/apps/kando/locales"
	"kando/apps/kando/subexport"
	"kando/packages/core"

	"github.com/nsgo-dev/nsgo/base"
	ns "github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// App is what the dialog reads and changes, inside its bindings through live.Func and through live.Do for its actions (package live).
type App interface {
	live.State
	// LibexportOpen reports whether the dialog shows.
	LibexportOpen() bool
	// LibexportChoice is how the films are exported, the 导出字幕 dialog's choice (subexport.Choice) in one of Kando's own formats; SetLibexportChoice changes it.
	LibexportChoice() subexport.Choice
	SetLibexportChoice(c subexport.Choice)
	// LibexportFilms is how many films of the library have subtitles to export.
	LibexportFilms() int
	// LibexportBeside reports whether the export goes beside each video rather than into a folder (LibexportFolder); SetLibexportBeside changes it. LibexportBesideOK reports whether any film can be exported beside its video, as none of a library bundle's own can, its videos being Kando's.
	LibexportBeside() bool
	SetLibexportBeside(on bool)
	LibexportBesideOK() bool
	// LibexportFolder is the folder an export into a folder goes in, "" while none is picked; LibexportPickFolder asks for one.
	LibexportFolder() string
	LibexportPickFolder()
	// LibexportProgress is how many films the export running did of how many, both 0 while none runs.
	LibexportProgress() Progress
	// LibexportDone reports whether the export ran to its end; LibexportResult is what it did then.
	LibexportDone() bool
	LibexportResult() Result
	// LibexportOld is how many films still have what a Kando before wrote beside their videos (OldBeside), which LibexportTrashOld moves to the Trash.
	LibexportOld() int
	LibexportTrashOld()
	// LibexportStart starts the export; CloseLibexport closes the dialog.
	LibexportStart()
	CloseLibexport()
}

// Progress is how many films an export did, of how many it exports.
type Progress struct{ Done, Total int }

// Result is what an export did with the films of a library: the files it wrote, those of its own it left as they were, already holding the export (Same), the files of the user's it kept under the names it would take, the films it could not export, and those it did not, as their videos are not on disk beside which to write.
type Result struct {
	Written, Same, Kept, Failed, Skipped int
}

// Dialog is the 导出资料库的字幕 dialog while the app's LibexportOpen holds: how the subtitles are exported, as the 导出字幕 dialog chooses it, and where, beside each video or into a folder; then, once it ran, what it did, and what a Kando before left beside the videos, which it offers to move to the Trash. 取消, Escape and closing it close it.
type Dialog struct{ A App }

// Render implements ns.Node.
func (d Dialog) Render(cx *ns.Scope) ns.Node {
	a := d.A
	return ns.Show{When: live.Func(a, a.LibexportOpen), Then: func(cx *ns.Scope) ns.Node {
		colors := ui.ColorsOf(cx)
		choice := live.Func(a, a.LibexportChoice)
		set := func(change func(c *subexport.Choice)) {
			live.Do(a, func() {
				c := a.LibexportChoice()
				change(&c)
				a.SetLibexportChoice(c)
			})()
		}
		done := live.Func(a, a.LibexportDone)
		running := func() bool { return live.Func(a, a.LibexportProgress)().Total > 0 && !done() }
		tabs := func(id string, get func(subexport.Choice) string, put func(*subexport.Choice, string), options ...ui.Option[string]) ns.Node {
			return ui.Tabs[string]{TestID: id, Size: ui.SizeSmall, Options: options,
				Value:    func() string { return get(choice()) },
				OnChange: func(v string) { set(func(c *subexport.Choice) { put(c, v) }) }}
		}
		format := tabs("libexport-format", func(c subexport.Choice) string { return c.Format }, func(c *subexport.Choice, v string) { c.Format = v },
			ui.Option[string]{Value: core.FormatSRT, Label: "SRT", TestID: "libexport-format-srt"},
			ui.Option[string]{Value: core.FormatVTT, Label: "WebVTT", TestID: "libexport-format-vtt"},
			ui.Option[string]{Value: core.FormatASS, Label: "ASS", TestID: "libexport-format-ass"})
		content := tabs("libexport-content", func(c subexport.Choice) string { return c.Content }, func(c *subexport.Choice, v string) { c.Content = v },
			ui.Option[string]{Value: core.SubOriginal, Label: locales.SubexportOriginal(cx), TestID: "libexport-content-original"},
			ui.Option[string]{Value: core.SubTranslation, Label: locales.SubexportTranslation(cx), TestID: "libexport-content-translation"},
			ui.Option[string]{Value: core.SubBilingual, Label: locales.SubexportBilingual(cx), TestID: "libexport-content-bilingual"})
		order := tabs("libexport-order", func(c subexport.Choice) string { return c.Order }, func(c *subexport.Choice, v string) { c.Order = v },
			ui.Option[string]{Value: core.SubTranslationFirst, Label: locales.SubexportTranslationFirst(cx)},
			ui.Option[string]{Value: core.SubOriginalFirst, Label: locales.SubexportOriginalFirst(cx)})
		missing := ui.Select[string]{TestID: "libexport-missing", Label: locales.SubexportMissing(cx),
			Options: []ui.Option[string]{
				{Value: core.MissingSkip, Label: locales.SubexportMissingSkip(cx)},
				{Value: core.MissingPlaceholder, Label: locales.SubexportMissingPlaceholder(cx)},
				{Value: core.MissingOriginal, Label: locales.SubexportMissingOriginal(cx)},
			},
			Value:    func() string { return choice().Missing },
			OnChange: func(v string) { set(func(c *subexport.Choice) { c.Missing = v }) },
			Disabled: func() bool { return choice().Content == core.SubOriginal }}
		delay := ui.Switch{TestID: "libexport-delay", Label: locales.LibexportDelay(cx),
			Checked:  func() bool { return choice().Delay },
			OnChange: func(on bool) { set(func(c *subexport.Choice) { c.Delay = on }) }}
		beside := live.Func(a, a.LibexportBeside)
		dest := ui.Tabs[string]{TestID: "libexport-dest", Size: ui.SizeSmall,
			Options: []ui.Option[string]{
				{Value: "beside", Label: locales.LibexportBeside(cx), TestID: "libexport-dest-beside"},
				{Value: "folder", Label: locales.LibexportFolder(cx), TestID: "libexport-dest-folder"},
			},
			Value: func() string {
				if beside() {
					return "beside"
				}
				return "folder"
			},
			OnChange: func(v string) { live.Do(a, func() { a.SetLibexportBeside(v == "beside") })() }}
		folder := ns.Show{When: func() bool { return !beside() }, Then: func(cx *ns.Scope) ns.Node {
			return ns.View{Style: ns.Style{Direction: ns.Row, AlignItems: ns.AlignCenter, Gap: 8}, Children: []ns.Node{
				ns.Text{TestID: "libexport-folder-path", Selectable: ns.SelectableOn,
					ContentFunc: func() string {
						if f := live.Func(a, a.LibexportFolder)(); f != "" {
							return subexport.Display(f)
						}
						return locales.LibexportNoFolder(cx)
					},
					Style: ui.TypeCaption.Merge(ns.Style{FontFamily: ui.Mono, FlexShrink: 1}), StyleFunc: ui.Colored(colors.Muted)},
				ui.Button{Text: locales.LibexportPick(cx), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, TestID: "libexport-pick", OnPress: live.Do(a, a.LibexportPickFolder)},
			}}
		}}
		rows := ns.Show{When: func() bool { return !done() }, Then: func(cx *ns.Scope) ns.Node {
			return ns.View{Style: ns.Style{Gap: 10, AlignItems: ns.AlignStretch}, Children: []ns.Node{
				row(func() string { return locales.SubexportFormat(cx) }, format),
				row(func() string { return locales.SubexportContent(cx) }, content),
				ns.Show{When: func() bool { return choice().Content == core.SubBilingual }, Then: func(cx *ns.Scope) ns.Node {
					return row(func() string { return locales.SubexportOrder(cx) }, order)
				}},
				row(func() string { return locales.SubexportMissing(cx) }, missing),
				row(func() string { return locales.LibexportDelay(cx) }, delay),
				ns.Show{When: live.Func(a, a.LibexportBesideOK), Then: func(cx *ns.Scope) ns.Node {
					return row(func() string { return locales.LibexportDest(cx) }, dest)
				}},
				folder,
			}}
		}}
		status := ns.Text{TestID: "libexport-status", Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted), ContentFunc: func() string {
			if done() {
				return ResultText(cx, live.Func(a, a.LibexportResult)())
			}
			if p := live.Func(a, a.LibexportProgress)(); p.Total > 0 {
				return locales.LibexportRunning(cx, p.Done, p.Total)
			}
			return locales.LibexportFilms(cx, live.Func(a, a.LibexportFilms)())
		}}
		old := ns.Show{When: func() bool { return done() && live.Func(a, a.LibexportOld)() > 0 }, Then: func(cx *ns.Scope) ns.Node {
			return ns.View{Style: ns.Style{Gap: 8, AlignItems: ns.AlignStart}, Children: []ns.Node{
				ns.Text{TestID: "libexport-old", Style: ui.TypeBody, ContentFunc: func() string { return locales.LibexportOld(cx, live.Func(a, a.LibexportOld)()) }},
				ns.Text{ContentFunc: func() string { return locales.LibexportOldNote(cx) }, Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
				ui.Button{Text: locales.LibexportTrashOld(cx), Variant: ui.ButtonSecondary, TestID: "libexport-trash-old", OnPress: live.Do(a, a.LibexportTrashOld)},
			}}
		}}
		cannot := func() bool {
			return running() || live.Func(a, a.LibexportFilms)() == 0 || (!beside() && live.Func(a, a.LibexportFolder)() == "")
		}
		return ui.Dialog{
			TestID:                 "libexport-dialog",
			NoCloseOnBackdropPress: true,
			Open:                   func() bool { return true },
			TitleFunc:              func() string { return locales.LibexportTitle(cx) },
			DescriptionFunc:        func() string { return locales.LibexportDescription(cx) },
			Children:               []ns.Node{ns.View{Style: ns.Style{Gap: 12, AlignItems: ns.AlignStretch}, Children: []ns.Node{rows, status, old}}},
			OnOpenChange: func(open bool, _ base.Reason) {
				if !open {
					live.Do(a, a.CloseLibexport)()
				}
			},
			Footer: []ns.Node{
				ns.Show{When: func() bool { return !done() }, Then: func(cx *ns.Scope) ns.Node {
					return ns.View{Style: ns.Style{Direction: ns.Row, Gap: 8}, Children: []ns.Node{
						ui.Button{Text: locales.LibexportCancel(cx), Variant: ui.ButtonSecondary, TestID: "libexport-cancel", OnPress: live.Do(a, a.CloseLibexport)},
						ui.Button{Text: locales.LibexportStart(cx), TestID: "libexport-start", Disabled: cannot, OnPress: live.Do(a, a.LibexportStart)},
					}}
				}, Else: func(cx *ns.Scope) ns.Node {
					return ui.Button{Text: locales.LibexportDone(cx), TestID: "libexport-done", OnPress: live.Do(a, a.CloseLibexport)}
				}},
			},
		}
	}}
}

// row is a row of the dialog: what it chooses, then its control at the end.
func row(label func() string, control ns.Node) ns.Node {
	return ns.View{Style: ns.Style{Direction: ns.Row, AlignItems: ns.AlignCenter, JustifyContent: ns.JustifySpaceBetween, Gap: 16, MinHeight: ns.Pt(28)}, Children: []ns.Node{
		ns.Text{ContentFunc: label, Style: ui.TypeBody.Merge(ns.Style{FlexShrink: 1})},
		control,
	}}
}

// ResultText is what an export did, as result says, in the language of cx.
func ResultText(cx *ns.Scope, result Result) string {
	parts := []string{locales.LibexportWritten(cx, result.Written)}
	if result.Same > 0 {
		parts = append(parts, locales.LibexportSame(cx, result.Same))
	}
	if result.Kept > 0 {
		parts = append(parts, locales.LibexportKept(cx, result.Kept))
	}
	if result.Skipped > 0 {
		parts = append(parts, locales.LibexportSkipped(cx, result.Skipped))
	}
	if result.Failed > 0 {
		parts = append(parts, locales.LibexportFailed(cx, result.Failed))
	}
	return strings.Join(parts, " · ")
}
