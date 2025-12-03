// Package stringutil provides common string manipulation utilities.
package stringutil

// Truncate truncates a string to maxLen characters and adds "..." if truncated.
// If the string is shorter than or equal to maxLen, it returns the original string.
func Truncate(s string, maxLen int) string {
	if maxLen <= 3 {
		if len(s) <= maxLen {
			return s
		}
		return s[:maxLen]
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// TruncateMiddle truncates a string in the middle, preserving the beginning and end.
// This is useful for URLs where both the domain and the endpoint are important.
// Example: "https://api.example.com/v1/users/12345" -> "https://api.example...rs/12345"
func TruncateMiddle(s string, maxLen int) string {
	if maxLen <= 5 {
		return Truncate(s, maxLen)
	}
	if len(s) <= maxLen {
		return s
	}

	// Reserve 3 characters for "..."
	remaining := maxLen - 3
	// Split remaining between start and end
	startLen := remaining / 2
	endLen := remaining - startLen

	return s[:startLen] + "..." + s[len(s)-endLen:]
}
