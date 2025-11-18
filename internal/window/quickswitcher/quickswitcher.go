package quickswitcher

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"libdb.so/dissent/internal/gresources"
	"libdb.so/dissent/internal/gtkcord"
)

// QuickSwitcher is a search box capable of looking up guilds and channels for
// quickly jumping to them. It replicates the Ctrl+K dialog of the desktop
// client.
type QuickSwitcher struct {
	*adw.Dialog
	ctx   gtkutil.Cancellable
	text  string
	index index

	search     *gtk.SearchEntry
	chosenFunc func()
	qsStack    *adw.ViewStack

	entryScroll     *gtk.ScrolledWindow
	guildsEntryList *gtk.Box
	entryList       *gtk.ListBox
	entries         []channelEntry
}

type channelEntry struct {
	*gtk.ListBoxRow
	indexItem channelIndexItem
}

// ShowDialog shows a new Quick Switcher dialog.
func ShowDialog(ctx context.Context) {
	d := NewQuickSwitcher(ctx)
	d.Present(app.GTKWindowFromContext(ctx))
}

// NewQuickSwitcher creates a new Quick Switcher instance.
func NewQuickSwitcher(ctx context.Context) *QuickSwitcher {
	uiFile := gresources.New("quickswitcher.ui")
	qs := QuickSwitcher{Dialog: uiFile.GetRoot().(*adw.Dialog)}
	qs.search = uiFile.GetComponent("Search").(*gtk.SearchEntry)
	qs.qsStack = uiFile.GetComponent("QSStack").(*adw.ViewStack)
	qs.index.update(ctx)

	qs.SetTitle(app.FromContext(ctx).SuffixedTitle("Quick Switcher"))

	qs.search.ConnectActivate(func() { qs.selectEntry() })
	qs.search.ConnectNextMatch(func() { qs.moveDown() })
	qs.search.ConnectPreviousMatch(func() { qs.moveUp() })
	qs.search.ConnectSearchChanged(func() {
		qs.text = qs.search.Text()
		qs.do()
	})

	qs.ConnectShow(func() {
		qs.Clear()
		qs.search.GrabFocus()
	})

	qs.ConnectChosen(func() {
		qs.Close()
	})

	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(val, _ uint, state gdk.ModifierType) bool {
		switch val {
		case gdk.KEY_Up:
			return qs.moveUp()
		case gdk.KEY_Down, gdk.KEY_Tab:
			return qs.moveDown()
		case gdk.KEY_Escape:
			qs.Close()
			return true
		default:
			return false
		}
	})
	qs.search.AddController(keyCtrl)

	qs.guildsEntryList = uiFile.GetComponent("GuildListBox").(*gtk.Box)
	qs.entryList = uiFile.GetComponent("ChannelsListBox").(*gtk.ListBox)

	qs.entryList.ConnectRowActivated(func(row *gtk.ListBoxRow) {
		qs.chooseChannel(row.Index())
	})

	qs.ctx = gtkutil.WithVisibility(ctx, qs.search)
	qs.search.SetKeyCaptureWidget(qs)

	return &qs
}

func (qs *QuickSwitcher) Clear() {
	qs.search.SetText("")
	qs.text = ""
	qs.do()
}

func (qs *QuickSwitcher) do() {
	if qs.text == "" {
		qs.qsStack.SetVisibleChildName("emptyPage")
	} else {
		qs.qsStack.SetVisibleChildName("searchResults")
	}

	qs.entryList.RemoveAll()

	for child := qs.guildsEntryList.FirstChild(); child != nil; child = qs.guildsEntryList.FirstChild() {
		qs.guildsEntryList.Remove(child)
	}

	if qs.text == "" {
		return
	}

	channelsFound, guildsFound := qs.index.search(qs.text)
	for _, match := range channelsFound {
		channelItem := match.Row(qs.ctx.Take())
		e := channelEntry{
			ListBoxRow: channelItem,
			indexItem:  match,
		}

		qs.entryList.Append(channelItem)

		for len(qs.entries) <= channelItem.Index() {
			qs.entries = append(qs.entries, channelEntry{})
		}

		qs.entries[channelItem.Index()] = e
	}

	for _, match := range guildsFound {
		guildIcon := match.QSItem(qs.ctx.Take())
		guildIcon.ConnectClicked(func() {
			qs.chooseGuild(match)
		})
		qs.guildsEntryList.Append(guildIcon)
	}

	if len(qs.entries) > 0 {
		qs.entryList.SelectRow(qs.entryList.RowAtIndex(0))
	}
}

func (qs *QuickSwitcher) chooseChannel(n int) {
	entry := qs.entries[n]
	parent := gtk.BaseWidget(qs.Parent())

	var ok bool
	ok = parent.ActivateAction("app.open-channel", gtkcord.NewChannelIDVariant(entry.indexItem.ChannelID()))

	if !ok {
		slog.Error(
			"failed to activate opening action from quick switcher",
			"parent", fmt.Sprintf("%T", qs.Parent()),
			"item", fmt.Sprintf("%T", entry.indexItem))
	}

	if qs.chosenFunc != nil {
		qs.chosenFunc()
	}
}
func (qs *QuickSwitcher) chooseGuild(match guildIndexItem) {
	parent := gtk.BaseWidget(qs.Parent())
	ok := parent.ActivateAction("app.open-guild", gtkcord.NewGuildIDVariant(match.GuildID()))

	if !ok {
		slog.Error(
			"failed to activate opening action from quick switcher",
			"parent", fmt.Sprintf("%T", qs.Parent()),
			"item", fmt.Sprintf("%T", match.String()))
	}

	if qs.chosenFunc != nil {
		qs.chosenFunc()
	}
}

// ConnectChosen connects a function to be called when an entry is chosen.
func (qs *QuickSwitcher) ConnectChosen(f func()) {
	if qs.chosenFunc != nil {
		add := f
		old := qs.chosenFunc
		f = func() {
			old()
			add()
		}
	}
	qs.chosenFunc = f
}

func (qs *QuickSwitcher) selectEntry() bool {
	if len(qs.entries) == 0 {
		return false
	}

	row := qs.entryList.SelectedRow()
	if row == nil {
		return false
	}

	qs.chooseChannel(row.Index())
	return true
}

func (qs *QuickSwitcher) moveUp() bool   { return qs.move(false) }
func (qs *QuickSwitcher) moveDown() bool { return qs.move(true) }

func (qs *QuickSwitcher) move(down bool) bool {
	if len(qs.entries) == 0 {
		return false
	}

	row := qs.entryList.SelectedRow()
	if row == nil {
		qs.entryList.SelectRow(qs.entryList.RowAtIndex(0))
		return true
	}

	ix := row.Index()
	var newFocusedRow *gtk.ListBoxRow
	if down {
		ix += 1
		newFocusedRow = qs.entryList.RowAtIndex(ix)
	} else {
		ix -= 1
		newFocusedRow = qs.entryList.RowAtIndex(ix)
	}

	if newFocusedRow == nil {
		return false
	}

	qs.entryList.SelectRow(newFocusedRow)

	// Steal focus. This is a hack to scroll to the selected item without having
	// to manually calculate the coordinates.
	var target gtk.Widgetter = qs.search
	if focused := app.WindowFromContext(qs.ctx.Take()).Focus(); focused != nil {
		target = focused
	}
	targetBase := gtk.BaseWidget(target)
	qs.entries[ix].ListBoxRow.GrabFocus()
	targetBase.GrabFocus()

	return true
}
