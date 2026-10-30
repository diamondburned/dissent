package messages

import (
	"cmp"
	"context"
	_ "embed"
	"slices"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/locale"
	"libdb.so/dissent/internal/gtkcord"
)

var (
	//go:embed component_select.ui
	componentSelectUI string
	//go:embed component_select_option.ui
	componentSelectOptionUI string
)

// selectSearchThreshold is the number of options above which a select menu
// shows a search entry.
const selectSearchThreshold = 10

// selectWidth is the natural width of a select menu.
const selectWidth = 280

// selectMaxOptions is the maximum number of options shown for user, role and
// channel select menus.
const selectMaxOptions = 250

type selectOption struct {
	value       string
	label       string
	description string
	emoji       *discord.ComponentEmoji
	selected    bool
}

// componentSelect is a select menu. The options of user, role, mentionable
// and channel select menus are taken from the state.
type componentSelect struct {
	*gtk.MenuButton
	summary   *gtk.Label
	indicator *gtk.Stack
	checks    []*gtk.Image

	options     []selectOption
	placeholder string
	limits      [2]int
	required    bool

	// committed are the values that were last sent.
	committed []string
	// onChange is called with the new values once the user closes the menu
	// after changing the selection. It calls done once the change is applied,
	// with an error if it failed. If it is nil, the values are only kept.
	onChange func(values []string, done func(error))
}

func newComponentSelect(ctx context.Context, guildID discord.GuildID, component discord.InteractiveComponent) *componentSelect {
	ui := gtk.NewBuilderFromString(componentSelectUI)
	s := &componentSelect{
		MenuButton: ui.GetObject("rootContent").Cast().(*gtk.MenuButton),
		summary:    ui.GetObject("summary").Cast().(*gtk.Label),
		indicator:  ui.GetObject("indicator").Cast().(*gtk.Stack),
	}
	popover := ui.GetObject("popover").Cast().(*gtk.Popover)
	sizer := ui.GetObject("sizer").Cast().(*gtk.Overlay)
	search := ui.GetObject("search").Cast().(*gtk.SearchEntry)
	hint := ui.GetObject("hint").Cast().(*gtk.Label)
	list := ui.GetObject("options").Cast().(*gtk.ListBox)

	// Prefer being wide without stopping the window from getting narrower.
	width := newSizer(selectWidth, 1)
	sizer.AddOverlay(width)
	sizer.SetMeasureOverlay(width, true)

	state := gtkcord.FromContext(ctx)
	disabled := false

	switch component := component.(type) {
	case *discord.StringSelectComponent:
		for _, o := range component.Options {
			s.options = append(s.options, selectOption{
				value:       o.Value,
				label:       o.Label,
				description: o.Description,
				emoji:       o.Emoji,
				selected:    o.Default,
			})
		}
		s.placeholder = component.Placeholder
		s.limits = component.ValueLimits
		s.required = component.Required
		disabled = component.Disabled

	case *discord.UserSelectComponent:
		s.addMembers(state, guildID)
		selectDefaults(s, component.DefaultUsers)
		s.placeholder = component.Placeholder
		s.limits = component.ValueLimits
		s.required = component.Required
		disabled = component.Disabled

	case *discord.RoleSelectComponent:
		s.addRoles(state, guildID)
		selectDefaults(s, component.DefaultRoles)
		s.placeholder = component.Placeholder
		s.limits = component.ValueLimits
		s.required = component.Required
		disabled = component.Disabled

	case *discord.MentionableSelectComponent:
		s.addMembers(state, guildID)
		s.addRoles(state, guildID)
		s.placeholder = component.Placeholder
		s.limits = component.ValueLimits
		s.required = component.Required
		disabled = component.Disabled

	case *discord.ChannelSelectComponent:
		s.addChannels(state, guildID, component.ChannelTypes)
		selectDefaults(s, component.DefaultChannels)
		s.placeholder = component.Placeholder
		s.limits = component.ValueLimits
		s.required = component.Required
		disabled = component.Disabled
	}

	if s.limits == [2]int{} {
		s.limits = [2]int{1, 1}
	}
	if len(s.options) > 0 {
		s.limits[1] = min(s.limits[1], len(s.options))
	}

	s.SetSensitive(!disabled)
	s.committed = s.Values()

	switch {
	case s.limits[1] <= 1:
		hint.SetVisible(false)
	case s.limits[0] == s.limits[1]:
		hint.SetText(locale.Sprintf("Select %d", s.limits[1]))
	case s.limits[0] <= 1:
		hint.SetText(locale.Sprintf("Select up to %d", s.limits[1]))
	default:
		hint.SetText(locale.Sprintf("Select %d to %d", s.limits[0], s.limits[1]))
	}

	for i := range s.options {
		list.Append(s.newOption(ctx, &s.options[i]))
	}

	list.ConnectRowActivated(func(row *gtk.ListBoxRow) {
		s.toggle(row.Index())
		if s.limits[1] <= 1 {
			popover.Popdown()
		}
	})

	if len(s.options) > selectSearchThreshold {
		search.SetVisible(true)
		search.SetKeyCaptureWidget(popover)
		search.ConnectSearchChanged(list.InvalidateFilter)
		list.SetFilterFunc(func(row *gtk.ListBoxRow) bool {
			query := strings.ToLower(search.Text())
			return strings.Contains(strings.ToLower(s.options[row.Index()].label), query)
		})
	}

	popover.ConnectClosed(s.commit)

	s.update()
	return s
}

func (s *componentSelect) newOption(ctx context.Context, option *selectOption) *gtk.ListBoxRow {
	ui := gtk.NewBuilderFromString(componentSelectOptionUI)
	row := ui.GetObject("rootContent").Cast().(*gtk.ListBoxRow)
	icon := ui.GetObject("icon").Cast().(*gtk.Box)
	label := ui.GetObject("label").Cast().(*gtk.Label)
	description := ui.GetObject("description").Cast().(*gtk.Label)
	check := ui.GetObject("check").Cast().(*gtk.Image)

	setComponentEmoji(ctx, icon, option.emoji)
	label.SetText(option.label)
	description.SetText(option.description)
	description.SetVisible(option.description != "")

	s.checks = append(s.checks, check)
	return row
}

func (s *componentSelect) addMembers(state *gtkcord.State, guildID discord.GuildID) {
	members, _ := state.Cabinet.Members(guildID)
	slices.SortFunc(members, func(a, b discord.Member) int {
		return strings.Compare(memberName(&a), memberName(&b))
	})
	for _, member := range members[:min(len(members), selectMaxOptions)] {
		s.options = append(s.options, selectOption{
			value:       member.User.ID.String(),
			label:       memberName(&member),
			description: member.User.Username,
		})
	}
}

func memberName(member *discord.Member) string {
	if member.Nick != "" {
		return member.Nick
	}
	return member.User.DisplayOrUsername()
}

func (s *componentSelect) addRoles(state *gtkcord.State, guildID discord.GuildID) {
	roles, _ := state.Cabinet.Roles(guildID)
	slices.SortFunc(roles, func(a, b discord.Role) int {
		return cmp.Compare(b.Position, a.Position)
	})
	for _, role := range roles[:min(len(roles), selectMaxOptions)] {
		if role.ID == discord.RoleID(guildID) {
			continue // @everyone
		}
		s.options = append(s.options, selectOption{
			value: role.ID.String(),
			label: "@" + role.Name,
		})
	}
}

func (s *componentSelect) addChannels(state *gtkcord.State, guildID discord.GuildID, types []discord.ChannelType) {
	channels, _ := state.Cabinet.Channels(guildID)
	slices.SortFunc(channels, func(a, b discord.Channel) int {
		return cmp.Compare(a.Position, b.Position)
	})
	for _, channel := range channels {
		if len(types) > 0 && !slices.Contains(types, channel.Type) {
			continue
		}
		if len(types) == 0 && channel.Type == discord.GuildCategory {
			continue
		}
		s.options = append(s.options, selectOption{
			value: channel.ID.String(),
			label: "#" + channel.Name,
		})
		if len(s.options) >= selectMaxOptions {
			break
		}
	}
}

// selectDefaults selects the options with the given IDs.
func selectDefaults[T interface{ String() string }](s *componentSelect, ids []T) {
	for _, id := range ids {
		for i := range s.options {
			if s.options[i].value == id.String() {
				s.options[i].selected = true
			}
		}
	}
}

// Values returns the values of the selected options.
func (s *componentSelect) Values() []string {
	var values []string
	for _, option := range s.options {
		if option.selected {
			values = append(values, option.value)
		}
	}
	return values
}

func (s *componentSelect) toggle(i int) {
	option := &s.options[i]
	switch {
	case s.limits[1] <= 1:
		for j := range s.options {
			s.options[j].selected = j == i
		}
	case option.selected:
		option.selected = false
	case len(s.Values()) < s.limits[1]:
		option.selected = true
	}
	s.update()
}

// commit calls onChange if the selection changed since it was last called.
func (s *componentSelect) commit() {
	values := s.Values()
	if s.onChange == nil || slices.Equal(values, s.committed) || len(values) < s.limits[0] {
		return
	}

	previous := s.committed
	s.committed = values

	s.indicator.SetVisibleChildName("busy")
	s.SetSensitive(false)

	s.onChange(values, func(err error) {
		s.indicator.SetVisibleChildName("arrow")
		s.SetSensitive(true)
		if err != nil {
			s.committed = previous
			for i := range s.options {
				s.options[i].selected = slices.Contains(previous, s.options[i].value)
			}
			s.update()
		}
	})
}

// update shows the selection in the button and the option list.
func (s *componentSelect) update() {
	var labels []string
	for i, option := range s.options {
		s.checks[i].SetVisible(option.selected)
		if option.selected {
			labels = append(labels, option.label)
		}
	}

	if len(labels) > 0 {
		s.summary.SetText(strings.Join(labels, ", "))
		s.summary.RemoveCSSClass("dim-label")
	} else {
		placeholder := s.placeholder
		if placeholder == "" {
			placeholder = locale.Get("Make a selection")
		}
		s.summary.SetText(placeholder)
		s.summary.AddCSSClass("dim-label")
	}
}
