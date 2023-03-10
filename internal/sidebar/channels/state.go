package channels

import (
	"context"
	"log"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/ningen/v3"
)

type any = interface{}

type treeColumn = int

const (
	columnName treeColumn = iota
	columnID
	columnUnread
	columnTooltip

	maxTreeColumn
)

var allTreeColumns = []treeColumn{
	columnName,
	columnID,
	columnUnread,
	columnTooltip,
}

var columnTypes = []glib.Type{
	glib.TypeString,
	glib.TypeUint64,
	glib.TypeString,
	glib.TypeString,
}

// GuildTree is the channel tree that holds the state of all channels.
type GuildTree struct {
	*gtk.TreeStore
	nodes map[discord.ChannelID]Node
	ctx   context.Context
}

// NewGuildTree creates a new GuildTree.
func NewGuildTree(ctx context.Context) *GuildTree {
	return &GuildTree{
		TreeStore: gtk.NewTreeStore(columnTypes),
		nodes:     make(map[discord.ChannelID]Node),
		ctx:       ctx,
	}
}

var okChTypes = map[discord.ChannelType]bool{
	discord.GuildText:               true,
	discord.GuildCategory:           false, // handled separately
	discord.GuildPublicThread:       true,
	discord.GuildPrivateThread:      true,
	discord.GuildForum:              true,
	discord.GuildAnnouncement:       true,
	discord.GuildAnnouncementThread: true,
	discord.GuildVoice:              true,
	discord.GuildStageVoice:         true,
}

// Add adds the given list of channels into the guild tree.
func (t *GuildTree) Add(channels []discord.Channel) {
	chs := drainer(channels)
	chs.sort()

	// Set channels without categories.
	chs.drain(func(ch discord.Channel) bool {
		if ch.ParentID.IsValid() || !okChTypes[ch.Type] {
			return false
		}

		base := t.append(&ch, nil)
		node := newChannelNode(base)
		node.Update(&ch)

		t.keep(node)
		return true
	})

	// Set categories.
	chs.drain(func(ch discord.Channel) bool {
		if ch.Type != discord.GuildCategory {
			return false
		}

		base := t.append(&ch, nil)
		node := newCategoryNode(base, &ch)
		node.Update(&ch)

		t.keep(node)
		return true
	})

	// Set nested text channels that are inside catagories.
	chs.drain(func(ch discord.Channel) bool {
		if !ch.ParentID.IsValid() {
			return false
		}

		if ch.Type != discord.GuildText && ch.Type != discord.GuildForum {
			// Other channel types are handled in the drain function below.
			return false
		}

		parent := t.nodes[ch.ParentID]
		if parent == nil {
			log.Println("channel", ch.Name, "has unknown parent ID")
			return false
		}

		parentIter, ok := t.Iter(parent.TreePath())
		if !ok {
			return false
		}

		base := t.append(&ch, parentIter)

		var node Node
		switch ch.Type {
		case discord.GuildForum:
			node = newForumNode(base)
			node.Update(&ch)
		default:
			node = newChannelNode(base)
			node.Update(&ch)
		}

		t.keep(node)
		return true
	})

	// Set nested threads that are inside channels.
	chs.drain(func(ch discord.Channel) bool {
		if !ch.ParentID.IsValid() || !okChTypes[ch.Type] {
			return false
		}

		parent := t.nodes[ch.ParentID]
		if parent == nil {
			log.Println("nested channel", ch.Name, "has unknown parent ID")
			return false
		}

		parentIter, ok := t.Iter(parent.TreePath())
		if !ok {
			return false
		}

		base := t.append(&ch, parentIter)
		var node Node

		switch ch.Type {
		case discord.GuildPrivateThread, discord.GuildPublicThread, discord.GuildAnnouncementThread:
			node = newThreadNode(base)
			node.Update(&ch)
		case discord.GuildVoice, discord.GuildStageVoice:
			node = newVoiceChannelNode(base)
			node.Update(&ch)
		default:
			node = newChannelNode(base)
			node.Update(&ch)
		}

		t.keep(node)
		return true
	})
}

// keep saves n into the internal registry.
func (t *GuildTree) keep(n Node) {
	t.nodes[n.ID()] = n
	t.UpdateUnread(n.ID())
}

// append appends a new empty node and returns its iterator.
func (t *GuildTree) append(ch *discord.Channel, parent *gtk.TreeIter) baseChannelNode {
	iter := t.TreeStore.Append(parent)
	base := baseChannelNode{
		path: t.Path(iter),
		head: t,
		id:   ch.ID,
	}
	base.zeroInit(ch)
	return base
}

// Remove removes the channel node with the given ID.
func (t *GuildTree) Remove(id discord.ChannelID) {
	// TODO: this doesn't handle removing categories.
	n, ok := t.nodes[id]
	if ok {
		it, ok := t.TreeStore.Iter(n.TreePath())
		if ok {
			t.TreeStore.Remove(it)
		}

		delete(t.nodes, id)
	}
}

func (t *GuildTree) state() *gtkcord.State {
	return gtkcord.FromContext(t.ctx)
}

// NodeFromIter returns the channel from the given TreeIter.
func (t *GuildTree) NodeFromIter(iter *gtk.TreeIter) Node {
	gv := t.TreeStore.Value(iter, columnID)
	id := gv.GoValue().(uint64)
	return t.nodes[discord.ChannelID(id)]
}

// NodeFromPath quickly looks up the channel tree for a node from the given tree
// path.
func (t *GuildTree) NodeFromPath(path *gtk.TreePath) Node {
	it, ok := t.TreeStore.Iter(path)
	if !ok {
		return nil
	}
	return t.NodeFromIter(it)
}

// Has returns true if the guild tree has the given channel.
func (t *GuildTree) Has(id discord.ChannelID) bool {
	_, ok := t.nodes[id]
	return ok
}

// Node quickly looks up the channel tree for a node.
func (t *GuildTree) Node(id discord.ChannelID) Node {
	return t.nodes[id]
}

// UpdateChannel updates the channel node with the given ID, or if the node is
// not known, then it does nothing.
func (t *GuildTree) UpdateChannel(id discord.ChannelID) {
	node := t.Node(id)
	if node == nil {
		return
	}

	state := t.state()

	ch, err := state.Offline().Channel(id)
	if err != nil {
		return
	}

	node.Update(ch)
}

// UpdateUnread updates the unread state of the channel with the given ID.
func (t *GuildTree) UpdateUnread(id discord.ChannelID) {
	node := t.Node(id)
	if node == nil {
		return
	}

	node.UpdateUnread()
}

func (t *GuildTree) set(path *gtk.TreePath, v [maxTreeColumn]any) {
	it, ok := t.Iter(path)
	if !ok {
		return
	}

	values := make([]glib.Value, len(v))
	for i, value := range v {
		if value == nil {
			panic("unexpected nil value given to set [maxTreeColumn]any")
		}
		values[i] = *glib.NewValue(value)
	}

	t.TreeStore.Set(it, allTreeColumns, values)
}

func (t *GuildTree) setValues(path *gtk.TreePath, values [maxTreeColumn]any) {
	it, ok := t.Iter(path)
	if !ok {
		return
	}

	for col, val := range values {
		if val == nil {
			continue
		}
		t.TreeStore.SetValue(it, col, glib.NewValue(val))
	}
}

// Node describes a channel node in the channel tree.
type Node interface {
	// ID is the ID of the channel node.
	ID() discord.ChannelID
	// Update passes the new Channel object into the Node for it to update its
	// own information.
	Update(*discord.Channel)
	// UpdateUnread updates the unread state of the node.
	UpdateUnread()
	// TreePath is the tree path pointing to the channel node.
	TreePath() *gtk.TreePath

	nodeInternals
}

type nodeInternals interface {
	setUnread(ningen.UnreadIndication, bool)
	getUnread() ningen.UnreadIndication
}
