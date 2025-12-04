// Package tui provides terminal user interface components including
// a fuzzy-search selector for selecting items from lists.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/sahilm/fuzzy"
)

// Item interface for selectable items
type Item interface {
	// FilterValue returns the string used for fuzzy matching
	FilterValue() string
	// Title returns the main display text
	Title() string
	// Description returns secondary text (displayed dimmed)
	Description() string
}

// Styles holds the styling configuration for the selector
type Styles struct {
	Title         lipgloss.Style
	SelectedItem  lipgloss.Style
	NormalItem    lipgloss.Style
	MatchedChars  lipgloss.Style
	Cursor        lipgloss.Style
	Help          lipgloss.Style
	NoResults     lipgloss.Style
	SearchPrompt  lipgloss.Style
	MethodGET     lipgloss.Style
	MethodPOST    lipgloss.Style
	MethodPUT     lipgloss.Style
	MethodDELETE  lipgloss.Style
	MethodPATCH   lipgloss.Style
	MethodDefault lipgloss.Style
	Description   lipgloss.Style
	Index         lipgloss.Style
	SelectedIndex lipgloss.Style
}

// DefaultStyles returns the default styling for the selector
func DefaultStyles() Styles {
	return Styles{
		Title:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
		SelectedItem:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
		NormalItem:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		MatchedChars:  lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		Cursor:        lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true),
		Help:          lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		NoResults:     lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true),
		SearchPrompt:  lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		MethodGET:     lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true),
		MethodPOST:    lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		MethodPUT:     lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true),
		MethodDELETE:  lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		MethodPATCH:   lipgloss.NewStyle().Foreground(lipgloss.Color("135")).Bold(true),
		MethodDefault: lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true),
		Description:   lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		Index:         lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		SelectedIndex: lipgloss.NewStyle().Foreground(lipgloss.Color("212")),
	}
}

// NoColorStyles returns styles without colors for non-color mode
func NoColorStyles() Styles {
	return Styles{
		Title:         lipgloss.NewStyle().Bold(true),
		SelectedItem:  lipgloss.NewStyle().Bold(true),
		NormalItem:    lipgloss.NewStyle(),
		MatchedChars:  lipgloss.NewStyle().Bold(true),
		Cursor:        lipgloss.NewStyle().Bold(true),
		Help:          lipgloss.NewStyle(),
		NoResults:     lipgloss.NewStyle().Italic(true),
		SearchPrompt:  lipgloss.NewStyle(),
		MethodGET:     lipgloss.NewStyle().Bold(true),
		MethodPOST:    lipgloss.NewStyle().Bold(true),
		MethodPUT:     lipgloss.NewStyle().Bold(true),
		MethodDELETE:  lipgloss.NewStyle().Bold(true),
		MethodPATCH:   lipgloss.NewStyle().Bold(true),
		MethodDefault: lipgloss.NewStyle().Bold(true),
		Description:   lipgloss.NewStyle(),
		Index:         lipgloss.NewStyle(),
		SelectedIndex: lipgloss.NewStyle(),
	}
}

// filteredItem holds an item with its match info
type filteredItem struct {
	item        Item
	originalIdx int
	matchedIdxs []int
}

// Model is the Bubbletea model for the selector
type Model struct {
	items         []Item
	filtered      []filteredItem
	cursor        int
	textInput     textinput.Model
	selected      Item
	selectedIndex int
	cancelled     bool
	styles        Styles
	maxVisible    int
	scrollOffset  int
}

// NewModel creates a new selector model
func NewModel(items []Item, useColors bool) Model {
	ti := textinput.New()
	ti.Placeholder = "Type to search..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	styles := DefaultStyles()
	if !useColors {
		styles = NoColorStyles()
	}

	// Initialize filtered items with all items
	filtered := make([]filteredItem, len(items))
	for i, item := range items {
		filtered[i] = filteredItem{
			item:        item,
			originalIdx: i,
		}
	}

	return Model{
		items:         items,
		filtered:      filtered,
		textInput:     ti,
		styles:        styles,
		maxVisible:    20,
		selectedIndex: -1,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			if len(m.filtered) > 0 {
				m.selected = m.filtered[m.cursor].item
				m.selectedIndex = m.filtered[m.cursor].originalIdx
			}
			return m, tea.Quit

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				// Adjust scroll if cursor goes above visible area
				if m.cursor < m.scrollOffset {
					m.scrollOffset = m.cursor
				}
			}
			return m, nil

		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				// Adjust scroll if cursor goes below visible area
				if m.cursor >= m.scrollOffset+m.maxVisible {
					m.scrollOffset = m.cursor - m.maxVisible + 1
				}
			}
			return m, nil

		case "pgup":
			m.cursor -= m.maxVisible
			if m.cursor < 0 {
				m.cursor = 0
			}
			if m.cursor < m.scrollOffset {
				m.scrollOffset = m.cursor
			}
			return m, nil

		case "pgdown":
			m.cursor += m.maxVisible
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			if m.cursor >= m.scrollOffset+m.maxVisible {
				m.scrollOffset = m.cursor - m.maxVisible + 1
			}
			return m, nil

		case "home", "ctrl+a":
			m.cursor = 0
			m.scrollOffset = 0
			return m, nil

		case "end", "ctrl+e":
			m.cursor = max(len(m.filtered)-1, 0)
			if m.cursor >= m.maxVisible {
				m.scrollOffset = m.cursor - m.maxVisible + 1
			}
			return m, nil
		}
	}

	// Handle text input updates
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Filter items based on search query
	m.filterItems()

	return m, cmd
}

// filterItems filters the items based on the current search query
func (m *Model) filterItems() {
	query := m.textInput.Value()

	if query == "" {
		// No filter - show all items
		m.filtered = make([]filteredItem, len(m.items))
		for i, item := range m.items {
			m.filtered[i] = filteredItem{
				item:        item,
				originalIdx: i,
			}
		}
	} else {
		// Build list of filter values
		filterValues := make([]string, len(m.items))
		for i, item := range m.items {
			filterValues[i] = item.FilterValue()
		}

		// Fuzzy search
		matches := fuzzy.Find(query, filterValues)

		m.filtered = make([]filteredItem, len(matches))
		for i, match := range matches {
			m.filtered[i] = filteredItem{
				item:        m.items[match.Index],
				originalIdx: match.Index,
				matchedIdxs: match.MatchedIndexes,
			}
		}
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	// Reset scroll offset
	m.scrollOffset = 0
	if m.cursor >= m.maxVisible {
		m.scrollOffset = m.cursor - m.maxVisible + 1
	}
}

// View renders the selector
func (m Model) View() string {
	var b strings.Builder

	// If selection is done (either selected or cancelled), show final state
	if m.selected != nil {
		// Show just the selected item (no prompt)
		b.WriteString(fmt.Sprintf("[%d] %s", m.selectedIndex+1, m.selected.Title()))
		if desc := m.selected.Description(); desc != "" {
			b.WriteString("  ")
			b.WriteString(m.styles.Description.Render(desc))
		}
		b.WriteString("\n")
		return b.String()
	}
	if m.cancelled {
		// Return empty on cancel
		return ""
	}

	// Search input
	b.WriteString(m.styles.SearchPrompt.Render("Search: "))
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	// Items list
	if len(m.filtered) == 0 {
		b.WriteString(m.styles.NoResults.Render("  No matching requests"))
		b.WriteString("\n")
	} else {
		// Calculate visible range
		start := m.scrollOffset
		end := min(start+m.maxVisible, len(m.filtered))

		// Show scroll indicator if needed
		if start > 0 {
			b.WriteString(m.styles.Help.Render("  ↑ more above"))
			b.WriteString("\n")
		}

		for i := start; i < end; i++ {
			fi := m.filtered[i]
			isSelected := i == m.cursor

			// Build the line
			var line strings.Builder

			// Cursor indicator
			if isSelected {
				line.WriteString(m.styles.Cursor.Render("> "))
			} else {
				line.WriteString("  ")
			}

			// Index (1-based)
			indexStr := fmt.Sprintf("[%d] ", fi.originalIdx+1)
			if isSelected {
				line.WriteString(m.styles.SelectedIndex.Render(indexStr))
			} else {
				line.WriteString(m.styles.Index.Render(indexStr))
			}

			// Title with matched characters highlighted
			title := fi.item.Title()
			if m.textInput.Value() != "" {
				// Re-run fuzzy match on the title to get correct highlight positions
				titleMatches := fuzzy.Find(m.textInput.Value(), []string{title})
				if len(titleMatches) > 0 && len(titleMatches[0].MatchedIndexes) > 0 {
					line.WriteString(m.highlightMatches(title, titleMatches[0].MatchedIndexes, isSelected, false))
				} else {
					if isSelected {
						line.WriteString(m.styles.SelectedItem.Render(title))
					} else {
						line.WriteString(m.styles.NormalItem.Render(title))
					}
				}
			} else {
				if isSelected {
					line.WriteString(m.styles.SelectedItem.Render(title))
				} else {
					line.WriteString(m.styles.NormalItem.Render(title))
				}
			}

			// Description
			desc := fi.item.Description()
			if desc != "" {
				line.WriteString("  ")
				if m.textInput.Value() != "" {
					// Re-run fuzzy match on the description to get correct highlight positions
					descMatches := fuzzy.Find(m.textInput.Value(), []string{desc})
					if len(descMatches) > 0 && len(descMatches[0].MatchedIndexes) > 0 {
						line.WriteString(m.highlightMatches(desc, descMatches[0].MatchedIndexes, isSelected, true))
					} else {
						line.WriteString(m.styles.Description.Render(desc))
					}
				} else {
					line.WriteString(m.styles.Description.Render(desc))
				}
			}

			b.WriteString(line.String())
			b.WriteString("\n")
		}

		// Show scroll indicator if needed
		if end < len(m.filtered) {
			b.WriteString(m.styles.Help.Render("  ↓ more below"))
			b.WriteString("\n")
		}
	}

	// Help text
	b.WriteString("\n")
	b.WriteString(m.styles.Help.Render("↑/↓: navigate  enter: select  esc: cancel"))

	// Show result count if filtering
	if m.textInput.Value() != "" {
		b.WriteString(m.styles.Help.Render(fmt.Sprintf("  (%d/%d)", len(m.filtered), len(m.items))))
	}

	return b.String()
}

// highlightMatches highlights matched characters in the text
func (m Model) highlightMatches(text string, matchedIdxs []int, isSelected bool, isDescription bool) string {
	// Create a set of matched indexes for quick lookup
	matchSet := make(map[int]bool)
	for _, idx := range matchedIdxs {
		matchSet[idx] = true
	}

	var result strings.Builder
	runes := []rune(text)

	for i, r := range runes {
		char := string(r)
		if matchSet[i] {
			result.WriteString(m.styles.MatchedChars.Render(char))
		} else if isDescription {
			result.WriteString(m.styles.Description.Render(char))
		} else if isSelected {
			result.WriteString(m.styles.SelectedItem.Render(char))
		} else {
			result.WriteString(m.styles.NormalItem.Render(char))
		}
	}

	return result.String()
}

// Selected returns the selected item, or nil if cancelled
func (m Model) Selected() Item {
	return m.selected
}

// SelectedIndex returns the original index of the selected item, or -1 if cancelled
func (m Model) SelectedIndex() int {
	return m.selectedIndex
}

// Cancelled returns true if the selection was cancelled
func (m Model) Cancelled() bool {
	return m.cancelled
}

// ErrCancelled is returned when the user cancels the selection
var ErrCancelled = errors.ErrCanceled

// Run runs the selector and returns the selected item and its original index
// Returns nil, -1, ErrCancelled if user cancelled, or nil, -1, error for other errors
func Run(items []Item, useColors bool) (Item, int, error) {
	fmt.Println() // Blank line before TUI
	model := NewModel(items, useColors)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return nil, -1, errors.Wrap(err, "failed to run selector")
	}

	m := finalModel.(Model)
	if m.Cancelled() {
		return nil, -1, ErrCancelled
	}

	return m.Selected(), m.SelectedIndex(), nil
}
