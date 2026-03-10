package cs_ai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestDetailed_RedactsAuthorizationHeaderInLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	// Reset global HTTP logger singleton to use a temp dir for this test.
	tmpDir := t.TempDir()
	globalHTTPLoggerOnce = sync.Once{}
	globalHTTPLogger = nil
	t.Setenv("CS_AI_HTTP_LOG_DIR", tmpDir)

	// Capture stdout to verify the secret doesn't leak there.
	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	_, _, reqErr := RequestDetailed(server.URL, http.MethodPost, map[string]interface{}{"hello": "world"}, func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer super-secret-token")
	})
	require.NoError(t, reqErr)

	_ = w.Close()
	os.Stdout = originalStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	stdoutLogged := buf.String()

	// The full secret token must never appear in stdout.
	require.NotContains(t, stdoutLogged, "super-secret-token")

	// The JSONL log file should exist and contain a redacted Authorization value.
	entries, readErr := os.ReadDir(tmpDir)
	require.NoError(t, readErr)
	require.NotEmpty(t, entries, "expected at least one log file in %s", tmpDir)

	logContent, readErr := os.ReadFile(filepath.Join(tmpDir, entries[0].Name()))
	require.NoError(t, readErr)
	logStr := string(logContent)

	// The full secret must not appear in the log file.
	require.NotContains(t, logStr, "super-secret-token")
	// A truncated/redacted form should be present.
	require.True(t, strings.Contains(logStr, "Bearer su") || strings.Contains(logStr, "REDACTED") || strings.Contains(logStr, "…"),
		"expected redacted Authorization in log file, got: %s", logStr)
}
