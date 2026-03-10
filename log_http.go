package cs_ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultHTTPLogDir = "log/cs-ai-http"

// HTTPLogEntry captures a single API request/response exchange.
type HTTPLogEntry struct {
	Timestamp    string            `json:"timestamp"`
	Direction    string            `json:"direction"` // "REQUEST" or "RESPONSE"
	URL          string            `json:"url,omitempty"`
	Method       string            `json:"method,omitempty"`
	StatusCode   int               `json:"status_code,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Body         json.RawMessage   `json:"body,omitempty"`
	Error        string            `json:"error,omitempty"`
	DurationMs   int64             `json:"duration_ms,omitempty"`
	ModelName    string            `json:"model_name,omitempty"`
	ProviderName string            `json:"provider_name,omitempty"`
}

// HTTPLogger writes structured HTTP logs to disk.
type HTTPLogger struct {
	mu      sync.Mutex
	dir     string
	entries []HTTPLogEntry
}

var (
	globalHTTPLogger     *HTTPLogger
	globalHTTPLoggerOnce sync.Once
)

// GetHTTPLogger returns the singleton HTTP logger.
func GetHTTPLogger() *HTTPLogger {
	globalHTTPLoggerOnce.Do(func() {
		dir := resolveHTTPLogDir()
		globalHTTPLogger = &HTTPLogger{dir: dir}
	})
	return globalHTTPLogger
}

func resolveHTTPLogDir() string {
	if p := strings.TrimSpace(os.Getenv("CS_AI_HTTP_LOG_DIR")); p != "" {
		return p
	}
	if projectRoot := findProjectRoot(); projectRoot != "" {
		return filepath.Join(projectRoot, defaultHTTPLogDir)
	}
	return defaultHTTPLogDir
}

// LogRequest records an outgoing API request.
func (l *HTTPLogger) LogRequest(method, url string, headers map[string]string, body []byte, modelName, providerName string) {
	if l == nil {
		return
	}

	// Redact sensitive headers
	safeHeaders := make(map[string]string, len(headers))
	for k, v := range headers {
		if strings.EqualFold(k, "Authorization") {
			if len(v) > 20 {
				safeHeaders[k] = v[:10] + "…" + v[len(v)-6:]
			} else {
				safeHeaders[k] = "[REDACTED]"
			}
		} else {
			safeHeaders[k] = v
		}
	}

	entry := HTTPLogEntry{
		Timestamp:    time.Now().Format(time.RFC3339Nano),
		Direction:    "REQUEST",
		URL:          url,
		Method:       method,
		Headers:      safeHeaders,
		Body:         compactOrRaw(body),
		ModelName:    modelName,
		ProviderName: providerName,
	}

	l.mu.Lock()
	l.entries = append(l.entries, entry)
	l.mu.Unlock()
}

// LogResponse records an incoming API response.
func (l *HTTPLogger) LogResponse(statusCode int, headers map[string]string, body []byte, durationMs int64, errMsg string) {
	if l == nil {
		return
	}

	entry := HTTPLogEntry{
		Timestamp:  time.Now().Format(time.RFC3339Nano),
		Direction:  "RESPONSE",
		StatusCode: statusCode,
		Headers:    headers,
		Body:       compactOrRaw(body),
		DurationMs: durationMs,
		Error:      errMsg,
	}

	l.mu.Lock()
	l.entries = append(l.entries, entry)
	l.mu.Unlock()
}

// Flush writes all buffered entries to a timestamped log file and clears the buffer.
func (l *HTTPLogger) Flush() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	if len(l.entries) == 0 {
		l.mu.Unlock()
		return nil
	}
	snapshot := make([]HTTPLogEntry, len(l.entries))
	copy(snapshot, l.entries)
	l.entries = l.entries[:0]
	l.mu.Unlock()

	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return fmt.Errorf("failed to create http log dir: %w", err)
	}

	fileName := fmt.Sprintf("%s.jsonl", time.Now().Format("2006-01-02"))
	filePath := filepath.Join(l.dir, fileName)

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open http log file: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, entry := range snapshot {
		if err := encoder.Encode(entry); err != nil {
			return fmt.Errorf("failed to write http log entry: %w", err)
		}
	}

	return nil
}

// FlushPair is a convenience method to flush after each request/response pair.
func (l *HTTPLogger) FlushPair() {
	if l == nil {
		return
	}
	_ = l.Flush()
}

func compactOrRaw(data []byte) json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	// Try to validate as JSON; if valid, return as-is for structured logging.
	var js json.RawMessage
	if json.Unmarshal(data, &js) == nil {
		return js
	}
	// Not valid JSON, wrap as string.
	quoted, _ := json.Marshal(string(data))
	return quoted
}
