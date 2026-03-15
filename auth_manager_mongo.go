package cs_ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	defaultMongoAuthStoreCollection = "auth_profiles"
	defaultMongoAuthStoreDocumentID = "auth_store"
	defaultAuthProfilesImportEnv    = "CS_AI_AUTH_IMPORT_JSON_PATH"
)

var errMongoAuthStoreWriteConflict = errors.New("mongo auth store write conflict")

type mongoAuthStoreDocument struct {
	ID          string                           `bson:"_id"`
	Revision    int64                            `bson:"revision"`
	Version     int                              `bson:"version"`
	Profiles    map[string]AuthProfileCredential `bson:"profiles"`
	Order       map[string][]string              `bson:"order,omitempty"`
	UsageStats  map[string]ProfileUsageStats     `bson:"usage_stats,omitempty"`
	SessionPins map[string]SessionPin            `bson:"session_pins,omitempty"`
	CreatedAt   time.Time                        `bson:"created_at,omitempty"`
	UpdatedAt   time.Time                        `bson:"updated_at,omitempty"`
}

type mongoLegacyProfileDocument struct {
	ProfileID      string         `bson:"profile_id"`
	Provider       string         `bson:"provider"`
	Type           string         `bson:"type"`
	Access         string         `bson:"access"`
	Refresh        string         `bson:"refresh"`
	Expires        int64          `bson:"expires"`
	Email          string         `bson:"email"`
	LastUsed       int64          `bson:"last_used"`
	CooldownUntil  int64          `bson:"cooldown_until"`
	DisabledUntil  int64          `bson:"disabled_until"`
	DisabledReason string         `bson:"disabled_reason"`
	ErrorCount     int            `bson:"error_count"`
	FailureCounts  map[string]int `bson:"failure_counts"`
	LastFailureAt  int64          `bson:"last_failure_at"`
}

type mongoLegacyOrderDocument struct {
	Provider string   `bson:"provider"`
	Order    []string `bson:"order"`
}

type MongoAuthManager struct {
	client      *mongo.Client
	collection  *mongo.Collection
	documentID  string
	storeLabel  string
	timeout     time.Duration
	now         func() time.Time
	refreshFunc OAuthRefreshFunc
}

func NewMongoAuthManager(config StorageConfig) (*MongoAuthManager, error) {
	uri := strings.TrimSpace(config.MongoURI)
	if uri == "" {
		return nil, fmt.Errorf("MongoDB URI is required")
	}

	database := strings.TrimSpace(config.MongoDatabase)
	if database == "" {
		database = "cs_ai"
	}

	collection := strings.TrimSpace(config.MongoCollection)
	if collection == "" {
		collection = defaultMongoAuthStoreCollection
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect MongoDB: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	coll := client.Database(database).Collection(collection)
	label := fmt.Sprintf("mongodb/%s.%s#%s", database, collection, defaultMongoAuthStoreDocumentID)

	manager := &MongoAuthManager{
		client:      client,
		collection:  coll,
		documentID:  defaultMongoAuthStoreDocumentID,
		storeLabel:  label,
		timeout:     timeout,
		now:         time.Now,
		refreshFunc: refreshOAuthCredential,
	}

	importPath := resolveAuthProfilesImportPath(config)
	if importPath != "" {
		if err := manager.ImportProfilesFromJSON(importPath); err != nil {
			_ = client.Disconnect(context.Background())
			return nil, fmt.Errorf("failed to import auth profiles from JSON: %w", err)
		}
	}

	return manager, nil
}

func (m *MongoAuthManager) StorePath() string {
	if m == nil {
		return ""
	}
	return m.storeLabel
}

func (m *MongoAuthManager) Close() error {
	if m == nil || m.client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	return m.client.Disconnect(ctx)
}

func (m *MongoAuthManager) ResolveAuth(ctx context.Context, sessionID string, provider string) (*AuthSelection, error) {
	if m == nil {
		return nil, fmt.Errorf("auth manager is nil")
	}
	provider = normalizeProviderName(provider)
	if provider == "" {
		return nil, fmt.Errorf("provider is required")
	}

	var selected *AuthSelection
	err := m.withLockedStore(func(store *AuthProfileStore) (bool, error) {
		now := m.now().UnixMilli()
		changed := clearExpiredWindows(store, now)

		ordered := resolveProviderOrder(store, provider)
		if len(ordered) == 0 {
			return changed, fmt.Errorf("no auth profile configured for provider %s", provider)
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
					markFailure(store, profileID, AuthFailureReasonAuth, now)
					changed = true
					continue
				}
				if strings.TrimSpace(refreshed.Access) == "" {
					markFailure(store, profileID, AuthFailureReasonAuth, now)
					changed = true
					continue
				}
				store.Profiles[profileID] = refreshed
				cred = refreshed
				changed = true
			}

			if sessionID != "" {
				store.SessionPins[pinKey] = SessionPin{ProfileID: profileID, UpdatedAt: now}
				changed = true
			}

			selected = &AuthSelection{Provider: provider, ProfileID: profileID, Token: cred.Access}
			return changed, nil
		}

		if len(ordered) == 1 {
			profileID := ordered[0]
			cred, exists := store.Profiles[profileID]
			if exists && normalizeProviderName(cred.Provider) == provider && strings.TrimSpace(cred.Access) != "" {
				if cred.Expires > 0 && now >= cred.Expires {
					refreshed, refreshErr := m.refreshFunc(ctx, cred)
					if refreshErr == nil && strings.TrimSpace(refreshed.Access) != "" {
						store.Profiles[profileID] = refreshed
						cred = refreshed
						changed = true
					}
				}
				if strings.TrimSpace(cred.Access) != "" {
					if sessionID != "" {
						store.SessionPins[pinKey] = SessionPin{ProfileID: profileID, UpdatedAt: now}
						changed = true
					}
					selected = &AuthSelection{Provider: provider, ProfileID: profileID, Token: cred.Access}
					return changed, nil
				}
			}
		}

		return changed, fmt.Errorf("no available auth profile for provider %s", provider)
	})
	if err != nil {
		return nil, err
	}
	return selected, nil
}

func (m *MongoAuthManager) MarkSuccess(ctx context.Context, sessionID string, provider string, profileID string) error {
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

func (m *MongoAuthManager) MarkFailure(ctx context.Context, sessionID string, provider string, profileID string, reason AuthFailureReason) error {
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

func (m *MongoAuthManager) RecordRateLimit(ctx context.Context, provider string, profileID string, statusCode int, headers map[string]string) error {
	_ = ctx
	if m == nil || strings.TrimSpace(profileID) == "" {
		return nil
	}
	provider = normalizeProviderName(provider)
	return m.withLockedStore(func(store *AuthProfileStore) (bool, error) {
		cred, exists := store.Profiles[profileID]
		if !exists {
			return false, nil
		}
		if provider != "" && normalizeProviderName(cred.Provider) != provider {
			return false, nil
		}

		stats := store.UsageStats[profileID]
		updateProfileRateLimitStats(&stats, statusCode, headers, m.now())
		store.UsageStats[profileID] = stats
		return true, nil
	})
}

func (m *MongoAuthManager) UpsertOAuthProfile(provider string, input OAuthProfileInput) (string, error) {
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
			Type:     "oauth",
			Provider: provider,
			Access:   strings.TrimSpace(input.Access),
			Refresh:  strings.TrimSpace(input.Refresh),
			Expires:  input.Expires,
			Email:    email,
		}

		stats := store.UsageStats[profileID]
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

func (m *MongoAuthManager) ListProfiles(provider string) ([]AuthProfileView, error) {
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

func (m *MongoAuthManager) SetOrder(provider string, order []string) error {
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

func (m *MongoAuthManager) LoadStore() (*AuthProfileStore, error) {
	return m.loadStore()
}

// ImportProfilesFromJSON imports only the `profiles` map from a file that
// follows the auth-profiles.json structure. Other fields in the file are
// ignored by design.
func (m *MongoAuthManager) ImportProfilesFromJSON(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("import path is empty")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var filePayload struct {
		Profiles map[string]AuthProfileCredential `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &filePayload); err != nil {
		return err
	}
	if len(filePayload.Profiles) == 0 {
		return fmt.Errorf("profiles is empty in %s", path)
	}

	normalized := normalizeImportedProfiles(filePayload.Profiles)
	if len(normalized) == 0 {
		return fmt.Errorf("no valid profiles found in %s", path)
	}

	return m.withLockedStore(func(store *AuthProfileStore) (bool, error) {
		store.Profiles = normalized

		for profileID := range store.UsageStats {
			if _, ok := normalized[profileID]; !ok {
				delete(store.UsageStats, profileID)
			}
		}
		for profileID := range normalized {
			stats := store.UsageStats[profileID]
			// New token import should clear temporary lockouts.
			stats.CooldownUntil = 0
			stats.DisabledUntil = 0
			stats.DisabledReason = ""
			stats.ErrorCount = 0
			stats.FailureCounts = map[AuthFailureReason]int{}
			store.UsageStats[profileID] = stats
		}

		for provider, order := range store.Order {
			filtered := make([]string, 0, len(order))
			for _, profileID := range dedupeProfileIDs(order) {
				cred, ok := normalized[profileID]
				if !ok {
					continue
				}
				if normalizeProviderName(cred.Provider) != normalizeProviderName(provider) {
					continue
				}
				filtered = append(filtered, profileID)
			}
			if len(filtered) == 0 {
				delete(store.Order, provider)
				continue
			}
			store.Order[provider] = filtered
		}

		for pinKey, pin := range store.SessionPins {
			if _, ok := normalized[pin.ProfileID]; !ok {
				delete(store.SessionPins, pinKey)
			}
		}

		return true, nil
	})
}

func (m *MongoAuthManager) withLockedStore(fn func(store *AuthProfileStore) (bool, error)) error {
	if m == nil || m.collection == nil {
		return fmt.Errorf("mongo auth manager is not initialized")
	}

	const maxAttempts = 8
	for attempt := 0; attempt < maxAttempts; attempt++ {
		doc, exists, err := m.loadStoreDocument()
		if err != nil {
			return err
		}

		store := defaultAuthProfileStore()
		if exists {
			store = doc.toStore()
		}

		changed, err := fn(store)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}

		if err := m.saveStoreDocument(doc.Revision, exists, store); err != nil {
			if errors.Is(err, errMongoAuthStoreWriteConflict) {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			return err
		}
		return nil
	}

	return fmt.Errorf("failed to update mongo auth store after retries")
}

func (m *MongoAuthManager) loadStore() (*AuthProfileStore, error) {
	doc, exists, err := m.loadStoreDocument()
	if err != nil {
		return nil, err
	}
	if !exists {
		return defaultAuthProfileStore(), nil
	}
	return doc.toStore(), nil
}

func (m *MongoAuthManager) loadStoreDocument() (*mongoAuthStoreDocument, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	var doc mongoAuthStoreDocument
	err := m.collection.FindOne(ctx, bson.M{"_id": m.documentID}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			legacyDoc, legacyErr := m.loadLegacyStoreDocument(ctx)
			if legacyErr != nil {
				return nil, false, legacyErr
			}
			if legacyDoc != nil {
				return legacyDoc, true, nil
			}
			return &mongoAuthStoreDocument{ID: m.documentID, Revision: 0}, false, nil
		}
		return nil, false, err
	}

	doc.ensureMaps()
	if doc.Version == 0 {
		doc.Version = authStoreVersion
	}
	return &doc, true, nil
}

func (m *MongoAuthManager) saveStoreDocument(previousRevision int64, exists bool, store *AuthProfileStore) error {
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

	now := m.now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	filter := bson.M{"_id": m.documentID}
	if exists {
		filter["revision"] = previousRevision
	} else {
		filter["revision"] = bson.M{"$exists": false}
	}

	update := bson.M{
		"$set": bson.M{
			"version":      store.Version,
			"profiles":     store.Profiles,
			"order":        store.Order,
			"usage_stats":  store.UsageStats,
			"session_pins": store.SessionPins,
			"revision":     previousRevision + 1,
			"updated_at":   now,
		},
		"$setOnInsert": bson.M{
			"created_at": now,
		},
	}

	result, err := m.collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return errMongoAuthStoreWriteConflict
		}
		return err
	}
	if result.MatchedCount == 0 && result.UpsertedCount == 0 {
		return errMongoAuthStoreWriteConflict
	}
	return nil
}

func (d *mongoAuthStoreDocument) ensureMaps() {
	if d.Profiles == nil {
		d.Profiles = map[string]AuthProfileCredential{}
	}
	if d.Order == nil {
		d.Order = map[string][]string{}
	}
	if d.UsageStats == nil {
		d.UsageStats = map[string]ProfileUsageStats{}
	}
	if d.SessionPins == nil {
		d.SessionPins = map[string]SessionPin{}
	}
}

func (d *mongoAuthStoreDocument) toStore() *AuthProfileStore {
	d.ensureMaps()
	store := &AuthProfileStore{
		Version:     d.Version,
		Profiles:    d.Profiles,
		Order:       d.Order,
		UsageStats:  d.UsageStats,
		SessionPins: d.SessionPins,
	}
	if store.Version == 0 {
		store.Version = authStoreVersion
	}
	return store
}

func resolveAuthProfilesImportPath(config StorageConfig) string {
	if path := strings.TrimSpace(config.AuthProfilesImportPath); path != "" {
		return path
	}
	if path := strings.TrimSpace(os.Getenv(defaultAuthProfilesImportEnv)); path != "" {
		return path
	}
	return ""
}

func normalizeImportedProfiles(profiles map[string]AuthProfileCredential) map[string]AuthProfileCredential {
	normalized := make(map[string]AuthProfileCredential, len(profiles))
	for profileID, cred := range profiles {
		rawID := strings.TrimSpace(profileID)
		if rawID == "" {
			continue
		}

		provider := normalizeProviderName(cred.Provider)
		if provider == "" {
			if idx := strings.Index(rawID, ":"); idx > 0 {
				provider = normalizeProviderName(rawID[:idx])
			}
		}
		if provider == "" {
			continue
		}

		email := strings.TrimSpace(strings.ToLower(cred.Email))
		if email == "" {
			if idx := strings.Index(rawID, ":"); idx >= 0 && idx < len(rawID)-1 {
				email = strings.TrimSpace(strings.ToLower(rawID[idx+1:]))
			}
		}
		if email == "" {
			email = "default"
		}

		access := strings.TrimSpace(cred.Access)
		if access == "" {
			continue
		}

		credType := strings.TrimSpace(cred.Type)
		if credType == "" {
			credType = "oauth"
		}

		normalizedProfileID := provider + ":" + email
		normalized[normalizedProfileID] = AuthProfileCredential{
			Type:     credType,
			Provider: provider,
			Access:   access,
			Refresh:  strings.TrimSpace(cred.Refresh),
			Expires:  cred.Expires,
			Email:    email,
		}
	}
	return normalized
}

func (m *MongoAuthManager) loadLegacyStoreDocument(ctx context.Context) (*mongoAuthStoreDocument, error) {
	profileCursor, err := m.collection.Find(ctx, bson.M{"kind": "profile"})
	if err != nil {
		return nil, err
	}
	defer profileCursor.Close(ctx)

	doc := &mongoAuthStoreDocument{
		ID:          m.documentID,
		Revision:    0,
		Version:     authStoreVersion,
		Profiles:    map[string]AuthProfileCredential{},
		Order:       map[string][]string{},
		UsageStats:  map[string]ProfileUsageStats{},
		SessionPins: map[string]SessionPin{},
	}

	foundAny := false
	for profileCursor.Next(ctx) {
		var legacy mongoLegacyProfileDocument
		if err := profileCursor.Decode(&legacy); err != nil {
			continue
		}
		profileID := strings.TrimSpace(legacy.ProfileID)
		if profileID == "" {
			continue
		}
		provider := normalizeProviderName(legacy.Provider)
		if provider == "" {
			continue
		}
		foundAny = true

		credType := strings.TrimSpace(legacy.Type)
		if credType == "" {
			credType = "oauth"
		}
		doc.Profiles[profileID] = AuthProfileCredential{
			Type:     credType,
			Provider: provider,
			Access:   strings.TrimSpace(legacy.Access),
			Refresh:  strings.TrimSpace(legacy.Refresh),
			Expires:  legacy.Expires,
			Email:    strings.TrimSpace(strings.ToLower(legacy.Email)),
		}

		failureCounts := map[AuthFailureReason]int{}
		for reason, count := range legacy.FailureCounts {
			reason = strings.TrimSpace(reason)
			if reason == "" {
				continue
			}
			failureCounts[AuthFailureReason(reason)] = count
		}
		doc.UsageStats[profileID] = ProfileUsageStats{
			LastUsed:       legacy.LastUsed,
			CooldownUntil:  legacy.CooldownUntil,
			DisabledUntil:  legacy.DisabledUntil,
			DisabledReason: legacy.DisabledReason,
			ErrorCount:     legacy.ErrorCount,
			FailureCounts:  failureCounts,
			LastFailureAt:  legacy.LastFailureAt,
		}
	}

	orderCursor, err := m.collection.Find(ctx, bson.M{"kind": "order"})
	if err == nil {
		defer orderCursor.Close(ctx)
		for orderCursor.Next(ctx) {
			var legacyOrder mongoLegacyOrderDocument
			if err := orderCursor.Decode(&legacyOrder); err != nil {
				continue
			}
			provider := normalizeProviderName(legacyOrder.Provider)
			if provider == "" {
				continue
			}
			foundAny = true
			doc.Order[provider] = dedupeProfileIDs(legacyOrder.Order)
		}
	}

	if !foundAny {
		return nil, nil
	}
	return doc, nil
}
