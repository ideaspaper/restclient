package cmd

import (
	"fmt"

	"github.com/fatih/color"
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
	successBold  = color.New(color.FgGreen, color.Bold)
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

// formatKey returns a key string with color if enabled
func formatKey(key string) string {
	if useColors() {
		return keyColor.Sprint(key)
	}
	return key
}

// printMethodURL prints method and URL with bold green color
func printMethodURL(method, url string) {
	if useColors() {
		successBold.Printf("%s %s\n", method, url)
	} else {
		fmt.Printf("%s %s\n", method, url)
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
