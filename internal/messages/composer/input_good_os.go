//go:build !nogtksource && !windows

package composer

import (
	"log/slog"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/prefs"
	"libdb.so/gotk4-sourceview/pkg/gtksource/v5"
	"libdb.so/gotk4-spelling/pkg/spelling"
)

var spellCheck = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Spell Check",
	Section:     "Composer",
	Description: "Enable spell checking in the composer.",
})

func initializeInput() initializedInput {
	languageManager := gtksource.LanguageManagerGetDefault()
	spellChecker := spelling.CheckerGetDefault()

	buffer := gtksource.NewBuffer(nil)

	// Set up the buffer's highlighting language.
	markdownLanguage := languageManager.Language("markdown")
	if markdownLanguage != nil {
		buffer.SetLanguage(markdownLanguage)
	} else {
		slog.Warn(
			"language 'markdown' not found in gtksource, not setting one",
			"languages", languageManager.LanguageIDs())
	}

	// Set up the buffer's spell checking.
	spellingAdapter := spelling.NewTextBufferAdapter(buffer, spellChecker)
	spellingAdapter.SetEnabled(spellCheck.Value())
	spellingAdapter.NotifyProperty("enabled", func() {
		nowEnabled := spellingAdapter.Enabled()
		if spellCheck.Value() != nowEnabled {
			spellCheck.Publish(nowEnabled)
		}
	})

	view := gtk.NewTextViewWithBuffer(&buffer.TextBuffer)
	view.AddCSSClass("composer-input-with-gtksource")
	view.SetExtraMenu(spellingAdapter.MenuModel())
	view.InsertActionGroup("spelling", spellingAdapter)

	return initializedInput{
		View:   view,
		Buffer: &buffer.TextBuffer,
	}
}
