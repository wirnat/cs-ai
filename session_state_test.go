package cs_ai

import (
	"testing"
	"time"
)

func TestSessionStateRoundTrip_InMemoryStorage(t *testing.T) {
	cs := New("", nil, Options{
		StorageProvider: mustNewInMemoryStorageProviderForStateTest(t),
		SessionTTL:      time.Hour,
	})

	want := map[string]interface{}{
		"has_greeted":            true,
		"language":               "id",
		"known_services":         []interface{}{"Haircut"},
		"customer_phone_present": true,
	}
	if err := cs.SaveSessionState("state-1", want); err != nil {
		t.Fatalf("save session state failed: %v", err)
	}

	got, err := cs.GetSessionState("state-1")
	if err != nil {
		t.Fatalf("get session state failed: %v", err)
	}
	if got["has_greeted"] != true {
		t.Fatalf("expected has_greeted=true, got %#v", got["has_greeted"])
	}
	if got["language"] != "id" {
		t.Fatalf("expected language=id, got %#v", got["language"])
	}
}

func mustNewInMemoryStorageProviderForStateTest(t *testing.T) StorageProvider {
	t.Helper()
	storage, err := NewInMemoryStorageProvider(StorageConfig{SessionTTL: time.Hour})
	if err != nil {
		t.Fatalf("failed to create in-memory storage: %v", err)
	}
	return storage
}
