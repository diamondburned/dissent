package channels

import (
	"fmt"
	"html"
	"sort"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/author"
)

// CategoryNode is a category node.
type CategoryNode struct {
	baseChannelNode
	unreadMentioned map[discord.ChannelID]bool
}

func newCategoryNode(base baseChannelNode, ch *discord.Channel) *CategoryNode {
	return &CategoryNode{
		baseChannelNode: base,
		unreadMentioned: make(map[discord.ChannelID]bool),
	}
}

func (n *CategoryNode) Update(ch *discord.Channel) {
	n.baseChannelNode.Update(ch)
	n.head.setValues(n.path, [maxTreeColumn]any{
		columnName:    dimText(ch.Name, n.isMuted()),
		columnTooltip: html.EscapeString(ch.Name),
	})
}

// ChannelNode is a regular text channel node.
type ChannelNode struct {
	baseChannelNode
	parentID discord.ChannelID
}

func newChannelNode(base baseChannelNode) *ChannelNode {
	return &ChannelNode{
		baseChannelNode: base,
	}
}

const (
	chHash     = `<span face="monospace"><b><span size="large" rise="-600">#</span><span size="x-small" rise="-2000"> </span></b></span>`
	chNSFWHash = `<span face="monospace"><b><span size="large" rise="-600">#</span><span size="x-small" rise="-2000">!</span></b></span>`
)

func (n *ChannelNode) Update(ch *discord.Channel) {
	n.baseChannelNode.Update(ch)
	n.parentID = ch.ParentID

	hash := chHash
	if ch.NSFW {
		hash = chNSFWHash
	}

	n.head.setValues(n.path, [maxTreeColumn]any{
		// Add a space at the end because the channel's height is otherwise a
		// bit shorter.
		columnName:    dimMarkup(hash+html.EscapeString(ch.Name)+" ", n.isMuted()),
		columnTooltip: "#" + html.EscapeString(ch.Name),
	})
}

// ForumNode is a node indicating a Discord forum.
type ForumNode struct {
	baseChannelNode
}

func newForumNode(base baseChannelNode) *ForumNode {
	return &ForumNode{
		baseChannelNode: base,
	}
}

func (n *ForumNode) Update(ch *discord.Channel) {
	n.baseChannelNode.Update(ch)
	n.head.setValues(n.path, [maxTreeColumn]any{
		columnName: ch.Name,
	})
}

// ThreadNode is a node indicating a Discord thread.
type ThreadNode struct {
	baseChannelNode
	parentID discord.ChannelID
}

func newThreadNode(base baseChannelNode) *ThreadNode {
	return &ThreadNode{
		baseChannelNode: base,
	}
}

func (n *ThreadNode) Update(ch *discord.Channel) {
	n.baseChannelNode.Update(ch)
	n.parentID = ch.ParentID
	n.head.setValues(n.path, [maxTreeColumn]any{
		columnName: dimMarkup(html.EscapeString(ch.Name)+" ", n.isMuted()),
	})
}

type VoiceChannelNode struct {
	baseChannelNode
	guildID discord.GuildID
}

func newVoiceChannelNode(base baseChannelNode) *VoiceChannelNode {
	return &VoiceChannelNode{
		baseChannelNode: base,
	}
}

const vcIcon = `🔊 `

func (n *VoiceChannelNode) Update(ch *discord.Channel) {
	n.baseChannelNode.Update(ch)
	n.guildID = ch.GuildID

	states, _ := n.head.state().VoiceStates(ch.GuildID)
	if states == nil {
		n.setVoiceUsers(nil)
		n.head.setValues(n.path, [maxTreeColumn]any{
			columnName: vcIcon + ch.Name,
		})
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
