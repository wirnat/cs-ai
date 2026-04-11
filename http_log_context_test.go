package cs_ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestDetailedWithContext_LogsSessionTurnStageAndHopMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	globalHTTPLoggerOnce = sync.Once{}
	globalHTTPLogger = nil
	t.Setenv("CS_AI_HTTP_LOG_DIR", tmpDir)

	ctx := context.Background()
	ctx = withStreamRuntime(ctx, &streamRuntime{turnID: "turn-http-meta"})
	ctx = withStageStreaming(ctx, AgentStageAnswer, StageStreamingConfig{})
	ctx = WithHTTPLogMetadata(ctx, HTTPLogMetadata{
		SessionID:    "session-http-meta",
		RequestKind:  "answer.after_tools",
		Hop:          2,
		ProviderName: "openai-codex",
	})

	_, _, _, err := RequestDetailedWithContext(ctx, server.URL, http.MethodPost, map[string]interface{}{"hello": "world"}, func(request *http.Request) {})
	require.NoError(t, err)

	entries, readErr := os.ReadDir(tmpDir)
	require.NoError(t, readErr)
	require.Len(t, entries, 1)

	raw, readErr := os.ReadFile(filepath.Join(tmpDir, entries[0].Name()))
	require.NoError(t, readErr)

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	require.Len(t, lines, 2)

	var requestEntry HTTPLogEntry
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &requestEntry))
	require.Equal(t, "REQUEST", requestEntry.Direction)
	require.Equal(t, "session-http-meta", requestEntry.SessionID)
	require.Equal(t, "turn-http-meta", requestEntry.TurnID)
	require.Equal(t, "answer", requestEntry.Stage)
	require.Equal(t, "answer.after_tools", requestEntry.RequestKind)
	require.Equal(t, 2, requestEntry.Hop)
	require.Equal(t, "openai-codex", requestEntry.ProviderName)

	var responseEntry HTTPLogEntry
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &responseEntry))
	require.Equal(t, "RESPONSE", responseEntry.Direction)
	require.Equal(t, "session-http-meta", responseEntry.SessionID)
	require.Equal(t, "turn-http-meta", responseEntry.TurnID)
	require.Equal(t, "answer", responseEntry.Stage)
	require.Equal(t, "answer.after_tools", responseEntry.RequestKind)
	require.Equal(t, 2, responseEntry.Hop)
	require.Equal(t, "openai-codex", responseEntry.ProviderName)
}
