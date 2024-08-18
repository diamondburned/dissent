//go:build !nospellcheck

package composer

import (
	"github.com/diamondburned/gotkit/app/prefs"
	"libdb.so/gotk4-spelling/pkg/spelling"
)

var spellCheck = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Spell Check",
	Section:     "Composer",
	Description: "Enable spell checking in the composer.",
})

func hookSpellChecker(i *Input) {
	spellChecker := spelling.CheckerGetDefault()

	spellingAdapter := spelling.NewTextBufferAdapter(i.Buffer, spellChecker)
	spellingAdapter.SetEnabled(spellCheck.Value())
	spellingAdapter.NotifyProperty("enabled", func() {
		nowEnabled := spellingAdapter.Enabled()
		if spellCheck.Value() != nowEnabled {
			spellCheck.Publish(nowEnabled)
		}
	})

	i.TextView.SetExtraMenu(spellingAdapter.MenuModel())
	i.TextView.InsertActionGroup("spelling", spellingAdapter)
}
