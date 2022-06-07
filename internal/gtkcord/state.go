package gtkcord

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/diamondburned/arikawa/v3/utils/ws"
	"github.com/diamondburned/chatkit/components/author"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/ningen/v3"
	"github.com/diamondburned/ningen/v3/discordmd"
)

func init() {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "PC"
	}

	api.UserAgent = "gtkcord4 (https://github.com/diamondburned/arikawa/v3)"
	gateway.DefaultIdentity = gateway.IdentifyProperties{
		OS:      runtime.GOOS,
		Device:  hostname,
		Browser: "gtkcord4",
	}
}

type ctxKey uint8

const (
	_ ctxKey = iota
	stateKey
)

// State extends the Discord state controller.
type State struct {
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

func init() {
	ws.EnableRawEvents = true
}

// Wrap wraps the given state.
func Wrap(state *state.State) *State {
	state.Client.OnRequest = append(state.Client.OnRequest, func(r httpdriver.Request) error {
		req := (*http.Request)(r.(*httpdriver.DefaultRequest))
		log.Println("Discord API:", req.Method, req.URL.Path)
		return nil
	})

	/*
		dir := filepath.Join(os.TempDir(), "gtkcord4-events")
		os.RemoveAll(dir)

		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			log.Println("cannot mkdir -p for ev logginf:", err)
		}

		var atom uint64
		state.AddHandler(func(ev *ws.RawEvent) {
			id := atomic.AddUint64(&atom, 1)

			f, err := os.Create(filepath.Join(
				dir,
				fmt.Sprintf("%05d-%d-%s.json", id, ev.OriginalCode, ev.OriginalType),
			))
			if err != nil {
				log.Println("cannot log op:", err)
				return
			}
			defer f.Close()

			if _, err := f.Write(ev.Raw); err != nil {
				log.Println("event json error:", err)
			}
		})
	*/

	return &State{
		State: ningen.FromState(state),
	}
}

// InjectState injects the given state to a new context.
func InjectState(ctx context.Context, state *State) context.Context {
	return context.WithValue(ctx, stateKey, state)
}

// WithContext creates a copy of State with a new context.
func (s *State) WithContext(ctx context.Context) *State {
	return &State{
		State: s.State.WithContext(ctx),
	}
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
			glib.IdleAdd(func() { fn(ev) })
		})
	})
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

// MemberMarkup is like AuthorMarkup but for any member inside a guild.
func (s *State) MemberMarkup(gID discord.GuildID, u *discord.GuildUser, mods ...author.MarkupMod) string {
	name := u.Username
	var suffix string

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

		c, ok := state.MemberColor(u.Member, func(id discord.RoleID) *discord.Role {
			role, _ := s.Cabinet.Role(gID, id)
			return role
		})
		if ok {
			mods = append(mods, author.WithColor(c.String()))
		}
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
		mods = append(mods, author.WithSuffixMarkup(suffix))
	}

	return author.Markup(name, mods...)
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

// https://math.stackexchange.com/a/291494/963524
func roundSize(size int) int {
	const mult = 16
	return ((size - 1) | (mult - 1)) + 1
}

// EmojiURL returns a sized emoji URL.
func EmojiURL(emojiID string, gif bool) string {
	return InjectSize(discordmd.EmojiURL(emojiID, gif), 64)
}

// ChannelNameFromID returns the channel's name in plain text from the channel
// with the given ID.
func ChannelNameFromID(ctx context.Context, id discord.ChannelID) string {
	state := FromContext(ctx)
	ch, _ := state.Cabinet.Channel(id)
	if ch != nil {
		return ChannelName(ctx, ch)
	}
	return "Unknown channel"
}

// ChannelName returns the channel's name in plain text.
func ChannelName(ctx context.Context, ch *discord.Channel) string {
	switch ch.Type {
	case discord.DirectMessage:
		if len(ch.DMRecipients) == 0 {
			return recipientNames(ctx, ch)
		}
		return ch.DMRecipients[0].Username
	case discord.GroupDM:
		if ch.Name != "" {
			return ch.Name
		}
		return recipientNames(ctx, ch)
	case discord.GuildPublicThread, discord.GuildPrivateThread:
		return ch.Name
	default:
		return "#" + ch.Name
	}
}

func recipientNames(ctx context.Context, ch *discord.Channel) string {
	name := func(ix int) string {
		return ch.DMRecipients[ix].Username
	}

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
			str.WriteString(u.Username)
			str.WriteString(", ")
		}
		str.WriteString(" and ")
		str.WriteString(ch.DMRecipients[len(ch.DMRecipients)-1].Username)
		return str.String()
	}
}
