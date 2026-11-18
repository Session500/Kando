package films

import (
	"kando/apps/kando/film/subtitle"
	"kando/apps/kando/live"
	"kando/apps/kando/locales"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// retranslateLook is what the confirmation of 重新翻译为 says: the film's name, the language it would be translated into and the one its translation is in now; or, for several films, how many they are (count) and how many of their translations it replaces (replaced).
type retranslateLook struct {
	name, language, current string
	count, replaced         int
}

// RetranslateDialog confirms translating a done film, or several selected, into another language than its subtitles are in, which replaces their translation and uses the AI service. 取消, Escape and closing the dialog keep the film as it is. The app draws it over the window, so the film page's 重新翻译为 asks too.
type RetranslateDialog struct{ App App }

// Render implements core.Node.
func (r RetranslateDialog) Render(cx *core.Scope) core.Node {
	a := r.App
	open := live.Func(a, func() bool {
		open, _, _ := a.Retranslating()
		return *open
	})
	look := live.Memo(cx, a, func() retranslateLook {
		_, items, language := a.Retranslating()
		if len(items) == 1 {
			return retranslateLook{name: filmName(items[0]), language: language, current: items[0].Language, count: 1}
		}
		return retranslateLook{language: language, count: len(items), replaced: len(Replacing(items, language))}
	})
	cancel := live.Do(a, a.CloseRetranslate)
	translate := live.Do(a, a.Retranslate)
	return core.Show{When: open, Then: func(cx *core.Scope) core.Node {
		return core.Dynamic(func(cx *core.Scope) core.Node {
			// TODO(nsgo#145): give ui.Dialog and ui.AlertDialog TitleFunc and DescriptionFunc twins so the words follow the film and the language without building the dialog again, then drop this Dynamic https://github.com/nsgo-dev/nsgo/issues/145
			l := look()
			title, message := locales.FilmsRetranslateConfirm(cx, l.name, subtitle.TargetLanguageName(cx, l.language)), locales.FilmsRetranslateConfirmMessage(cx, subtitle.TargetLanguageName(cx, l.current))
			if l.count > 1 {
				title, message = locales.FilmsRetranslateConfirmMany(cx, l.count, subtitle.TargetLanguageName(cx, l.language)), locales.FilmsRetranslateManyMessage(cx, l.replaced)
			}
			return ui.Dialog{
				TestID:                 "retranslate-confirm",
				Role:                   core.RoleAlertDialog,
				NoCloseOnBackdropPress: true,
				Open:                   func() bool { return true },
				Title:                  title,
				Description:            message,
				OnOpenChange: func(open bool, _ base.Reason) {
					if !open {
						cancel()
					}
				},
				Footer: []core.Node{
					ui.Button{Text: locales.FilmsCancel(cx), Variant: ui.ButtonSecondary, TestID: "retranslate-cancel", OnPress: cancel},
					ui.Button{Text: locales.FilmsTranslate(cx), TestID: "retranslate-confirm-run", OnPress: translate},
				},
			}
		})
	}}
}

// Replacing is the films of items whose translation translating them into language replaces with one in another language, which is what the confirmation of 重新翻译为 asks about: those translated into another language. A film that was not translated at all has no translation to replace, whatever language its job was asked for.
func Replacing(items []Film, language string) []Film {
	var out []Film
	for _, item := range items {
		if current := translatedInto(item); current != "" && current != language {
			out = append(out, item)
		}
	}
	return out
}
