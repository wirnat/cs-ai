package main

import "testing"

func TestParseGlobalFlags_AuthDefaultsIgnoreSessionEnv(t *testing.T) {
	t.Setenv("CS_AI_AUTH_MONGO_URI", "")
	t.Setenv("CS_AI_MONGO_URI", "")
	t.Setenv("MONGODB_URI", "mongodb://from-mongodb-uri")
	t.Setenv("MONGO_URI", "")

	t.Setenv("CS_AI_AUTH_MONGO_DATABASE", "")
	t.Setenv("CS_AI_MONGO_DATABASE", "")
	t.Setenv("MONGODB_DATABASE", "hairkatz_ai")
	t.Setenv("MONGO_DATABASE", "legacy_db")

	t.Setenv("CS_AI_AUTH_MONGO_COLLECTION", "")
	t.Setenv("CS_AI_MONGO_COLLECTION", "")
	t.Setenv("MONGODB_AUTH_COLLECTION", "")
	t.Setenv("MONGO_AUTH_COLLECTION", "")
	t.Setenv("MONGODB_COLLECTION", "sessions")
	t.Setenv("MONGO_COLLECTION", "legacy_sessions")

	flags := parseGlobalFlags([]string{"login", "--provider", "openai-codex"})
	if flags.MongoURI != "mongodb://from-mongodb-uri" {
		t.Fatalf("unexpected MongoURI: %q", flags.MongoURI)
	}
	if flags.MongoDatabase != "cs_ai" {
		t.Fatalf("expected auth default database cs_ai, got %q", flags.MongoDatabase)
	}
	if flags.MongoCollection != "auth_profiles" {
		t.Fatalf("expected auth default collection auth_profiles, got %q", flags.MongoCollection)
	}
}

func TestParseGlobalFlags_AuthSpecificEnvWins(t *testing.T) {
	t.Setenv("CS_AI_AUTH_MONGO_URI", "mongodb://auth-uri")
	t.Setenv("CS_AI_AUTH_MONGO_DATABASE", "auth_db")
	t.Setenv("CS_AI_AUTH_MONGO_COLLECTION", "auth_coll")

	flags := parseGlobalFlags([]string{"profiles", "list"})
	if flags.MongoURI != "mongodb://auth-uri" {
		t.Fatalf("unexpected MongoURI: %q", flags.MongoURI)
	}
	if flags.MongoDatabase != "auth_db" {
		t.Fatalf("unexpected MongoDatabase: %q", flags.MongoDatabase)
	}
	if flags.MongoCollection != "auth_coll" {
		t.Fatalf("unexpected MongoCollection: %q", flags.MongoCollection)
	}
}

func TestParseGlobalFlags_ExplicitFlagsOverrideEnv(t *testing.T) {
	t.Setenv("CS_AI_AUTH_MONGO_URI", "mongodb://env-uri")
	t.Setenv("CS_AI_AUTH_MONGO_DATABASE", "env_db")
	t.Setenv("CS_AI_AUTH_MONGO_COLLECTION", "env_coll")

	flags := parseGlobalFlags([]string{
		"--mongo-uri", "mongodb://flag-uri",
		"--mongo-database", "flag_db",
		"--mongo-collection", "flag_coll",
		"status",
	})
	if flags.MongoURI != "mongodb://flag-uri" {
		t.Fatalf("unexpected MongoURI: %q", flags.MongoURI)
	}
	if flags.MongoDatabase != "flag_db" {
		t.Fatalf("unexpected MongoDatabase: %q", flags.MongoDatabase)
	}
	if flags.MongoCollection != "flag_coll" {
		t.Fatalf("unexpected MongoCollection: %q", flags.MongoCollection)
	}
}
