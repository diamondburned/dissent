package channels

import (
	"fmt"
	"log"
	"sort"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/signaling"
)

type modelManager struct {
	*gtk.TreeListModel
	state   *gtkcord.State
	guildID discord.GuildID
}

func newModelManager(state *gtkcord.State, guildID discord.GuildID) *modelManager {
	m := &modelManager{
		state:   state,
		guildID: guildID,
	}
	m.TreeListModel = gtk.NewTreeListModel(
		m.Model(0), true, true,
		func(item *glib.Object) *gio.ListModel {
			chID := channelIDFromItem(item)

			model := m.Model(chID)
			if model == nil {
				return nil
			}

			return &model.ListModel
		})
	return m
}

// Model returns the list model containing all channels within the given channel
// ID. If chID is 0, then the guild's root channels will be returned. This
// function may return nil, indicating that the channel will never have any
// children.
func (m *modelManager) Model(chID discord.ChannelID) *gtk.StringList {
	model := gtk.NewStringList(nil)

	list := newChannelList(m.state, glib.NewWeakRef(model))

	var unbind signaling.DisconnectStack
	list.ConnectDestroy(func() { unbind.Disconnect() })

	unbind.Push(
		m.state.AddHandler(func(ev *gateway.ChannelCreateEvent) {
			if ev.GuildID != m.guildID {
				return
			}
			if ev.Channel.ParentID == chID {
				list.Append(ev.Channel)
			}
		}),
		m.state.AddHandler(func(ev *gateway.ChannelUpdateEvent) {
			if ev.GuildID != m.guildID {
				return
			}
			// Handle channel position moves.
			if ev.Channel.ParentID == chID {
				list.Append(ev.Channel)
			} else {
				list.Remove(ev.Channel.ID)
			}
		}),
		m.state.AddHandler(func(ev *gateway.ThreadCreateEvent) {
			if ev.GuildID != m.guildID {
				return
			}
			if ev.Channel.ParentID == chID {
				list.Append(ev.Channel)
			}
		}),
		m.state.AddHandler(func(ev *gateway.ThreadDeleteEvent) {
			if ev.GuildID != m.guildID {
				return
			}
			if ev.ParentID == chID {
				list.Remove(ev.ID)
			}
		}),
		m.state.AddHandler(func(ev *gateway.ThreadListSyncEvent) {
			if ev.GuildID != m.guildID {
				return
			}

			if ev.ChannelIDs == nil {
				// The entire guild was synced, so invalidate everything.
				m.invalidateAll(chID, list)
				return
			}

			for _, parentID := range ev.ChannelIDs {
				if parentID == chID {
					// This sync event is also for us.
					m.invalidateAll(chID, list)
					break
				}
			}
		}),
	)

	m.addAllChannels(chID, list)
	return model
}

func (m *modelManager) invalidateAll(parentID discord.ChannelID, list *channelList) {
	list.Clear()
	m.addAllChannels(parentID, list)
}

func (m *modelManager) addAllChannels(parentID discord.ChannelID, list *channelList) {
	for _, ch := range fetchSortedChannels(m.state, m.guildID, parentID) {
		list.Append(ch)
	}
}

// channelList wraps a StringList to maintain a set of channel IDs.
// Because this is a set, each channel ID can only appear once.
type channelList struct {
	state *gtkcord.State
	list  *glib.WeakRef[*gtk.StringList]
	set   map[string]struct{}
}

func newChannelList(state *gtkcord.State, ref *glib.WeakRef[*gtk.StringList]) *channelList {
	return &channelList{
		state: state,
		list:  ref,
		set:   make(map[string]struct{}),
	}
}

// CalculatePosition converts the position of a channel given by Discord to the
// position relative to the list. If the channel is not found, then this
// function returns the end of the list.
func (l *channelList) CalculatePosition(target discord.Channel) uint {
	list := l.list.Get()
	end := list.NItems()

	// Find this particular channel in the list.
	for i, ch := range fetchSortedChannels(l.state, target.GuildID, target.ParentID) {
		if ch.ID == target.ID {
			// Sanity check.
			if i > int(end) {
				log.Printf("CalculatePosition: channel %d is out of bounds", target.ID)
				return end
			}
			return uint(i)
		}
	}

	return end
}

// Append appends a channel to the list. If the channel already exists, then
// this function does nothing.
func (l *channelList) Append(ch discord.Channel) {
	str := ch.ID.String()
	if _, exists := l.set[str]; exists {
		return
	}
	l.set[str] = struct{}{}

	pos := l.CalculatePosition(ch)
	list := l.list.Get()
	list.Splice(pos, 0, []string{str})
}

// Remove removes the channel ID from the list. If the channel ID is not in the
// list, then this function does nothing.
func (l *channelList) Remove(chID discord.ChannelID) {
	str := chID.String()
	if _, exists := l.set[str]; !exists {
		return
	}
	if i := l.Index(chID); i != -1 {
		list := l.list.Get()
		list.Remove(uint(i))
	}
	delete(l.set, str)
}

// Contains returns whether the channel ID is in the list.
func (l *channelList) Contains(chID discord.ChannelID) bool {
	_, exists := l.set[chID.String()]
	return exists
}

// Index returns the index of the channel ID in the list. If the channel ID is
// not in the list, then this function returns -1.
func (l *channelList) Index(chID discord.ChannelID) int {
	ix := -1
	iter := l.All()
	iter(func(i int, id discord.ChannelID) bool {
		if id == chID {
			ix = i
			return false
		}
		return true
	})
	return ix
}

// Clear clears the list.
func (l *channelList) Clear() {
	list := l.list.Get()
	list.Splice(0, list.NItems(), nil)
	l.set = make(map[string]struct{})
}

// All returns a function that iterates over all channel IDs in the list.
func (l *channelList) All() func(yield func(i int, id discord.ChannelID) bool) {
	list := l.list.Get()
	n := list.NItems()
	return func(yield func(int, discord.ChannelID) bool) {
		for i := uint(0); i < n; i++ {
			id, err := discord.ParseSnowflake(list.String(i))
			if err != nil {
				panic(fmt.Sprintf("channelList: invalid channel ID %q", list.String(i)))
			}
			if !yield(int(i), discord.ChannelID(id)) {
				return
			}
		}
	}
}

func (l *channelList) ConnectDestroy(f func()) {
	// I think this is the only way to know if a ListModel is no longer
	// being used? At least from reading the source code, which just calls
	// g_clear_pointer.
	list := l.list.Get()
	glib.WeakRefObject(list, f)
}

func fetchSortedChannels(state *gtkcord.State, guildID discord.GuildID, parentID discord.ChannelID) []discord.Channel {
	channels, err := state.Offline().Channels(guildID, gtkcord.AllowedChannelTypes)
	if err != nil {
		log.Printf("CalculatePosition: failed to get channels: %v", err)
		return nil
	}

	// Filter out all channels that are not in the same parent channel.
	filtered := channels[:0]
	for i, ch := range channels {
		if ch.ParentID == parentID || (parentID == 0 && !ch.ParentID.IsValid()) {
			filtered = append(filtered, channels[i])
		}
	}

	// Sort so that the channels are in increasing order.
	sort.Slice(filtered, func(i, j int) bool {
		a := filtered[i]
		b := filtered[j]
		if a.Position == b.Position {
			return a.ID < b.ID
		}
		return a.Position < b.Position
	})

	return filtered
}
