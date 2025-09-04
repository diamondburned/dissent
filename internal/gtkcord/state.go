package gtkcord

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/diamondburned/arikawa/v3/utils/ws"
	"github.com/diamondburned/chatkit/components/author"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/ningen/v3"
	"github.com/diamondburned/ningen/v3/discordmd"
	"libdb.so/dissent/internal/colorhash"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

func init() {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "PC"
	}

	api.UserAgent = "Dissent (https://libdb.so/dissent)"
	gateway.DefaultIdentity = gateway.IdentifyProperties{
		gateway.IdentifyOS:      runtime.GOOS,
		gateway.IdentifyDevice:  "Arikawa",
		gateway.IdentifyBrowser: "Dissent on " + hostname,
	}
}

// AllowedChannelTypes are the channel types that are shown.
var AllowedChannelTypes = []discord.ChannelType{
	discord.GuildText,
	discord.GuildCategory,
	discord.GuildPublicThread,
	discord.GuildPrivateThread,
	discord.GuildForum,
	discord.GuildAnnouncement,
	discord.GuildAnnouncementThread,
	discord.GuildVoice,
	discord.GuildStageVoice,
}

type ctxKey uint8

const (
	_ ctxKey = iota
	stateKey
)

// State extends the Discord state controller.
type State struct {
	*MainThreadHandler
	*ningen.State
}

// FromContext gets the Discord state controller from the given context.
func FromContext(ctx context.Context) *State {
	state, _ := ctx.Value(stateKey).(*State)
	if state != nil {
		return state.WithContext(ctx)
	}
	return nil
}

// Wrap wraps the given state.
func Wrap(state *state.State) *State {
	c := state.Client.Client
	c.OnRequest = append(c.OnRequest, func(r httpdriver.Request) error {
		// req := (*http.Request)(r.(*httpdriver.DefaultRequest))
		// log.Println("Discord API:", req.Method, req.URL.Path)
		return nil
	})
	c.OnResponse = append(c.OnResponse, func(dreq httpdriver.Request, dresp httpdriver.Response) error {
		req := (*http.Request)(dreq.(*httpdriver.DefaultRequest))
		if dresp == nil {
			return nil
		}

		resp := (*http.Response)(dresp.(*httpdriver.DefaultResponse))
		if resp.StatusCode >= 400 {
			slog.Warn(
				"Discord API returned HTTP error",
				"method", req.Method,
				"path", req.URL.Path,
				"status", resp.Status)
		}

		return nil
	})

	state.StateLog = func(err error) {
		slog.Error(
			"unexpected Discord state error occured",
			"err", err)
	}

	if os.Getenv("DISSENT_DEBUG_DUMP_ALL_EVENTS_PLEASE") == "1" {
		dir := filepath.Join(os.TempDir(), "gtkcord4-events")
		slog.Warn(
			"ATTENTION: DISSENT_DEBUG_DUMP_ALL_EVENTS_PLEASE is set to 1, dumping all raw events",
			"dir", dir)
		dumpRawEvents(state, dir)
	}

	ningen := ningen.FromState(state)
	return &State{
		MainThreadHandler: NewMainThreadHandler(ningen.Handler),
		State:             ningen,
	}
}

var rawEventsOnce sync.Once

func dumpRawEvents(state *state.State, dir string) {
	rawEventsOnce.Do(func() {
		ws.EnableRawEvents = true
	})

	os.RemoveAll(dir)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		slog.Error(
			"cannot mkdir -p for debug event logging, not logging events",
			"dir", dir,
			"err", err)
		return
	}

	var atom uint64
	state.AddHandler(func(ev *ws.RawEvent) {
		id := atomic.AddUint64(&atom, 1)

		f, err := os.Create(filepath.Join(
			dir,
			fmt.Sprintf("%05d-%d-%s.json", id, ev.OriginalCode, ev.OriginalType),
		))
		if err != nil {
			slog.Error(
				"cannot create file to log one debug event",
				"event_code", ev.OriginalCode,
				"event_type", ev.OriginalType,
				"err", err)
			return
		}
		defer f.Close()

		if _, err := f.Write(ev.Raw); err != nil {
			slog.Error(
				"cannot write file to log one debug event",
				"event_code", ev.OriginalCode,
				"event_type", ev.OriginalType,
				"err", err)
			return
		}
	})
}

// InjectState injects the given state to a new context.
func InjectState(ctx context.Context, state *State) context.Context {
	return context.WithValue(ctx, stateKey, state)
}

// Offline creates a copy of State with a new offline state.
func (s *State) Offline() *State {
	s2 := *s
	s2.State = s.State.Offline()
	return &s2
}

// Online creates a copy of State with a new online state.
func (s *State) Online() *State {
	s2 := *s
	s2.State = s.State.Online()
	return &s2
}

// WithContext creates a copy of State with a new context.
func (s *State) WithContext(ctx context.Context) *State {
	s2 := *s
	s2.State = s.State.WithContext(ctx)
	return &s2
}

// BindHandler is similar to BindWidgetHandler, except the lifetime of the
// handler is bound to the context.
func (s *State) BindHandler(ctx gtkutil.Cancellable, fn func(gateway.Event), filters ...gateway.Event) {
	eventTypes := make([]reflect.Type, len(filters))
	for i, filter := range filters {
		eventTypes[i] = reflect.TypeOf(filter)
	}
	ctx.OnRenew(func(context.Context) func() {
		return s.AddSyncHandler(func(ev gateway.Event) {
			// Optionally filter out events.
			if len(eventTypes) > 0 {
				evType := reflect.TypeOf(ev)

				for _, typ := range eventTypes {
					if typ == evType {
						goto filtered
					}
				}

				return
			}

		filtered:
			glib.IdleAddPriority(glib.PriorityDefault, func() { fn(ev) })
		})
	})
}

// BindWidget is similar to BindHandler, except it doesn't rely on contexts.
func (s *State) BindWidget(w gtk.Widgetter, fn func(gateway.Event), filters ...gateway.Event) {
	eventTypes := make([]reflect.Type, len(filters))
	for i, filter := range filters {
		eventTypes[i] = reflect.TypeOf(filter)
	}

	ref := coreglib.NewWeakRef(w)

	var unbind func()
	bind := func() {
		if unbind != nil {
			return
		}

		w := ref.Get()
		slog.Debug(
			"binding state handler lifetime to widget",
			"widget_type", fmt.Sprintf("%T", w),
			"event_types", eventTypes)

		unbind = s.AddSyncHandler(func(ev gateway.Event) {
			// Optionally filter out events.
			if len(eventTypes) > 0 {
				evType := reflect.TypeOf(ev)

				for _, typ := range eventTypes {
					if typ == evType {
						goto filtered
					}
				}

				return
			}

		filtered:
			glib.IdleAddPriority(glib.PriorityDefault, func() { fn(ev) })
		})
	}

	bind()

	base := gtk.BaseWidget(w)
	base.NotifyProperty("parent", func() {
		if base.Parent() != nil {
			return
		}

		if unbind != nil {
			unbind()
			unbind = nil

			slog.Debug(
				"widget unparented, unbinded handler",
				"func", "BindWidget",
				"widget_type", gtk.BaseWidget(w).Type())
		}
	})
	base.ConnectDestroy(func() {
		if unbind != nil {
			unbind()
			unbind = nil

			slog.Debug(
				"widget destroyed, unbinded handler",
				"func", "BindWidget",
				"widget_type", gtk.BaseWidget(w).Type())
		}
	})
}

// AddHandler adds a handler to the state. The handler is removed when the
// returned function is called.
func (s *State) AddHandler(fns ...any) func() {
	if len(fns) == 1 {
		return s.MainThreadHandler.AddHandler(fns[0])
	}

	unbinds := make([]func(), 0, len(fns))
	for _, fn := range fns {
		unbind := s.MainThreadHandler.AddHandler(fn)
		unbinds = append(unbinds, unbind)
	}

	return func() {
		for _, unbind := range unbinds {
			unbind()
		}
		unbinds = unbinds[:0]
	}
}

// AddHandlerForWidget replaces BindWidget and provides a way to bind a handler
// that only receives events as long as the widget is mapped. As soon as the
// widget is unmapped, the handler is unbound.
func (s *State) AddHandlerForWidget(w gtk.Widgetter, fns ...any) func() {
	unbinds := make([]func(), 0, len(fns))

	unbind := func() {
		for _, unbind := range unbinds {
			unbind()
		}
		unbinds = unbinds[:0]
	}

	bind := func() {
		for _, fn := range fns {
			unbind := s.AddHandler(fn)
			unbinds = append(unbinds, unbind)
		}
	}

	bind()

	base := gtk.BaseWidget(w)
	base.NotifyProperty("parent", func() {
		unbind()
		if base.Parent() != nil {
			bind()
		} else {
			slog.Debug(
				"widget unparented, unbinding handler",
				"func", "AddHandlerForWidget",
				"widget_type", gtk.BaseWidget(w).Type())
		}
	})

	return unbind
}

// AuthorMarkup renders the markup for the message author's name. It makes no
// API calls.
func (s *State) AuthorMarkup(m *gateway.MessageCreateEvent, mods ...author.MarkupMod) string {
	user := &discord.GuildUser{User: m.Author, Member: m.Member}
	return s.MemberMarkup(m.GuildID, user, mods...)
}

// UserMarkup is like AuthorMarkup but for any user optionally inside a guild.
func (s *State) UserMarkup(gID discord.GuildID, u *discord.User, mods ...author.MarkupMod) string {
	user := &discord.GuildUser{User: *u}
	return s.MemberMarkup(gID, user, mods...)
}

// UserIDMarkup gets the User markup from just the channel and user IDs.
func (s *State) UserIDMarkup(chID discord.ChannelID, uID discord.UserID, mods ...author.MarkupMod) string {
	chs, err := s.Cabinet.Channel(chID)
	if err != nil {
		return html.EscapeString(uID.Mention())
	}

	if chs.GuildID.IsValid() {
		member, err := s.Cabinet.Member(chs.GuildID, uID)
		if err != nil {
			return html.EscapeString(uID.Mention())
		}

		return s.MemberMarkup(chs.GuildID, &discord.GuildUser{
			User:   member.User,
			Member: member,
		}, mods...)
	}

	for _, recipient := range chs.DMRecipients {
		if recipient.ID == uID {
			return s.UserMarkup(0, &recipient)
		}
	}

	return html.EscapeString(uID.Mention())
}

var overrideMemberColors = prefs.NewBool(false, prefs.PropMeta{
	Name:        "Override Member Colors",
	Section:     "Discord",
	Description: "Use generated colors instead of role colors for members.",
})

// MemberMarkup is like AuthorMarkup but for any member inside a guild.
func (s *State) MemberMarkup(gID discord.GuildID, u *discord.GuildUser, mods ...author.MarkupMod) string {
	name := u.DisplayOrUsername()

	var suffix string
	var prefixMods []author.MarkupMod

	if gID.IsValid() {
		if u.Member == nil {
			u.Member, _ = s.Cabinet.Member(gID, u.ID)
		}

		if u.Member == nil {
			s.MemberState.RequestMember(gID, u.ID)
			goto noMember
		}

		if u.Member != nil && u.Member.Nick != "" {
			name = u.Member.Nick
			suffix += fmt.Sprintf(
				` <span weight="normal">(%s)</span>`,
				html.EscapeString(u.Member.User.Tag()),
			)
		}

		if !overrideMemberColors.Value() {
			c, ok := state.MemberColor(u.Member, func(id discord.RoleID) *discord.Role {
				role, _ := s.Cabinet.Role(gID, id)
				return role
			})
			if ok {
				prefixMods = append(prefixMods, author.WithColor(c.String()))
			}
		}
	}

	if overrideMemberColors.Value() {
		prefixMods = append(prefixMods, author.WithColor(hashUserColor(&u.User)))
	}

noMember:
	if u.Bot {
		bot := "bot"
		if u.Discriminator == "0000" {
			bot = "webhook"
		}
		suffix += ` <span color="#6f78db" weight="normal">(` + bot + `)</span>`
	}

	if suffix != "" {
		suffix = strings.TrimSpace(suffix)
		prefixMods = append(prefixMods, author.WithSuffixMarkup(suffix))
	}

	return author.Markup(name, append(prefixMods, mods...)...)
}

func hashUserColor(user *discord.User) string {
	input := user.Tag()
	color := colorhash.DefaultHasher().Hash(input)
	return colorhash.RGBHex(color)
}

// MessagePreview renders the message into a short content string.
func (s *State) MessagePreview(msg *discord.Message) string {
	b := strings.Builder{}
	b.Grow(len(msg.Content))

	src := []byte(msg.Content)
	node := discordmd.ParseWithMessage(src, *s.Cabinet, msg, true)
	discordmd.DefaultRenderer.Render(&b, src, node)

	preview := strings.TrimRight(b.String(), "\n")
	if preview != "" {
		return preview
	}

	if len(msg.Attachments) > 0 {
		for _, attachment := range msg.Attachments {
			preview += fmt.Sprintf("%s, ", attachment.Filename)
		}
		preview = strings.TrimSuffix(preview, ", ")
		return preview
	}

	if len(msg.Embeds) > 0 {
		return "[embed]"
	}

	return ""
}

// InjectAvatarSize calls InjectSize with size being 64px.
func InjectAvatarSize(urlstr string) string {
	return InjectSize(urlstr, 64)
}

// InjectSize injects the size query parameter into the URL. Size is
// automatically scaled up to 2x or more.
func InjectSize(urlstr string, size int) string {
	if urlstr == "" {
		return ""
	}

	if scale := gtkutil.ScaleFactor(); scale > 2 {
		size *= scale
	} else {
		size *= 2
	}

	return InjectSizeUnscaled(urlstr, size)
}

// InjectSizeUnscaled is like InjectSize, except the size is not scaled
// according to the scale factor.
func InjectSizeUnscaled(urlstr string, size int) string {
	// Round size up to the nearest power of 2.
	size = roundSize(size)

	u, err := url.Parse(urlstr)
	if err != nil {
		return urlstr
	}

	q := u.Query()
	q.Set("size", strconv.Itoa(size))
	u.RawQuery = q.Encode()

	return u.String()
}

func roundSize(size int) int {
	// Round size up to the nearest power of 2.
	return int(math.Pow(2, math.Ceil(math.Log2(float64(size)))))
}

// EmojiURL returns a sized emoji URL.
func EmojiURL(emojiID string, gif bool) string {
	return InjectSize(discordmd.EmojiURL(emojiID, gif), 64)
}

// WindowTitleFromID returns the window title from the channel with the given
// ID.
func WindowTitleFromID(ctx context.Context, id discord.ChannelID) string {
	state := FromContext(ctx)
	ch, _ := state.Cabinet.Channel(id)
	if ch == nil {
		return ""
	}

	title := ChannelName(ch)
	if ch.GuildID.IsValid() {
		guild, _ := state.Cabinet.Guild(ch.GuildID)
		if guild != nil {
			title += " - " + guild.Name
		}
	}

	return title
}

// ChannelNameFromID returns the channel's name in plain text from the channel
// with the given ID.
func ChannelNameFromID(ctx context.Context, id discord.ChannelID) string {
	state := FromContext(ctx)
	ch, _ := state.Cabinet.Channel(id)
	return ChannelName(ch)
}

// ChannelName returns the channel's name in plain text.
func ChannelName(ch *discord.Channel) string {
	return channelName(ch, true)
}

// ChannelNameWithoutHash returns the channel's name in plain text without the
// hash.
func ChannelNameWithoutHash(ch *discord.Channel) string {
	return channelName(ch, false)
}

func channelName(ch *discord.Channel, hash bool) string {
	if ch == nil {
		return locale.Get("Unknown channel")
	}
	switch ch.Type {
	case discord.DirectMessage:
		if len(ch.DMRecipients) == 0 {
			return RecipientNames(ch)
		}
		return userName(&ch.DMRecipients[0])
	case discord.GroupDM:
		if ch.Name != "" {
			return ch.Name
		}
		return RecipientNames(ch)
	case discord.GuildPublicThread, discord.GuildPrivateThread:
		return ch.Name
	default:
		if hash {
			return "#" + ch.Name
		}
		return ch.Name
	}
}

// RecipientNames formats the string for the list of recipients inside the given
// channel.
func RecipientNames(ch *discord.Channel) string {
	name := func(ix int) string { return userName(&ch.DMRecipients[ix]) }

	// TODO: localize

	switch len(ch.DMRecipients) {
	case 0:
		return "Empty channel"
	case 1:
		return name(0)
	case 2:
		return name(0) + " and " + name(1)
	default:
		var str strings.Builder
		for _, u := range ch.DMRecipients[:len(ch.DMRecipients)-1] {
			str.WriteString(userName(&u))
			str.WriteString(", ")
		}
		str.WriteString(" and ")
		str.WriteString(userName(&ch.DMRecipients[len(ch.DMRecipients)-1]))
		return str.String()
	}
}

func userName(u *discord.User) string {
	if u.DisplayName == "" {
		return u.Username
	}
	if strings.EqualFold(u.DisplayName, u.Username) {
		return u.DisplayName
	}
	return fmt.Sprintf("%s (%s)", u.DisplayName, u.Username)
}

// SnowflakeVariant is the variant type for a [discord.Snowflake].
var SnowflakeVariant = glib.NewVariantType("x")

// NewSnowflakeVariant creates a new Snowflake variant.
func NewSnowflakeVariant(snowflake discord.Snowflake) *glib.Variant {
	return glib.NewVariantInt64(int64(snowflake))
}

// NewChannelIDVariant creates a new ChannelID variant.
func NewChannelIDVariant(id discord.ChannelID) *glib.Variant {
	return glib.NewVariantInt64(int64(id))
}

// NewGuildIDVariant creates a new GuildID variant.
func NewGuildIDVariant(id discord.GuildID) *glib.Variant {
	return glib.NewVariantInt64(int64(id))
}

// NewMessageIDVariant creates a new MessageID variant.
func NewMessageIDVariant(id discord.MessageID) *glib.Variant {
	return glib.NewVariantInt64(int64(id))
}
