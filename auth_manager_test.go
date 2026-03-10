package cs_ai

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFileAuthManager_RotateOnRateLimit(t *testing.T) {
	manager := NewFileAuthManagerWithPath(filepath.Join(t.TempDir(), "auth-profiles.json"))

	_, err := manager.UpsertOAuthProfile("openai-codex", OAuthProfileInput{
		Access:  "token-a",
		Refresh: "refresh-a",
		Expires: time.Now().Add(2 * time.Hour).UnixMilli(),
		Email:   "a@example.com",
	})
	require.NoError(t, err)
	_, err = manager.UpsertOAuthProfile("openai-codex", OAuthProfileInput{
		Access:  "token-b",
		Refresh: "refresh-b",
		Expires: time.Now().Add(2 * time.Hour).UnixMilli(),
		Email:   "b@example.com",
	})
	require.NoError(t, err)

	ctx := context.Background()
	selected, err := manager.ResolveAuth(ctx, "session-1", "openai-codex")
	require.NoError(t, err)
	require.Equal(t, "openai-codex:a@example.com", selected.ProfileID)

	err = manager.MarkFailure(ctx, "session-1", "openai-codex", selected.ProfileID, AuthFailureReasonRateLimit)
	require.NoError(t, err)

	next, err := manager.ResolveAuth(ctx, "session-1", "openai-codex")
	require.NoError(t, err)
	require.Equal(t, "openai-codex:b@example.com", next.ProfileID)
}

func TestFileAuthManager_SessionPinAndDisable(t *testing.T) {
	manager := NewFileAuthManagerWithPath(filepath.Join(t.TempDir(), "auth-profiles.json"))

	_, err := manager.UpsertOAuthProfile("openai-codex", OAuthProfileInput{
		Access:  "token-a",
		Refresh: "refresh-a",
		Expires: time.Now().Add(2 * time.Hour).UnixMilli(),
		Email:   "a@example.com",
	})
	require.NoError(t, err)
	_, err = manager.UpsertOAuthProfile("openai-codex", OAuthProfileInput{
		Access:  "token-b",
		Refresh: "refresh-b",
		Expires: time.Now().Add(2 * time.Hour).UnixMilli(),
		Email:   "b@example.com",
	})
	require.NoError(t, err)

	ctx := context.Background()
	first, err := manager.ResolveAuth(ctx, "session-pin", "openai-codex")
	require.NoError(t, err)
	second, err := manager.ResolveAuth(ctx, "session-pin", "openai-codex")
	require.NoError(t, err)
	require.Equal(t, first.ProfileID, second.ProfileID)

	err = manager.MarkFailure(ctx, "session-pin", "openai-codex", first.ProfileID, AuthFailureReasonFull)
	require.NoError(t, err)

	store, err := manager.LoadStore()
	require.NoError(t, err)
	stats := store.UsageStats[first.ProfileID]
	require.Greater(t, stats.DisabledUntil, time.Now().UnixMilli())
	require.Equal(t, string(AuthFailureReasonFull), stats.DisabledReason)
}

func TestFileAuthManager_UpsertOAuthProfile_ClearsLockouts(t *testing.T) {
	manager := NewFileAuthManagerWithPath(filepath.Join(t.TempDir(), "auth-profiles.json"))

	profileID, err := manager.UpsertOAuthProfile("openai-codex", OAuthProfileInput{
		Access:  "token-a-1",
		Refresh: "refresh-a-1",
		Expires: time.Now().Add(2 * time.Hour).UnixMilli(),
		Email:   "a@example.com",
	})
	require.NoError(t, err)
	require.NotEmpty(t, profileID)

	err = manager.MarkFailure(context.Background(), "session-a", "openai-codex", profileID, AuthFailureReasonAuth)
	require.NoError(t, err)

	store, err := manager.LoadStore()
	require.NoError(t, err)
	stats := store.UsageStats[profileID]
	require.Greater(t, stats.DisabledUntil, time.Now().UnixMilli())
	require.Equal(t, string(AuthFailureReasonAuth), stats.DisabledReason)

	// Re-login/upsert the same profile should clear disabled/cooldown windows.
	_, err = manager.UpsertOAuthProfile("openai-codex", OAuthProfileInput{
		Access:  "token-a-2",
		Refresh: "refresh-a-2",
		Expires: time.Now().Add(3 * time.Hour).UnixMilli(),
		Email:   "a@example.com",
	})
	require.NoError(t, err)

	store, err = manager.LoadStore()
	require.NoError(t, err)
	stats = store.UsageStats[profileID]
	require.Equal(t, int64(0), stats.DisabledUntil)
	require.Equal(t, "", stats.DisabledReason)
	require.Equal(t, int64(0), stats.CooldownUntil)
}

func TestFileAuthManager_SingleProfileDisabledStillResolved(t *testing.T) {
	manager := NewFileAuthManagerWithPath(filepath.Join(t.TempDir(), "auth-profiles.json"))

	profileID, err := manager.UpsertOAuthProfile("openai-codex", OAuthProfileInput{
		Access:  "token-a",
		Refresh: "refresh-a",
		Expires: time.Now().Add(2 * time.Hour).UnixMilli(),
		Email:   "a@example.com",
	})
	require.NoError(t, err)
	require.NotEmpty(t, profileID)

	ctx := context.Background()
	err = manager.MarkFailure(ctx, "session-single", "openai-codex", profileID, AuthFailureReasonFull)
	require.NoError(t, err)

	store, err := manager.LoadStore()
	require.NoError(t, err)
	stats := store.UsageStats[profileID]
	require.Greater(t, stats.DisabledUntil, time.Now().UnixMilli())
	require.Equal(t, string(AuthFailureReasonFull), stats.DisabledReason)

	selection, err := manager.ResolveAuth(ctx, "session-single", "openai-codex")
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, profileID, selection.ProfileID)
	require.Equal(t, "token-a", selection.Token)
}
