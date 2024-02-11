package messages

import (
	"context"
	"fmt"
	"html"
	"log"
	"sort"
	"time"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/sendpart"
	"github.com/diamondburned/chatkit/components/author"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/autoscroll"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gtkcord4/internal/components/hoverpopover"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/messages/composer"
	"github.com/pkg/errors"
)

type messageRow struct {
	*gtk.ListBoxRow
	message Message
	info    messageInfo
}

type messageInfo struct {
	id        discord.MessageID
	author    messageAuthor
	timestamp discord.Timestamp
}

func newMessageInfo(msg *discord.Message) messageInfo {
	return messageInfo{
		id:        msg.ID,
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
	focused gtk.Widgetter

	LoadMore *gtk.Button
	Scroll   *autoscroll.Window
	List     *gtk.ListBox
	Composer *composer.View

	msgs    map[messageKey]messageRow
	chName  string
	guildID discord.GuildID

	summaries map[discord.Snowflake]messageSummaryWidget

	state struct {
		row      *gtk.ListBoxRow
		editing  bool
		replying bool
	}

	ctx  context.Context
	chID discord.ChannelID
}

var viewCSS = cssutil.Applier("message-view", `
	.message-list {
		background: none;
	}
	.message-list > row {
		transition: linear 150ms background-color;
		box-shadow: none;
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
		background-color: alpha(@theme_fg_color, 0.075);
	}
	.message-list > row.message-editing,
	.message-list > row.message-replying {
		background-color: alpha(@theme_selected_bg_color, 0.15);
		border-color: alpha(@theme_selected_bg_color, 0.55);
	}
	.message-list > row.message-sending {
		opacity: 0.65;
	}
	.message-list > row.message-first-prepended {
		border-bottom: 1.5px dashed alpha(@theme_fg_color, 0.25);
		padding-bottom: 2.5px;
	}
	.message-show-more {
		background: none;
		border-radius: 0;
		font-size: 0.85em;
		opacity: 0.65;
	}
	.message-show-more:hover {
		background: alpha(@theme_fg_color, 0.075);
	}
`)

const (
	loadMoreBatch = 25 // load this many more messages on scroll
	initialBatch  = 15 // load this many messages on startup
	idealMaxCount = 50 // ideally keep this many messages in the view
)

func applyViewClamp(clamp *adw.Clamp) {
	clamp.SetMaximumSize(messagesWidth.Value())
	// Set tightening threshold to 90% of the clamp's width.
	clamp.SetTighteningThreshold(int(float64(messagesWidth.Value()) * 0.9))
}

// NewView creates a new View widget associated with the given channel ID. All
// methods call on it will act on that channel.
func NewView(ctx context.Context, chID discord.ChannelID) *View {
	v := &View{
		msgs: make(map[messageKey]messageRow),
		chID: chID,
		ctx:  ctx,
	}

	v.LoadMore = gtk.NewButton()
	v.LoadMore.AddCSSClass("message-show-more")
	v.LoadMore.SetLabel(locale.Get("Show More"))
	v.LoadMore.SetHExpand(true)
	v.LoadMore.SetSensitive(true)
	v.LoadMore.ConnectClicked(v.loadMore)

	v.List = gtk.NewListBox()
	v.List.AddCSSClass("message-list")
	v.List.SetSelectionMode(gtk.SelectionNone)

	clampBox := gtk.NewBox(gtk.OrientationVertical, 0)
	clampBox.SetHExpand(true)
	clampBox.SetVExpand(true)
	clampBox.Append(v.LoadMore)
	clampBox.Append(v.List)

	// Require 2 clamps, one inside the scroll view and another outside the
	// scroll view. This way, the scrollbars will be on the far right rather
	// than being stuck in the middle.
	clampScroll := adw.NewClamp()
	clampScroll.SetChild(clampBox)
	applyViewClamp(clampScroll)

	v.Scroll = autoscroll.NewWindow()
	v.Scroll.AddCSSClass("message-scroll")
	v.Scroll.SetVExpand(true)
	v.Scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	v.Scroll.SetPropagateNaturalWidth(true)
	v.Scroll.SetPropagateNaturalHeight(true)
	v.Scroll.SetChild(clampScroll)

	v.Scroll.OnBottomed(v.onScrollBottomed)

	scrollAdjustment := v.Scroll.VAdjustment()
	scrollAdjustment.ConnectValueChanged(func() {
		// Replicate adw.ToolbarView's behavior: if the user scrolls up, then
		// show a small drop shadow at the bottom of the view. We're not using
		// the actual widget, because it adds a WindowHandle at the bottom,
		// which breaks double-clicking.
		const undershootClass = "undershoot-bottom"

		value := scrollAdjustment.Value()
		upper := scrollAdjustment.Upper()
		psize := scrollAdjustment.PageSize()
		if value < upper-psize {
			v.Scroll.AddCSSClass(undershootClass)
		} else {
			v.Scroll.RemoveCSSClass(undershootClass)
		}
	})

	vp := v.Scroll.Viewport()
	vp.SetScrollToFocus(true)
	v.List.SetAdjustment(v.Scroll.VAdjustment())

	v.Composer = composer.NewView(ctx, v, chID)
	gtkutil.ForwardTyping(v.List, v.Composer.Input)

	composerClamp := adw.NewClamp()
	composerClamp.SetChild(v.Composer)
	applyViewClamp(composerClamp)

	outerBox := gtk.NewBox(gtk.OrientationVertical, 0)
	outerBox.SetHExpand(true)
	outerBox.SetVExpand(true)
	outerBox.Append(v.Scroll)
	outerBox.Append(composerClamp)

	// This becomes the outermost widget.
	v.focused = outerBox

	v.LoadablePage = adaptive.NewLoadablePage()
	v.LoadablePage.SetTransitionDuration(125)
	v.setPageToMain()

	// If the window gains focus, try to carefully mark the channel as read.
	var windowSignal glib.SignalHandle
	v.ConnectMap(func() {
		window := app.GTKWindowFromContext(ctx)
		windowSignal = window.NotifyProperty("is-active", func() {
			if v.IsActive() {
				v.MarkRead()
			}
		})
	})
	// Immediately disconnect the signal when the widget is unmapped.
	// This should prevent v from being referenced forever.
	v.ConnectUnmap(func() {
		window := app.GTKWindowFromContext(ctx)
		window.HandlerDisconnect(windowSignal)
		windowSignal = 0
	})

	state := gtkcord.FromContext(v.ctx)
	if ch, err := state.Cabinet.Channel(v.chID); err == nil {
		v.chName = ch.Name
		v.guildID = ch.GuildID
	}

	state.BindWidget(v, func(ev gateway.Event) {
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
					delete(v.msgs, key)

					key = messageKeyID(ev.ID)
					// Known sent message. Update this instead.
					v.msgs[key] = msg

					msg.ListBoxRow.SetName(string(key))
					msg.message.Update(ev)
					return
				}
			}

			if !v.ignoreMessage(&ev.Message) {
				msg := v.upsertMessage(ev.ID, newMessageInfo(&ev.Message), 0)
				msg.Update(ev)
			}

		case *gateway.MessageUpdateEvent:
			if ev.ChannelID != v.chID {
				return
			}

			m, err := state.Cabinet.Message(ev.ChannelID, ev.ID)
			if err == nil && !v.ignoreMessage(&ev.Message) {
				msg := v.upsertMessage(ev.ID, newMessageInfo(m), 0)
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

		case *gateway.ConversationSummaryUpdateEvent:
			if ev.ChannelID != v.chID {
				return
			}

			v.updateSummaries(ev.Summaries)
		}
	})

	v.load()

	viewCSS(v)
	return v
}

// HeaderButtons returns the header buttons widget for the message view.
// This widget is kept on the header bar for as long as the message view is
// active.
func (v *View) HeaderButtons() []gtk.Widgetter {
	var buttons []gtk.Widgetter

	if v.guildID.IsValid() {
		summariesButton := hoverpopover.NewPopoverButton(v.initSummariesPopover)
		summariesButton.SetIconName("speaker-notes-symbolic")
		summariesButton.SetTooltipText(locale.Get("Message Summaries"))
		buttons = append(buttons, summariesButton)

		state := gtkcord.FromContext(v.ctx)
		if len(state.SummaryState.Summaries(v.chID)) == 0 {
			summariesButton.SetSensitive(false)
			var unbind func()
			unbind = state.AddHandlerForWidget(summariesButton, func(ev *gateway.ConversationSummaryUpdateEvent) {
				if ev.ChannelID == v.chID && len(ev.Summaries) > 0 {
					summariesButton.SetSensitive(true)
					unbind()
				}
			})
		}

		infoButton := hoverpopover.NewPopoverButton(func(popover *gtk.Popover) bool {
			popover.AddCSSClass("message-channel-info-popover")
			popover.SetPosition(gtk.PosBottom)

			label := gtk.NewLabel("")
			label.AddCSSClass("popover-label")
			popover.SetChild(label)

			state := gtkcord.FromContext(v.ctx)
			ch, _ := state.Offline().Channel(v.chID)
			if ch == nil {
				label.SetText(locale.Get("Channel information unavailable."))
				return true
			}

			markup := fmt.Sprintf(
				`<b>%s</b>`,
				html.EscapeString(ch.Name))

			if ch.NSFW {
				markup += fmt.Sprintf(
					"\n<i><small>%s</small></i>",
					locale.Get("This channel is NSFW."))
			}

			if ch.Topic != "" {
				markup += fmt.Sprintf(
					"\n<small>%s</small>",
					html.EscapeString(ch.Topic))
			} else {
				markup += fmt.Sprintf(
					"\n<i><small>%s</small></i>",
					locale.Get("No topic set."))
			}

			label.SetSizeRequest(100, -1)
			label.SetMaxWidthChars(100)
			label.SetWrap(true)
			label.SetWrapMode(pango.WrapWordChar)
			label.SetJustify(gtk.JustifyLeft)
			label.SetXAlign(0)
			label.SetMarkup(markup)
			return true
		})
		infoButton.SetIconName("dialog-information-symbolic")
		infoButton.SetTooltipText(locale.Get("Channel Info"))
		buttons = append(buttons, infoButton)
	}

	return buttons
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

// messageSummaries returns the message summaries for the channel that the
// message view is displaying for. If showSummaries is false, then nil is
// returned.
func (v *View) messageSummaries() map[discord.MessageID]gateway.ConversationSummary {
	if !showSummaries.Value() {
		return nil
	}

	state := gtkcord.FromContext(v.ctx)
	summaries := state.SummaryState.Summaries(v.chID)
	if len(summaries) == 0 {
		return nil
	}

	summariesMap := make(map[discord.MessageID]gateway.ConversationSummary, len(summaries))
	for _, summary := range summaries {
		summariesMap[summary.EndID] = summary
	}

	return summariesMap
}

func (v *View) load() {
	log.Println("loading message view for", v.chID)

	v.LoadablePage.SetLoading()
	v.unload()

	state := gtkcord.FromContext(v.ctx).Online()

	gtkutil.Async(v.ctx, func() func() {
		msgs, err := state.Messages(v.chID, 15)
		if err != nil {
			return func() {
				v.LoadablePage.SetError(err)
			}
		}

		sort.Slice(msgs, func(i, j int) bool {
			return msgs[i].ID < msgs[j].ID
		})

		return func() {
			v.setPageToMain()
			v.Scroll.ScrollToBottom()

			summariesMap := v.messageSummaries()

			for _, msg := range v.filterIgnoredMessages(msgs) {
				w := v.upsertMessage(msg.ID, newMessageInfo(&msg), 0)
				w.Update(&gateway.MessageCreateEvent{Message: msg})
				if summary, ok := summariesMap[msg.ID]; ok {
					v.appendSummary(summary)
				}
			}
		}
	})
}

func (v *View) loadMore() {
	firstRow, ok := v.firstMessage()
	if !ok {
		return
	}

	firstID := firstRow.info.id

	log.Println("loading more messages for", v.chID)

	ctx := v.ctx
	state := gtkcord.FromContext(ctx).Online()

	prevScrollVal := v.Scroll.VAdjustment().Value()
	prevScrollMax := v.Scroll.VAdjustment().Upper()

	upsertMessages := func(msgs []discord.Message) {
		msgs = v.filterIgnoredMessages(msgs)

		infos := make([]messageInfo, len(msgs))
		for i := range msgs {
			infos[i] = newMessageInfo(&msgs[i])
		}

		summariesMap := v.messageSummaries()

		for i, msg := range msgs {
			flags := 0 |
				upsertFlagOverrideCollapse |
				upsertFlagPrepend

			// Manually prepend our own messages. This also requires us to
			// manually check if we should collapse the message.
			if i != len(msgs)-1 {
				curr := infos[i]
				last := infos[i+1]
				if shouldBeCollapsed(curr, last) {
					flags |= upsertFlagCollapsed
				}
			}

			// These messages are prepended, so we insert the "end summary" of
			// the message before it.
			if summary, ok := summariesMap[msg.ID]; ok {
				v.appendSummary(summary)
			}

			w := v.upsertMessage(msg.ID, infos[i], flags)
			w.Update(&gateway.MessageCreateEvent{Message: msg})
		}

		// Do this on the next iteration of the main loop so that the scroll
		// adjustment has time to update.
		glib.IdleAdd(func() {
			// Calculate the offset at which to scroll to after loading more
			// messages.
			currentScrollMax := v.Scroll.VAdjustment().Upper()

			vadj := v.Scroll.VAdjustment()
			vadj.SetValue(prevScrollVal + (currentScrollMax - prevScrollMax))
		})

		// Style the first prepended message to add a visual indicator for the
		// user.
		first := v.msgs[messageKeyID(msgs[0].ID)]
		first.ListBoxRow.AddCSSClass("message-first-prepended")

		// Remove this visual indicator after a short while.
		glib.TimeoutSecondsAdd(10, func() {
			first.ListBoxRow.RemoveCSSClass("message-first-prepended")
		})
	}

	stateMessages, err := state.Cabinet.Messages(v.chID)
	if err == nil && len(stateMessages) > 0 {
		// State messages are ordered last first, so we can traverse them.
		var found bool
		for i, m := range stateMessages {
			if m.ID < firstID {
				log.Println("found earlier message in state, content:", m.Content)
				stateMessages = stateMessages[i:]
				found = true
				break
			}
		}

		if found {
			if len(stateMessages) > loadMoreBatch {
				stateMessages = stateMessages[:loadMoreBatch]
			}
			upsertMessages(stateMessages)
			return
		}
	}

	gtkutil.Async(ctx, func() func() {
		messages, err := state.MessagesBefore(v.chID, firstID, loadMoreBatch)
		if err != nil {
			app.Error(ctx, fmt.Errorf("failed to load more messages: %w", err))
			return nil
		}

		return func() {
			if len(messages) > 0 {
				upsertMessages(messages)
			}

			if len(messages) < loadMoreBatch {
				// We've reached the end of the channel's history.
				// Disable the load more button.
				v.LoadMore.SetSensitive(false)
			}
		}
	})
}

func (v *View) setPageToMain() {
	v.LoadablePage.SetChild(v.focused)
}

func (v *View) unload() {
	for k, msg := range v.msgs {
		v.List.Remove(msg)
		delete(v.msgs, k)
	}
}

// filterIgnoredMessages filters in-place the given messages, removing any
// messages that should be ignored.
func (v *View) filterIgnoredMessages(msgs []discord.Message) []discord.Message {
	if showBlockedMessages.Value() {
		return msgs // doesn't matter
	}

	filtered := msgs[:0]
	for i := range msgs {
		if !v.ignoreMessage(&msgs[i]) {
			filtered = append(filtered, msgs[i])
		}
	}
	return filtered
}

func (v *View) ignoreMessage(msg *discord.Message) bool {
	state := gtkcord.FromContext(v.ctx)

	if !showBlockedMessages.Value() && state.UserIsBlocked(msg.Author.ID) {
		log.Println("ignoring message from blocked user", msg.Author.Tag())
		return true
	}

	return false
}

type upsertFlags int

const (
	upsertFlagCollapsed upsertFlags = 1 << iota
	upsertFlagOverrideCollapse
	upsertFlagPrepend
)

// upsertMessage inserts or updates a new message row.
// TODO: move boolean args to flags.
func (v *View) upsertMessage(id discord.MessageID, info messageInfo, flags upsertFlags) Message {
	if flags&upsertFlagOverrideCollapse == 0 && v.shouldBeCollapsed(info) {
		flags |= upsertFlagCollapsed
	}
	return v.upsertMessageKeyed(messageKeyID(id), info, flags)
}

// upsertMessageKeyed inserts or updates a new message row with the given key.
func (v *View) upsertMessageKeyed(key messageKey, info messageInfo, flags upsertFlags) Message {
	if msg, ok := v.msgs[key]; ok {
		return msg.message
	}

	msg := v.createMessageKeyed(key, info, flags)
	v.msgs[key] = msg

	if flags&upsertFlagPrepend != 0 {
		v.List.Prepend(msg.ListBoxRow)
	} else {
		v.List.Append(msg.ListBoxRow)
	}

	v.List.SetFocusChild(msg.ListBoxRow)
	return msg.message
}

func (v *View) createMessageKeyed(key messageKey, info messageInfo, flags upsertFlags) messageRow {
	var message Message
	if flags&upsertFlagCollapsed != 0 {
		message = NewCollapsedMessage(v.ctx, v)
	} else {
		message = NewCozyMessage(v.ctx, v)
	}

	row := gtk.NewListBoxRow()
	row.AddCSSClass("message-row")
	row.SetName(string(key))
	row.SetChild(message)

	return messageRow{
		ListBoxRow: row,
		message:    message,
		info:       info,
	}
}

// resetMessage resets the message with the given messageRow.
// Its main point is to re-evaluate the collapsed state of the message.
func (v *View) resetMessage(key messageKey) {
	row, ok := v.msgs[key]
	if !ok || row.message == nil {
		return
	}

	var message Message
	if v.shouldBeCollapsed(row.info) {
		message = NewCollapsedMessage(v.ctx, v)
	} else {
		message = NewCozyMessage(v.ctx, v)
	}

	message.Update(&gateway.MessageCreateEvent{
		Message: *row.message.Message(),
	})

	row.message = message
	row.ListBoxRow.SetChild(message)

	v.msgs[key] = row
}

// surroundingMessagesResetter creates a function that resets the messages
// surrounding the given message.
func (v *View) surroundingMessagesResetter(key messageKey) func() {
	msg, ok := v.msgs[key]
	if !ok {
		return func() {}
	}

	// Just be really safe.
	resets := make([]func(), 0, 2)
	if key, ok := v.nextMessageKey(msg); ok {
		resets = append(resets, func() { v.resetMessage(key) })
	}
	if key, ok := v.prevMessageKey(msg); ok {
		resets = append(resets, func() { v.resetMessage(key) })
	}

	return func() {
		for _, reset := range resets {
			reset()
		}
	}
}

func (v *View) deleteMessage(id discord.MessageID) {
	key := messageKeyID(id)
	v.deleteMessageKeyed(key)
}

func (v *View) deleteMessageKeyed(key messageKey) {
	msg, ok := v.msgs[key]
	if !ok {
		return
	}

	if redactMessages.Value() && msg.message != nil {
		msg.message.Redact()
		return
	}

	reset := v.surroundingMessagesResetter(key)
	defer reset()

	v.List.Remove(msg)
	delete(v.msgs, key)
}

func (v *View) shouldBeCollapsed(info messageInfo) bool {
	var last messageRow
	var lastOK bool
	if curr, ok := v.msgs[messageKeyID(info.id)]; ok {
		prev, ok := v.prevMessageKey(curr)
		if ok {
			last, lastOK = v.msgs[prev]
		}
	} else {
		// Assume we're about to append a new message.
		last, lastOK = v.lastMessage()
	}
	if !lastOK || last.message == nil {
		return false
	}
	return shouldBeCollapsed(info, last.info)
}

func shouldBeCollapsed(curr, last messageInfo) bool {
	return true &&
		// same author
		last.author == curr.author &&
		// within the last 10 minutes
		last.timestamp.Time().Add(10*time.Minute).After(curr.timestamp.Time())
}

func (v *View) nextMessageKey(row messageRow) (messageKey, bool) {
	next, _ := row.NextSibling().(*gtk.ListBoxRow)
	if next != nil {
		return messageKeyRow(next), true
	}
	return "", false
}

func (v *View) prevMessageKey(row messageRow) (messageKey, bool) {
	prev, _ := row.PrevSibling().(*gtk.ListBoxRow)
	if prev != nil {
		return messageKeyRow(prev), true
	}
	return "", false
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
	state := gtkcord.FromContext(v.ctx)
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

func (v *View) firstMessage() (messageRow, bool) {
	row, _ := v.List.FirstChild().(*gtk.ListBoxRow)
	if row != nil {
		msg, ok := v.msgs[messageKeyRow(row)]
		return msg, ok
	}

	return messageRow{}, false
}

// eachMessage iterates over each message in the view, starting from the bottom.
// If the callback returns true, the loop will break.
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
	if !ok || widget.message == nil {
		return
	}

	state := gtkcord.FromContext(v.ctx)

	msg, _ := state.Cabinet.Message(v.chID, id)
	if msg == nil {
		return
	}

	content := widget.message.Content()
	content.SetReactions(msg.Reactions)
}

// SendMessage implements composer.Controller.
func (v *View) SendMessage(msg composer.SendingMessage) {
	state := gtkcord.FromContext(v.ctx)

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

	var flags upsertFlags
	if v.shouldBeCollapsed(info) {
		flags |= upsertFlagCollapsed
	}

	key := messageKeyLocal()
	row := v.upsertMessageKeyed(key, info, flags)

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

	uploading := newUploadingLabel(v.ctx, len(msg.Files))
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
			AllowedMentions: &api.AllowedMentions{
				RepliedUser: &msg.ReplyMention,
				Parse: []api.AllowedMentionType{
					api.AllowUserMention,
					api.AllowRoleMention,
					api.AllowEveryoneMention,
				},
			},
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

		state := state.Online()
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
	msg, ok := v.msgs[messageKeyID(id)]
	if !ok {
		log.Println("cannot scroll to message", messageKeyID(id))
		return false
	}

	position := msg.ListBoxRow.Index()
	v.List.ActivateAction("list.scroll-to-item", glib.NewVariantUint32(uint32(position)))

	log.Println("scrolled to message", id)
	return true
}

// AddReaction adds an reaction to the message with the given ID.
func (v *View) AddReaction(id discord.MessageID, emoji discord.APIEmoji) {
	state := gtkcord.FromContext(v.ctx)

	emoji = discord.APIEmoji(gtkcord.SanitizeEmoji(string(emoji)))

	go func() {
		if error := state.React(v.chID, id, emoji); error != nil {
			app.Error(state.Context(), errors.Wrap(error, "Failed to react:"))
		}
	}()
}

// ReplyTo starts replying to the message with the given ID.
func (v *View) ReplyTo(id discord.MessageID) {
	v.stopEditingOrReplying()

	msg, ok := v.msgs[messageKeyID(id)]
	if !ok || msg.message == nil || msg.message.Message() == nil {
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
	if !ok || msg.message == nil || msg.message.Message() == nil {
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

// Delete deletes the message with the given ID. It may prompt the user to
// confirm the deletion.
func (v *View) Delete(id discord.MessageID) {
	if !askBeforeDelete.Value() {
		v.delete(id)
		return
	}

	user := "?" // juuust in case

	row, ok := v.msgs[messageKeyID(id)]
	if ok {
		message := row.message.Message()
		state := gtkcord.FromContext(v.ctx)
		user = state.AuthorMarkup(&gateway.MessageCreateEvent{Message: *message},
			author.WithMinimal())
		user = "<b>" + user + "</b>"
	}

	window := app.GTKWindowFromContext(v.ctx)
	dialog := adw.NewMessageDialog(window,
		locale.Get("Delete Message"),
		locale.Sprintf("Are you sure you want to delete %s's message?", user))
	dialog.SetBodyUseMarkup(true)
	dialog.AddResponse("cancel", locale.Get("_Cancel"))
	dialog.AddResponse("delete", locale.Get("_Delete"))
	dialog.SetResponseAppearance("delete", adw.ResponseDestructive)
	dialog.SetDefaultResponse("cancel")
	dialog.SetCloseResponse("cancel")
	dialog.ConnectResponse(func(response string) {
		switch response {
		case "delete":
			v.delete(id)
		}
	})
	dialog.Show()
}

func (v *View) delete(id discord.MessageID) {
	if msg, ok := v.msgs[messageKeyID(id)]; ok {
		// Visual indicator.
		msg.SetSensitive(false)
	}

	state := gtkcord.FromContext(v.ctx)
	go func() {
		// This is a fairly important operation, so ensure it goes through even
		// if the user switches away.
		state = state.WithContext(context.Background())

		if err := state.DeleteMessage(v.chID, id, ""); err != nil {
			app.Error(v.ctx, errors.Wrap(err, "cannot delete message"))
		}
	}()
}

func (v *View) onScrollBottomed() {
	if v.IsActive() {
		v.MarkRead()
	}

	// Try to clean up the top messages.
	// Fast path: check our cache first.
	if len(v.msgs) > idealMaxCount {
		var count int

		row, _ := v.List.LastChild().(*gtk.ListBoxRow)
		for row != nil {
			next, _ := row.PrevSibling().(*gtk.ListBoxRow)

			if count < idealMaxCount {
				count++
			} else {
				// Start purging messages.
				v.List.Remove(row)
				delete(v.msgs, messageKeyRow(row))
			}

			row = next
		}
	}
}

// MarkRead marks the view's latest messages as read.
func (v *View) MarkRead() {
	state := gtkcord.FromContext(v.ctx)
	// Grab the last message from the state cache, since we sometimes don't even
	// render blocked messages.
	msgs, _ := state.Cabinet.Messages(v.ChannelID())
	if len(msgs) == 0 {
		return
	}

	state.ReadState.MarkRead(v.ChannelID(), msgs[0].ID)

	readState := state.ReadState.ReadState(v.ChannelID())
	if readState != nil {
		log.Println("message.View.MarkRead: marked", msgs[0].ID, "as read, last read", readState.LastMessageID)
	}
}

// IsActive returns true if View is active and visible. This implies that the
// window is focused.
func (v *View) IsActive() bool {
	win := app.GTKWindowFromContext(v.ctx)
	return win.IsActive() && v.Mapped()
}
