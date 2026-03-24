package cs_ai

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	authStoreVersion = 1
)

type AuthProfileCredential struct {
	Type     string `json:"type"`
	Provider string `json:"provider"`
	Access   string `json:"access,omitempty"`
	Refresh  string `json:"refresh,omitempty"`
	Expires  int64  `json:"expires,omitempty"`
	Email    string `json:"email,omitempty"`
	// RefreshFailed menandai percobaan refresh token terakhir gagal.
	RefreshFailed bool `json:"refresh_failed,omitempty" bson:"refresh_failed,omitempty"`
}

type ProfileUsageStats struct {
	LastUsed       int64                     `json:"last_used,omitempty"`
	CooldownUntil  int64                     `json:"cooldown_until,omitempty"`
	DisabledUntil  int64                     `json:"disabled_until,omitempty"`
	DisabledReason string                    `json:"disabled_reason,omitempty"`
	ErrorCount     int                       `json:"error_count,omitempty"`
	FailureCounts  map[AuthFailureReason]int `json:"failure_counts,omitempty"`
	LastFailureAt  int64                     `json:"last_failure_at,omitempty"`
	RateLimit      *ProfileRateLimitStats    `json:"rate_limit,omitempty"`
}

type ProfileRateLimitWindowStats struct {
	WindowMinutes             int   `json:"window_minutes,omitempty"`
	UsedPercent               int   `json:"used_percent,omitempty"`
	OverSecondaryLimitPercent int   `json:"over_secondary_limit_percent,omitempty"`
	ResetAt                   int64 `json:"reset_at,omitempty"`
	ResetAfterSeconds         int64 `json:"reset_after_seconds,omitempty"`
	WindowStart               int64 `json:"window_start,omitempty"`
	RequestCount              int64 `json:"request_count,omitempty"`
	UpdatedAt                 int64 `json:"updated_at,omitempty"`
}

type ProfileRateLimitStats struct {
	LastStatusCode int   `json:"last_status_code,omitempty"`
	LastRequestAt  int64 `json:"last_request_at,omitempty"`

	ActiveLimit string `json:"active_limit,omitempty"`
	PlanType    string `json:"plan_type,omitempty"`
	HasCredits  string `json:"has_credits,omitempty"`
	Unlimited   string `json:"unlimited,omitempty"`

	FiveHour ProfileRateLimitWindowStats `json:"five_hour,omitempty"`
	Weekly   ProfileRateLimitWindowStats `json:"weekly,omitempty"`
}

type SessionPin struct {
	ProfileID string `json:"profile_id"`
	UpdatedAt int64  `json:"updated_at"`
}

type AuthProfileStore struct {
	Version     int                              `json:"version"`
	Profiles    map[string]AuthProfileCredential `json:"profiles"`
	Order       map[string][]string              `json:"order,omitempty"`
	UsageStats  map[string]ProfileUsageStats     `json:"usage_stats,omitempty"`
	SessionPins map[string]SessionPin            `json:"session_pins,omitempty"`
}

type OAuthProfileInput struct {
	Access  string
	Refresh string
	Expires int64
	Email   string
}

type AuthProfileView struct {
	ProfileID      string
	Provider       string
	Type           string
	Email          string
	Expires        int64
	LastUsed       int64
	CooldownUntil  int64
	DisabledUntil  int64
	DisabledReason string
}

type OAuthRefreshFunc func(ctx context.Context, profile AuthProfileCredential) (AuthProfileCredential, error)

type FileAuthManager struct {
	storePath   string
	now         func() time.Time
	refreshFunc OAuthRefreshFunc
}

func NewFileAuthManager() *FileAuthManager {
	return &FileAuthManager{
		storePath:   ResolveDefaultAuthStorePath(),
		now:         time.Now,
		refreshFunc: refreshOAuthCredential,
	}
}

func NewFileAuthManagerWithPath(path string) *FileAuthManager {
	m := NewFileAuthManager()
	if strings.TrimSpace(path) != "" {
		m.storePath = path
	}
	return m
}

func ResolveDefaultAuthStorePath() string {
	if p := strings.TrimSpace(os.Getenv("CS_AI_AUTH_STORE_PATH")); p != "" {
		return p
	}

	// Prefer project-root-relative path by walking up from CWD to find go.mod.
	// This ensures auth-profiles live alongside the consumer project, not in a
	// global SDK-specific directory.
	if projectRoot := findProjectRoot(); projectRoot != "" {
		return filepath.Join(projectRoot, ".cs-ai", "auth-profiles.json")
	}

	// Fallback: global home directory when no project root is detected.
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ".cs-ai/auth-profiles.json"
	}
	return filepath.Join(home, ".cs-ai", "auth-profiles.json")
}

// findProjectRoot walks up from the current working directory looking for a
// go.mod file, which indicates the root of a Go project. If the walk-up fails
// (common when CWD is an IDE workspace root), it also checks immediate
// subdirectories of CWD for go.mod. Returns the directory containing go.mod,
// or "" if none is found.
func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Strategy 1: walk up from CWD.
	if root := walkUpForGoMod(dir); root != "" {
		return root
	}

	// Strategy 2: check immediate subdirectories of CWD (handles IDE
	// multi-root workspaces where CWD is the workspace root and the Go
	// project lives one level down, e.g. workspace/go-hairkatz/go.mod).
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return ""
	}

	var firstCandidate string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		goModPath := filepath.Join(dir, entry.Name(), "go.mod")
		if _, statErr := os.Stat(goModPath); statErr != nil {
			continue
		}
		candidate := filepath.Join(dir, entry.Name())
		// Prefer the subdir whose go.mod imports cs-ai.
		if goModContent, readErr := os.ReadFile(goModPath); readErr == nil {
			if strings.Contains(string(goModContent), "wirnat/cs-ai") {
				return candidate
			}
		}
		if firstCandidate == "" {
			firstCandidate = candidate
		}
	}
	return firstCandidate
}

func walkUpForGoMod(dir string) string {
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func (m *FileAuthManager) StorePath() string {
	if m == nil {
		return ""
	}
	return m.storePath
}

func (m *FileAuthManager) ResolveAuth(ctx context.Context, sessionID string, provider string) (*AuthSelection, error) {
	if m == nil {
		return nil, fmt.Errorf("auth manager is nil")
	}
	provider = normalizeProviderName(provider)
	if provider == "" {
		return nil, fmt.Errorf("provider is required")
	}

	var selected *AuthSelection
	var refreshedProfile *AuthProfileCredential
	var resolveErr error
	err := m.withLockedStore(func(store *AuthProfileStore) (bool, error) {
		now := m.now().UnixMilli()
		changed := clearExpiredWindows(store, now)

		ordered := resolveProviderOrder(store, provider)
		if len(ordered) == 0 {
			resolveErr = fmt.Errorf("no auth profile configured for provider %s", provider)
			return changed, nil
		}

		pinKey := sessionProviderKey(sessionID, provider)
		if pin, ok := store.SessionPins[pinKey]; ok && pin.ProfileID != "" {
			ordered = moveProfileToFront(ordered, pin.ProfileID)
		}

		for _, profileID := range ordered {
			cred, exists := store.Profiles[profileID]
			if !exists {
				continue
			}
			if normalizeProviderName(cred.Provider) != provider {
				continue
			}
			if isProfileUnusable(store, profileID, now) {
				continue
			}
			if strings.TrimSpace(cred.Access) == "" {
				continue
			}

			if cred.Expires > 0 && now >= cred.Expires {
				refreshed, refreshErr := m.refreshFunc(ctx, cred)
				if refreshErr != nil {
					cred.RefreshFailed = true
					store.Profiles[profileID] = cred
					markFailure(store, profileID, AuthFailureReasonAuth, now)
					changed = true
					continue
				}
				if strings.TrimSpace(refreshed.Access) == "" {
					refreshed.RefreshFailed = true
					store.Profiles[profileID] = refreshed
					markFailure(store, profileID, AuthFailureReasonAuth, now)
					changed = true
					continue
				}
				refreshed.RefreshFailed = false
				store.Profiles[profileID] = refreshed
				cred = refreshed
				refreshedCopy := refreshed
				refreshedProfile = &refreshedCopy
				changed = true
			}

			if sessionID != "" {
				store.SessionPins[pinKey] = SessionPin{ProfileID: profileID, UpdatedAt: now}
				changed = true
			}

			selected = &AuthSelection{
				Provider:  provider,
				ProfileID: profileID,
				Token:     cred.Access,
			}
			return changed, nil
		}

		// Single-profile fallback:
		// If there is only one configured profile for this provider and it is
		// currently in cooldown/disabled window, we still return it so runtime
		// can attempt OpenAI first before model fallback (DeepSeek). This keeps
		// behaviour intuitive for 1-account setups where lockout signals may be
		// overly conservative.
		if len(ordered) == 1 {
			profileID := ordered[0]
			cred, exists := store.Profiles[profileID]
			if exists && normalizeProviderName(cred.Provider) == provider && strings.TrimSpace(cred.Access) != "" {
				// Try refresh when expired; if refresh fails, still try current
				// token as last-resort.
				if cred.Expires > 0 && now >= cred.Expires {
					refreshed, refreshErr := m.refreshFunc(ctx, cred)
					if refreshErr == nil && strings.TrimSpace(refreshed.Access) != "" {
						refreshed.RefreshFailed = false
						store.Profiles[profileID] = refreshed
						cred = refreshed
						refreshedCopy := refreshed
						refreshedProfile = &refreshedCopy
						changed = true
					} else {
						cred.RefreshFailed = true
						store.Profiles[profileID] = cred
						changed = true
					}
				}
				if strings.TrimSpace(cred.Access) != "" {
					if sessionID != "" {
						store.SessionPins[pinKey] = SessionPin{ProfileID: profileID, UpdatedAt: now}
						changed = true
					}
					selected = &AuthSelection{
						Provider:  provider,
						ProfileID: profileID,
						Token:     cred.Access,
					}
					return changed, nil
				}
			}
		}

		resolveErr = fmt.Errorf("no available auth profile for provider %s", provider)
		return changed, nil
	})
	if err != nil {
		return nil, err
	}
	if selected == nil {
		if resolveErr != nil {
			return nil, resolveErr
		}
		return nil, fmt.Errorf("no available auth profile for provider %s", provider)
	}
	if refreshedProfile != nil {
		if mirrorErr := mirrorRefreshedOAuthProfile(ctx, refreshMirrorSourceFile, *refreshedProfile); mirrorErr != nil {
			return nil, mirrorErr
		}
	}
	return selected, nil
}

func (m *FileAuthManager) MarkSuccess(ctx context.Context, sessionID string, provider string, profileID string) error {
	_ = ctx
	if m == nil || strings.TrimSpace(profileID) == "" {
		return nil
	}
	provider = normalizeProviderName(provider)
	return m.withLockedStore(func(store *AuthProfileStore) (bool, error) {
		now := m.now().UnixMilli()
		stats := store.UsageStats[profileID]
		stats.LastUsed = now
		stats.ErrorCount = 0
		stats.FailureCounts = map[AuthFailureReason]int{}
		stats.CooldownUntil = 0
		stats.DisabledUntil = 0
		stats.DisabledReason = ""
		store.UsageStats[profileID] = stats

		if sessionID != "" {
			store.SessionPins[sessionProviderKey(sessionID, provider)] = SessionPin{ProfileID: profileID, UpdatedAt: now}
		}
		return true, nil
	})
}

func (m *FileAuthManager) MarkFailure(ctx context.Context, sessionID string, provider string, profileID string, reason AuthFailureReason) error {
	_ = ctx
	if m == nil || strings.TrimSpace(profileID) == "" {
		return nil
	}
	provider = normalizeProviderName(provider)
	return m.withLockedStore(func(store *AuthProfileStore) (bool, error) {
		now := m.now().UnixMilli()
		markFailure(store, profileID, reason, now)
		if sessionID != "" {
			pinKey := sessionProviderKey(sessionID, provider)
			if pin, ok := store.SessionPins[pinKey]; ok && pin.ProfileID == profileID {
				delete(store.SessionPins, pinKey)
			}
		}
		return true, nil
	})
}

func (m *FileAuthManager) UpsertOAuthProfile(provider string, input OAuthProfileInput) (string, error) {
	provider = normalizeProviderName(provider)
	if provider == "" {
		return "", fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(input.Access) == "" {
		return "", fmt.Errorf("access token is required")
	}
	email := strings.TrimSpace(strings.ToLower(input.Email))
	if email == "" {
		email = "default"
	}
	profileID := provider + ":" + email

	err := m.withLockedStore(func(store *AuthProfileStore) (bool, error) {
		store.Profiles[profileID] = AuthProfileCredential{
			Type:          "oauth",
			Provider:      provider,
			Access:        strings.TrimSpace(input.Access),
			Refresh:       strings.TrimSpace(input.Refresh),
			Expires:       input.Expires,
			Email:         email,
			RefreshFailed: false,
		}
		stats := store.UsageStats[profileID]
		// Fresh login / token upsert should clear temporary lockouts so the
		// profile can be retried immediately with the new credential state.
		stats.CooldownUntil = 0
		stats.DisabledUntil = 0
		stats.DisabledReason = ""
		stats.ErrorCount = 0
		stats.FailureCounts = map[AuthFailureReason]int{}
		store.UsageStats[profileID] = stats
		return true, nil
	})
	if err != nil {
		return "", err
	}
	return profileID, nil
}

func (m *FileAuthManager) ListProfiles(provider string) ([]AuthProfileView, error) {
	provider = normalizeProviderName(provider)
	store, err := m.loadStore()
	if err != nil {
		return nil, err
	}
	views := make([]AuthProfileView, 0, len(store.Profiles))
	for profileID, cred := range store.Profiles {
		if provider != "" && normalizeProviderName(cred.Provider) != provider {
			continue
		}
		stats := store.UsageStats[profileID]
		views = append(views, AuthProfileView{
			ProfileID:      profileID,
			Provider:       cred.Provider,
			Type:           cred.Type,
			Email:          cred.Email,
			Expires:        cred.Expires,
			LastUsed:       stats.LastUsed,
			CooldownUntil:  stats.CooldownUntil,
			DisabledUntil:  stats.DisabledUntil,
			DisabledReason: stats.DisabledReason,
		})
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Provider == views[j].Provider {
			return views[i].ProfileID < views[j].ProfileID
		}
		return views[i].Provider < views[j].Provider
	})
	return views, nil
}

func (m *FileAuthManager) SetOrder(provider string, order []string) error {
	provider = normalizeProviderName(provider)
	if provider == "" {
		return fmt.Errorf("provider is required")
	}
	clean := make([]string, 0, len(order))
	seen := map[string]struct{}{}
	for _, profileID := range order {
		profileID = strings.TrimSpace(profileID)
		if profileID == "" {
			continue
		}
		if _, ok := seen[profileID]; ok {
			continue
		}
		seen[profileID] = struct{}{}
		clean = append(clean, profileID)
	}
	return m.withLockedStore(func(store *AuthProfileStore) (bool, error) {
		if len(clean) == 0 {
			delete(store.Order, provider)
			return true, nil
		}
		store.Order[provider] = clean
		return true, nil
	})
}

func (m *FileAuthManager) LoadStore() (*AuthProfileStore, error) {
	return m.loadStore()
}

func (m *FileAuthManager) withLockedStore(fn func(store *AuthProfileStore) (bool, error)) error {
	if strings.TrimSpace(m.storePath) == "" {
		return fmt.Errorf("store path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(m.storePath), 0o700); err != nil {
		return err
	}
	return withFileLock(m.storePath+".lock", 10*time.Second, func() error {
		store, err := m.loadStore()
		if err != nil {
			return err
		}
		changed, err := fn(store)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return m.saveStore(store)
	})
}

func (m *FileAuthManager) loadStore() (*AuthProfileStore, error) {
	if strings.TrimSpace(m.storePath) == "" {
		return nil, fmt.Errorf("store path is empty")
	}
	if _, err := os.Stat(m.storePath); errors.Is(err, os.ErrNotExist) {
		return defaultAuthProfileStore(), nil
	}
	raw, err := os.ReadFile(m.storePath)
	if err != nil {
		return nil, err
	}
	store := defaultAuthProfileStore()
	if len(strings.TrimSpace(string(raw))) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(raw, store); err != nil {
		return nil, err
	}
	if store.Version == 0 {
		store.Version = authStoreVersion
	}
	if store.Profiles == nil {
		store.Profiles = map[string]AuthProfileCredential{}
	}
	if store.Order == nil {
		store.Order = map[string][]string{}
	}
	if store.UsageStats == nil {
		store.UsageStats = map[string]ProfileUsageStats{}
	}
	if store.SessionPins == nil {
		store.SessionPins = map[string]SessionPin{}
	}
	return store, nil
}

func (m *FileAuthManager) saveStore(store *AuthProfileStore) error {
	store.Version = authStoreVersion
	if store.Profiles == nil {
		store.Profiles = map[string]AuthProfileCredential{}
	}
	if store.Order == nil {
		store.Order = map[string][]string{}
	}
	if store.UsageStats == nil {
		store.UsageStats = map[string]ProfileUsageStats{}
	}
	if store.SessionPins == nil {
		store.SessionPins = map[string]SessionPin{}
	}

	if err := os.MkdirAll(filepath.Dir(m.storePath), 0o700); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := m.storePath + ".tmp"
	if err = os.WriteFile(tmpFile, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpFile, m.storePath)
}

func defaultAuthProfileStore() *AuthProfileStore {
	return &AuthProfileStore{
		Version:     authStoreVersion,
		Profiles:    map[string]AuthProfileCredential{},
		Order:       map[string][]string{},
		UsageStats:  map[string]ProfileUsageStats{},
		SessionPins: map[string]SessionPin{},
	}
}

func withFileLock(lockPath string, timeout time.Duration, fn func() error) error {
	deadline := time.Now().Add(timeout)
	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = file.WriteString(fmt.Sprintf("pid=%d\n", os.Getpid()))
			_ = file.Close()
			defer os.Remove(lockPath)
			return fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}

		if info, statErr := os.Stat(lockPath); statErr == nil {
			if time.Since(info.ModTime()) > 2*time.Minute {
				_ = os.Remove(lockPath)
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("failed to acquire lock %s", lockPath)
		}
		time.Sleep(120 * time.Millisecond)
	}
}

func sessionProviderKey(sessionID, provider string) string {
	if sessionID == "" || provider == "" {
		return ""
	}
	return sessionID + "::" + provider
}

func normalizeProviderName(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func resolveProviderOrder(store *AuthProfileStore, provider string) []string {
	explicit := dedupeProfileIDs(store.Order[provider])
	if len(explicit) > 0 {
		result := make([]string, 0, len(explicit))
		for _, profileID := range explicit {
			cred, ok := store.Profiles[profileID]
			if !ok {
				continue
			}
			if normalizeProviderName(cred.Provider) != provider {
				continue
			}
			result = append(result, profileID)
		}
		if len(result) > 0 {
			return result
		}
	}

	type scored struct {
		profileID string
		typeScore int
		lastUsed  int64
	}
	scoredProfiles := make([]scored, 0)
	for profileID, cred := range store.Profiles {
		if normalizeProviderName(cred.Provider) != provider {
			continue
		}
		typeScore := 2
		switch cred.Type {
		case "oauth":
			typeScore = 0
		case "token":
			typeScore = 1
		case "api_key":
			typeScore = 2
		}
		stats := store.UsageStats[profileID]
		scoredProfiles = append(scoredProfiles, scored{profileID: profileID, typeScore: typeScore, lastUsed: stats.LastUsed})
	}
	sort.Slice(scoredProfiles, func(i, j int) bool {
		if scoredProfiles[i].typeScore != scoredProfiles[j].typeScore {
			return scoredProfiles[i].typeScore < scoredProfiles[j].typeScore
		}
		if scoredProfiles[i].lastUsed != scoredProfiles[j].lastUsed {
			return scoredProfiles[i].lastUsed < scoredProfiles[j].lastUsed
		}
		return scoredProfiles[i].profileID < scoredProfiles[j].profileID
	})
	result := make([]string, 0, len(scoredProfiles))
	for _, entry := range scoredProfiles {
		result = append(result, entry.profileID)
	}
	return result
}

func dedupeProfileIDs(ids []string) []string {
	result := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func moveProfileToFront(order []string, profileID string) []string {
	if profileID == "" || len(order) == 0 {
		return order
	}
	idx := -1
	for i, id := range order {
		if id == profileID {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return order
	}
	result := make([]string, 0, len(order))
	result = append(result, profileID)
	result = append(result, order[:idx]...)
	result = append(result, order[idx+1:]...)
	return result
}

func isProfileUnusable(store *AuthProfileStore, profileID string, now int64) bool {
	stats := store.UsageStats[profileID]
	if stats.DisabledUntil > now {
		return true
	}
	if stats.CooldownUntil > now {
		return true
	}
	return false
}

func clearExpiredWindows(store *AuthProfileStore, now int64) bool {
	changed := false
	for profileID, stats := range store.UsageStats {
		profileChanged := false
		if stats.CooldownUntil > 0 && now >= stats.CooldownUntil {
			stats.CooldownUntil = 0
			profileChanged = true
		}
		if stats.DisabledUntil > 0 && now >= stats.DisabledUntil {
			stats.DisabledUntil = 0
			stats.DisabledReason = ""
			profileChanged = true
		}
		if profileChanged && stats.CooldownUntil == 0 && stats.DisabledUntil == 0 {
			stats.ErrorCount = 0
			stats.FailureCounts = map[AuthFailureReason]int{}
		}
		if profileChanged {
			store.UsageStats[profileID] = stats
			changed = true
		}
	}
	return changed
}

func markFailure(store *AuthProfileStore, profileID string, reason AuthFailureReason, now int64) {
	stats := store.UsageStats[profileID]
	if stats.FailureCounts == nil {
		stats.FailureCounts = map[AuthFailureReason]int{}
	}
	stats.ErrorCount++
	stats.LastFailureAt = now
	stats.FailureCounts[reason]++

	switch reason {
	case AuthFailureReasonFull, AuthFailureReasonAuth:
		backoff := calculateDisabledBackoff(stats.FailureCounts[reason])
		if stats.DisabledUntil <= now {
			stats.DisabledUntil = now + backoff.Milliseconds()
		}
		stats.DisabledReason = string(reason)
	case AuthFailureReasonRateLimit, AuthFailureReasonTimeout:
		cooldown := calculateCooldownBackoff(stats.ErrorCount)
		if stats.CooldownUntil <= now {
			stats.CooldownUntil = now + cooldown.Milliseconds()
		}
	default:
		cooldown := calculateCooldownBackoff(stats.ErrorCount)
		if stats.CooldownUntil <= now {
			stats.CooldownUntil = now + cooldown.Milliseconds()
		}
	}

	store.UsageStats[profileID] = stats
}

func updateProfileRateLimitStats(stats *ProfileUsageStats, statusCode int, headers map[string]string, now time.Time) {
	if stats == nil {
		return
	}
	if stats.RateLimit == nil {
		stats.RateLimit = &ProfileRateLimitStats{}
	}

	rateLimit := stats.RateLimit
	nowMs := now.UnixMilli()
	rateLimit.LastStatusCode = statusCode
	rateLimit.LastRequestAt = nowMs

	if value := lookupHeaderValue(headers, "X-Codex-Active-Limit"); value != "" {
		rateLimit.ActiveLimit = value
	}
	if value := lookupHeaderValue(headers, "X-Codex-Plan-Type"); value != "" {
		rateLimit.PlanType = value
	}
	if value := lookupHeaderValue(headers, "X-Codex-Credits-Has-Credits"); value != "" {
		rateLimit.HasCredits = value
	}
	if value := lookupHeaderValue(headers, "X-Codex-Credits-Unlimited"); value != "" {
		rateLimit.Unlimited = value
	}

	updateRateLimitWindowStats(&rateLimit.FiveHour, headers, "X-Codex-Primary-", 300, nowMs)
	updateRateLimitWindowStats(&rateLimit.Weekly, headers, "X-Codex-Secondary-", 10080, nowMs)
}

func updateRateLimitWindowStats(
	window *ProfileRateLimitWindowStats,
	headers map[string]string,
	headerPrefix string,
	fallbackWindowMinutes int,
	nowMs int64,
) {
	if window == nil {
		return
	}

	if parsed, ok := parseHeaderInt(headers, headerPrefix+"Window-Minutes"); ok && parsed > 0 {
		window.WindowMinutes = parsed
	}
	if window.WindowMinutes <= 0 {
		window.WindowMinutes = fallbackWindowMinutes
	}

	if parsed, ok := parseHeaderInt(headers, headerPrefix+"Used-Percent"); ok {
		window.UsedPercent = parsed
	}
	if parsed, ok := parseHeaderInt(headers, headerPrefix+"Over-Secondary-Limit-Percent"); ok {
		window.OverSecondaryLimitPercent = parsed
	}
	if parsed, ok := parseHeaderInt64(headers, headerPrefix+"Reset-After-Seconds"); ok {
		window.ResetAfterSeconds = parsed
	}

	var resetAt int64
	hasResetAt := false
	if parsed, ok := parseHeaderInt64(headers, headerPrefix+"Reset-At"); ok {
		resetAt = parsed
		hasResetAt = true
	}

	windowDurationMs := int64(time.Duration(window.WindowMinutes) * time.Minute / time.Millisecond)
	if windowDurationMs <= 0 {
		windowDurationMs = int64(time.Duration(fallbackWindowMinutes) * time.Minute / time.Millisecond)
	}

	if hasResetAt {
		if window.ResetAt != resetAt {
			window.RequestCount = 0
			if windowDurationMs > 0 {
				window.WindowStart = (resetAt * 1000) - windowDurationMs
			}
		}
		window.ResetAt = resetAt
	} else if window.WindowStart == 0 || (windowDurationMs > 0 && nowMs-window.WindowStart >= windowDurationMs) {
		window.WindowStart = nowMs
		window.RequestCount = 0
		window.ResetAt = 0
	}

	window.RequestCount++
	window.UpdatedAt = nowMs
}

func lookupHeaderValue(headers map[string]string, target string) string {
	if len(headers) == 0 || strings.TrimSpace(target) == "" {
		return ""
	}
	for key, value := range headers {
		if strings.EqualFold(strings.TrimSpace(key), target) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseHeaderInt(headers map[string]string, key string) (int, bool) {
	parsed, ok := parseHeaderInt64(headers, key)
	if !ok {
		return 0, false
	}
	return int(parsed), true
}

func parseHeaderInt64(headers map[string]string, key string) (int64, bool) {
	raw := strings.TrimSpace(lookupHeaderValue(headers, key))
	if raw == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func calculateCooldownBackoff(errorCount int) time.Duration {
	if errorCount < 1 {
		errorCount = 1
	}
	base := time.Minute
	factor := 1
	switch {
	case errorCount == 1:
		factor = 1
	case errorCount == 2:
		factor = 5
	case errorCount == 3:
		factor = 25
	default:
		factor = 60
	}
	d := time.Duration(factor) * base
	if d > time.Hour {
		return time.Hour
	}
	return d
}

func calculateDisabledBackoff(errorCount int) time.Duration {
	if errorCount < 1 {
		errorCount = 1
	}
	base := 5 * time.Hour
	d := base
	for i := 1; i < errorCount; i++ {
		d *= 2
		if d >= 24*time.Hour {
			return 24 * time.Hour
		}
	}
	if d > 24*time.Hour {
		return 24 * time.Hour
	}
	return d
}

func randomID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%x", prefix, buf)
}
