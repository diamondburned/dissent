package channels

import (
	"context"
	"fmt"
	"html"
	"log"
	"sort"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/author"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/ningen/v3"
)

type any = interface{}

type unreadState = int

const (
	allRead unreadState = iota
	unreadMessages
	unreadMentions
	channelMuted
)

const (
	valueUnread    = "●"
	valueMentioned = "! " + valueUnread
)

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

var allowedChannelTypes = []discord.ChannelType{
	discord.GuildText,
	discord.GuildCategory,
	discord.GuildPublicThread,
	discord.GuildPrivateThread,
	discord.GuildVoice,
	discord.GuildStageVoice,
}

// Add adds the given list of channels into the guild tree.
func (t *GuildTree) Add(channels []discord.Channel) {
	chs := drainer(channels)
	chs.sort()

	// Set channels without categories.
	chs.drain(func(ch discord.Channel) bool {
		if ch.Type != discord.GuildText || ch.ParentID.IsValid() {
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
		if !ch.ParentID.IsValid() || ch.Type != discord.GuildText {
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
		node := newChannelNode(base)
		node.Update(&ch)

		t.keep(node)
		return true
	})

	// Set nested threads that are inside channels.
	chs.drain(func(ch discord.Channel) bool {
		if !ch.ParentID.IsValid() {
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
		case discord.GuildPrivateThread:
			node = newThreadNode(base)
			node.Update(&ch)
		case discord.GuildPublicThread:
			node = newThreadNode(base)
			node.Update(&ch)
		case discord.GuildVoice, discord.GuildStageVoice:
			node = newVoiceChannelNode(base)
			node.Update(&ch)
		default:
			// Remove the iterator that we've just appended in.
			if iter, ok := t.Iter(base.path); ok {
				t.TreeStore.Remove(iter)
			}
			return false
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
func (t *GuildTree) append(ch *discord.Channel, parent *gtk.TreeIter) BaseChannelNode {
	iter := t.TreeStore.Append(parent)
	base := BaseChannelNode{
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
	t.SetUnread(id, t.state().ChannelIsUnread(id))
}

// SetUnread marks the given channel as read or unread.
func (t *GuildTree) SetUnread(id discord.ChannelID, unread ningen.UnreadIndication) {
	node := t.Node(id)
	if node == nil {
		return
	}

	switch node := t.Node(id).(type) {
	case *ChannelNode:
		node.SetUnread(unread)
	case *ThreadNode:
		node.SetUnread(unread)
	}
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
	// TreePath is the tree path pointing to the channel node.
	TreePath() *gtk.TreePath
}

// BaseChannelNode is the base of all channel nodes. It implements the Node
// interface and contains common information that all channels have.
type BaseChannelNode struct {
	path *gtk.TreePath
	head *GuildTree

	id discord.ChannelID
}

// ID implements Node.
func (n *BaseChannelNode) ID() discord.ChannelID { return n.id }

// Update implements Node. It does nothing.
func (n *BaseChannelNode) Update(ch *discord.Channel) {}

// TreePath implements Node.
func (n *BaseChannelNode) TreePath() *gtk.TreePath { return n.path }

func (n *BaseChannelNode) treeIter() (*gtk.TreeIter, bool) {
	return n.head.Iter(n.path)
}

// zeroInit initializes the row with a nil icon and a channel name.
func (n *BaseChannelNode) zeroInit(ch *discord.Channel) {
	muted := n.head.state().ChannelIsMuted(n.id, true)

	n.head.set(n.path, [...]any{
		dimText(ch.Name, muted),
		uint64(n.id),
		"",
		html.EscapeString(ch.Name),
	})
}

// setUnread sets the unread column.
func (n *BaseChannelNode) setUnread(unread ningen.UnreadIndication) {
	var col string

	switch unread {
	case ningen.ChannelUnread:
		col = valueUnread
	case ningen.ChannelMentioned:
		col = valueMentioned
	}

	n.head.setValues(n.path, [maxTreeColumn]any{
		columnUnread: col,
	})
}

// CategoryNode is a category node.
type CategoryNode struct {
	BaseChannelNode
	unreadMentioned map[discord.ChannelID]bool
}

func newCategoryNode(base BaseChannelNode, ch *discord.Channel) *CategoryNode {
	return &CategoryNode{
		BaseChannelNode: base,
		unreadMentioned: make(map[discord.ChannelID]bool),
	}
}

func (n *CategoryNode) Update(ch *discord.Channel) {
	muted := n.head.state().ChannelIsMuted(n.id, true)

	n.head.setValues(n.path, [maxTreeColumn]any{
		columnName:    dimText(ch.Name, muted),
		columnTooltip: html.EscapeString(ch.Name),
	})
}

// setUnread registers the channel inside the category as read or unread. Note
// that it does not check if the given channel is actually inside CategoryNode
// or not.
func (n *CategoryNode) setUnread(ch discord.ChannelID, unread ningen.UnreadIndication) {
	if unread == ningen.ChannelRead {
		delete(n.unreadMentioned, ch)
	} else {
		n.unreadMentioned[ch] = (unread == ningen.ChannelMentioned)
	}

	if len(n.unreadMentioned) == 0 {
		n.BaseChannelNode.setUnread(ningen.ChannelRead)
		return
	}

	unread = ningen.ChannelUnread
	for _, mentioned := range n.unreadMentioned {
		if mentioned {
			unread = ningen.ChannelMentioned
			break
		}
	}

	n.BaseChannelNode.setUnread(unread)
}

// ChannelNode is a regular text channel node.
type ChannelNode struct {
	BaseChannelNode
	parentID discord.ChannelID
}

func newChannelNode(base BaseChannelNode) *ChannelNode {
	return &ChannelNode{
		BaseChannelNode: base,
	}
}

const (
	chHash     = `<span face="monospace"><b><span size="large" rise="-600">#</span><span size="x-small" rise="-2000"> </span></b></span>`
	chNSFWHash = `<span face="monospace"><b><span size="large" rise="-600">#</span><span size="x-small" rise="-2000">!</span></b></span>`
)

func (n *ChannelNode) Update(ch *discord.Channel) {
	n.parentID = ch.ParentID

	hash := chHash
	if ch.NSFW {
		hash = chNSFWHash
	}

	muted := n.head.state().ChannelIsMuted(n.id, true)

	n.head.setValues(n.path, [maxTreeColumn]any{
		// Add a space at the end because the channel's height is otherwise a
		// bit shorter.
		columnName:    dimMarkup(hash+html.EscapeString(ch.Name)+" ", muted),
		columnTooltip: "#" + html.EscapeString(ch.Name),
	})
}

// SetUnread sets whether the channel is unread and mentioned.
func (n *ChannelNode) SetUnread(unread ningen.UnreadIndication) {
	n.setUnread(unread)

	if n.parentID.IsValid() {
		if parent, ok := n.head.Node(n.parentID).(*CategoryNode); ok {
			parent.setUnread(n.id, unread)
		}
	}
}

// ThreadNode is a node indicating a Discord thread.
type ThreadNode struct {
	BaseChannelNode
}

func newThreadNode(base BaseChannelNode) *ThreadNode {
	return &ThreadNode{
		BaseChannelNode: base,
	}
}

func (n *ThreadNode) Update(ch *discord.Channel) {
	n.head.setValues(n.path, [maxTreeColumn]any{
		columnName: ch.Name,
	})
}

func (n *ThreadNode) SetUnread(unread ningen.UnreadIndication) {
	n.setUnread(unread)
}

type VoiceChannelNode struct {
	BaseChannelNode
	guildID discord.GuildID
}

func newVoiceChannelNode(base BaseChannelNode) *VoiceChannelNode {
	return &VoiceChannelNode{
		BaseChannelNode: base,
	}
}

const vcIcon = `🔊 `

func (n *VoiceChannelNode) Update(ch *discord.Channel) {
	n.guildID = ch.GuildID

	states, _ := n.head.state().VoiceStates(ch.GuildID)
	if states == nil {
		n.setVoiceUsers(nil)
		return
	}

	members := make([]discord.Member, 0, len(states))
	for _, state := range states {
		if state.ChannelID != ch.ID {
			continue
		}

		member := state.Member
		if member == nil {
			member, _ = n.head.state().Member(ch.GuildID, state.UserID)
		}
		if member == nil {
			continue
		}
		members = append(members, *member)
	}

	name := vcIcon + ch.Name
	if len(members) > 0 {
		name += fmt.Sprintf(" (%d)", len(members))
	}

	n.setVoiceUsers(members)
	n.head.setValues(n.path, [maxTreeColumn]any{
		columnName: name,
	})
}

func (n *VoiceChannelNode) setVoiceUsers(members []discord.Member) {
	// Defer clearing so GTK doesn't hide the node when we're replacing it.
	clear := n.deferClear()
	defer clear()

	if len(members) == 0 {
		return
	}

	parent, ok := n.head.TreeStore.Iter(n.path)
	if !ok {
		return
	}

	sort.SliceStable(members, func(i, j int) bool {
		return memberName(&members[i]) < memberName(&members[j])
	})

	for _, member := range members {
		iter := n.head.TreeStore.Append(parent)
		path := n.head.TreeStore.Path(iter)

		n.head.set(path, [...]any{
			n.head.state().MemberMarkup(
				n.guildID,
				&discord.GuildUser{User: member.User, Member: &member},
				author.WithMinimal(),
				author.WithColor(""), // no color for consistency
			),
			uint64(discord.NullSnowflake),
			"",
			member.User.Tag(),
		})
	}
}

func memberName(member *discord.Member) string {
	if member.Nick != "" {
		return member.Nick
	}
	return member.User.Tag()
}

func (n *VoiceChannelNode) deferClear() func() {
	parent, ok := n.head.Iter(n.path)
	if !ok {
		return func() {}
	}

	len := n.head.TreeStore.IterNChildren(parent)

	return func() {
		it, ok := n.head.TreeStore.IterChildren(parent)
		for i := 0; ok && i < len; i++ {
			ok = n.head.TreeStore.Remove(it)
		}
	}
}

func dimMarkup(str string, dimmed bool) string {
	if dimmed {
		str = fmt.Sprintf(`<span alpha="50%%">%s</span>`, str)
	}
	return str
}

func dimText(text string, dimmed bool) string {
	return dimMarkup(html.EscapeString(text), dimmed)
}
