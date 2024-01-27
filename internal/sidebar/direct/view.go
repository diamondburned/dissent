package direct

import (
	"context"
	"log"
	"strings"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/ningen/v3/states/read"
)

// ChannelView displays a list of direct messaging channels.
type ChannelView struct {
	*adaptive.LoadablePage
	box *adw.ToolbarView

	scroll *gtk.ScrolledWindow
	list   *gtk.ListBox

	searchBar    *gtk.SearchBar
	searchEntry  *gtk.SearchEntry
	searchString string

	ctx      context.Context
	channels map[discord.ChannelID]*Channel
	selectID discord.ChannelID // delegate to be selected later
}

// Opener is the parent controller that ChannelView controls.
type Opener interface {
	OpenChannel(discord.ChannelID)
}

var _ = cssutil.WriteCSS(`
	.direct-searchbar > revealer > box {
		border-bottom: 0;
		background: none;
		box-shadow: none;
	}
	.direct-searchbar > revealer > box > entry {
		min-height: 28px;
	}
`)

var lastOpenKey = app.NewSingleStateKey[discord.ChannelID]("direct-last-open")

// NewChannelView creates a new view.
func NewChannelView(ctx context.Context, ctrl Opener) *ChannelView {
	v := ChannelView{
		ctx:      ctx,
		channels: make(map[discord.ChannelID]*Channel, 50),
	}

	v.list = gtk.NewListBox()
	v.list.SetCSSClasses([]string{"direct-list", "navigation-sidebar"})
	v.list.SetHExpand(true)
	v.list.SetSortFunc(v.sort)
	v.list.SetFilterFunc(v.filter)
	v.list.SetSelectionMode(gtk.SelectionBrowse)
	v.list.SetActivateOnSingleClick(true)

	var currentCh discord.ChannelID
	lastOpen := lastOpenKey.Acquire(ctx)

	v.list.ConnectRowSelected(func(r *gtk.ListBoxRow) {
		if r == nil {
			// This should not happen.
			return
		}

		// Invalidate our selection state.
		v.selectID = 0

		ch := v.rowChannel(r)
		if ch == nil || ch.id == currentCh {
			return
		}

		currentCh = ch.id
		lastOpen.Set(ch.id)
		ctrl.OpenChannel(ch.id)
	})

	v.scroll = gtk.NewScrolledWindow()
	v.scroll.SetPropagateNaturalHeight(true)
	v.scroll.SetHExpand(true)
	v.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	v.scroll.SetChild(v.list)

	v.searchEntry = gtk.NewSearchEntry()
	v.searchEntry.SetHExpand(true)
	v.searchEntry.SetVAlign(gtk.AlignCenter)
	v.searchEntry.SetObjectProperty("placeholder-text", locale.Get("Search Users"))
	v.searchEntry.ConnectSearchChanged(func() {
		v.searchString = strings.ToLower(v.searchEntry.Text())
		v.list.InvalidateFilter()
	})

	v.searchBar = gtk.NewSearchBar()
	v.searchBar.AddCSSClass("titlebar")
	v.searchBar.AddCSSClass("direct-searchbar")
	v.searchBar.ConnectEntry(&v.searchEntry.EditableTextWidget)
	v.searchBar.SetSearchMode(true)
	v.searchBar.SetShowCloseButton(false)
	v.searchBar.SetChild(v.searchEntry)

	v.box = adw.NewToolbarView()
	v.box.SetTopBarStyle(adw.ToolbarFlat)
	v.box.SetContent(v.scroll)
	v.box.AddTopBar(v.searchBar)

	v.LoadablePage = adaptive.NewLoadablePage()
	v.LoadablePage.SetLoading()

	vis := gtkutil.WithVisibility(ctx, v)

	state := gtkcord.FromContext(ctx)
	state.BindHandler(vis, func(ev gateway.Event) {
		// TODO: Channel events

		switch ev := ev.(type) {
		case *gateway.ChannelCreateEvent:
			if !ev.GuildID.IsValid() {
				v.Invalidate() // recreate everything
			}
		case *gateway.ChannelDeleteEvent:
			v.deleteCh(ev.ID)

		case *gateway.MessageCreateEvent:
			if ch, ok := v.channels[ev.ChannelID]; ok {
				ch.Invalidate()
			}
		case *read.UpdateEvent:
			if ch, ok := v.channels[ev.ChannelID]; ok {
				ch.Invalidate()
			}
		}
	},
		(*gateway.ChannelCreateEvent)(nil),
		(*gateway.ChannelDeleteEvent)(nil),
		(*gateway.MessageCreateEvent)(nil),
		(*read.UpdateEvent)(nil),
	)

	// Restore the last open channel. We must delay this until the view is
	// realized so the parent view can be realized first.
	gtkutil.OnFirstMap(v, func() {
		lastOpen.Get(func(id discord.ChannelID) {
			// Only restore selection if we're not already selecting something.
			if v.list.SelectedRow() == nil {
				v.SelectChannel(id)
			}
		})
	})

	return &v
}

// SelectChannel selects a known channel. If none is known, then it is selected
// later when the list is changed or never selected if the user selects
// something else.
func (v *ChannelView) SelectChannel(chID discord.ChannelID) {
	ch, ok := v.channels[chID]
	if !ok {
		v.selectID = chID
		return
	}

	v.selectID = 0
	v.list.SelectRow(ch.ListBoxRow)
}

// Invalidate invalidates the whole channel view.
func (v *ChannelView) Invalidate() {
	state := gtkcord.FromContext(v.ctx)

	// Temporarily disable the sort function. We'll re-enable it once we're
	// done and force a full re-sort.
	v.list.SetSortFunc(nil)
	defer func() {
		v.list.SetSortFunc(v.sort)
		v.list.InvalidateSort()
	}()

	chs, err := state.Cabinet.PrivateChannels()
	if err != nil {
		v.SetError(err)
		return
	}

	v.SetChild(v.box)

	// Keep track of channels that aren't in the list anymore.
	keep := make(map[discord.ChannelID]bool, len(v.channels))
	for id := range v.channels {
		keep[id] = false
	}

	for i, channel := range chs {
		ch := NewChannel(v.ctx, channel.ID)
		ch.Update(&chs[i])

		v.channels[channel.ID] = ch

		if _, ok := keep[channel.ID]; ok {
			keep[channel.ID] = true
		} else {
			v.list.Append(ch)
		}
	}

	// Remove channels that didn't appear in the tracking map.
	for id, new := range keep {
		if !new {
			v.deleteCh(id)
		}
	}

	// If we have a channel to be selectedd, then select it.
	if v.selectID.IsValid() {
		if ch, ok := v.channels[v.selectID]; ok {
			v.list.SelectRow(ch.ListBoxRow)
		}
	}
}

func (v *ChannelView) deleteCh(id discord.ChannelID) {
	ch, ok := v.channels[id]
	if !ok {
		return
	}

	v.list.Remove(ch)
	delete(v.channels, id)
}

func (v *ChannelView) sort(r1, r2 *gtk.ListBoxRow) int { // -1 == less == r1 first
	ch1 := v.rowChannel(r1)
	ch2 := v.rowChannel(r2)
	if ch1 == nil {
		return 1
	}
	if ch2 == nil {
		return -1
	}

	last1 := ch1.LastMessageID()
	last2 := ch2.LastMessageID()

	if !last1.IsValid() {
		return 1
	}
	if !last2.IsValid() {
		return -1
	}
	if last1 > last2 {
		// ch1 is older, put first.
		return -1
	}
	if last1 == last2 {
		return 0
	}
	return 1 // newer
}

func (v *ChannelView) filter(r *gtk.ListBoxRow) bool {
	if v.searchString == "" {
		return true
	}

	ch := v.rowChannel(r)
	if ch == nil {
		return false
	}

	name := strings.ToLower(ch.Name())
	return strings.Contains(name, v.searchString)
}

func (v *ChannelView) rowChannel(r *gtk.ListBoxRow) *Channel {
	id, err := discord.ParseSnowflake(r.Name())
	if err != nil {
		log.Panicln("cannot parse channel row name:", err)
	}

	ch, ok := v.channels[discord.ChannelID(id)]
	if !ok {
		log.Println("warning: ChannelView: row has unknown channel ID", id)
		return nil
	}

	return ch
}
