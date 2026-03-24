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

func TestFileAuthManager_ResolveAuth_SetsRefreshFailedFlagOnRefreshError(t *testing.T) {
	manager := NewFileAuthManagerWithPath(filepath.Join(t.TempDir(), "auth-profiles.json"))

	profileID, err := manager.UpsertOAuthProfile("openai-codex", OAuthProfileInput{
		Access:  "token-a",
		Refresh: "refresh-a",
		Expires: time.Now().Add(-1 * time.Hour).UnixMilli(),
		Email:   "a@example.com",
	})
	require.NoError(t, err)
	require.NotEmpty(t, profileID)

	manager.refreshFunc = func(_ context.Context, _ AuthProfileCredential) (AuthProfileCredential, error) {
		return AuthProfileCredential{}, context.DeadlineExceeded
	}

	selection, err := manager.ResolveAuth(context.Background(), "session-refresh-fail", "openai-codex")
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, profileID, selection.ProfileID)

	store, err := manager.LoadStore()
	require.NoError(t, err)
	cred, ok := store.Profiles[profileID]
	require.True(t, ok)
	require.True(t, cred.RefreshFailed)
}

func TestFileAuthManager_ResolveAuth_ClearsRefreshFailedFlagOnRefreshSuccess(t *testing.T) {
	manager := NewFileAuthManagerWithPath(filepath.Join(t.TempDir(), "auth-profiles.json"))

	profileID, err := manager.UpsertOAuthProfile("openai-codex", OAuthProfileInput{
		Access:  "token-a",
		Refresh: "refresh-a",
		Expires: time.Now().Add(-1 * time.Hour).UnixMilli(),
		Email:   "a@example.com",
	})
	require.NoError(t, err)
	require.NotEmpty(t, profileID)

	manager.refreshFunc = func(_ context.Context, _ AuthProfileCredential) (AuthProfileCredential, error) {
		return AuthProfileCredential{}, context.DeadlineExceeded
	}
	_, err = manager.ResolveAuth(context.Background(), "session-refresh-fail", "openai-codex")
	require.NoError(t, err)

	manager.refreshFunc = func(_ context.Context, profile AuthProfileCredential) (AuthProfileCredential, error) {
		profile.Access = "token-a-refreshed"
		profile.Expires = time.Now().Add(2 * time.Hour).UnixMilli()
		return profile, nil
	}
	selection, err := manager.ResolveAuth(context.Background(), "session-refresh-ok", "openai-codex")
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, "token-a-refreshed", selection.Token)

	store, err := manager.LoadStore()
	require.NoError(t, err)
	cred, ok := store.Profiles[profileID]
	require.True(t, ok)
	require.False(t, cred.RefreshFailed)
}

func TestFileAuthManager_ResolveAuth_PersistsRefreshFailedWhenNoProfileAvailable(t *testing.T) {
	manager := NewFileAuthManagerWithPath(filepath.Join(t.TempDir(), "auth-profiles.json"))

	profileA, err := manager.UpsertOAuthProfile("openai-codex", OAuthProfileInput{
		Access:  "token-a",
		Refresh: "refresh-a",
		Expires: time.Now().Add(-1 * time.Hour).UnixMilli(),
		Email:   "a@example.com",
	})
	require.NoError(t, err)
	require.NotEmpty(t, profileA)

	profileB, err := manager.UpsertOAuthProfile("openai-codex", OAuthProfileInput{
		Access:  "token-b",
		Refresh: "refresh-b",
		Expires: time.Now().Add(-1 * time.Hour).UnixMilli(),
		Email:   "b@example.com",
	})
	require.NoError(t, err)
	require.NotEmpty(t, profileB)

	manager.refreshFunc = func(_ context.Context, _ AuthProfileCredential) (AuthProfileCredential, error) {
		return AuthProfileCredential{}, context.DeadlineExceeded
	}

	selection, err := manager.ResolveAuth(context.Background(), "session-refresh-fail-all", "openai-codex")
	require.Error(t, err)
	require.Nil(t, selection)
	require.Contains(t, err.Error(), "no available auth profile")

	store, err := manager.LoadStore()
	require.NoError(t, err)

	credA, ok := store.Profiles[profileA]
	require.True(t, ok)
	require.True(t, credA.RefreshFailed)

	credB, ok := store.Profiles[profileB]
	require.True(t, ok)
	require.True(t, credB.RefreshFailed)
}

func TestUpdateProfileRateLimitStats_TracksFiveHourAndWeeklyWindows(t *testing.T) {
	now := time.Unix(1773137048, 0)
	stats := ProfileUsageStats{}
	headers := map[string]string{
		"X-Codex-Active-Limit":                         "codex",
		"X-Codex-Plan-Type":                            "plus",
		"X-Codex-Credits-Has-Credits":                  "False",
		"X-Codex-Credits-Unlimited":                    "False",
		"X-Codex-Primary-Window-Minutes":               "300",
		"X-Codex-Primary-Used-Percent":                 "17",
		"X-Codex-Primary-Reset-At":                     "1773151806",
		"X-Codex-Primary-Reset-After-Seconds":          "14758",
		"X-Codex-Primary-Over-Secondary-Limit-Percent": "0",
		"X-Codex-Secondary-Window-Minutes":             "10080",
		"X-Codex-Secondary-Used-Percent":               "7",
		"X-Codex-Secondary-Reset-At":                   "1773720582",
		"X-Codex-Secondary-Reset-After-Seconds":        "583535",
	}

	updateProfileRateLimitStats(&stats, 200, headers, now)
	require.NotNil(t, stats.RateLimit)
	require.Equal(t, 200, stats.RateLimit.LastStatusCode)
	require.Equal(t, "codex", stats.RateLimit.ActiveLimit)
	require.Equal(t, "plus", stats.RateLimit.PlanType)
	require.Equal(t, int64(1), stats.RateLimit.FiveHour.RequestCount)
	require.Equal(t, int64(1), stats.RateLimit.Weekly.RequestCount)
	require.Equal(t, 300, stats.RateLimit.FiveHour.WindowMinutes)
	require.Equal(t, 10080, stats.RateLimit.Weekly.WindowMinutes)
	require.Equal(t, 17, stats.RateLimit.FiveHour.UsedPercent)
	require.Equal(t, 7, stats.RateLimit.Weekly.UsedPercent)
	require.Equal(t, int64(1773151806), stats.RateLimit.FiveHour.ResetAt)
	require.Equal(t, int64(1773720582), stats.RateLimit.Weekly.ResetAt)
}

func TestUpdateProfileRateLimitStats_ResetsCounterWhenWindowCycleChanges(t *testing.T) {
	baseTime := time.Unix(1773137048, 0)
	stats := ProfileUsageStats{}

	headers := map[string]string{
		"X-Codex-Primary-Window-Minutes":   "300",
		"X-Codex-Primary-Reset-At":         "1773151806",
		"X-Codex-Secondary-Window-Minutes": "10080",
		"X-Codex-Secondary-Reset-At":       "1773720582",
	}

	updateProfileRateLimitStats(&stats, 200, headers, baseTime)
	updateProfileRateLimitStats(&stats, 200, headers, baseTime.Add(30*time.Second))
	require.NotNil(t, stats.RateLimit)
	require.Equal(t, int64(2), stats.RateLimit.FiveHour.RequestCount)
	require.Equal(t, int64(2), stats.RateLimit.Weekly.RequestCount)

	nextCycleHeaders := map[string]string{
		"X-Codex-Primary-Window-Minutes":   "300",
		"X-Codex-Primary-Reset-At":         "1773169806",
		"X-Codex-Secondary-Window-Minutes": "10080",
		"X-Codex-Secondary-Reset-At":       "1774325382",
	}
	updateProfileRateLimitStats(&stats, 200, nextCycleHeaders, baseTime.Add(6*time.Hour))
	require.Equal(t, int64(1), stats.RateLimit.FiveHour.RequestCount)
	require.Equal(t, int64(1), stats.RateLimit.Weekly.RequestCount)
}
