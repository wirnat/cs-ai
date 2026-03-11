package main

import "testing"

func TestParseGlobalFlags_FromArgs(t *testing.T) {
	t.Setenv("CS_AI_AUTH_MONGO_URI", "")
	t.Setenv("CS_AI_MONGO_URI", "")
	t.Setenv("MONGODB_URI", "")
	t.Setenv("MONGO_URI", "")
	t.Setenv("CS_AI_AUTH_MONGO_DATABASE", "")
	t.Setenv("CS_AI_MONGO_DATABASE", "")
	t.Setenv("MONGODB_DATABASE", "")
	t.Setenv("MONGO_DATABASE", "")
	t.Setenv("CS_AI_AUTH_MONGO_COLLECTION", "")
	t.Setenv("CS_AI_MONGO_COLLECTION", "")
	t.Setenv("MONGODB_COLLECTION", "")
	t.Setenv("MONGO_COLLECTION", "")

	flags := parseGlobalFlags([]string{
		"--store-path", "/tmp/auth.json",
		"--mongo-uri", "mongodb://localhost:27017",
		"--mongo-database", "auth_db",
		"--mongo-collection", "oauth_profiles",
		"login",
	})

	if flags.StorePath != "/tmp/auth.json" {
		t.Fatalf("unexpected store path: %s", flags.StorePath)
	}
	if flags.MongoURI != "mongodb://localhost:27017" {
		t.Fatalf("unexpected mongo uri: %s", flags.MongoURI)
	}
	if flags.MongoDatabase != "auth_db" {
		t.Fatalf("unexpected mongo database: %s", flags.MongoDatabase)
	}
	if flags.MongoCollection != "oauth_profiles" {
		t.Fatalf("unexpected mongo collection: %s", flags.MongoCollection)
	}
	if len(flags.CommandArgs) != 1 || flags.CommandArgs[0] != "login" {
		t.Fatalf("unexpected command args: %#v", flags.CommandArgs)
	}
}

func TestParseGlobalFlags_FromEnvFallback(t *testing.T) {
	t.Setenv("CS_AI_AUTH_MONGO_URI", "mongodb://from-auth-env:27017")
	t.Setenv("CS_AI_AUTH_MONGO_DATABASE", "env_db")
	t.Setenv("CS_AI_AUTH_MONGO_COLLECTION", "env_collection")

	flags := parseGlobalFlags([]string{"status"})

	if flags.MongoURI != "mongodb://from-auth-env:27017" {
		t.Fatalf("unexpected mongo uri from env: %s", flags.MongoURI)
	}
	if flags.MongoDatabase != "env_db" {
		t.Fatalf("unexpected mongo database from env: %s", flags.MongoDatabase)
	}
	if flags.MongoCollection != "env_collection" {
		t.Fatalf("unexpected mongo collection from env: %s", flags.MongoCollection)
	}
}

func TestParseGlobalFlags_DefaultMongoNames(t *testing.T) {
	t.Setenv("CS_AI_AUTH_MONGO_URI", "")
	t.Setenv("CS_AI_MONGO_URI", "")
	t.Setenv("MONGODB_URI", "")
	t.Setenv("MONGO_URI", "")
	t.Setenv("CS_AI_AUTH_MONGO_DATABASE", "")
	t.Setenv("CS_AI_MONGO_DATABASE", "")
	t.Setenv("MONGODB_DATABASE", "")
	t.Setenv("MONGO_DATABASE", "")
	t.Setenv("CS_AI_AUTH_MONGO_COLLECTION", "")
	t.Setenv("CS_AI_MONGO_COLLECTION", "")
	t.Setenv("MONGODB_COLLECTION", "")
	t.Setenv("MONGO_COLLECTION", "")

	flags := parseGlobalFlags([]string{"profiles", "list"})

	if flags.MongoDatabase != "cs_ai" {
		t.Fatalf("unexpected default mongo database: %s", flags.MongoDatabase)
	}
	if flags.MongoCollection != "auth_profiles" {
		t.Fatalf("unexpected default mongo collection: %s", flags.MongoCollection)
	}
}
