package message

import (
	"context"
	"encoding/json"
	"fmt"
	"html"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/chatkit/md/hl"
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
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
)

// ExtraMenuSetter is an interface for types that implement SetExtraMenu.
type ExtraMenuSetter interface {
	SetExtraMenu(gio.MenuModeller)
}

// Message describes a Message widget.
type Message interface {
	gtk.Widgetter
	Update(*gateway.MessageCreateEvent)
	Redact()
	Content() *Content
	Message() *discord.Message
}

// MessageWithUser extends Message for types that also show user information.
type MessageWithUser interface {
	Message
	UpdateMember(*discord.Member)
}

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
	m.message = message
	m.bind(parent)
	m.content.Update(message)

	state := gtkcord.FromContext(m.ctx())
	if state.RelationshipState.IsBlocked(message.Author.ID) {
		blockedCSS(parent)
	}
}

// Redact implements Message.
func (m *message) Redact() {
	m.content.Redact()
}

func (m *message) parent() *View {
	return m.content.parent
}

func (m *message) bind(parent gtk.Widgetter) *gio.Menu {
	if m.menu != nil {
		return m.menu
	}

	actions := map[string]func(){
		"message.show-source": func() { m.ShowSource() },
		"message.reply":       func() { m.parent().ReplyTo(m.message.ID) },
	}

	state := gtkcord.FromContext(m.ctx())
	me, _ := state.Cabinet.Me()

	if me != nil && m.message.Author.ID == me.ID {
		actions["message.edit"] = func() { m.parent().Edit(m.message.ID) }
		actions["message.delete"] = func() { m.parent().Delete(m.message.ID) }
	}

	if state.HasPermissions(m.message.ChannelID, discord.PermissionManageMessages) {
		actions["message.delete"] = func() { m.parent().Delete(m.message.ID) }
	}

	menuItems := []gtkutil.PopoverMenuItem{
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

func menuItemIfOK(actions map[string]func(), label, action string) gtkutil.PopoverMenuItem {
	_, ok := actions[action]
	return gtkutil.MenuItem(label, action, ok)
}

var sourceCSS = cssutil.Applier("message-source", `
	.message-source {
		padding: 6px 4px;
		font-family: monospace;
	}
`)

// ShowSource opens a JSON showing the message JSON.
func (m *message) ShowSource() {
	d := gtk.NewDialog()
	d.SetTransientFor(app.GTKWindowFromContext(m.ctx()))
	d.SetModal(true)
	d.SetDefaultSize(400, 300)

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

	box := d.ContentArea()
	box.Append(s)

	d.Show()
}

// cozyMessage is a large cozy message with an avatar.
type cozyMessage struct {
	*gtk.Box
	Avatar   *onlineimage.Avatar
	RightBox *gtk.Box
	TopLabel *gtk.Label

	message
}

var _ MessageWithUser = (*cozyMessage)(nil)

var cozyCSS = cssutil.Applier("message-cozy", `
	.message-cozy {
		margin-top: 2px;
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
	m.TopLabel.SetHAlign(gtk.AlignStart)
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
		"<b>%s</b> (%s)\n%s",
		html.EscapeString(message.Author.Tag()), message.Author.ID,
		html.EscapeString(locale.Time(message.Timestamp.Time(), true)),
	)

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
		locale.TimeAgo(m.ctx(), message.Timestamp.Time()) +
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
		margin-bottom: 1px;
	}
	.message-collapsed-timestamp {
		opacity: 0;
		font-size: 0.7em;
		min-width: calc((8px * 2) + {$message_avatar_size});
	}
	.message-row:hover .message-collapsed-timestamp {
		opacity: 1;
		color: alpha(@theme_fg_color, 0.75);
	}
`)

// NewCollapsedMessage creates a new collapsed cozy message.
func NewCollapsedMessage(ctx context.Context, v *View) Message {
	m := collapsedMessage{
		message: newMessage(ctx, v),
	}

	m.Timestamp = gtk.NewLabel("")
	m.Timestamp.AddCSSClass("message-collapsed-timestamp")
	m.Timestamp.SetEllipsize(pango.EllipsizeEnd)

	m.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	m.Box.Append(m.Timestamp)
	m.Box.Append(m.message.content)

	collapsedCSS(m)
	return &m
}

func (m *collapsedMessage) Update(message *gateway.MessageCreateEvent) {
	m.message.update(m, &message.Message)
	m.Timestamp.SetLabel(locale.Time(message.Timestamp.Time(), false))
	m.Timestamp.SetTooltipText(locale.Time(message.Timestamp.Time(), true))
}
