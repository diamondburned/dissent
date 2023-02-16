package channels

import (
	"html"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/ningen/v3"
)

const (
	valueUnread         = "●"
	valueChildUnread    = "○"
	valueMentioned      = "! " + valueUnread
	valueChildMentioned = "! " + valueChildUnread
)

// baseChannelNode is the base of all channel nodes. It implements the Node
// interface and contains common information that all channels have.
type baseChannelNode struct {
	path *gtk.TreePath
	head *GuildTree

	id     discord.ChannelID
	unread ningen.UnreadIndication
}

var _ Node = (*baseChannelNode)(nil)

// ID implements Node.
func (n *baseChannelNode) ID() discord.ChannelID { return n.id }

// Update implements Node. It does nothing.
func (n *baseChannelNode) Update(ch *discord.Channel) {
	n.UpdateUnread()
}

// TreePath implements Node.
func (n *baseChannelNode) TreePath() *gtk.TreePath { return n.path }

// EachChildren calls the given function for each child of the node. If f
// returns false, then the iteration is stopped.
func (n *baseChannelNode) EachChildren(f func(Node) bool) {
	iter, ok := n.head.TreeStore.Iter(n.path)
	if !ok {
		return
	}

	iter, ok = n.head.TreeStore.IterChildren(iter)
	if !ok {
		return
	}

	for ok {
		node := n.head.NodeFromIter(iter)
		if node != nil && !f(node) {
			break
		}
		ok = n.head.TreeStore.IterNext(iter)
	}
}

func (n *baseChannelNode) UpdateUnread() {
	// Update self's unread indicator.
	n.unread = n.head.state().ChannelIsUnread(n.id)

	var fromChild bool
	n.EachChildren(func(child Node) bool {
		unread := child.getUnread()
		if unread > n.unread {
			n.unread = unread
			fromChild = true
		}
		// Loop until we find the highest unread indicator.
		return n.unread != ningen.ChannelMentioned
	})

	n.setUnread(n.unread, fromChild)
}

func (n *baseChannelNode) getUnread() ningen.UnreadIndication {
	return n.unread
}

func (n *baseChannelNode) setUnread(unread ningen.UnreadIndication, fromChild bool) {
	var col string
	if !n.isMuted() {
		if fromChild {
			switch unread {
			case ningen.ChannelUnread:
				col = valueChildUnread
			case ningen.ChannelMentioned:
				col = valueChildMentioned
			}
		} else {
			switch unread {
			case ningen.ChannelUnread:
				col = valueUnread
			case ningen.ChannelMentioned:
				col = valueMentioned
			}
		}
	}

	n.head.setValues(n.path, [maxTreeColumn]any{
		columnUnread: col,
	})

	n.unread = unread
	n.updateParentUnreadIndicator()
}

func (n *baseChannelNode) isMuted() bool {
	return n.head.state().ChannelIsMuted(n.id, true)
}

// zeroInit initializes the row with a nil icon and a channel name.
func (n *baseChannelNode) zeroInit(ch *discord.Channel) {
	n.head.set(n.path, [...]any{
		dimText(ch.Name, n.isMuted()),
		uint64(n.id),
		"",
		html.EscapeString(ch.Name),
	})
}

func (n *baseChannelNode) self() Node {
	return n.head.nodes[n.id]
}

func (n *baseChannelNode) parent() Node {
	iter, ok := n.head.TreeStore.Iter(n.TreePath())
	if !ok {
		return nil
	}

	iter, ok = n.head.TreeStore.IterParent(iter)
	if !ok {
		return nil
	}

	parent := n.head.NodeFromIter(iter)
	return parent
}

func (n *baseChannelNode) updateParentUnreadIndicator() {
	parent := n.parent()
	if parent == nil {
		return
	}
	parent.UpdateUnread()
}
