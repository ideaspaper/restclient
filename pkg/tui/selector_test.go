package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ideaspaper/restclient/pkg/errors"
)

// testItem is a simple implementation of Item for testing
type testItem struct {
	filterValue string
	title       string
	description string
}

func (t testItem) FilterValue() string { return t.filterValue }
func (t testItem) Title() string       { return t.title }
func (t testItem) Description() string { return t.description }
func (t testItem) String() string      { return t.title + "  " + t.description }

func TestNewModel(t *testing.T) {
	items := []Item{
		testItem{filterValue: "GET /users", title: "GET /users", description: "Get all users"},
		testItem{filterValue: "POST /users", title: "POST /users", description: "Create user"},
	}

	t.Run("creates model with colors", func(t *testing.T) {
		model := NewModel(items, true)

		if len(model.items) != 2 {
			t.Errorf("expected 2 items, got %d", len(model.items))
		}
		if len(model.filtered) != 2 {
			t.Errorf("expected 2 filtered items, got %d", len(model.filtered))
		}
		if model.cursor != 0 {
			t.Errorf("expected cursor at 0, got %d", model.cursor)
		}
		if model.selectedIndex != -1 {
			t.Errorf("expected selectedIndex -1, got %d", model.selectedIndex)
		}
		if model.cancelled {
			t.Error("expected cancelled to be false")
		}
	})

	t.Run("creates model without colors", func(t *testing.T) {
		model := NewModel(items, false)

		if len(model.items) != 2 {
			t.Errorf("expected 2 items, got %d", len(model.items))
		}
	})

	t.Run("handles empty items", func(t *testing.T) {
		model := NewModel([]Item{}, true)

		if len(model.items) != 0 {
			t.Errorf("expected 0 items, got %d", len(model.items))
		}
		if len(model.filtered) != 0 {
			t.Errorf("expected 0 filtered items, got %d", len(model.filtered))
		}
	})
}

func TestModelUpdate(t *testing.T) {
	items := []Item{
		testItem{filterValue: "GET /users", title: "GET /users", description: "Get all users"},
		testItem{filterValue: "POST /users", title: "POST /users", description: "Create user"},
		testItem{filterValue: "DELETE /users", title: "DELETE /users", description: "Delete user"},
	}

	t.Run("escape cancels selection", func(t *testing.T) {
		model := NewModel(items, true)

		newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m := newModel.(Model)

		if !m.Cancelled() {
			t.Error("expected cancelled to be true after Esc")
		}
		if cmd == nil {
			t.Error("expected quit command")
		}
	})

	t.Run("ctrl+c cancels selection", func(t *testing.T) {
		model := NewModel(items, true)

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		m := newModel.(Model)

		if !m.Cancelled() {
			t.Error("expected cancelled to be true after Ctrl+C")
		}
	})

	t.Run("enter selects current item", func(t *testing.T) {
		model := NewModel(items, true)

		newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m := newModel.(Model)

		if m.Cancelled() {
			t.Error("expected cancelled to be false after Enter")
		}
		if m.SelectedIndex() != 0 {
			t.Errorf("expected selectedIndex 0, got %d", m.SelectedIndex())
		}
		if m.Selected() == nil {
			t.Error("expected selected item to be non-nil")
		}
		if cmd == nil {
			t.Error("expected quit command")
		}
	})

	t.Run("down arrow moves cursor down", func(t *testing.T) {
		model := NewModel(items, true)

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		m := newModel.(Model)

		if m.cursor != 1 {
			t.Errorf("expected cursor at 1, got %d", m.cursor)
		}
	})

	t.Run("up arrow moves cursor up", func(t *testing.T) {
		model := NewModel(items, true)
		model.cursor = 2

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyUp})
		m := newModel.(Model)

		if m.cursor != 1 {
			t.Errorf("expected cursor at 1, got %d", m.cursor)
		}
	})

	t.Run("up arrow at top stays at top", func(t *testing.T) {
		model := NewModel(items, true)
		model.cursor = 0

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyUp})
		m := newModel.(Model)

		if m.cursor != 0 {
			t.Errorf("expected cursor at 0, got %d", m.cursor)
		}
	})

	t.Run("down arrow at bottom stays at bottom", func(t *testing.T) {
		model := NewModel(items, true)
		model.cursor = 2

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		m := newModel.(Model)

		if m.cursor != 2 {
			t.Errorf("expected cursor at 2, got %d", m.cursor)
		}
	})

	t.Run("home moves cursor to first item", func(t *testing.T) {
		model := NewModel(items, true)
		model.cursor = 2

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyHome})
		m := newModel.(Model)

		if m.cursor != 0 {
			t.Errorf("expected cursor at 0, got %d", m.cursor)
		}
	})

	t.Run("end moves cursor to last item", func(t *testing.T) {
		model := NewModel(items, true)

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnd})
		m := newModel.(Model)

		if m.cursor != 2 {
			t.Errorf("expected cursor at 2, got %d", m.cursor)
		}
	})
}

func TestModelFilterItems(t *testing.T) {
	items := []Item{
		testItem{filterValue: "GET /users", title: "GET /users", description: "Get all users"},
		testItem{filterValue: "POST /users", title: "POST /users", description: "Create user"},
		testItem{filterValue: "GET /albums", title: "GET /albums", description: "Get all albums"},
	}

	t.Run("empty query shows all items", func(t *testing.T) {
		model := NewModel(items, true)
		model.filterItems()

		if len(model.filtered) != 3 {
			t.Errorf("expected 3 filtered items, got %d", len(model.filtered))
		}
	})

	t.Run("filters by unique term", func(t *testing.T) {
		model := NewModel(items, true)
		model.textInput.SetValue("albums")
		model.filterItems()

		if len(model.filtered) != 1 {
			t.Errorf("expected 1 filtered item, got %d", len(model.filtered))
		}
		if model.filtered[0].item.Title() != "GET /albums" {
			t.Errorf("expected 'GET /albums', got '%s'", model.filtered[0].item.Title())
		}
	})

	t.Run("filters multiple matches", func(t *testing.T) {
		model := NewModel(items, true)
		model.textInput.SetValue("users")
		model.filterItems()

		if len(model.filtered) != 2 {
			t.Errorf("expected 2 filtered items, got %d", len(model.filtered))
		}
	})

	t.Run("fuzzy matches", func(t *testing.T) {
		model := NewModel(items, true)
		model.textInput.SetValue("gus") // matches "GET /users"
		model.filterItems()

		if len(model.filtered) < 1 {
			t.Errorf("expected at least 1 filtered item, got %d", len(model.filtered))
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		model := NewModel(items, true)
		model.textInput.SetValue("xyz123")
		model.filterItems()

		if len(model.filtered) != 0 {
			t.Errorf("expected 0 filtered items, got %d", len(model.filtered))
		}
	})

	t.Run("cursor resets when out of bounds", func(t *testing.T) {
		model := NewModel(items, true)
		model.cursor = 2
		model.textInput.SetValue("albums")
		model.filterItems()

		// Only 1 result, cursor should reset to 0
		if model.cursor != 0 {
			t.Errorf("expected cursor at 0, got %d", model.cursor)
		}
	})

	t.Run("preserves original index", func(t *testing.T) {
		model := NewModel(items, true)
		model.textInput.SetValue("albums")
		model.filterItems()

		if len(model.filtered) != 1 {
			t.Fatalf("expected 1 filtered item, got %d", len(model.filtered))
		}
		// "GET /albums" is at original index 2
		if model.filtered[0].originalIdx != 2 {
			t.Errorf("expected originalIdx 2, got %d", model.filtered[0].originalIdx)
		}
	})
}

func TestModelView(t *testing.T) {
	items := []Item{
		testItem{filterValue: "GET /users", title: "GET /users", description: "Get all users"},
		testItem{filterValue: "POST /users", title: "POST /users", description: "Create user"},
	}

	t.Run("renders search prompt", func(t *testing.T) {
		model := NewModel(items, true)
		view := model.View()

		if !containsString(view, "Search:") {
			t.Error("expected view to contain 'Search:'")
		}
	})

	t.Run("renders items", func(t *testing.T) {
		model := NewModel(items, true)
		view := model.View()

		if !containsString(view, "GET /users") {
			t.Error("expected view to contain 'GET /users'")
		}
		if !containsString(view, "POST /users") {
			t.Error("expected view to contain 'POST /users'")
		}
	})

	t.Run("renders descriptions", func(t *testing.T) {
		model := NewModel(items, true)
		view := model.View()

		if !containsString(view, "Get all users") {
			t.Error("expected view to contain 'Get all users'")
		}
	})

	t.Run("renders help text", func(t *testing.T) {
		model := NewModel(items, true)
		view := model.View()

		if !containsString(view, "navigate") {
			t.Error("expected view to contain navigation help")
		}
		if !containsString(view, "select") {
			t.Error("expected view to contain select help")
		}
		if !containsString(view, "cancel") {
			t.Error("expected view to contain cancel help")
		}
	})

	t.Run("renders no results message", func(t *testing.T) {
		model := NewModel(items, true)
		model.textInput.SetValue("xyz123")
		model.filterItems()
		view := model.View()

		if !containsString(view, "No matching") {
			t.Error("expected view to contain no results message")
		}
	})

	t.Run("renders result count when filtering", func(t *testing.T) {
		model := NewModel(items, true)
		model.textInput.SetValue("GET")
		model.filterItems()
		view := model.View()

		if !containsString(view, "(1/2)") {
			t.Error("expected view to contain result count")
		}
	})

	t.Run("renders cursor indicator", func(t *testing.T) {
		model := NewModel(items, true)
		view := model.View()

		if !containsString(view, ">") {
			t.Error("expected view to contain cursor indicator")
		}
	})

	t.Run("renders index numbers", func(t *testing.T) {
		model := NewModel(items, true)
		view := model.View()

		if !containsString(view, "[1]") {
			t.Error("expected view to contain [1]")
		}
		if !containsString(view, "[2]") {
			t.Error("expected view to contain [2]")
		}
	})
}

func TestHighlightMatches(t *testing.T) {
	model := NewModel([]Item{}, false) // no colors for simpler testing

	t.Run("highlights matched characters", func(t *testing.T) {
		result := model.highlightMatches("hello", []int{0, 2}, false, false)
		// The result should contain the text (styling is applied but text is preserved)
		if !containsString(result, "h") || !containsString(result, "e") || !containsString(result, "l") || !containsString(result, "o") {
			t.Error("expected all characters to be present")
		}
	})

	t.Run("handles empty match indexes", func(t *testing.T) {
		result := model.highlightMatches("hello", []int{}, false, false)
		if !containsString(result, "hello") {
			t.Error("expected 'hello' to be present")
		}
	})

	t.Run("handles empty text", func(t *testing.T) {
		result := model.highlightMatches("", []int{}, false, false)
		if result != "" {
			t.Errorf("expected empty string, got '%s'", result)
		}
	})
}

func TestDefaultStyles(t *testing.T) {
	styles := DefaultStyles()

	// Just verify styles are created without panicking
	_ = styles.Title.Render("test")
	_ = styles.SelectedItem.Render("test")
	_ = styles.NormalItem.Render("test")
	_ = styles.MatchedChars.Render("test")
	_ = styles.Cursor.Render("test")
	_ = styles.Help.Render("test")
	_ = styles.NoResults.Render("test")
	_ = styles.Description.Render("test")
	_ = styles.Index.Render("test")
	_ = styles.SelectedIndex.Render("test")
}

func TestNoColorStyles(t *testing.T) {
	styles := NoColorStyles()

	// Just verify styles are created without panicking
	_ = styles.Title.Render("test")
	_ = styles.SelectedItem.Render("test")
	_ = styles.NormalItem.Render("test")
	_ = styles.MatchedChars.Render("test")
}

func TestErrCancelled(t *testing.T) {
	if ErrCancelled == nil {
		t.Error("ErrCancelled should not be nil")
	}
	// Check that ErrCancelled is errors.ErrCanceled using errors.Is
	if !errors.Is(ErrCancelled, errors.ErrCanceled) {
		t.Error("ErrCancelled should be errors.ErrCanceled")
	}
}

func TestScrolling(t *testing.T) {
	// Create more items than maxVisible (default 10)
	items := make([]Item, 15)
	for i := 0; i < 15; i++ {
		items[i] = testItem{
			filterValue: "item",
			title:       "Item",
			description: "Description",
		}
	}

	t.Run("scroll offset adjusts when moving down past visible", func(t *testing.T) {
		model := NewModel(items, true)
		model.maxVisible = 5

		// Move cursor to position 5 (beyond visible range of 0-4)
		for i := 0; i < 5; i++ {
			newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
			model = newModel.(Model)
		}

		if model.scrollOffset == 0 {
			t.Error("expected scrollOffset to increase")
		}
	})

	t.Run("scroll offset adjusts when moving up past visible", func(t *testing.T) {
		model := NewModel(items, true)
		model.maxVisible = 5
		model.cursor = 10
		model.scrollOffset = 6

		// Move cursor up past visible area
		for i := 0; i < 5; i++ {
			newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyUp})
			model = newModel.(Model)
		}

		if model.scrollOffset > model.cursor {
			t.Error("scrollOffset should be <= cursor")
		}
	})

	t.Run("page down moves by maxVisible", func(t *testing.T) {
		model := NewModel(items, true)
		model.maxVisible = 5

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m := newModel.(Model)

		if m.cursor != 5 {
			t.Errorf("expected cursor at 5, got %d", m.cursor)
		}
	})

	t.Run("page up moves by maxVisible", func(t *testing.T) {
		model := NewModel(items, true)
		model.maxVisible = 5
		model.cursor = 10

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		m := newModel.(Model)

		if m.cursor != 5 {
			t.Errorf("expected cursor at 5, got %d", m.cursor)
		}
	})

	t.Run("page down at end stays at end", func(t *testing.T) {
		model := NewModel(items, true)
		model.cursor = 14

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m := newModel.(Model)

		if m.cursor != 14 {
			t.Errorf("expected cursor at 14, got %d", m.cursor)
		}
	})

	t.Run("page up at start stays at start", func(t *testing.T) {
		model := NewModel(items, true)
		model.cursor = 2

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		m := newModel.(Model)

		if m.cursor != 0 {
			t.Errorf("expected cursor at 0, got %d", m.cursor)
		}
	})
}

func TestEmptyList(t *testing.T) {
	t.Run("enter on empty list does not panic", func(t *testing.T) {
		model := NewModel([]Item{}, true)

		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m := newModel.(Model)

		if m.Selected() != nil {
			t.Error("expected nil selected item")
		}
		if m.SelectedIndex() != -1 {
			t.Errorf("expected selectedIndex -1, got %d", m.SelectedIndex())
		}
	})

	t.Run("navigation on empty list does not panic", func(t *testing.T) {
		model := NewModel([]Item{}, true)

		// These should not panic
		model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model.Update(tea.KeyMsg{Type: tea.KeyUp})
		model.Update(tea.KeyMsg{Type: tea.KeyHome})
		model.Update(tea.KeyMsg{Type: tea.KeyEnd})
		model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	})

	t.Run("view on empty list shows no results", func(t *testing.T) {
		model := NewModel([]Item{}, true)
		view := model.View()

		if !containsString(view, "No matching") {
			t.Error("expected no results message")
		}
	})
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
