package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"html"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/chatkit/md/hl"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/ningen/v3"
	"libdb.so/dissent/internal/gtkcord"
)

var _ = cssutil.WriteCSS(`
	.message-box {
		border: 2px solid transparent;
		transition: linear 150ms background-color;
	}
	row:focus .message-box,
	row:hover .message-box {
		transition: none;
	}
	row:focus .message-box {
		background-color: alpha(@theme_fg_color, 0.125);
	}
	row:hover .message-box {
		background-color: alpha(@theme_fg_color, 0.075);
	}
	.message-box.message-editing,
	.message-box.message-replying {
		background-color: alpha(@theme_selected_bg_color, 0.15);
		border-color: alpha(@theme_selected_bg_color, 0.55);
	}
	.message-box.message-sending {
		opacity: 0.65;
	}
	.message-box.message-first-prepended {
		border-bottom: 1.5px dashed alpha(@theme_fg_color, 0.25);
		padding-bottom: 2.5px;
	}
	.message-mentioned {
		border-left: 2px solid @mentioned;
		border-top: 0;
		border-bottom: 0;
		background: alpha(@mentioned, 0.075);
	}
	row:hover .message-mentioned {
		background: alpha(@mentioned, 0.125);
	}
`)

// ExtraMenuSetter is an interface for types that implement SetExtraMenu.
type ExtraMenuSetter interface {
	SetExtraMenu(gio.MenuModeller)
}

// TODO: Implement BlockedMessage widget
// Message describes a Message widget.
type Message interface {
	gtk.Widgetter
	Update(*gateway.MessageCreateEvent)
	Redact()
	Content() *Content
	Message() *discord.Message
	AddCSSClass(string)
	RemoveCSSClass(string)
}

var (
	_ Message = (*cozyMessage)(nil)
	_ Message = (*collapsedMessage)(nil)
)

// MessageWithUser extends Message for types that also show user information.
type MessageWithUser interface {
	Message
	UpdateMember(*discord.Member)
}

var (
	_ MessageWithUser = (*cozyMessage)(nil)
)

var blockedCSS = cssutil.Applier("message-blocked", `
	.message-blocked {
		transition-property: all;
		transition-duration: 100ms;
	}
	.message-blocked:not(:hover) {
		opacity: 0.35;
	}
`)

// message is a base that implements Message.
type message struct {
	content *Content
	message *discord.Message
	menu    *gio.Menu
}

func newMessage(ctx context.Context, v *View) message {
	return message{
		content: NewContent(ctx, v),
	}
}

func (m *message) ctx() context.Context {
	return m.content.ctx
}

// Message implements Message.
func (m *message) Message() *discord.Message {
	return m.message
}

// Content implements Message.
func (m *message) Content() *Content {
	return m.content
}

func (m *message) update(parent gtk.Widgetter, message *discord.Message) {
	parentWidget := gtk.BaseWidget(parent)
	parentWidget.AddCSSClass("message-box")

	m.message = message
	m.bind(parent)
	m.content.Update(message)

	state := gtkcord.FromContext(m.ctx())

	if state.RelationshipState.IsBlocked(message.Author.ID) {
		blockedCSS(parent)

		parentRef := glib.NewWeakRef(parentWidget)
		update := func() {
			parentWidget := parentRef.Get()
			parentWidget.SetVisible(showBlockedMessages.Value())
		}

		unbind := showBlockedMessages.Subscribe(update)
		parentWidget.ConnectDestroy(unbind)
	}

	if state.MessageMentions(message).Has(ningen.MessageMentions) {
		parentWidget.AddCSSClass("message-mentioned")
	} else {
		parentWidget.RemoveCSSClass("message-mentioned")
	}
}

// Redact implements Message.
func (m *message) Redact() {
	m.content.Redact()
}

func (m *message) view() *View {
	return m.content.view
}

func (m *message) bind(parent gtk.Widgetter) *gio.Menu {
	if m.menu != nil {
		return m.menu
	}

	actions := map[string]func(){
		"message.show-source": func() { m.ShowSource() },
		"message.reply":       func() { m.view().ReplyTo(m.message.ID) },
	}

	state := gtkcord.FromContext(m.ctx())
	me, _ := state.Cabinet.Me()
	channel, _ := state.Cabinet.Channel(m.message.ChannelID)

	if me != nil && m.message.Author.ID == me.ID {
		actions["message.edit"] = func() { m.view().Edit(m.message.ID) }
		actions["message.delete"] = func() { m.view().Delete(m.message.ID) }
	}

	if state.Offline().HasPermissions(m.message.ChannelID, discord.PermissionManageMessages) {
		actions["message.delete"] = func() { m.view().Delete(m.message.ID) }
	}

	if channel != nil && (channel.Type == discord.DirectMessage || channel.Type == discord.GroupDM) {
		actions["message.add-reaction"] = func() { m.ShowEmojiChooser() }
	}

	if state.Offline().HasPermissions(m.message.ChannelID, discord.PermissionAddReactions) {
		actions["message.add-reaction"] = func() { m.ShowEmojiChooser() }
	}

	menuItems := []gtkutil.PopoverMenuItem{
		menuItemIfOK(actions, "Add _Reaction", "message.add-reaction"),
		menuItemIfOK(actions, "_Reply", "message.reply"),
		menuItemIfOK(actions, "_Edit", "message.edit"),
		menuItemIfOK(actions, "_Delete", "message.delete"),
		menuItemIfOK(actions, "Show _Source", "message.show-source"),
	}

	gtkutil.BindActionMap(parent, actions)
	gtkutil.BindPopoverMenuCustom(parent, gtk.PosTop, menuItems)

	m.menu = gtkutil.CustomMenu(menuItems)
	m.content.SetExtraMenu(m.menu)

	return m.menu
}

func menuItemIfOK(actions map[string]func(), label locale.Localized, action string) gtkutil.PopoverMenuItem {
	_, ok := actions[action]
	return gtkutil.MenuItem(label, action, ok)
}

var sourceCSS = cssutil.Applier("message-source", `
	.message-source {
		padding: 6px 4px;
		font-family: monospace;
	}
`)

// ShowEmojiChooser opens a Gtk.EmojiChooser popover.
func (m *message) ShowEmojiChooser() {
	e := gtk.NewEmojiChooser()
	e.SetParent(m.content)
	e.SetHasArrow(false)

	e.ConnectEmojiPicked(func(text string) {
		emoji := discord.APIEmoji(text)
		m.view().AddReaction(m.content.msgID, emoji)
	})

	e.Present()
	e.Popup()
}

// ShowSource opens a dialog showing a JSON representation of the message.
func (m *message) ShowSource() {
	d := adw.NewDialog()
	d.SetTitle(locale.Get("View Source"))
	d.SetContentWidth(500)
	d.SetContentHeight(300)

	h := adw.NewHeaderBar()
	h.SetCenteringPolicy(adw.CenteringPolicyStrict)

	toolbarView := adw.NewToolbarView()
	toolbarView.SetTopBarStyle(adw.ToolbarFlat)
	toolbarView.AddTopBar(h)

	buf := gtk.NewTextBuffer(nil)

	if raw, err := json.MarshalIndent(m.message, "", "\t"); err != nil {
		buf.SetText("Error marshaing JSON: " + err.Error())
	} else {
		buf.SetText(string(raw))
		hl.Highlight(m.ctx(), buf.StartIter(), buf.EndIter(), "json")
	}

	t := gtk.NewTextViewWithBuffer(buf)
	t.SetEditable(false)
	t.SetCursorVisible(false)
	t.SetWrapMode(gtk.WrapWordChar)
	sourceCSS(t)
	textutil.SetTabSize(t)

	s := gtk.NewScrolledWindow()
	s.SetVExpand(true)
	s.SetHExpand(true)
	s.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	s.SetChild(t)

	copyBtn := gtk.NewButtonFromIconName("edit-copy-symbolic")
	copyBtn.SetTooltipText(locale.Get("Copy JSON"))
	copyBtn.ConnectClicked(func() {
		clipboard := m.view().Clipboard()
		sourceText := buf.Text(buf.StartIter(), buf.EndIter(), false)
		clipboard.SetText(sourceText)
	})
	h.PackStart(copyBtn)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(h)
	box.Append(s)

	toolbarView.SetContent(box)

	d.SetChild(toolbarView)
	d.Present(app.GTKWindowFromContext(m.ctx()))
}

// cozyMessage is a large cozy message with an avatar.
type cozyMessage struct {
	*gtk.Box
	Avatar   *onlineimage.Avatar
	RightBox *gtk.Box
	TopLabel *gtk.Label

	message
	tooltip string // markup
}

var _ MessageWithUser = (*cozyMessage)(nil)

var cozyCSS = cssutil.Applier("message-cozy", `
	.message-cozy {
		padding-top: 0.25em;
		padding-bottom: 0.15em;
	}
	.message-cozy-header {
		min-height: 1.75em;
		margin-top: 2px;
		font-size: 0.95em;
	}
	.message-cozy-avatar {
		padding: 0 8px;
	}
`)

// NewCozyMessage creates a new cozy message.
func NewCozyMessage(ctx context.Context, v *View) Message {
	m := cozyMessage{
		message: newMessage(ctx, v),
	}

	m.TopLabel = gtk.NewLabel("")
	m.TopLabel.AddCSSClass("message-cozy-header")
	m.TopLabel.SetXAlign(0)
	m.TopLabel.SetEllipsize(pango.EllipsizeEnd)
	m.TopLabel.SetSingleLineMode(true)

	m.RightBox = gtk.NewBox(gtk.OrientationVertical, 0)
	m.RightBox.SetHExpand(true)
	m.RightBox.Append(m.TopLabel)
	m.RightBox.Append(m.message.content)

	m.Avatar = onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.MessageAvatarSize)
	m.Avatar.AddCSSClass("message-cozy-avatar")
	m.Avatar.SetVAlign(gtk.AlignStart)
	m.Avatar.EnableAnimation().OnHover()

	m.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	m.Box.Append(m.Avatar)
	m.Box.Append(m.RightBox)

	cozyCSS(m)
	return &m
}

func (m *cozyMessage) Update(message *gateway.MessageCreateEvent) {
	m.message.update(m, &message.Message)
	m.updateAuthor(message)

	tooltip := fmt.Sprintf(
		"<b>%s</b>\n%s",
		html.EscapeString(message.Author.Tag()),
		html.EscapeString(locale.Time(message.Timestamp.Time(), true)),
	)

	// TODO: query tooltip
	m.Avatar.SetTooltipMarkup(tooltip)
	m.TopLabel.SetTooltipMarkup(tooltip)
}

func (m *cozyMessage) UpdateMember(member *discord.Member) {
	if m.message.message == nil {
		return
	}

	m.updateAuthor(&gateway.MessageCreateEvent{
		Message: *m.message.message,
		Member:  member,
	})
}

func (m *cozyMessage) updateAuthor(message *gateway.MessageCreateEvent) {
	var avatarURL string
	if message.Member != nil && message.Member.Avatar != "" {
		avatarURL = message.Member.AvatarURL(message.GuildID)
	} else {
		avatarURL = message.Author.AvatarURL()
	}
	m.Avatar.SetFromURL(gtkcord.InjectAvatarSize(avatarURL))

	state := gtkcord.FromContext(m.ctx())

	markup := "<b>" + state.AuthorMarkup(message) + "</b>"
	markup += ` <span alpha="75%" size="small">` +
		locale.TimeAgo(message.Timestamp.Time()) +
		"</span>"

	m.TopLabel.SetMarkup(markup)
}

// collapsedMessage is a collapsed cozy message.
type collapsedMessage struct {
	*gtk.Box
	Timestamp *gtk.Label

	message
}

var _ Message = (*collapsedMessage)(nil)

var collapsedCSS = cssutil.Applier("message-collapsed", `
	.message-collapsed {
		padding-bottom: 0.15em;
	}
	.message-collapsed-timestamp {
		opacity: 0;
		font-size: 0.7em;
		min-height: calc(1em + 0.7rem);
	}
	.message-row:hover .message-collapsed-timestamp {
		opacity: 1;
		color: alpha(@theme_fg_color, 0.75);
	}
`)

const collapsedTimestampWidth = (8 * 2) + (gtkcord.MessageAvatarSize)

// NewCollapsedMessage creates a new collapsed cozy message.
func NewCollapsedMessage(ctx context.Context, v *View) Message {
	m := collapsedMessage{
		message: newMessage(ctx, v),
	}

	m.Timestamp = gtk.NewLabel("")
	m.Timestamp.AddCSSClass("message-collapsed-timestamp")
	m.Timestamp.SetSizeRequest(collapsedTimestampWidth, -1)

	// This widget will not ellipsize properly, so we're forced to wrap.
	m.Timestamp.SetWrap(true)
	m.Timestamp.SetWrapMode(pango.WrapWordChar)
	m.Timestamp.SetNaturalWrapMode(gtk.NaturalWrapWord)

	m.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	m.Box.Append(m.Timestamp)
	m.Box.Append(m.message.content)

	collapsedCSS(m)
	return &m
}

func (m *collapsedMessage) Update(message *gateway.MessageCreateEvent) {
	m.message.update(m, &message.Message)

	// view := m.view()

	var timestampLabel string

	switch collapsedMessageTimestamp.Value() {
	case compactTimestampStyle:
		timestampLabel = locale.Time(message.Timestamp.Time(), false)
		// case relativeTimestampStyle:
		// 	prevKey, _ := view.prevMessageKeyFromID(message.Message.ID)
		// 	prev, ok := view.rows[prevKey]
		// 	if ok {
		// 		prevMsg := prev.message.Message()
		// 		if prevMsg != nil {
		// 			currTimestamp := message.Timestamp.Time()
		// 			prevTimestamp := prevMsg.Timestamp.Time()
		//
		// 			delta := currTimestamp.Sub(prevTimestamp)
		// 			switch {
		// 			case delta < time.Second:
		// 				// leave empty
		// 			case delta < time.Minute:
		// 				timestampLabel = "+" + locale.Sprintf("%ds", int(math.Round(delta.Seconds())))
		// 			default:
		// 				// This is always at most 10 minutes.
		// 				timestampLabel = "+" + locale.Sprintf("%dm", int(math.Round(delta.Minutes())))
		// 			}
		// 		}
		// 	}
	}

	m.Timestamp.SetLabel(timestampLabel)
	m.Timestamp.SetTooltipText(locale.Time(message.Timestamp.Time(), true))
}
