package tui

import (
	"testing"
)

func TestNewInputFormModel(t *testing.T) {
	fields := []InputField{
		{Name: "id", Default: "123"},
		{Name: "page", Default: ""},
	}

	model := NewInputFormModel(fields, true)

	if len(model.inputs) != 2 {
		t.Errorf("NewInputFormModel() created %d inputs, want %d", len(model.inputs), 2)
	}

	// First field should be focused
	if !model.inputs[0].Focused() {
		t.Error("First input should be focused")
	}

	// Second field should not be focused
	if model.inputs[1].Focused() {
		t.Error("Second input should not be focused")
	}

	// First field should have default value
	if model.inputs[0].Value() != "123" {
		t.Errorf("First input value = %v, want %v", model.inputs[0].Value(), "123")
	}

	// Second field should be empty
	if model.inputs[1].Value() != "" {
		t.Errorf("Second input value = %v, want empty", model.inputs[1].Value())
	}
}

func TestInputFormModel_Values(t *testing.T) {
	fields := []InputField{
		{Name: "id"},
		{Name: "page"},
	}

	model := NewInputFormModel(fields, true)

	// Set values on the inputs
	model.inputs[0].SetValue("42")
	model.inputs[1].SetValue("1")

	values := model.Values()

	if values["id"] != "42" {
		t.Errorf("Values()[id] = %v, want %v", values["id"], "42")
	}
	if values["page"] != "1" {
		t.Errorf("Values()[page] = %v, want %v", values["page"], "1")
	}
}

func TestInputFormModel_Canceled(t *testing.T) {
	fields := []InputField{
		{Name: "id"},
	}

	model := NewInputFormModel(fields, true)
	model.canceled = true

	if !model.Canceled() {
		t.Error("Canceled() should return true when canceled")
	}

	values := model.Values()
	if values != nil {
		t.Error("Values() should return nil when canceled")
	}
}

func TestInputFormModel_NoFields(t *testing.T) {
	fields := []InputField{}

	model := NewInputFormModel(fields, true)

	if len(model.inputs) != 0 {
		t.Errorf("NewInputFormModel() with empty fields created %d inputs, want 0", len(model.inputs))
	}

	values := model.Values()
	if len(values) != 0 {
		t.Errorf("Values() with no fields returned %d values, want 0", len(values))
	}
}

func TestDefaultInputFormStyles(t *testing.T) {
	styles := DefaultInputFormStyles()

	// Just verify that styles are created without panicking
	_ = styles.Title.Render("test")
	_ = styles.Label.Render("test")
	_ = styles.LabelFocused.Render("test")
	_ = styles.Help.Render("test")
	_ = styles.Cursor.Render("test")
}

func TestNoColorInputFormStyles(t *testing.T) {
	styles := NoColorInputFormStyles()

	// Just verify that styles are created without panicking
	_ = styles.Title.Render("test")
	_ = styles.Label.Render("test")
	_ = styles.LabelFocused.Render("test")
	_ = styles.Help.Render("test")
	_ = styles.Cursor.Render("test")
}

func TestInputFormModel_View(t *testing.T) {
	fields := []InputField{
		{Name: "id", Description: "Post ID"},
		{Name: "page"},
	}

	model := NewInputFormModel(fields, false)

	view := model.View()

	// Check that the view contains expected elements
	if view == "" {
		t.Error("View() returned empty string")
	}

	// Check for title
	if !contains(view, "Enter values") {
		t.Error("View() should contain title")
	}

	// Check for field names
	if !contains(view, "id") {
		t.Error("View() should contain field name 'id'")
	}
	if !contains(view, "page") {
		t.Error("View() should contain field name 'page'")
	}

	// Check for description
	if !contains(view, "Post ID") {
		t.Error("View() should contain field description")
	}

	// Check for help text
	if !contains(view, "esc: cancel") {
		t.Error("View() should contain help text")
	}
}

func TestInputFormModel_SubmittedView(t *testing.T) {
	fields := []InputField{
		{Name: "id"},
	}

	model := NewInputFormModel(fields, false)
	model.inputs[0].SetValue("123")
	model.submitted = true

	view := model.View()

	// Check for summary
	if !contains(view, "id: 123") {
		t.Error("Submitted view should show value summary")
	}
}

func TestInputFormModel_CanceledView(t *testing.T) {
	fields := []InputField{
		{Name: "id"},
	}

	model := NewInputFormModel(fields, false)
	model.canceled = true

	view := model.View()

	if view != "" {
		t.Error("Canceled view should be empty")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Edge case tests for input form

func TestInputFormModel_SingleField(t *testing.T) {
	fields := []InputField{
		{Name: "id", Default: "42"},
	}

	model := NewInputFormModel(fields, true)

	if len(model.inputs) != 1 {
		t.Errorf("Expected 1 input, got %d", len(model.inputs))
	}

	if model.cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", model.cursor)
	}

	// Single field should be focused
	if !model.inputs[0].Focused() {
		t.Error("Single field should be focused")
	}
}

func TestInputFormModel_ManyFields(t *testing.T) {
	fields := make([]InputField, 10)
	for i := 0; i < 10; i++ {
		fields[i] = InputField{Name: string(rune('a' + i))}
	}

	model := NewInputFormModel(fields, true)

	if len(model.inputs) != 10 {
		t.Errorf("Expected 10 inputs, got %d", len(model.inputs))
	}

	// Only first should be focused
	for i, input := range model.inputs {
		if i == 0 && !input.Focused() {
			t.Error("First input should be focused")
		}
		if i != 0 && input.Focused() {
			t.Errorf("Input %d should not be focused", i)
		}
	}
}

func TestInputFormModel_FieldWithLongDefault(t *testing.T) {
	longValue := "This is a very long default value that might exceed typical input width limits in some UI implementations"
	fields := []InputField{
		{Name: "data", Default: longValue},
	}

	model := NewInputFormModel(fields, true)

	if model.inputs[0].Value() != longValue {
		t.Errorf("Long default value not preserved, got %v", model.inputs[0].Value())
	}
}

func TestInputFormModel_FieldWithSpecialCharacters(t *testing.T) {
	specialValue := "special!@#$%^&*()_+{}|:<>?`~[]\\;',./\""
	fields := []InputField{
		{Name: "special", Default: specialValue},
	}

	model := NewInputFormModel(fields, true)

	if model.inputs[0].Value() != specialValue {
		t.Errorf("Special characters not preserved, got %v", model.inputs[0].Value())
	}
}

func TestInputFormModel_FieldWithUnicode(t *testing.T) {
	unicodeValue := "日本語 한국어 中文 العربية 🎉"
	fields := []InputField{
		{Name: "unicode", Default: unicodeValue},
	}

	model := NewInputFormModel(fields, true)

	if model.inputs[0].Value() != unicodeValue {
		t.Errorf("Unicode value not preserved, got %v", model.inputs[0].Value())
	}
}

func TestInputFormModel_FieldWithWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string // textinput normalizes certain whitespace
	}{
		{"spaces", "  spaces  ", "  spaces  "},
		{"tabs", "\ttabs\t", " tabs "},              // tabs are normalized to spaces
		{"newlines", "line1\nline2", "line1 line2"}, // newlines are normalized to spaces
		{"mixed", " \t\n ", "    "},                 // tabs and newlines normalized to spaces
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := []InputField{
				{Name: "whitespace", Default: tt.value},
			}

			model := NewInputFormModel(fields, true)

			if model.inputs[0].Value() != tt.expected {
				t.Errorf("Whitespace value not preserved, got %q, want %q", model.inputs[0].Value(), tt.expected)
			}
		})
	}
}

func TestInputFormModel_EmptyFieldName(t *testing.T) {
	fields := []InputField{
		{Name: "", Default: "value"},
	}

	model := NewInputFormModel(fields, true)

	// Should still create the input
	if len(model.inputs) != 1 {
		t.Errorf("Expected 1 input even with empty name, got %d", len(model.inputs))
	}
}

func TestInputFormModel_DuplicateFieldNames(t *testing.T) {
	fields := []InputField{
		{Name: "id", Default: "1"},
		{Name: "id", Default: "2"},
		{Name: "id", Default: "3"},
	}

	model := NewInputFormModel(fields, true)

	// Should create all inputs even with duplicate names
	if len(model.inputs) != 3 {
		t.Errorf("Expected 3 inputs, got %d", len(model.inputs))
	}

	// Each should have its own default
	if model.inputs[0].Value() != "1" {
		t.Errorf("First input value = %v, want 1", model.inputs[0].Value())
	}
	if model.inputs[1].Value() != "2" {
		t.Errorf("Second input value = %v, want 2", model.inputs[1].Value())
	}
	if model.inputs[2].Value() != "3" {
		t.Errorf("Third input value = %v, want 3", model.inputs[2].Value())
	}
}

func TestInputFormModel_Submitted(t *testing.T) {
	fields := []InputField{
		{Name: "id"},
	}

	model := NewInputFormModel(fields, true)

	if model.Submitted() {
		t.Error("Submitted() should return false initially")
	}

	model.submitted = true

	if !model.Submitted() {
		t.Error("Submitted() should return true after submission")
	}
}

func TestInputFormModel_EmptyValueDisplay(t *testing.T) {
	fields := []InputField{
		{Name: "empty"},
	}

	model := NewInputFormModel(fields, false)
	model.submitted = true

	view := model.View()

	if !contains(view, "(empty)") {
		t.Error("Submitted view should show (empty) for empty values")
	}
}

func TestInputFormModel_SubmittedViewNewline(t *testing.T) {
	fields := []InputField{
		{Name: "id"},
	}

	model := NewInputFormModel(fields, false)
	model.inputs[0].SetValue("123")
	model.submitted = true

	view := model.View()

	// Check that view ends with newline for proper formatting
	if len(view) < 2 || view[len(view)-2:] != "\n\n" {
		// Actually it should end with single \n after the trailing \n we added
		if len(view) < 1 || view[len(view)-1:] != "\n" {
			t.Error("Submitted view should end with newline")
		}
	}
}

func TestInputFormModel_ColorModes(t *testing.T) {
	fields := []InputField{
		{Name: "test"},
	}

	// Test with colors
	modelWithColors := NewInputFormModel(fields, true)
	viewWithColors := modelWithColors.View()

	// Test without colors
	modelNoColors := NewInputFormModel(fields, false)
	viewNoColors := modelNoColors.View()

	// Both should produce non-empty views
	if viewWithColors == "" {
		t.Error("View with colors should not be empty")
	}
	if viewNoColors == "" {
		t.Error("View without colors should not be empty")
	}
}

func TestRunInputForm_EmptyFields(t *testing.T) {
	// RunInputForm with empty fields should return empty map without error
	values, err := RunInputForm([]InputField{}, true)

	if err != nil {
		t.Errorf("RunInputForm() with empty fields returned error: %v", err)
	}

	if values == nil {
		t.Error("RunInputForm() with empty fields should return non-nil map")
	}

	if len(values) != 0 {
		t.Errorf("RunInputForm() with empty fields returned %d values, want 0", len(values))
	}
}
