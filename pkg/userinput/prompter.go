package userinput

import (
	"github.com/ideaspaper/restclient/pkg/session"
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

// Prompter handles prompting users for input values with session integration.
type Prompter struct {
	session     *session.SessionManager
	detector    *Detector
	forcePrompt bool
	useColors   bool
}

// NewPrompter creates a new prompter with session integration.
// If forcePrompt is true, the user will always be prompted even if values exist in session.
func NewPrompter(session *session.SessionManager, forcePrompt bool, useColors bool) *Prompter {
	return &Prompter{
		session:     session,
		detector:    NewDetector(),
		forcePrompt: forcePrompt,
		useColors:   useColors,
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

	// Generate session key for this URL pattern
	urlKey := p.detector.GenerateKey(url)

	// Get stored entries from session
	storedEntries := make(map[string]session.UserInputEntry)
	if p.session != nil {
		if stored := p.session.GetUserInputs(urlKey); stored != nil {
			storedEntries = stored
		}
	}

	// Build a map of which parameters are secrets (from patterns or stored entries)
	secrets := make(map[string]bool)
	for _, pattern := range patterns {
		if pattern.IsSecret {
			secrets[pattern.Name] = true
		} else if entry, ok := storedEntries[pattern.Name]; ok && entry.IsSecret {
			secrets[pattern.Name] = true
		}
	}

	// Determine if we need to prompt
	needPrompt := p.forcePrompt
	if !needPrompt {
		// Check if any pattern is missing a value
		for _, pattern := range patterns {
			if _, ok := storedEntries[pattern.Name]; !ok {
				needPrompt = true
				break
			}
		}
	}

	var values map[string]string
	if needPrompt {
		// Build input fields for the form
		fields := make([]tui.InputField, len(patterns))
		for i, pattern := range patterns {
			defaultVal := ""
			if entry, ok := storedEntries[pattern.Name]; ok {
				defaultVal = entry.Value
			}
			fields[i] = tui.InputField{
				Name:     pattern.Name,
				Default:  defaultVal,
				IsSecret: secrets[pattern.Name],
			}
		}

		// Show the input form
		var err error
		values, err = tui.RunInputForm(fields, p.useColors)
		if err != nil {
			return nil, err
		}

		// Save the new values to session with secret metadata
		if p.session != nil {
			entries := make(map[string]session.UserInputEntry, len(values))
			for k, v := range values {
				entries[k] = session.UserInputEntry{
					Value:    v,
					IsSecret: secrets[k],
				}
			}
			p.session.SetUserInputs(urlKey, entries)
		}
	} else {
		// Use stored values
		values = make(map[string]string, len(storedEntries))
		for k, entry := range storedEntries {
			values[k] = entry.Value
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
func (p *Prompter) ProcessContent(content string, urlKey string) (string, error) {
	// Detect patterns in content
	patterns := p.detector.Detect(content)
	if len(patterns) == 0 {
		return content, nil
	}

	// Get stored entries from session
	storedEntries := make(map[string]session.UserInputEntry)
	if p.session != nil {
		if stored := p.session.GetUserInputs(urlKey); stored != nil {
			storedEntries = stored
		}
	}

	// Build a map of which parameters are secrets (from patterns or stored entries)
	secrets := make(map[string]bool)
	for _, pattern := range patterns {
		if pattern.IsSecret {
			secrets[pattern.Name] = true
		} else if entry, ok := storedEntries[pattern.Name]; ok && entry.IsSecret {
			secrets[pattern.Name] = true
		}
	}

	// Determine if we need to prompt
	needPrompt := p.forcePrompt
	if !needPrompt {
		for _, pattern := range patterns {
			if _, ok := storedEntries[pattern.Name]; !ok {
				needPrompt = true
				break
			}
		}
	}

	var values map[string]string
	if needPrompt {
		fields := make([]tui.InputField, len(patterns))
		for i, pattern := range patterns {
			defaultVal := ""
			if entry, ok := storedEntries[pattern.Name]; ok {
				defaultVal = entry.Value
			}
			fields[i] = tui.InputField{
				Name:     pattern.Name,
				Default:  defaultVal,
				IsSecret: secrets[pattern.Name],
			}
		}

		var err error
		values, err = tui.RunInputForm(fields, p.useColors)
		if err != nil {
			return "", err
		}

		// Save the new values to session with secret metadata
		if p.session != nil {
			entries := make(map[string]session.UserInputEntry, len(values))
			for k, v := range values {
				entries[k] = session.UserInputEntry{
					Value:    v,
					IsSecret: secrets[k],
				}
			}
			p.session.SetUserInputs(urlKey, entries)
		}
	} else {
		// Use stored values
		values = make(map[string]string, len(storedEntries))
		for k, entry := range storedEntries {
			values[k] = entry.Value
		}
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
func (p *Prompter) GenerateKey(url string) string {
	return p.detector.GenerateKey(url)
}
