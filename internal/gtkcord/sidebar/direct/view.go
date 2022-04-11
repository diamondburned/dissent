package direct

import (
	"context"
	"log"
	"strings"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

// ChannelView displays a list of direct messaging channels.
type ChannelView struct {
	*adaptive.LoadablePage
	box *gtk.Box // direct child

	scroll *gtk.ScrolledWindow
	list   *gtk.ListBox

	searchBar    *gtk.SearchBar
	searchEntry  *gtk.SearchEntry
	searchString string

	ctx      context.Context
	channels map[discord.ChannelID]*Channel
}

// Controller is the parent controller that ChannelView controls.
type Controller interface {
	OpenChannel(discord.ChannelID)
}

var _ = cssutil.WriteCSS(`
	.direct-searchbar > revealer > box {
		border-bottom: 0;
	}
`)

// NewChannelView creates a new view.
func NewChannelView(ctx context.Context, ctrl Controller) *ChannelView {
	v := ChannelView{
		ctx:      ctx,
		channels: make(map[discord.ChannelID]*Channel, 50),
	}

	v.list = gtk.NewListBox()
	v.list.AddCSSClass("direct-list")
	v.list.SetHExpand(true)
	v.list.SetSortFunc(v.sort)
	v.list.SetFilterFunc(v.filter)
	v.list.SetActivateOnSingleClick(true)
	v.list.ConnectRowActivated(func(r *gtk.ListBoxRow) {
		ch := v.rowChannel(r)
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
	v.searchEntry.SetObjectProperty("placeholder-text", "Search Users")
	v.searchEntry.ConnectSearchChanged(func() {
		v.searchString = strings.ToLower(v.searchEntry.Text())
		v.list.InvalidateFilter()
	})

	v.searchBar = gtk.NewSearchBar()
	v.searchBar.AddCSSClass("titlebar")
	v.searchBar.AddCSSClass("direct-searchbar")
	v.searchBar.ConnectEntry(&v.searchEntry.Editable)
	v.searchBar.SetSearchMode(true)
	v.searchBar.SetShowCloseButton(false)
	v.searchBar.SetChild(v.searchEntry)

	v.box = gtk.NewBox(gtk.OrientationVertical, 0)
	v.box.Append(v.searchBar)
	v.box.Append(v.scroll)

	v.LoadablePage = adaptive.NewLoadablePage()
	v.LoadablePage.SetLoading()

	vis := gtkutil.WithVisibility(ctx, v)

	state := gtkcord.FromContext(ctx)
	state.BindHandler(vis, func(ev gateway.Event) {
		// TODO: Channel events

		switch ev := ev.(type) {
		case *gateway.ChannelDeleteEvent:
			v.deleteCh(ev.ID)
		case *gateway.MessageCreateEvent:
			if ch, ok := v.channels[ev.ChannelID]; ok {
				ch.InvalidateSort()
			}
		}
	})

	// TODO: search

	return &v
}

// Invalidate invalidates the whole channel view.
func (v *ChannelView) Invalidate() {
	v.SetLoading()

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

	// Keep track of channels that aren't in the list anymore.
	keep := make(map[discord.ChannelID]bool, len(v.channels))
	for id := range v.channels {
		keep[id] = false
	}

	for i, channel := range chs {
		ch := NewChannel(v.ctx, channel.ID)
		ch.Update(&chs[i])

		v.channels[channel.ID] = ch
		v.list.Append(ch)

		if _, ok := keep[channel.ID]; ok {
			keep[channel.ID] = true
		}
	}

	// Remove channels that didn't appear in the tracking map.
	for id, new := range keep {
		if !new {
			v.deleteCh(id)
		}
	}

	v.SetChild(v.box)
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
		log.Panicln("row has unknown channel ID", id)
	}

	return ch
}
