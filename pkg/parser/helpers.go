package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// readFileContent reads content from a file relative to the parser's base directory
func (p *HttpRequestParser) readFileContent(filePath, encoding string) (string, error) {
	// Try absolute path first
	if filepath.IsAbs(filePath) {
		return readFile(filePath, encoding)
	}

	// Try relative to base directory
	if p.baseDir != "" {
		absPath := filepath.Join(p.baseDir, filePath)
		if content, err := readFile(absPath, encoding); err == nil {
			return content, nil
		}
	}

	// Try current working directory
	cwd, _ := os.Getwd()
	absPath := filepath.Join(cwd, filePath)
	return readFile(absPath, encoding)
}

// readFile reads a file and returns its content.
// The encoding parameter is reserved for future use (e.g., handling different character encodings).
func readFile(path, _ string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}

// isFormUrlEncoded checks if content type is form-urlencoded
func isFormUrlEncoded(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "application/x-www-form-urlencoded")
}
