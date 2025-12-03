package cmd

import (
	"fmt"

	"github.com/fatih/color"

	"github.com/ideaspaper/restclient/internal/stringutil"
)

// useColors returns true if colored output should be used.
// Colors are controlled by the showColors config option.
func useColors() bool {
	return v.GetBool("showColors")
}

// Color definitions for consistent styling across commands
var (
	indexColor = color.New(color.FgHiBlack)
	nameColor  = color.New(color.FgHiBlack)

	headerColor = color.New(color.FgCyan)

	successColor = color.New(color.FgGreen)
	errorColor   = color.New(color.FgRed)
	warnColor    = color.New(color.FgYellow)

	keyColor   = color.New(color.FgCyan)
	valueColor = color.New(color.FgWhite)
)

// getMethodColor returns the appropriate color for an HTTP method
func getMethodColor(method string) *color.Color {
	switch method {
	case "GET":
		return color.New(color.FgGreen, color.Bold)
	case "POST":
		return color.New(color.FgYellow, color.Bold)
	case "PUT":
		return color.New(color.FgBlue, color.Bold)
	case "DELETE":
		return color.New(color.FgRed, color.Bold)
	case "PATCH":
		return color.New(color.FgMagenta, color.Bold)
	default:
		return color.New(color.FgWhite, color.Bold)
	}
}

// printHeader prints a colored header/title line
func printHeader(title string) {
	if useColors() {
		headerColor.Println(title)
	} else {
		fmt.Println(title)
	}
}

// printListIndex formats a list index like [1], [2], etc.
func printListIndex(index int) string {
	if useColors() {
		return indexColor.Sprintf("[%d]", index)
	}
	return fmt.Sprintf("[%d]", index)
}

// printMethod formats an HTTP method with color
func printMethod(method string) string {
	if useColors() {
		return getMethodColor(method).Sprint(method)
	}
	return method
}

// printDimText formats text in dim/muted color
func printDimText(text string) string {
	if useColors() {
		return nameColor.Sprint(text)
	}
	return text
}

// printKeyValue prints a key-value pair with colors
func printKeyValue(key, value string) {
	if useColors() {
		fmt.Printf("  %s = %s\n", keyColor.Sprint(key), valueColor.Sprint(value))
	} else {
		fmt.Printf("  %s = %s\n", key, value)
	}
}

// printSuccess prints a success message
func printSuccess(msg string) {
	if useColors() {
		successColor.Println(msg)
	} else {
		fmt.Println(msg)
	}
}

// printError prints an error message
func printError(msg string) {
	if useColors() {
		errorColor.Println(msg)
	} else {
		fmt.Println(msg)
	}
}

// printMarker prints a marker (like * for current item)
func printMarker(marked bool) string {
	if marked {
		if useColors() {
			return successColor.Sprint("* ")
		}
		return "* "
	}
	return "  "
}

// truncateString truncates a string to maxLen and adds ellipsis
// Deprecated: Use stringutil.Truncate from pkg/internal/stringutil instead
func truncateString(s string, maxLen int) string {
	return stringutil.Truncate(s, maxLen)
}
