package cs_ai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRefreshOAuthCredential_LogsRequestAndResponseWithRedactedTokens(t *testing.T) {
	const (
		requestRefreshToken  = "rt_request_super_secret_token_123456789"
		responseAccessToken  = "at_response_super_secret_access_123456789"
		responseRefreshToken = "rt_response_super_secret_refresh_987654321"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/oauth/token", r.URL.Path)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		form, err := url.ParseQuery(string(raw))
		require.NoError(t, err)
		require.Equal(t, "refresh_token", form.Get("grant_type"))
		require.Equal(t, requestRefreshToken, form.Get("refresh_token"))
		require.Equal(t, OpenAICodexOAuthDefaultClientID, form.Get("client_id"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"` + responseAccessToken + `","refresh_token":"` + responseRefreshToken + `","expires_in":3600}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	globalHTTPLoggerOnce = sync.Once{}
	globalHTTPLogger = nil
	t.Setenv("CS_AI_HTTP_LOG_DIR", tmpDir)

	oldEndpoint := openAIOAuthTokenURL
	openAIOAuthTokenURL = server.URL + "/oauth/token"
	t.Cleanup(func() {
		openAIOAuthTokenURL = oldEndpoint
	})

	before := time.Now().UnixMilli()
	updated, err := refreshOAuthCredential(context.Background(), AuthProfileCredential{
		Type:     "oauth",
		Provider: "openai-codex",
		Refresh:  requestRefreshToken,
		Expires:  before - 1000,
	})
	require.NoError(t, err)
	require.Equal(t, responseAccessToken, updated.Access)
	require.Equal(t, responseRefreshToken, updated.Refresh)
	require.Greater(t, updated.Expires, before)

	logFiles, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.NotEmpty(t, logFiles)

	logPath := filepath.Join(tmpDir, logFiles[0].Name())
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	logStr := string(content)

	require.Contains(t, logStr, openAIOAuthTokenURL)
	require.Contains(t, logStr, maskSecretForLog(requestRefreshToken))
	require.Contains(t, logStr, maskSecretForLog(responseAccessToken))
	require.Contains(t, logStr, maskSecretForLog(responseRefreshToken))
	require.NotContains(t, logStr, requestRefreshToken)
	require.NotContains(t, logStr, responseAccessToken)
	require.NotContains(t, logStr, responseRefreshToken)
}
