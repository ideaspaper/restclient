package userinput

import (
	"github.com/ideaspaper/restclient/pkg/tui"
)

// ProcessResult contains the result of processing user input patterns.
type ProcessResult struct {
	URL      string            // The processed URL with patterns replaced
	Values   map[string]string // The values used for replacement (in order of appearance)
	Patterns []Pattern         // The patterns that were found (preserves order)
	Prompted bool              // Whether the user was prompted for values
	Secrets  map[string]bool   // Which parameters are marked as secrets
}

// Prompter handles prompting users for input values.
// Values are collected during a single request execution and not persisted.
type Prompter struct {
	detector  *Detector
	useColors bool
	// collectedValues stores values collected during the current request execution
	// This allows sharing values between URL, headers, body within the same request
	collectedValues map[string]string
	// collectedSecrets tracks which collected values are secrets
	collectedSecrets map[string]bool
}

// NewPrompter creates a new prompter.
// The forcePrompt parameter is kept for API compatibility but is ignored since
// prompts always happen now (no session persistence).
func NewPrompter(_ interface{}, forcePrompt bool, useColors bool) *Prompter {
	return &Prompter{
		detector:         NewDetector(),
		useColors:        useColors,
		collectedValues:  make(map[string]string),
		collectedSecrets: make(map[string]bool),
	}
}

// ProcessURL detects patterns, prompts if needed, and returns processed URL.
// Returns a ProcessResult containing the URL with all {{:paramName}} patterns replaced,
// the values used, and whether the user was prompted.
// If the user cancels the prompt, returns an error.
func (p *Prompter) ProcessURL(url string) (*ProcessResult, error) {
	// Detect patterns in URL
	patterns := p.detector.Detect(url)
	if len(patterns) == 0 {
		// No patterns found, return URL as-is
		return &ProcessResult{
			URL:      url,
			Values:   nil,
			Patterns: nil,
			Prompted: false,
			Secrets:  nil,
		}, nil
	}

	// Build a map of which parameters are secrets (from patterns)
	secrets := make(map[string]bool)
	for _, pattern := range patterns {
		if pattern.IsSecret {
			secrets[pattern.Name] = true
		}
	}

	// Check which patterns need prompting (not already collected in this request)
	var patternsToPrompt []Pattern
	for _, pattern := range patterns {
		if _, ok := p.collectedValues[pattern.Name]; !ok {
			patternsToPrompt = append(patternsToPrompt, pattern)
		}
	}

	needPrompt := len(patternsToPrompt) > 0

	if needPrompt {
		// Build input fields for the form
		fields := make([]tui.InputField, len(patternsToPrompt))
		for i, pattern := range patternsToPrompt {
			fields[i] = tui.InputField{
				Name:     pattern.Name,
				Default:  "",
				IsSecret: secrets[pattern.Name],
			}
		}

		// Show the input form
		newValues, err := tui.RunInputForm(fields, p.useColors)
		if err != nil {
			return nil, err
		}

		// Store the new values in collected values for this request
		for k, v := range newValues {
			p.collectedValues[k] = v
			if secrets[k] {
				p.collectedSecrets[k] = true
			}
		}
	}

	// Build values map from collected values
	values := make(map[string]string, len(patterns))
	for _, pattern := range patterns {
		values[pattern.Name] = p.collectedValues[pattern.Name]
		if p.collectedSecrets[pattern.Name] {
			secrets[pattern.Name] = true
		}
	}

	// Replace patterns in URL
	processedURL := p.detector.Replace(url, values)
	return &ProcessResult{
		URL:      processedURL,
		Values:   values,
		Patterns: patterns,
		Prompted: needPrompt,
		Secrets:  secrets,
	}, nil
}

// ProcessContent processes user input patterns in any content string.
// This can be used for headers, body, or other content.
// The urlKey parameter is kept for API compatibility but is ignored.
func (p *Prompter) ProcessContent(content string, urlKey string) (string, error) {
	// Detect patterns in content
	patterns := p.detector.Detect(content)
	if len(patterns) == 0 {
		return content, nil
	}

	// Build a map of which parameters are secrets (from patterns)
	secrets := make(map[string]bool)
	for _, pattern := range patterns {
		if pattern.IsSecret {
			secrets[pattern.Name] = true
		}
	}

	// Check which patterns need prompting (not already collected in this request)
	var patternsToPrompt []Pattern
	for _, pattern := range patterns {
		if _, ok := p.collectedValues[pattern.Name]; !ok {
			patternsToPrompt = append(patternsToPrompt, pattern)
		}
	}

	needPrompt := len(patternsToPrompt) > 0

	if needPrompt {
		fields := make([]tui.InputField, len(patternsToPrompt))
		for i, pattern := range patternsToPrompt {
			fields[i] = tui.InputField{
				Name:     pattern.Name,
				Default:  "",
				IsSecret: secrets[pattern.Name],
			}
		}

		newValues, err := tui.RunInputForm(fields, p.useColors)
		if err != nil {
			return "", err
		}

		// Store the new values in collected values for this request
		for k, v := range newValues {
			p.collectedValues[k] = v
			if secrets[k] {
				p.collectedSecrets[k] = true
			}
		}
	}

	// Build values map from collected values
	values := make(map[string]string, len(patterns))
	for _, pattern := range patterns {
		values[pattern.Name] = p.collectedValues[pattern.Name]
	}

	// Use ReplaceRaw for content (headers, body, multipart) - no URL encoding needed
	processedContent := p.detector.ReplaceRaw(content, values)
	return processedContent, nil
}

// HasPatterns checks if the URL contains any user input patterns.
func (p *Prompter) HasPatterns(url string) bool {
	return p.detector.HasPatterns(url)
}

// GenerateKey creates a session storage key from a URL pattern.
// This method is kept for API compatibility but the key is not used for persistence.
func (p *Prompter) GenerateKey(url string) string {
	return p.detector.GenerateKey(url)
}

// SetValues allows pre-setting values (useful for testing or programmatic use).
func (p *Prompter) SetValues(values map[string]string) {
	for k, v := range values {
		p.collectedValues[k] = v
	}
}

// SetSecrets marks specific parameters as secrets.
func (p *Prompter) SetSecrets(secrets map[string]bool) {
	for k, v := range secrets {
		p.collectedSecrets[k] = v
	}
}
