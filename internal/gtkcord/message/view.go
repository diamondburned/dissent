package message

import (
	"context"
	"log"
	"sort"
	"time"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/sendpart"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/components/autoscroll"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/gtkcord/message/composer"
	"github.com/pkg/errors"
)

type messageRow struct {
	*gtk.ListBoxRow
	message Message
	info    messageInfo
}

type messageInfo struct {
	author    messageAuthor
	timestamp discord.Timestamp
}

func newMessageInfo(msg *discord.Message) messageInfo {
	return messageInfo{
		author:    newMessageAuthor(&msg.Author),
		timestamp: msg.Timestamp,
	}
}

type messageAuthor struct {
	userID  discord.UserID
	userTag string
}

func newMessageAuthor(author *discord.User) messageAuthor {
	return messageAuthor{
		userID:  author.ID,
		userTag: author.Tag(),
	}
}

// View is a message view widget.
type View struct {
	*adaptive.LoadablePage
	Box      *gtk.Box
	Scroll   *autoscroll.Window
	List     *gtk.ListBox
	Composer *composer.View

	msgs    map[messageKey]messageRow
	chName  string
	guildID discord.GuildID

	state struct {
		row      *gtk.ListBoxRow
		editing  bool
		replying bool
	}

	ctx  gtkutil.Canceller
	chID discord.ChannelID
}

var viewCSS = cssutil.Applier("message-view", `
	.message-list {
		background: none;
	}
	.message-list > row {
		transition: linear 150ms background-color;
		background: none;
		background-image: none;
		background-color: transparent;
		padding: 0;
		border: 2px solid transparent;
	}
	.message-list > row:focus,
	.message-list > row:hover {
		transition: none;
	}
	.message-list > row:focus {
		background-color: alpha(@theme_fg_color, 0.125);
	}
	.message-list > row:hover {
		background-color: alpha(@theme_fg_color, 0.05);
	}
	.message-list > row.message-editing,
	.message-list > row.message-replying {
		background-color: alpha(@theme_selected_bg_color, 0.15);
		border-color: alpha(@theme_selected_bg_color, 0.55);
	}
	.message-list > row.message-sending {
		opacity: 0.65;
	}
`)

// NewView creates a new View widget associated with the given channel ID. All
// methods call on it will act on that channel.
func NewView(ctx context.Context, chID discord.ChannelID) *View {
	v := &View{
		msgs: make(map[messageKey]messageRow),
		chID: chID,
	}

	v.List = gtk.NewListBox()
	v.List.AddCSSClass("message-list")
	v.List.SetSelectionMode(gtk.SelectionNone)

	v.Scroll = autoscroll.NewWindow()
	v.Scroll.AddCSSClass("message-scroll")
	v.Scroll.ScrollToBottom()
	v.Scroll.SetVExpand(true)
	v.Scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	v.Scroll.SetPropagateNaturalWidth(true)
	v.Scroll.SetPropagateNaturalHeight(true)
	v.Scroll.OnBottomed(v.onScrollBottomed)

	vp := v.Scroll.Viewport()
	vp.SetChild(v.List)
	vp.SetFocusChild(v.List)
	vp.SetScrollToFocus(true)

	v.Scroll.SetFocusChild(vp)
	v.List.SetAdjustment(v.Scroll.VAdjustment())

	v.Composer = composer.NewView(ctx, v, chID)
	gtkutil.ForwardTyping(v.List, v.Composer.Input)

	v.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	v.Box.Append(v.Scroll)
	v.Box.Append(v.Composer)
	v.Box.SetFocusChild(v.Composer)

	v.LoadablePage = adaptive.NewLoadablePage()
	v.LoadablePage.SetChild(v.Box)

	v.ctx = gtkutil.WithVisibility(ctx, v)

	state := gtkcord.FromContext(v.ctx.Take())
	if ch, err := state.Cabinet.Channel(v.chID); err == nil {
		v.chName = ch.Name
		v.guildID = ch.GuildID
	}

	state.BindHandler(v.ctx, func(ev gateway.Event) {
		switch ev := ev.(type) {
		case *gateway.MessageCreateEvent:
			if ev.ChannelID != v.chID {
				return
			}

			// Use this to update existing messages' members as well.
			if ev.Member != nil {
				v.updateMember(ev.Member)
			}

			if ev.Nonce != "" {
				// Try and look up the nonce.
				key := messageKeyNonce(ev.Nonce)

				if msg, ok := v.msgs[key]; ok {
					key = messageKeyID(ev.ID)

					// Known sent message. Update this instead.
					delete(v.msgs, key)
					v.msgs[key] = msg

					msg.ListBoxRow.SetName(string(key))
					msg.message.Update(ev)
					return
				}
			}

			// See if this message belongs to a blocked user. If it is, then
			// don't add it.
			if !showBlockedMessages.Value() && state.UserIsBlocked(ev.Author.ID) {
				log.Println("ignoring message from blocked user", ev.Author.Tag())
				return
			}

			msg := v.upsertMessage(ev.ID, newMessageInfo(&ev.Message))
			msg.Update(ev)

		case *gateway.MessageUpdateEvent:
			if ev.ChannelID != v.chID {
				return
			}

			m, err := state.Cabinet.Message(ev.ChannelID, ev.ID)
			if err == nil {
				msg := v.upsertMessage(ev.ID, newMessageInfo(m))
				msg.Update(&gateway.MessageCreateEvent{
					Message: *m,
					Member:  ev.Member,
				})
			}

		case *gateway.MessageDeleteEvent:
			if ev.ChannelID != v.chID {
				return
			}

			v.deleteMessage(ev.ID)

		case *gateway.MessageReactionAddEvent:
			if ev.ChannelID != v.chID {
				return
			}
			v.updateMessageReactions(ev.MessageID)

		case *gateway.MessageReactionRemoveEvent:
			if ev.ChannelID != v.chID {
				return
			}
			v.updateMessageReactions(ev.MessageID)

		case *gateway.MessageReactionRemoveAllEvent:
			if ev.ChannelID != v.chID {
				return
			}
			v.updateMessageReactions(ev.MessageID)

		case *gateway.MessageReactionRemoveEmojiEvent:
			if ev.ChannelID != v.chID {
				return
			}
			v.updateMessageReactions(ev.MessageID)

		case *gateway.MessageDeleteBulkEvent:
			if ev.ChannelID != v.chID {
				return
			}

			for _, id := range ev.IDs {
				v.deleteMessage(id)
			}

		case *gateway.GuildMemberAddEvent:
			log.Println("TODO: handle GuildMemberAddEvent")

		case *gateway.GuildMemberUpdateEvent:
			if ev.GuildID != v.guildID {
				return
			}

			member, _ := state.Cabinet.Member(ev.GuildID, ev.User.ID)
			if member != nil {
				v.updateMember(member)
			}

		case *gateway.GuildMemberRemoveEvent:
			log.Println("TODO: handle GuildMemberDeleteEvent")

		case *gateway.GuildMembersChunkEvent:
			// TODO: Discord isn't sending us this event. I'm not sure why.
			// Their client has to work somehow. Maybe they use the right-side
			// member list?
			if ev.GuildID != v.guildID {
				return
			}

			for i := range ev.Members {
				v.updateMember(&ev.Members[i])
			}
		}
	})

	v.ctx.OnRenew(func(ctx context.Context) func() {
		w := app.GTKWindowFromContext(ctx)
		h := w.NotifyProperty("is-active", v.onScrollBottomed)
		return func() { w.HandlerDisconnect(h) }
	})

	viewCSS(v)
	return v
}

// GuildID returns the guild ID of the channel that the message view is
// displaying for.
func (v *View) GuildID() discord.GuildID {
	return v.guildID
}

// ChannelID returns the channel ID of the message view.
func (v *View) ChannelID() discord.ChannelID {
	return v.chID
}

// ChannelName returns the name of the channel that the message view is
// displaying for.
func (v *View) ChannelName() string {
	return v.chName
}

func (v *View) Load() {
	v.ctx.Renew()
	v.LoadablePage.SetLoading()

	state := gtkcord.FromContext(v.ctx.Take())

	gtkutil.Async(v.ctx.Take(), func() func() {
		msgs, err := state.Messages(v.chID, 45)
		if err != nil {
			return func() {
				v.LoadablePage.SetError(err)
			}
		}

		sort.Slice(msgs, func(i, j int) bool {
			return msgs[i].ID < msgs[j].ID
		})

		return func() {
			v.LoadablePage.SetChild(v.Box)

			widgets := make([]Message, len(msgs))
			for i, msg := range msgs {
				if !showBlockedMessages.Value() && state.UserIsBlocked(msg.Author.ID) {
					log.Println("ignoring message from blocked user", msg.Author.Tag())
					continue
				}
				widgets[i] = v.upsertMessage(msg.ID, newMessageInfo(&msgs[i]))
			}

			// Render the messages from latest to oldest.
			for i := len(widgets) - 1; i >= 0; i-- {
				if widgets[i] != nil {
					widgets[i].Update(&gateway.MessageCreateEvent{Message: msgs[i]})
				}
			}
		}
	})
}

// upsertMessage inserts or updates a new message row.
func (v *View) upsertMessage(id discord.MessageID, info messageInfo) Message {
	return v.upsertMessageKeyed(messageKeyID(id), info, v.shouldBeCollapsed(info))
}

// upsertMessageKeyed inserts or updates a new message row with the given key.
func (v *View) upsertMessageKeyed(key messageKey, info messageInfo, collapsed bool) Message {
	if msg, ok := v.msgs[key]; ok {
		return msg.message
	}

	var message Message
	if collapsed {
		message = NewCollapsedMessage(v.ctx.Take(), v)
	} else {
		message = NewCozyMessage(v.ctx.Take(), v)
	}

	row := gtk.NewListBoxRow()
	row.AddCSSClass("message-row")
	row.SetName(string(key))
	row.SetChild(message)

	v.List.Append(row)
	v.List.SetFocusChild(row)

	v.msgs[key] = messageRow{
		ListBoxRow: row,
		message:    message,
		info:       info,
	}

	return message
}

func (v *View) deleteMessage(id discord.MessageID) {
	key := messageKeyID(id)

	msg, ok := v.msgs[key]
	if ok {
		msg.message.Redact()

		// delete(v.msgs, key)
		// v.List.Remove(msg)
	}
}

func (v *View) shouldBeCollapsed(info messageInfo) bool {
	last, ok := v.lastMessage()
	return ok &&
		// same author
		last.info.author == info.author &&
		// within the last 10 minutes
		last.info.timestamp.Time().Add(10*time.Minute).After(info.timestamp.Time())
}

func (v *View) lastMessage() (messageRow, bool) {
	row, _ := v.List.LastChild().(*gtk.ListBoxRow)
	if row != nil {
		msg, ok := v.msgs[messageKeyRow(row)]
		return msg, ok
	}

	return messageRow{}, false
}

func (v *View) lastUserMessage() Message {
	state := gtkcord.FromContext(v.ctx.Take())
	me, _ := state.Me()
	if me == nil {
		return nil
	}

	var msg Message
	v.eachMessageFromUser(me.ID, func(row messageRow) bool {
		msg = row.message
		return true
	})

	return msg
}

func (v *View) eachMessage(f func(messageRow) bool) {
	row, _ := v.List.LastChild().(*gtk.ListBoxRow)
	for row != nil {
		key := messageKey(row.Name())

		m, ok := v.msgs[key]
		if ok {
			if f(m) {
				break
			}
		}

		// This repeats until index is -1, at which the loop will break.
		row, _ = row.PrevSibling().(*gtk.ListBoxRow)
	}
}

func (v *View) eachMessageFromUser(id discord.UserID, f func(messageRow) bool) {
	v.eachMessage(func(row messageRow) bool {
		if row.info.author.userID == id {
			return f(row)
		}
		return false
	})
}

func (v *View) updateMember(member *discord.Member) {
	v.eachMessageFromUser(member.User.ID, func(msg messageRow) bool {
		m, ok := msg.message.(MessageWithUser)
		if ok {
			m.UpdateMember(member)
		}
		return false // keep looping
	})
}

func (v *View) updateMessageReactions(id discord.MessageID) {
	widget, ok := v.msgs[messageKeyID(id)]
	if !ok {
		return
	}

	state := gtkcord.FromContext(v.ctx.Take())

	msg, _ := state.Cabinet.Message(v.chID, id)
	if msg == nil {
		return
	}

	content := widget.message.Content()
	content.SetReactions(msg.Reactions)
}

// SendMessage implements composer.Controller.
func (v *View) SendMessage(msg composer.SendingMessage) {
	state := gtkcord.FromContext(v.ctx.Take())

	me, _ := state.Cabinet.Me()
	if me == nil {
		// Risk of leaking Files is too high. Just explode. This realistically
		// never happens anyway.
		panic("missing state.Cabinet.Me")
	}

	info := messageInfo{
		author:    newMessageAuthor(me),
		timestamp: discord.Timestamp(time.Now()),
	}

	key := messageKeyLocal()
	row := v.upsertMessageKeyed(key, info, v.shouldBeCollapsed(info))

	m := discord.Message{
		ChannelID: v.chID,
		GuildID:   v.guildID,
		Content:   msg.Content,
		Timestamp: discord.NowTimestamp(),
		Author:    *me,
	}

	if msg.ReplyingTo.IsValid() {
		m.Reference = &discord.MessageReference{
			ChannelID: v.chID,
			GuildID:   v.guildID,
			MessageID: msg.ReplyingTo,
		}
	}

	gtk.BaseWidget(row).AddCSSClass("message-sending")
	row.Update(&gateway.MessageCreateEvent{Message: m})

	uploading := newUploadingLabel(v.ctx.Take(), len(msg.Files))
	uploading.SetVisible(len(msg.Files) > 0)

	content := row.Content()
	content.Update(&m, uploading)

	// Use the Background context so things keep getting updated when we switch
	// away.
	gtkutil.Async(context.Background(), func() func() {
		sendData := api.SendMessageData{
			Content:   m.Content,
			Reference: m.Reference,
			Nonce:     key.Nonce(),
		}

		// Ensure that we open ALL files and defer-close them. Otherwise, we'll
		// leak files.
		for _, file := range msg.Files {
			f, err := file.Open()
			if err != nil {
				glib.IdleAdd(func() { uploading.AppendError(err) })
				continue
			}

			// This defer executes once we return (like all defers do).
			defer f.Close()

			sendData.Files = append(sendData.Files, sendpart.File{
				Name:   file.Name,
				Reader: wrappedReader{f, uploading},
			})
		}

		_, err := state.SendMessageComplex(m.ChannelID, sendData)

		return func() {
			gtk.BaseWidget(row).RemoveCSSClass("message-sending")

			if err != nil {
				uploading.AppendError(err)
			}

			// We'll let the gateway echo back our own event that's identified
			// using the nonce.
			uploading.SetVisible(uploading.HasErrored())
		}
	})
}

// ScrollToMessage scrolls to the message with the given ID. If the ID is
// unknown, then false is returned.
func (v *View) ScrollToMessage(id discord.MessageID) bool {
	v.Scroll.GrabFocus()

	msg, ok := v.msgs[messageKeyID(id)]
	if !ok {
		log.Println("cannot scroll to message", messageKeyID(id))
		return false
	}

	msg.ListBoxRow.GrabFocus()
	log.Println("scrolled to message", id)
	return true
}

// ReplyTo starts replying to the message with the given ID.
func (v *View) ReplyTo(id discord.MessageID) {
	v.stopEditingOrReplying()

	msg, ok := v.msgs[messageKeyID(id)]
	if !ok || msg.message.Message() == nil {
		return
	}

	v.state.row = msg.ListBoxRow
	v.state.replying = true

	msg.AddCSSClass("message-replying")
	v.Composer.StartReplyingTo(msg.message.Message())
}

// Edit starts editing the message with the given ID.
func (v *View) Edit(id discord.MessageID) {
	v.stopEditingOrReplying()

	msg, ok := v.msgs[messageKeyID(id)]
	if !ok || msg.message.Message() == nil {
		return
	}

	v.state.row = msg.ListBoxRow
	v.state.editing = true

	msg.AddCSSClass("message-editing")
	v.Composer.StartEditing(msg.message.Message())
}

// StopEditing implements composer.Controller.
func (v *View) StopEditing() {
	v.stopEditingOrReplying()
}

// StopReplying implements composer.Controller.
func (v *View) StopReplying() {
	v.stopEditingOrReplying()
}

func (v *View) stopEditingOrReplying() {
	if v.state.row == nil {
		return
	}

	if v.state.editing {
		v.Composer.StopEditing()
		v.state.row.RemoveCSSClass("message-editing")
	}
	if v.state.replying {
		v.Composer.StopReplying()
		v.state.row.RemoveCSSClass("message-replying")
	}
}

// EditLastMessage implements composer.Controller.
func (v *View) EditLastMessage() bool {
	msg := v.lastUserMessage()
	if msg == nil || msg.Message() == nil {
		return false
	}

	v.Edit(msg.Message().ID)
	return true
}

// Delete deletes the message with the given ID.
func (v *View) Delete(id discord.MessageID) {
	if msg, ok := v.msgs[messageKeyID(id)]; ok {
		// Visual indicator.
		msg.SetSensitive(false)
	}

	state := gtkcord.FromContext(v.ctx.Take())
	go func() {
		// This is a fairly important operation, so ensure it goes through even
		// if the user switches away.
		state = state.WithContext(context.Background())

		if err := state.DeleteMessage(v.chID, id, ""); err != nil {
			app.Error(v.ctx.Take(), errors.Wrap(err, "cannot delete message"))
		}
	}()
}

func (v *View) onScrollBottomed() {
	if !v.IsActive() {
		return
	}
	v.MarkRead()
}

// MarkRead marks the view's latest messages as read.
func (v *View) MarkRead() {
	last, ok := v.lastMessage()
	if !ok || last.message.Message() == nil {
		return
	}

	msg := last.message.Message()

	state := gtkcord.FromContext(v.ctx.Take())
	state.ReadState.MarkRead(v.chID, msg.ID)
}

// IsActive returns true if View is active and visible. This implies that the
// window is focused.
func (v *View) IsActive() bool {
	win := app.GTKWindowFromContext(v.ctx.Take())
	return win.IsActive() && v.Mapped()
}
