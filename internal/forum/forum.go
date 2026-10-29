// Package forum implements the forum browser, which lists the posts of a forum
// channel in place of the message view.
package forum

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"html"
	"log/slog"
	"slices"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"libdb.so/dissent/internal/gtkcord"
)

var (
	//go:embed forum.ui
	forumUI string
	//go:embed forum_post.ui
	postUI string
	//go:embed forum.css
	forumCSS string
)

var _ = cssutil.WriteCSS(forumCSS)

// pageSize is the number of archived posts fetched at once.
const pageSize = 25

const thumbnailSize = 88

var threadTypes = []discord.ChannelType{
	discord.GuildPublicThread,
	discord.GuildPrivateThread,
	discord.GuildAnnouncementThread,
}

// View is the forum browser of a single forum channel.
type View struct {
	*adw.ToolbarView
	ctx     context.Context
	channel discord.Channel

	searchButton *gtk.ToggleButton
	searchEntry  *gtk.SearchEntry
	sortDropDown *gtk.DropDown
	postList     *gtk.ListBox
	stack        *gtk.Stack
	errorPage    *adw.StatusPage
	emptyPage    *adw.StatusPage

	posts        map[*gtk.ListBoxRow]*post
	selectedTags map[discord.TagID]bool

	// generation is bumped on every reload, so that results of requests made
	// before the reload are dropped.
	generation int
	offset     int
	hasMore    bool
	loading    bool
}

type post struct {
	thread    discord.Channel
	preview   *gtk.Label
	thumbnail *adw.Clamp
	// searchText is the lowercased title and preview for searching.
	searchText string
}

// NewView creates a new forum browser for the given forum channel.
func NewView(ctx context.Context, chID discord.ChannelID) *View {
	state := gtkcord.FromContext(ctx)

	v := View{
		ctx:          ctx,
		posts:        make(map[*gtk.ListBoxRow]*post),
		selectedTags: make(map[discord.TagID]bool),
	}

	if ch, _ := state.Cabinet.Channel(chID); ch != nil {
		v.channel = *ch
	} else {
		v.channel.ID = chID
	}

	ui := gtk.NewBuilderFromString(forumUI)
	get := func(name string) glib.Objector { return ui.GetObject(name).Cast() }

	v.ToolbarView = get("rootContent").(*adw.ToolbarView)
	v.searchButton = get("searchButton").(*gtk.ToggleButton)
	v.searchEntry = get("searchEntry").(*gtk.SearchEntry)
	v.sortDropDown = get("sortDropDown").(*gtk.DropDown)
	v.postList = get("postList").(*gtk.ListBox)
	v.stack = get("contentStack").(*gtk.Stack)
	v.errorPage = get("errorPage").(*adw.StatusPage)
	v.emptyPage = get("emptyPage").(*adw.StatusPage)

	if !v.isForum() {
		v.searchEntry.SetPlaceholderText(locale.Get("Search threads"))
		v.emptyPage.SetTitle(locale.Get("No Threads Found"))
		v.errorPage.SetTitle(locale.Get("Cannot Load Threads"))
	}

	v.searchButton.ConnectToggled(func() {
		if v.searchButton.Active() {
			v.searchEntry.GrabFocus()
		}
	})
	v.searchEntry.ConnectSearchChanged(v.refilter)

	if v.channel.DefaultSoftOrder != nil && *v.channel.DefaultSoftOrder == discord.SoftOrderTypeCreationDate {
		v.sortDropDown.SetSelected(1)
	}
	v.sortDropDown.NotifyProperty("selected", v.reload)

	tagsBox := get("tagsBox").(*adw.WrapBox)
	for _, tag := range v.channel.AvailableTags {
		button := gtk.NewToggleButtonWithLabel(tagLabel(tag))
		button.AddCSSClass("forum-tag")
		button.ConnectToggled(func() {
			v.selectedTags[tag.ID] = button.Active()
			v.refilter()
		})
		tagsBox.Append(button)
	}
	tagsBox.SetVisible(len(v.channel.AvailableTags) > 0)

	v.postList.SetFilterFunc(v.filter)
	v.postList.SetSortFunc(func(row1, row2 *gtk.ListBoxRow) int {
		return v.comparePosts(v.posts[row1], v.posts[row2])
	})
	v.postList.ConnectRowActivated(func(row *gtk.ListBoxRow) {
		v.openPost(v.posts[row])
	})

	scroll := get("postScroll").(*gtk.ScrolledWindow)
	scroll.ConnectEdgeReached(func(pos gtk.PositionType) {
		if pos == gtk.PosBottom {
			v.loadMore()
		}
	})

	v.reload()
	return &v
}

// ChannelID returns the ID of the forum channel.
func (v *View) ChannelID() discord.ChannelID {
	return v.channel.ID
}

// HeaderButtons returns the buttons to be packed at the end of the header bar.
func (v *View) HeaderButtons() []gtk.Widgetter {
	return nil
}

// HeaderStartButtons returns the buttons to be packed at the start of the
// header bar.
func (v *View) HeaderStartButtons() []gtk.Widgetter {
	return []gtk.Widgetter{v.searchButton}
}

// isForum returns true if the channel is a forum, whose posts start with a
// first message, rather than a channel with threads.
func (v *View) isForum() bool {
	return v.channel.Type == discord.GuildForum
}

func (v *View) sortByCreation() bool {
	return v.sortDropDown.Selected() == 1
}

// reload clears the posts and loads them again: active posts from the state,
// then archived posts from the API.
func (v *View) reload() {
	v.generation++
	v.offset = 0
	v.hasMore = true
	v.loading = false

	for row := range v.posts {
		v.postList.Remove(row)
	}
	clear(v.posts)

	state := gtkcord.FromContext(v.ctx).Offline()
	threads, err := state.Channels(v.channel.GuildID, threadTypes)
	if err != nil {
		slog.Error("cannot get forum threads", "channel_id", v.channel.ID, "err", err)
	}
	for _, thread := range threads {
		if thread.ParentID != v.channel.ID ||
			(thread.ThreadMetadata != nil && thread.ThreadMetadata.Archived) {
			continue
		}
		p := v.addPost(thread)
		if v.isForum() {
			v.fetchFirstMessage(p)
		}
	}

	v.loadMore()
}

// fetchFirstMessage fetches the first message of an active post. The archived
// post search returns them along with the posts.
func (v *View) fetchFirstMessage(p *post) {
	generation := v.generation
	state := gtkcord.FromContext(v.ctx).Online()

	gtkutil.Async(v.ctx, func() func() {
		// The first message of a post has the same ID as the post.
		msg, err := state.Message(p.thread.ID, discord.MessageID(p.thread.ID))
		if err != nil {
			slog.Debug("cannot fetch first message of post", "thread_id", p.thread.ID, "err", err)
			return nil
		}
		return func() {
			if generation == v.generation {
				v.setFirstMessage(p, msg)
				v.postList.InvalidateFilter()
			}
		}
	})
}

// loadMore loads the next page of archived posts.
func (v *View) loadMore() {
	if v.loading || !v.hasMore {
		return
	}
	v.loading = true
	v.updatePage()

	generation := v.generation
	data := api.SearchThreadsData{
		Archived:   true,
		SortBy:     api.LastMessageTime,
		SortOrder:  api.SortDescending,
		TagSetting: api.MatchSome,
		Limit:      pageSize,
		Offset:     v.offset,
	}
	if v.sortByCreation() {
		data.SortBy = api.CreationTime
	}

	state := gtkcord.FromContext(v.ctx).Online()

	gtkutil.Async(v.ctx, func() func() {
		resp, err := state.SearchThreads(v.channel.ID, data)
		return func() {
			if generation != v.generation {
				return
			}
			v.loading = false

			if err != nil {
				slog.Error("cannot search forum posts", "channel_id", v.channel.ID, "err", err)
				v.hasMore = false
				if len(v.posts) == 0 {
					v.errorPage.SetDescription(err.Error())
					v.stack.SetVisibleChildName("error")
				}
				return
			}

			v.offset += len(resp.Threads)
			v.hasMore = resp.HasMore && len(resp.Threads) > 0

			for _, thread := range resp.Threads {
				p := v.addPost(thread)
				for i, msg := range resp.FirstMessages {
					if msg.ChannelID == thread.ID {
						v.setFirstMessage(p, &resp.FirstMessages[i])
						break
					}
				}
			}

			v.refilter()
		}
	})
}

func (v *View) addPost(thread discord.Channel) *post {
	ui := gtk.NewBuilderFromString(postUI)
	get := func(name string) glib.Objector { return ui.GetObject(name).Cast() }

	p := &post{
		thread:     thread,
		preview:    get("preview").(*gtk.Label),
		thumbnail:  get("thumbnailClamp").(*adw.Clamp),
		searchText: strings.ToLower(thread.Name),
	}

	get("title").(*gtk.Label).SetText(thread.Name)
	get("messageCount").(*gtk.Label).SetText(fmt.Sprint(thread.MessageCount))
	get("activity").(*gtk.Label).SetText(locale.TimeAgo(lastActivity(thread).Time()))
	get("pinnedIcon").(*gtk.Image).SetVisible(thread.Flags&discord.PinnedThread != 0)
	get("lockedIcon").(*gtk.Image).SetVisible(thread.ThreadMetadata != nil && thread.ThreadMetadata.Locked)

	tagsBox := get("tags").(*adw.WrapBox)
	for _, tag := range v.channel.AvailableTags {
		if slices.Contains(thread.AppliedTags, tag.ID) {
			label := gtk.NewLabel(tagLabel(tag))
			label.AddCSSClass("forum-tag")
			tagsBox.Append(label)
		}
	}
	tagsBox.SetVisible(tagsBox.FirstChild() != nil)

	row := get("rootContent").(*gtk.ListBoxRow)
	v.posts[row] = p
	v.postList.Append(row)

	return p
}

func (v *View) setFirstMessage(p *post, msg *discord.Message) {
	state := gtkcord.FromContext(v.ctx)
	preview := strings.Join(strings.Fields(state.MessagePreview(msg)), " ")

	p.searchText = strings.ToLower(p.thread.Name + " " + preview)
	p.preview.SetVisible(true)
	p.preview.SetMarkup(fmt.Sprintf(
		`<b>%s:</b> <span alpha="65%%">%s</span>`,
		html.EscapeString(msg.Author.DisplayOrUsername()),
		html.EscapeString(preview)))

	for _, att := range msg.Attachments {
		if !strings.HasPrefix(att.ContentType, "image/") {
			continue
		}

		picture := onlineimage.NewPicture(v.ctx, imgutil.HTTPProvider)
		picture.SetContentFit(gtk.ContentFitCover)
		picture.SetSizeRequest(thumbnailSize, thumbnailSize)
		picture.SetURL(gtkcord.InjectSize(att.Proxy, thumbnailSize))

		p.thumbnail.SetChild(picture)
		p.thumbnail.SetVisible(true)
		break
	}
}

func (v *View) filter(row *gtk.ListBoxRow) bool {
	p := v.posts[row]
	if p == nil {
		return false
	}

	query := strings.ToLower(strings.TrimSpace(v.searchEntry.Text()))
	if query != "" && !strings.Contains(p.searchText, query) {
		return false
	}

	for id, selected := range v.selectedTags {
		if selected && !slices.Contains(p.thread.AppliedTags, id) {
			return false
		}
	}

	return true
}

// comparePosts sorts pinned posts first, then the newest posts first.
func (v *View) comparePosts(a, b *post) int {
	if a == nil || b == nil {
		return 0
	}

	pinned := func(p *post) bool { return p.thread.Flags&discord.PinnedThread != 0 }
	key := lastActivity
	if v.sortByCreation() {
		key = func(ch discord.Channel) discord.Snowflake { return discord.Snowflake(ch.ID) }
	}

	return cmp.Or(
		-compareBool(pinned(a), pinned(b)),
		-cmp.Compare(key(a.thread), key(b.thread)),
	)
}

func (v *View) refilter() {
	v.postList.InvalidateFilter()
	v.updatePage()
}

// updatePage shows the posts, the loading spinner or the empty page.
func (v *View) updatePage() {
	for row := range v.posts {
		// GtkListBox hides filtered rows using child-visible.
		if row.ChildVisible() {
			v.stack.SetVisibleChildName("posts")
			return
		}
	}

	if v.loading {
		v.stack.SetVisibleChildName("loading")
	} else {
		v.stack.SetVisibleChildName("empty")
	}
}

func (v *View) openPost(p *post) {
	// Archived posts aren't in the state, so put them there for the message
	// view to find.
	state := gtkcord.FromContext(v.ctx)
	if ch, _ := state.Cabinet.Channel(p.thread.ID); ch == nil {
		if err := state.Cabinet.ChannelSet(&p.thread, false); err != nil {
			slog.Error("cannot store forum post", "thread_id", p.thread.ID, "err", err)
		}
	}

	v.ActivateAction("win.open-channel", gtkcord.NewChannelIDVariant(p.thread.ID))
}

// lastActivity returns the ID of the last message of the thread, or the
// thread's own ID if it has none.
func lastActivity(thread discord.Channel) discord.Snowflake {
	if thread.LastMessageID.IsValid() {
		return discord.Snowflake(thread.LastMessageID)
	}
	return discord.Snowflake(thread.ID)
}

func tagLabel(tag discord.Tag) string {
	if tag.EmojiName != nil && *tag.EmojiName != "" {
		return *tag.EmojiName + " " + tag.Name
	}
	return tag.Name
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}
