package quickswitcher

import (
	"context"
	"log"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

// QuickSwitcher is a search box capable of looking up guilds and channels for
// quickly jumping to them. It replicates the Ctrl+K dialog of the desktop
// client.
type QuickSwitcher struct {
	*gtk.Box
	ctx   gtkutil.Cancellable
	text  string
	index index

	search     *gtk.SearchEntry
	chosenFunc func()

	entryScroll *gtk.ScrolledWindow
	entryList   *gtk.ListBox
	entries     []entry
}

type entry struct {
	*gtk.ListBoxRow
	indexItem indexItem
}

var qsCSS = cssutil.Applier("quickswitcher", `
	.quickswitcher-search {
		font-size: 1.35em;
	}
	.quickswitcher-search image {
		min-width:  32px;
		min-height: 32px;
	}
	.quickswitcher-searchbar > revealer > box {
		padding: 12px;
	}
	.quickswitcher-list {
		font-size: 1.15em;
	}
`)

// NewQuickSwitcher creates a new Quick Switcher instance.
func NewQuickSwitcher(ctx context.Context) *QuickSwitcher {
	var qs QuickSwitcher
	qs.index.update(ctx)

	qs.search = gtk.NewSearchEntry()
	qs.search.AddCSSClass("quickswitcher-search")
	qs.search.SetHExpand(true)
	qs.search.SetObjectProperty("placeholder-text", "Search")
	qs.search.ConnectActivate(func() { qs.selectEntry() })
	qs.search.ConnectNextMatch(func() { qs.moveDown() })
	qs.search.ConnectPreviousMatch(func() { qs.moveUp() })
	qs.search.ConnectSearchChanged(func() {
		qs.text = qs.search.Text()
		qs.do()
	})

	if qs.search.ObjectProperty("search-delay") != nil {
		// Only GTK v4.8 and onwards.
		qs.search.SetObjectProperty("search-delay", 100)
	}

	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(val, _ uint, state gdk.ModifierType) bool {
		switch val {
		case gdk.KEY_Up:
			return qs.moveUp()
		case gdk.KEY_Down, gdk.KEY_Tab:
			return qs.moveDown()
		default:
			return false
		}
	})
	qs.search.AddController(keyCtrl)

	qs.entryList = gtk.NewListBox()
	qs.entryList.AddCSSClass("quickswitcher-list")
	qs.entryList.SetVExpand(true)
	qs.entryList.SetSelectionMode(gtk.SelectionSingle)
	qs.entryList.SetActivateOnSingleClick(true)
	qs.entryList.SetPlaceholder(qsListPlaceholder())
	qs.entryList.ConnectRowActivated(func(row *gtk.ListBoxRow) {
		qs.choose(row.Index())
	})

	entryViewport := gtk.NewViewport(nil, nil)
	entryViewport.SetScrollToFocus(true)
	entryViewport.SetChild(qs.entryList)

	qs.entryScroll = gtk.NewScrolledWindow()
	qs.entryScroll.AddCSSClass("quickswitcher-scroll")
	qs.entryScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	qs.entryScroll.SetChild(entryViewport)
	qs.entryScroll.SetVExpand(true)

	qs.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	qs.Box.SetVExpand(true)
	qs.Box.Append(qs.search)
	qs.Box.Append(qs.entryScroll)

	qs.ctx = gtkutil.WithVisibility(ctx, qs.search)
	qs.search.SetKeyCaptureWidget(qs)

	qsCSS(qs.Box)
	return &qs
}

func qsListLoading() gtk.Widgetter {
	loading := gtk.NewSpinner()
	loading.SetSizeRequest(24, 24)
	loading.SetVAlign(gtk.AlignCenter)
	loading.SetHAlign(gtk.AlignCenter)
	loading.Start()
	return loading
}

func qsListPlaceholder() gtk.Widgetter {
	l := gtk.NewLabel("Where would you like to go?")
	l.SetAttributes(textutil.Attrs(
		pango.NewAttrScale(1.15),
	))
	l.SetVAlign(gtk.AlignCenter)
	l.SetHAlign(gtk.AlignCenter)
	return l
}

func (qs *QuickSwitcher) do() {
	if qs.text == "" {
		return
	}

	for i, e := range qs.entries {
		qs.entryList.Remove(e)
		qs.entries[i] = entry{}
	}
	qs.entries = qs.entries[:0]

	for _, match := range qs.index.search(qs.text) {
		e := entry{
			ListBoxRow: match.Row(qs.ctx.Take()),
			indexItem:  match,
		}

		qs.entries = append(qs.entries, e)
		qs.entryList.Append(e)
	}

	if len(qs.entries) > 0 {
		qs.entryList.SelectRow(qs.entries[0].ListBoxRow)
	}
}

func (qs *QuickSwitcher) choose(n int) {
	entry := qs.entries[n]
	parent := gtk.BaseWidget(qs.Parent())

	var ok bool
	switch item := entry.indexItem.(type) {
	case channelItem:
		ok = parent.ActivateAction("app.open-channel", gtkcord.NewChannelIDVariant(item.ID))
	case guildItem:
		ok = parent.ActivateAction("app.open-guild", gtkcord.NewGuildIDVariant(item.ID))
	}
	if !ok {
		log.Println("quickswitcher: failed to activate action")
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

	qs.choose(row.Index())
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
		qs.entryList.SelectRow(qs.entries[0].ListBoxRow)
		return true
	}

	ix := row.Index()
	if down {
		ix++
		if ix == len(qs.entries) {
			ix = 0
		}
	} else {
		ix--
		if ix == -1 {
			ix = len(qs.entries) - 1
		}
	}

	qs.entryList.SelectRow(qs.entries[ix].ListBoxRow)

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
