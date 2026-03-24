package cs_ai

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type refreshMirrorSource string

const (
	refreshMirrorSourceFile  refreshMirrorSource = "file"
	refreshMirrorSourceMongo refreshMirrorSource = "mongo"
)

func mirrorRefreshedOAuthProfile(ctx context.Context, source refreshMirrorSource, profile AuthProfileCredential) error {
	provider := normalizeProviderName(profile.Provider)
	if provider != "openai-codex" {
		return nil
	}

	input := OAuthProfileInput{
		Access:  strings.TrimSpace(profile.Access),
		Refresh: strings.TrimSpace(profile.Refresh),
		Expires: profile.Expires,
		Email:   strings.TrimSpace(strings.ToLower(profile.Email)),
	}
	if input.Access == "" {
		return fmt.Errorf("cannot mirror refreshed profile: empty access token")
	}

	if source != refreshMirrorSourceFile {
		if err := mirrorRefreshedOAuthProfileToFile(provider, input); err != nil {
			return err
		}
	}

	if source != refreshMirrorSourceMongo {
		config, ok := resolveRefreshMirrorMongoStorageConfig()
		if ok {
			if err := mirrorRefreshedOAuthProfileToMongo(ctx, config, provider, input); err != nil {
				return err
			}
		}
	}

	return nil
}

func mirrorRefreshedOAuthProfileToFile(provider string, input OAuthProfileInput) error {
	manager := NewFileAuthManager()
	if _, err := manager.UpsertOAuthProfile(provider, input); err != nil {
		return fmt.Errorf("failed to mirror refreshed profile to file store (%s): %w", manager.StorePath(), err)
	}
	return nil
}

func mirrorRefreshedOAuthProfileToMongo(
	ctx context.Context,
	config StorageConfig,
	provider string,
	input OAuthProfileInput,
) error {
	manager, err := NewMongoAuthManager(config)
	if err != nil {
		return fmt.Errorf("failed to initialize mongo mirror store: %w", err)
	}
	defer manager.Close()

	if _, err := manager.UpsertOAuthProfile(provider, input); err != nil {
		return fmt.Errorf("failed to mirror refreshed profile to mongo store (%s): %w", manager.StorePath(), err)
	}
	return nil
}

func resolveRefreshMirrorMongoStorageConfig() (StorageConfig, bool) {
	uri := firstNonEmptyTrimmed(
		os.Getenv("CS_AI_AUTH_MONGO_URI"),
		os.Getenv("CS_AI_MONGO_URI"),
		os.Getenv("MONGODB_URI"),
		os.Getenv("MONGO_URI"),
	)
	if uri == "" {
		return StorageConfig{}, false
	}

	database := firstNonEmptyTrimmed(
		os.Getenv("CS_AI_AUTH_MONGO_DATABASE"),
		os.Getenv("CS_AI_MONGO_DATABASE"),
		"cs_ai",
	)

	collection := firstNonEmptyTrimmed(
		os.Getenv("CS_AI_AUTH_MONGO_COLLECTION"),
		os.Getenv("CS_AI_MONGO_COLLECTION"),
		os.Getenv("MONGODB_AUTH_COLLECTION"),
		os.Getenv("MONGO_AUTH_COLLECTION"),
		"auth_profiles",
	)

	timeoutSeconds := int64(10)
	if raw := firstNonEmptyTrimmed(
		os.Getenv("CS_AI_AUTH_MONGO_TIMEOUT"),
		os.Getenv("MONGODB_TIMEOUT"),
	); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			timeoutSeconds = parsed
		}
	}

	return StorageConfig{
		Type:            StorageTypeMongo,
		MongoURI:        uri,
		MongoDatabase:   database,
		MongoCollection: collection,
		Timeout:         time.Duration(timeoutSeconds) * time.Second,
	}, true
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
