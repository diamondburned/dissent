package threads

import (
	"context"
	"sort"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

// TODO: implement ArchivedThreadsListModel, ideally with pagination.
// TODO: implement ArchivedThreadsListModel.Search().

// ThreadsListModel wraps around a threadsListModel and manages updating
// it.
type ThreadsListModel struct {
	*gtkutil.ListModel[discord.ChannelID]
	order discord.SortOrderType
}

// NewThreadsListModel creates a new threads list model.
func NewThreadsListModel(order discord.SortOrderType) *ThreadsListModel {
	return &ThreadsListModel{
		gtkutil.NewListModel[discord.ChannelID](),
		order,
	}
}

// NewActiveThreadsModel creates a new threads list model for the active
// threads in the given parent channel.
func NewActiveThreadsModel(ctx context.Context, parentID discord.ChannelID) *ThreadsListModel {
	state := gtkcord.FromContext(ctx)
	list := NewThreadsListModel(discord.SoftOrderTypeCreationDate)

	channels, _ := state.Offline().NestedChannels(parentID)
	list.Update(channels)

	// TODO: figure this out lol
	// var unbind signaling.DisconnectStack

	return list
}

// Update refreshes the list with the latest threads from the parent channel.
func (m *ThreadsListModel) Update(channels []discord.Channel) {
	sortThreads(channels, m.order)

	channelIDs := make([]discord.ChannelID, len(channels))
	for i, ch := range channels {
		channelIDs[i] = ch.ID
	}

	// Update the list model in one go.
	m.ListModel.Splice(0, m.ListModel.NItems(), channelIDs...)
}

func sortThreads(channels []discord.Channel, order discord.SortOrderType) {
	switch order {
	case discord.SoftOrderTypeCreationDate:
		sort.Slice(channels, func(i, j int) bool {
			a := channels[i]
			b := channels[j]
			if a.ThreadMetadata == nil {
				return false
			}
			if b.ThreadMetadata == nil {
				return true
			}
			aTime := a.ThreadMetadata.CreateTimestamp.Time()
			bTime := b.ThreadMetadata.CreateTimestamp.Time()
			return aTime.After(bTime)
		})
	case discord.SortOrderTypeLatestActivity:
		sort.Slice(channels, func(i, j int) bool {
			a := channels[i]
			b := channels[j]
			aTime := a.LastMessageID.Time()
			bTime := b.LastMessageID.Time()
			return aTime.After(bTime)
		})
	}
}
