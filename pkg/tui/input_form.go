package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ideaspaper/restclient/pkg/errors"
)

// InputField represents a single input field in the form.
type InputField struct {
	Name        string // Parameter name (e.g., "id")
	Description string // Optional description for the field
	Default     string // Default value (from session)
	Value       string // Current value
}

// InputFormModel is a Bubbletea model for multi-field input.
type InputFormModel struct {
	fields    []InputField
	inputs    []textinput.Model
	cursor    int
	styles    InputFormStyles
	canceled  bool
	submitted bool
}

// InputFormStyles holds styling configuration for the input form.
type InputFormStyles struct {
	Title        lipgloss.Style
	Label        lipgloss.Style
	LabelFocused lipgloss.Style
	Help         lipgloss.Style
	Cursor       lipgloss.Style
}

// DefaultInputFormStyles returns the default styling for the input form.
func DefaultInputFormStyles() InputFormStyles {
	return InputFormStyles{
		Title:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
		Label:        lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		LabelFocused: lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true),
		Help:         lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		Cursor:       lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true),
	}
}

// NoColorInputFormStyles returns styles without colors.
func NoColorInputFormStyles() InputFormStyles {
	return InputFormStyles{
		Title:        lipgloss.NewStyle().Bold(true),
		Label:        lipgloss.NewStyle(),
		LabelFocused: lipgloss.NewStyle().Bold(true),
		Help:         lipgloss.NewStyle(),
		Cursor:       lipgloss.NewStyle().Bold(true),
	}
}

// NewInputFormModel creates a new input form with the given fields.
func NewInputFormModel(fields []InputField, useColors bool) InputFormModel {
	styles := DefaultInputFormStyles()
	if !useColors {
		styles = NoColorInputFormStyles()
	}

	inputs := make([]textinput.Model, len(fields))
	for i, field := range fields {
		ti := textinput.New()
		ti.Placeholder = fmt.Sprintf("Enter %s", field.Name)
		ti.CharLimit = 256
		ti.Width = 40

		// Pre-fill with default value if available
		if field.Default != "" {
			ti.SetValue(field.Default)
		}

		// Focus the first input
		if i == 0 {
			ti.Focus()
		}

		inputs[i] = ti
	}

	return InputFormModel{
		fields: fields,
		inputs: inputs,
		cursor: 0,
		styles: styles,
	}
}

// Init initializes the model.
func (m InputFormModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages and updates the model.
func (m InputFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.canceled = true
			return m, tea.Quit

		case "enter":
			// If on the last field, submit the form
			if m.cursor >= len(m.inputs)-1 {
				m.submitted = true
				return m, tea.Quit
			}
			// Otherwise, move to the next field
			return m.nextField()

		case "tab", "down":
			return m.nextField()

		case "shift+tab", "up":
			return m.prevField()
		}
	}

	// Update the currently focused input
	var cmd tea.Cmd
	m.inputs[m.cursor], cmd = m.inputs[m.cursor].Update(msg)
	return m, cmd
}

// nextField moves focus to the next field.
func (m InputFormModel) nextField() (tea.Model, tea.Cmd) {
	if m.cursor < len(m.inputs)-1 {
		m.inputs[m.cursor].Blur()
		m.cursor++
		m.inputs[m.cursor].Focus()
	}
	return m, nil
}

// prevField moves focus to the previous field.
func (m InputFormModel) prevField() (tea.Model, tea.Cmd) {
	if m.cursor > 0 {
		m.inputs[m.cursor].Blur()
		m.cursor--
		m.inputs[m.cursor].Focus()
	}
	return m, nil
}

// View renders the input form.
func (m InputFormModel) View() string {
	if m.canceled {
		return ""
	}

	if m.submitted {
		// Show summary of entered values as a bulleted list
		var b strings.Builder
		b.WriteString("\n")
		for i, field := range m.fields {
			value := m.inputs[i].Value()
			if value == "" {
				value = "(empty)"
			}
			b.WriteString(fmt.Sprintf("- %s: %s\n", field.Name, value))
		}
		b.WriteString("\n")
		return b.String()
	}

	var b strings.Builder

	// Title
	b.WriteString(m.styles.Title.Render("Enter values for user input parameters:"))
	b.WriteString("\n\n")

	// Render each field
	for i, field := range m.fields {
		isFocused := i == m.cursor

		// Cursor indicator
		if isFocused {
			b.WriteString(m.styles.Cursor.Render("> "))
		} else {
			b.WriteString("  ")
		}

		// Label
		label := field.Name
		if field.Description != "" {
			label = fmt.Sprintf("%s (%s)", field.Name, field.Description)
		}

		if isFocused {
			b.WriteString(m.styles.LabelFocused.Render(label))
		} else {
			b.WriteString(m.styles.Label.Render(label))
		}
		b.WriteString(": ")

		// Input field
		b.WriteString(m.inputs[i].View())
		b.WriteString("\n")
	}

	// Help text
	b.WriteString("\n")
	b.WriteString(m.styles.Help.Render("tab/↓: next  shift+tab/↑: prev  enter: submit  esc: cancel"))

	return b.String()
}

// Values returns the collected values as a map.
// Returns nil if the form was canceled.
func (m InputFormModel) Values() map[string]string {
	if m.canceled {
		return nil
	}

	values := make(map[string]string)
	for i, field := range m.fields {
		values[field.Name] = m.inputs[i].Value()
	}
	return values
}

// Canceled returns true if the form was canceled.
func (m InputFormModel) Canceled() bool {
	return m.canceled
}

// Submitted returns true if the form was submitted.
func (m InputFormModel) Submitted() bool {
	return m.submitted
}

// RunInputForm displays the form and returns collected values.
// Returns nil map if user cancels (Escape), or error on failure.
func RunInputForm(fields []InputField, useColors bool) (map[string]string, error) {
	if len(fields) == 0 {
		return make(map[string]string), nil
	}

	model := NewInputFormModel(fields, useColors)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run input form")
	}

	m := finalModel.(InputFormModel)
	if m.Canceled() {
		return nil, ErrCancelled
	}

	return m.Values(), nil
}
