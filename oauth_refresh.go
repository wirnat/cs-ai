package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var openAIOAuthTokenURL = "https://auth.openai.com/oauth/token"

func refreshOAuthCredential(ctx context.Context, profile AuthProfileCredential) (AuthProfileCredential, error) {
	provider := normalizeProviderName(profile.Provider)
	if provider != "openai-codex" {
		return profile, fmt.Errorf("oauth refresh unsupported for provider %s", profile.Provider)
	}
	if strings.TrimSpace(profile.Refresh) == "" {
		return profile, fmt.Errorf("refresh token is empty")
	}
	clientID := strings.TrimSpace(ResolveOpenAICodexClientID(""))
	endpoint := strings.TrimSpace(openAIOAuthTokenURL)
	if endpoint == "" {
		endpoint = "https://auth.openai.com/oauth/token"
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", strings.TrimSpace(profile.Refresh))
	form.Set("client_id", clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return profile, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpLog := GetHTTPLogger()
	httpLog.LogRequest(http.MethodPost, endpoint, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}, buildOAuthRefreshRequestLogBody(profile.Refresh, clientID), "", provider)

	client := &http.Client{Timeout: 20 * time.Second}
	startTime := time.Now()
	resp, err := client.Do(req)
	durationMs := time.Since(startTime).Milliseconds()
	if err != nil {
		httpLog.LogResponse(0, nil, nil, durationMs, err.Error())
		httpLog.FlushPair()
		return profile, err
	}
	defer resp.Body.Close()

	responseHeaders := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		switch {
		case strings.EqualFold(k, "Set-Cookie"), strings.EqualFold(k, "Authorization"), strings.EqualFold(k, "Cookie"):
			responseHeaders[k] = "[REDACTED]"
		default:
			responseHeaders[k] = strings.Join(v, ", ")
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		httpLog.LogResponse(resp.StatusCode, responseHeaders, nil, durationMs, err.Error())
		httpLog.FlushPair()
		return profile, err
	}

	errMsg := ""
	if resp.StatusCode >= 400 {
		errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	httpLog.LogResponse(resp.StatusCode, responseHeaders, sanitizeOAuthRefreshResponseLogBody(body), durationMs, errMsg)
	httpLog.FlushPair()

	if resp.StatusCode >= 400 {
		return profile, fmt.Errorf("oauth refresh failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	result := map[string]interface{}{}
	if len(strings.TrimSpace(string(body))) > 0 {
		if err = json.Unmarshal(body, &result); err != nil {
			return profile, err
		}
	}

	access := strings.TrimSpace(toString(result["access_token"]))
	if access == "" {
		return profile, fmt.Errorf("oauth refresh response missing access_token")
	}
	refresh := strings.TrimSpace(toString(result["refresh_token"]))
	if refresh == "" {
		refresh = profile.Refresh
	}
	expiresIn := toInt64(result["expires_in"])
	expiresAt := profile.Expires
	if expiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).UnixMilli()
	}

	updated := profile
	updated.Access = access
	updated.Refresh = refresh
	updated.Expires = expiresAt
	if email := extractEmailFromAnyToken(access, toString(result["id_token"])); email != "" {
		updated.Email = email
	}
	return updated, nil
}

func buildOAuthRefreshRequestLogBody(refreshToken string, clientID string) []byte {
	payload := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": maskSecretForLog(refreshToken),
		"client_id":     strings.TrimSpace(clientID),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return raw
}

func sanitizeOAuthRefreshResponseLogBody(body []byte) []byte {
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}

	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}

	redacted := redactOAuthSecretFields(payload)
	raw, err := json.Marshal(redacted)
	if err != nil {
		return body
	}
	return raw
}

func redactOAuthSecretFields(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, val := range typed {
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "access_token", "refresh_token", "id_token":
				out[key] = maskSecretForLog(toString(val))
			default:
				out[key] = redactOAuthSecretFields(val)
			}
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i := range typed {
			out[i] = redactOAuthSecretFields(typed[i])
		}
		return out
	default:
		return value
	}
}

func maskSecretForLog(value string) string {
	secret := strings.TrimSpace(value)
	if secret == "" {
		return ""
	}
	if len(secret) <= 16 {
		return "[REDACTED]"
	}
	return secret[:8] + "..." + secret[len(secret)-6:]
}

func toString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case []byte:
		return string(t)
	default:
		return ""
	}
}

func toInt64(v interface{}) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case float32:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		parsed, _ := strconv.ParseInt(t, 10, 64)
		return parsed
	default:
		return 0
	}
}
