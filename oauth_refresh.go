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

func refreshOAuthCredential(ctx context.Context, profile AuthProfileCredential) (AuthProfileCredential, error) {
	if normalizeProviderName(profile.Provider) != "openai-codex" {
		return profile, fmt.Errorf("oauth refresh unsupported for provider %s", profile.Provider)
	}
	if strings.TrimSpace(profile.Refresh) == "" {
		return profile, fmt.Errorf("refresh token is empty")
	}
	clientID := strings.TrimSpace(ResolveOpenAICodexClientID(""))

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", strings.TrimSpace(profile.Refresh))
	form.Set("client_id", clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return profile, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return profile, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return profile, err
	}

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
