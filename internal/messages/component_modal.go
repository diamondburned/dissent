package messages

import (
	_ "embed"
	"errors"
	"slices"
	"unicode/utf8"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/locale"
)

var (
	//go:embed component_modal.ui
	componentModalUI string
	//go:embed modal_field.ui
	modalFieldUI string
)

// modalInput is an input of a modal.
type modalInput struct {
	widget gtk.Widgetter
	// validate returns an error to show to the user if the input is invalid.
	validate func() error
	// submit returns the submitted component, or nil to leave it out.
	submit func() *api.ModalSubmitComponent
}

// componentModal is a form that an application asks the user to fill in.
type componentModal struct {
	*adw.Dialog
	view   *View
	ev     *gateway.InteractionModalCreateEvent
	toasts *adw.ToastOverlay
	inputs []modalInput
}

func newComponentModal(v *View, ev *gateway.InteractionModalCreateEvent) *componentModal {
	ui := gtk.NewBuilderFromString(componentModalUI)
	m := &componentModal{
		Dialog: ui.GetObject("rootContent").Cast().(*adw.Dialog),
		view:   v,
		ev:     ev,
		toasts: ui.GetObject("toasts").Cast().(*adw.ToastOverlay),
	}
	title := ui.GetObject("title").Cast().(*adw.WindowTitle)
	cancel := ui.GetObject("cancel").Cast().(*gtk.Button)
	submit := ui.GetObject("submit").Cast().(*gtk.Button)
	submitStack := ui.GetObject("submitStack").Cast().(*gtk.Stack)
	fields := ui.GetObject("fields").Cast().(*gtk.Box)

	m.SetTitle(ev.Title)
	title.SetTitle(ev.Title)
	title.SetSubtitle(ev.Application.Name)

	for _, component := range ev.Components {
		switch component := component.(type) {
		case *discord.LabelComponent:
			input := m.input(component.Component)
			fields.Append(newModalField(component.Label, component.Description, input.widget))
			m.inputs = append(m.inputs, modalInput{
				validate: input.validate,
				submit: func() *api.ModalSubmitComponent {
					inner := input.submit()
					if inner == nil {
						return nil
					}
					return &api.ModalSubmitComponent{
						Type:      discord.LabelComponentType,
						Component: inner,
					}
				},
			})

		case *discord.ActionRowComponent:
			// Older modals put each text input into its own action row.
			for _, component := range *component {
				textInput, ok := component.(*discord.TextInputComponent)
				if !ok {
					fields.Append(newUnsupportedComponent(component))
					continue
				}
				input := m.input(textInput)
				fields.Append(newModalField(textInput.Label, "", input.widget))
				m.inputs = append(m.inputs, modalInput{
					validate: input.validate,
					submit: func() *api.ModalSubmitComponent {
						return &api.ModalSubmitComponent{
							Type:       discord.ActionRowComponentType,
							Components: []api.ModalSubmitComponent{*input.submit()},
						}
					},
				})
			}

		case *discord.TextDisplayComponent:
			fields.Append(newComponentText(v.ctx, nil, component.Content))

		default:
			fields.Append(newUnsupportedComponent(component))
		}
	}

	cancel.ConnectClicked(func() { m.Close() })

	submit.ConnectClicked(func() {
		var components []api.ModalSubmitComponent
		for _, input := range m.inputs {
			if err := input.validate(); err != nil {
				m.toast(err.Error())
				return
			}
			if c := input.submit(); c != nil {
				components = append(components, *c)
			}
		}

		submitStack.SetVisibleChildName("busy")
		submit.SetSensitive(false)

		target := interactionTarget{
			AppID:     ev.Application.ID,
			ChannelID: ev.ChannelID,
		}
		data := api.ModalInteractionData{
			ID:         ev.ID,
			CustomID:   ev.CustomID,
			Components: components,
		}

		v.sendInteraction(target, discord.ModalInteractionType, data, func(err error) {
			submitStack.SetVisibleChildName("label")
			submit.SetSensitive(true)

			switch {
			case errors.Is(err, errInteractionFailed):
				m.toast(locale.Get("The application did not respond"))
			case err != nil:
				m.toast(locale.Get("Cannot submit the form"))
			default:
				m.Close()
			}
		})
	})

	return m
}

func (m *componentModal) toast(text string) {
	toast := adw.NewToast(text)
	toast.SetTimeout(5)
	m.toasts.AddToast(toast)
}

func newModalField(label, description string, input gtk.Widgetter) gtk.Widgetter {
	ui := gtk.NewBuilderFromString(modalFieldUI)
	box := ui.GetObject("rootContent").Cast().(*gtk.Box)
	labelWidget := ui.GetObject("label").Cast().(*gtk.Label)
	descriptionWidget := ui.GetObject("description").Cast().(*gtk.Label)
	inputBox := ui.GetObject("input").Cast().(*gtk.Box)

	labelWidget.SetText(label)
	descriptionWidget.SetText(description)
	descriptionWidget.SetVisible(description != "")
	inputBox.Append(input)
	return box
}

// input creates the input for a component inside a label or action row.
func (m *componentModal) input(component discord.Component) modalInput {
	switch component := component.(type) {
	case *discord.TextInputComponent:
		return m.textInput(component)

	case *discord.StringSelectComponent,
		*discord.UserSelectComponent,
		*discord.RoleSelectComponent,
		*discord.MentionableSelectComponent,
		*discord.ChannelSelectComponent:
		ic := component.(discord.InteractiveComponent)
		s := newComponentSelect(m.view.ctx, m.view.guildID, ic)
		s.SetHAlign(gtk.AlignFill)
		return modalInput{
			widget: s,
			validate: func() error {
				return validateCount(len(s.Values()), s.required, s.limits)
			},
			submit: func() *api.ModalSubmitComponent {
				return &api.ModalSubmitComponent{
					Type:     ic.Type(),
					CustomID: ic.ID(),
					Values:   s.Values(),
				}
			},
		}

	case *discord.RadioGroupComponent:
		box := gtk.NewBox(gtk.OrientationVertical, 6)
		var first *gtk.CheckButton
		buttons := make([]*gtk.CheckButton, len(component.Options))
		for i, option := range component.Options {
			button := gtk.NewCheckButtonWithLabel(option.Label)
			if first == nil {
				first = button
			} else {
				button.SetGroup(first)
			}
			button.SetActive(option.Default || option.Value == component.Value)
			buttons[i] = button
			box.Append(newModalOption(button, option.Description))
		}
		selected := func() string {
			for i, button := range buttons {
				if button.Active() {
					return component.Options[i].Value
				}
			}
			return ""
		}
		return modalInput{
			widget: box,
			validate: func() error {
				if component.Required && selected() == "" {
					return errors.New(locale.Get("Please fill in all required fields."))
				}
				return nil
			},
			submit: func() *api.ModalSubmitComponent {
				c := &api.ModalSubmitComponent{
					Type:     discord.RadioGroupComponentType,
					CustomID: component.CustomID,
				}
				if value := selected(); value != "" {
					c.Value = value
				}
				return c
			},
		}

	case *discord.CheckboxGroupComponent:
		box := gtk.NewBox(gtk.OrientationVertical, 6)
		buttons := make([]*gtk.CheckButton, len(component.Options))
		for i, option := range component.Options {
			button := gtk.NewCheckButtonWithLabel(option.Label)
			button.SetActive(option.Default || slices.Contains(component.Values, option.Value))
			buttons[i] = button
			box.Append(newModalOption(button, option.Description))
		}
		values := func() []string {
			var values []string
			for i, button := range buttons {
				if button.Active() {
					values = append(values, component.Options[i].Value)
				}
			}
			return values
		}
		return modalInput{
			widget: box,
			validate: func() error {
				return validateCount(len(values()), component.Required, component.ValueLimits)
			},
			submit: func() *api.ModalSubmitComponent {
				return &api.ModalSubmitComponent{
					Type:     discord.CheckboxGroupComponentType,
					CustomID: component.CustomID,
					Values:   values(),
				}
			},
		}

	case *discord.CheckboxComponent:
		button := gtk.NewCheckButtonWithLabel(locale.Get("Yes"))
		button.SetActive(component.Default || component.Value)
		return modalInput{
			widget:   button,
			validate: func() error { return nil },
			submit: func() *api.ModalSubmitComponent {
				return &api.ModalSubmitComponent{
					Type:     discord.CheckboxComponentType,
					CustomID: component.CustomID,
					Value:    button.Active(),
				}
			},
		}

	default:
		return modalInput{
			widget:   newUnsupportedComponent(component),
			validate: func() error { return nil },
			submit:   func() *api.ModalSubmitComponent { return nil },
		}
	}
}

func (m *componentModal) textInput(component *discord.TextInputComponent) modalInput {
	var widget gtk.Widgetter
	var text func() string

	if component.Style == discord.TextInputParagraphStyle {
		view := gtk.NewTextView()
		view.SetWrapMode(gtk.WrapWordChar)
		view.SetAcceptsTab(false)
		view.SetTopMargin(8)
		view.SetBottomMargin(8)
		view.SetLeftMargin(8)
		view.SetRightMargin(8)
		view.AddCSSClass("card")
		view.SetSizeRequest(-1, 100)
		view.SetTooltipText(component.Placeholder)

		buffer := view.Buffer()
		buffer.SetText(component.Value)
		widget = view
		text = func() string {
			start, end := buffer.Bounds()
			return buffer.Text(start, end, false)
		}
	} else {
		entry := gtk.NewEntry()
		entry.SetText(component.Value)
		entry.SetPlaceholderText(component.Placeholder)
		if component.LengthLimits[1] > 0 {
			entry.SetMaxLength(component.LengthLimits[1])
		}
		widget = entry
		text = entry.Text
	}

	return modalInput{
		widget: widget,
		validate: func() error {
			n := utf8.RuneCountInString(text())
			switch {
			case n == 0 && component.Required:
				return errors.New(locale.Get("Please fill in all required fields."))
			case n > 0 && n < component.LengthLimits[0]:
				return errors.New(locale.Sprintf("Text must be at least %d characters long.", component.LengthLimits[0]))
			case component.LengthLimits[1] > 0 && n > component.LengthLimits[1]:
				return errors.New(locale.Sprintf("Text must be at most %d characters long.", component.LengthLimits[1]))
			}
			return nil
		},
		submit: func() *api.ModalSubmitComponent {
			return &api.ModalSubmitComponent{
				Type:     discord.TextInputComponentType,
				CustomID: component.CustomID,
				Value:    text(),
			}
		},
	}
}

// newModalOption shows a radio or check button with its description.
func newModalOption(button *gtk.CheckButton, description string) gtk.Widgetter {
	if description == "" {
		return button
	}
	label := gtk.NewLabel(description)
	label.AddCSSClass("caption")
	label.AddCSSClass("dim-label")
	label.SetXAlign(0)
	label.SetWrap(true)
	label.SetMarginStart(28)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(button)
	box.Append(label)
	return box
}

// validateCount checks the number of selected values against the limits.
func validateCount(n int, required bool, limits [2]int) error {
	switch {
	case n == 0 && required:
		return errors.New(locale.Get("Please fill in all required fields."))
	case n > 0 && n < limits[0]:
		return errors.New(locale.Sprintf("Select at least %d options.", limits[0]))
	case limits[1] > 0 && n > limits[1]:
		return errors.New(locale.Sprintf("Select at most %d options.", limits[1]))
	}
	return nil
}
