package cs_ai

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMirrorRefreshedOAuthProfile_FromMongoUpdatesFileStore(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "auth-profiles.json")
	t.Setenv("CS_AI_AUTH_STORE_PATH", storePath)

	profile := AuthProfileCredential{
		Type:     "oauth",
		Provider: "openai-codex",
		Access:   "access-token-refreshed",
		Refresh:  "refresh-token-refreshed",
		Expires:  1893456000000,
		Email:    "User@Example.com",
	}

	if err := mirrorRefreshedOAuthProfile(context.Background(), refreshMirrorSourceMongo, profile); err != nil {
		t.Fatalf("mirror failed: %v", err)
	}

	manager := NewFileAuthManagerWithPath(storePath)
	store, err := manager.LoadStore()
	if err != nil {
		t.Fatalf("load store failed: %v", err)
	}

	profileID := "openai-codex:user@example.com"
	got, ok := store.Profiles[profileID]
	if !ok {
		t.Fatalf("profile %s not found in mirrored file store", profileID)
	}
	if got.Access != profile.Access {
		t.Fatalf("unexpected access token: %q", got.Access)
	}
	if got.Refresh != profile.Refresh {
		t.Fatalf("unexpected refresh token: %q", got.Refresh)
	}
	if got.Expires != profile.Expires {
		t.Fatalf("unexpected expires: %d", got.Expires)
	}
}

func TestResolveRefreshMirrorMongoStorageConfig_UsesAuthDefaults(t *testing.T) {
	t.Setenv("CS_AI_AUTH_MONGO_URI", "mongodb://mongo-host:27017")
	t.Setenv("CS_AI_AUTH_MONGO_DATABASE", "")
	t.Setenv("CS_AI_AUTH_MONGO_COLLECTION", "")
	t.Setenv("CS_AI_MONGO_DATABASE", "")
	t.Setenv("CS_AI_MONGO_COLLECTION", "")
	t.Setenv("MONGODB_DATABASE", "hairkatz_ai")
	t.Setenv("MONGODB_COLLECTION", "sessions")
	t.Setenv("MONGODB_AUTH_COLLECTION", "")
	t.Setenv("MONGO_AUTH_COLLECTION", "")

	cfg, ok := resolveRefreshMirrorMongoStorageConfig()
	if !ok {
		t.Fatal("expected mongo mirror config to be resolved")
	}
	if cfg.MongoDatabase != "cs_ai" {
		t.Fatalf("expected default auth db cs_ai, got %q", cfg.MongoDatabase)
	}
	if cfg.MongoCollection != "auth_profiles" {
		t.Fatalf("expected default auth collection auth_profiles, got %q", cfg.MongoCollection)
	}
}
