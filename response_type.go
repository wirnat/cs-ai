package cs_ai

import (
	"encoding/json"
	"strings"
)

type ResponseType string

const (
	ResponseTypeText     ResponseType = "text"
	ResponseTypeJSON     ResponseType = "json"
	ResponseTypeMarkdown ResponseType = "markdown"
	ResponseTypeHTML     ResponseType = "html"
)

// validateResponseType checks if the response content matches the expected response type
func validateResponseType(content string, responseType ResponseType) bool {
	switch responseType {
	case ResponseTypeJSON:
		var js json.RawMessage
		return json.Unmarshal([]byte(content), &js) == nil
	case ResponseTypeMarkdown:
		// Basic markdown validation - check for common markdown patterns
		markdownPatterns := []string{
			"#", "##", "###", // Headers
			"*", "_", // Emphasis
			"```", "`", // Code blocks
			"- ", "+ ", // Lists
			"[", "]", // Links
		}
		for _, pattern := range markdownPatterns {
			if strings.Contains(content, pattern) {
				return true
			}
		}
		return false
	case ResponseTypeHTML:
		// Basic HTML validation - check for HTML tags
		htmlPatterns := []string{
			"<html", "<body", "<div", "<p", "<span",
			"<h1", "<h2", "<h3", "<h4", "<h5", "<h6",
			"<ul", "<ol", "<li", "<a", "<img",
		}
		for _, pattern := range htmlPatterns {
			if strings.Contains(strings.ToLower(content), pattern) {
				return true
			}
		}
		return false
	case ResponseTypeText:
		// Text is the default type, always valid
		return true
	default:
		return true
	}
}
