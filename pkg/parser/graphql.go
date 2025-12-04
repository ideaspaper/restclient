package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// createGraphQLBody wraps the body in GraphQL JSON format
func createGraphQLBody(body string) string {
	// Split into query and variables
	parts := strings.SplitN(body, "\n\n", 2)
	query := parts[0]
	variables := "{}"
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		variables = strings.TrimSpace(parts[1])
	}

	// Extract operation name from query, mutation, or subscription
	operationName := ""
	opRegex := regexp.MustCompile(`^\s*(?:query|mutation|subscription)\s+(\w+)`)
	if matches := opRegex.FindStringSubmatch(query); matches != nil {
		operationName = matches[1]
	}

	// Build JSON payload
	query = strings.ReplaceAll(query, "\\", "\\\\")
	query = strings.ReplaceAll(query, "\"", "\\\"")
	query = strings.ReplaceAll(query, "\n", "\\n")
	query = strings.ReplaceAll(query, "\r", "\\r")
	query = strings.ReplaceAll(query, "\t", "\\t")

	result := fmt.Sprintf(`{"query":"%s"`, query)
	if operationName != "" {
		result += fmt.Sprintf(`,"operationName":"%s"`, operationName)
	}
	result += fmt.Sprintf(`,"variables":%s}`, variables)

	return result
}
