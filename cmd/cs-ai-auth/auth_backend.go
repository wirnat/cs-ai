package main

import (
	"os"
	"strings"

	cs_ai "github.com/wirnat/cs-ai"
)

type authStore interface {
	UpsertOAuthProfile(provider string, input cs_ai.OAuthProfileInput) (string, error)
	ListProfiles(provider string) ([]cs_ai.AuthProfileView, error)
	SetOrder(provider string, order []string) error
	LoadStore() (*cs_ai.AuthProfileStore, error)
	StorePath() string
	Close() error
}

type globalFlags struct {
	StorePath       string
	MongoURI        string
	MongoDatabase   string
	MongoCollection string
	CommandArgs     []string
}

func parseGlobalFlags(args []string) globalFlags {
	flags := globalFlags{}
	filteredArgs := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--store-path" && i+1 < len(args):
			flags.StorePath = args[i+1]
			i++
		case strings.HasPrefix(arg, "--store-path="):
			flags.StorePath = strings.TrimPrefix(arg, "--store-path=")
		case arg == "--mongo-uri" && i+1 < len(args):
			flags.MongoURI = args[i+1]
			i++
		case strings.HasPrefix(arg, "--mongo-uri="):
			flags.MongoURI = strings.TrimPrefix(arg, "--mongo-uri=")
		case arg == "--mongo-database" && i+1 < len(args):
			flags.MongoDatabase = args[i+1]
			i++
		case strings.HasPrefix(arg, "--mongo-database="):
			flags.MongoDatabase = strings.TrimPrefix(arg, "--mongo-database=")
		case arg == "--mongo-collection" && i+1 < len(args):
			flags.MongoCollection = args[i+1]
			i++
		case strings.HasPrefix(arg, "--mongo-collection="):
			flags.MongoCollection = strings.TrimPrefix(arg, "--mongo-collection=")
		default:
			filteredArgs = append(filteredArgs, arg)
		}
	}

	flags.CommandArgs = filteredArgs
	if strings.TrimSpace(flags.MongoURI) == "" {
		flags.MongoURI = firstNonEmpty(
			os.Getenv("CS_AI_AUTH_MONGO_URI"),
			os.Getenv("CS_AI_MONGO_URI"),
			os.Getenv("MONGODB_URI"),
			os.Getenv("MONGO_URI"),
		)
	}
	if strings.TrimSpace(flags.MongoDatabase) == "" {
		flags.MongoDatabase = firstNonEmpty(
			os.Getenv("CS_AI_AUTH_MONGO_DATABASE"),
			os.Getenv("CS_AI_MONGO_DATABASE"),
			"cs_ai",
		)
	}
	if strings.TrimSpace(flags.MongoCollection) == "" {
		flags.MongoCollection = firstNonEmpty(
			os.Getenv("CS_AI_AUTH_MONGO_COLLECTION"),
			os.Getenv("CS_AI_MONGO_COLLECTION"),
			os.Getenv("MONGODB_AUTH_COLLECTION"),
			os.Getenv("MONGO_AUTH_COLLECTION"),
			"auth_profiles",
		)
	}
	return flags
}

func createAuthStore(flags globalFlags) (authStore, error) {
	if strings.TrimSpace(flags.MongoURI) != "" {
		manager, err := cs_ai.NewMongoAuthManager(cs_ai.StorageConfig{
			Type:            cs_ai.StorageTypeMongo,
			MongoURI:        flags.MongoURI,
			MongoDatabase:   flags.MongoDatabase,
			MongoCollection: flags.MongoCollection,
		})
		if err != nil {
			return nil, err
		}
		return manager, nil
	}

	var manager *cs_ai.FileAuthManager
	if strings.TrimSpace(flags.StorePath) != "" {
		manager = cs_ai.NewFileAuthManagerWithPath(flags.StorePath)
	} else {
		manager = cs_ai.NewFileAuthManager()
	}
	return &fileAuthStore{manager: manager}, nil
}

type fileAuthStore struct {
	manager *cs_ai.FileAuthManager
}

func (f *fileAuthStore) UpsertOAuthProfile(provider string, input cs_ai.OAuthProfileInput) (string, error) {
	return f.manager.UpsertOAuthProfile(provider, input)
}

func (f *fileAuthStore) ListProfiles(provider string) ([]cs_ai.AuthProfileView, error) {
	return f.manager.ListProfiles(provider)
}

func (f *fileAuthStore) SetOrder(provider string, order []string) error {
	return f.manager.SetOrder(provider, order)
}

func (f *fileAuthStore) LoadStore() (*cs_ai.AuthProfileStore, error) {
	return f.manager.LoadStore()
}

func (f *fileAuthStore) StorePath() string {
	return f.manager.StorePath()
}

func (f *fileAuthStore) Close() error {
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
