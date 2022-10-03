package command

import (
	"context"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/chatkit/components/autocomplete"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gtkcord4/internal/gtkcord"
	"github.com/diamondburned/gtkcord4/internal/icons"
	"github.com/sahilm/fuzzy"
)

type commandSearcher struct {
	cmds     map[string]Command
	cmdNames []string

	cmdBuf  []Command
	dataBuf []autocomplete.Data
}

func newCommandSearcher(cmds map[string]Command) autocomplete.Searcher {
	cmdNames := make([]string, 0, len(cmds))
	for name := range cmds {
		cmdNames = append(cmdNames, name)
	}

	return commandSearcher{
		cmds:     cmds,
		cmdNames: cmdNames,

		cmdBuf:  make([]Command, 0, len(cmds)),
		dataBuf: make([]autocomplete.Data, 0, len(cmds)),
	}
}

func (c commandSearcher) Rune() rune { return '/' }

func (c commandSearcher) Search(ctx context.Context, str string) []autocomplete.Data {
	iters := autocomplete.IterDataFromContext(ctx)
	// Only autocomplete if we're at the start of the line.
	if iters.Start.Offset() != 0 {
		return nil
	}

	if str == "" {
		return c.cmdNameToData(c.cmdNames)
	}

	cmdBuf := c.cmdBuf[:0]
	for _, match := range fuzzy.Find(str, c.cmdNames) {
		command := c.cmds[match.Str]
		cmdBuf = append(cmdBuf, command)
	}

	return c.cmdToData(cmdBuf)
}

func (c commandSearcher) cmdNameToData(cmdNames []string) []autocomplete.Data {
	cmdBuf := c.cmdBuf[:0]
	for _, name := range cmdNames {
		cmdBuf = append(cmdBuf, c.cmds[name])
	}

	return c.cmdToData(cmdBuf)
}

func (c commandSearcher) cmdToData(cmdBuf []Command) []autocomplete.Data {
	dataBuf := c.dataBuf[:0]
	for _, cmd := range cmdBuf {
		dataBuf = append(dataBuf, autocompletedCommandData{
			cmd,
		})
	}

	return dataBuf
}

type autocompletedCommandData struct {
	cmd Command
}

var autocompletedCommandCSS = cssutil.Applier("command-autocompleted-row", `
	.command-autocompleted-row > box {
		margin-right: 0.5em;
	}
	.command-autocompleted-name {
		font-weight: bold;
	}
	.command-autocompleted-desc {
		font-size: 0.85em;
	}
	.command-autocompleted-guildname {
		font-size: 0.85em;
	}
`)

const guildIconSize = 28

var guildIconProviders = imgutil.NewProviders(
	imgutil.HTTPProvider,
	icons.Provider,
)

func (d autocompletedCommandData) Row(ctx context.Context) *gtk.ListBoxRow {
	botIcon := onlineimage.NewAvatar(ctx, guildIconProviders, guildIconSize)

	if botCmd, ok := d.cmd.(*BotCommand); ok {
		state := gtkcord.FromContext(ctx)

		member, err := state.Cabinet.Member(botCmd.GuildID, botCmd.BotID)
		if err != nil {
			botIcon.SetInitials("??")
			botIcon.SetTooltipText("Unknown bot")
		} else {
			guildUser := discord.GuildUser{
				Member: member,
				User:   member.User,
			}
			botIcon.SetTooltipMarkup(state.MemberMarkup(botCmd.GuildID, &guildUser))
			botIcon.SetFromURL(member.User.AvatarURL())
			botIcon.SetInitials(member.User.Username)
		}
	} else {
		botIcon.SetFromURL("icon://logo.png")
		botIcon.SetTooltipText("gtkcord4")
	}

	name := gtk.NewLabel("/" + d.cmd.Name())
	name.AddCSSClass("command-autocompleted-name")
	name.SetEllipsize(pango.EllipsizeEnd)
	name.SetXAlign(0)

	desc := gtk.NewLabel(d.cmd.Desc())
	desc.AddCSSClass("command-autocompleted-desc")
	desc.SetTooltipText(d.cmd.Desc())
	desc.SetEllipsize(pango.EllipsizeEnd)
	desc.SetXAlign(0)

	left := gtk.NewBox(gtk.OrientationVertical, 0)
	left.SetHExpand(true)
	left.Append(name)
	left.Append(desc)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(left)
	box.Append(botIcon)

	row := gtk.NewListBoxRow()
	row.SetChild(box)
	autocompletedCommandCSS(row)

	return row
}

type argumentsAutocompleter struct {
	args []Argument
	buf  []autocomplete.Data
	done []bool
	opts argumentsAutocompleterOpts
}

type argumentsAutocompleterOpts struct {
	inhibitors []func() bool
}

func newArgumentsAutocompleter(args []Argument, opts argumentsAutocompleterOpts) *argumentsAutocompleter {
	return &argumentsAutocompleter{
		args: args,
		buf:  make([]autocomplete.Data, 0, len(args)),
		done: make([]bool, len(args)),
		opts: opts,
	}
}

func (a *argumentsAutocompleter) setDone(doneArg Argument, done bool) {
	for i, arg := range a.args {
		if arg == doneArg {
			a.done[i] = done
		}
	}
}

func (a *argumentsAutocompleter) Rune() rune {
	return autocomplete.WhitespaceRune
}

func (a *argumentsAutocompleter) Search(ctx context.Context, str string) []autocomplete.Data {
	for _, inhibits := range a.opts.inhibitors {
		if inhibits() {
			return nil
		}
	}

	matches := a.buf[:0]
	for i, arg := range a.args {
		if !a.done[i] && strings.HasPrefix(arg.Name(), str) {
			matches = append(matches, autocompletedArgData{arg})
		}
	}

	return matches
}

type autocompletedArgData struct {
	arg Argument
}

var autocompletedArgCommandCSS = cssutil.Applier("command-autocompleted-arg-row", `
	.command-autocompleted-arg-name {
		font-weight: bold;
	}
	.command-autocompleted-arg-desc {
		font-size: 0.85em;
	}
`)

func (a autocompletedArgData) Row(ctx context.Context) *gtk.ListBoxRow {
	name := gtk.NewLabel(a.arg.Name())
	name.AddCSSClass("command-autocompleted-arg-name")
	name.SetEllipsize(pango.EllipsizeEnd)
	name.SetXAlign(0)

	desc := gtk.NewLabel(a.arg.Desc())
	desc.AddCSSClass("command-autocompleted-arg-desc")
	desc.SetTooltipText(a.arg.Desc())
	desc.SetEllipsize(pango.EllipsizeEnd)
	desc.SetXAlign(0)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.SetHExpand(true)
	box.Append(name)
	box.Append(desc)

	row := gtk.NewListBoxRow()
	row.SetChild(box)
	autocompletedArgCommandCSS(row)

	return row
}
