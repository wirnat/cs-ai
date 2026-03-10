package cs_ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockAuthManager implements AuthManager for testing profile rotation.
type mockAuthManager struct {
	profiles   []AuthSelection // profiles returned in order
	resolveIdx int32           // atomic counter for ResolveAuth calls
	failures   []markFailureCall
	successes  []markSuccessCall
	exhausted  bool // when true, ResolveAuth returns error after profiles are drained
}

type markFailureCall struct {
	SessionID string
	Provider  string
	ProfileID string
	Reason    AuthFailureReason
}

type markSuccessCall struct {
	SessionID string
	Provider  string
	ProfileID string
}

func (m *mockAuthManager) ResolveAuth(_ context.Context, sessionID string, provider string) (*AuthSelection, error) {
	idx := int(atomic.AddInt32(&m.resolveIdx, 1)) - 1
	if idx >= len(m.profiles) {
		m.exhausted = true
		return nil, nil
	}
	sel := m.profiles[idx]
	return &sel, nil
}

func (m *mockAuthManager) MarkSuccess(_ context.Context, sessionID string, provider string, profileID string) error {
	m.successes = append(m.successes, markSuccessCall{SessionID: sessionID, Provider: provider, ProfileID: profileID})
	return nil
}

func (m *mockAuthManager) MarkFailure(_ context.Context, sessionID string, provider string, profileID string, reason AuthFailureReason) error {
	m.failures = append(m.failures, markFailureCall{SessionID: sessionID, Provider: provider, ProfileID: profileID, Reason: reason})
	return nil
}

// TestSendWithModel_RetriesWithSecondProfileBeforeFallback verifies that when
// the first auth profile returns HTTP 429 with insufficient_quota, the system
// classifies it as "full", then tries the second profile for the same provider
// (OpenAI) BEFORE falling back to the DeepSeek model.
func TestSendWithModel_RetriesWithSecondProfileBeforeFallback(t *testing.T) {
	var requestCount int32

	// Server that rejects the first request (profile A) but accepts the second (profile B).
	openaiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(atomic.AddInt32(&requestCount, 1))
		authHeader := r.Header.Get("Authorization")

		if n == 1 {
			// First profile — simulate quota exceeded (429)
			require.Equal(t, "Bearer token-profile-a", authHeader)
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{"message": "insufficient_quota", "type": "insufficient_quota"},
			})
			return
		}

		// Second profile — successful response
		require.Equal(t, "Bearer token-profile-b", authHeader)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "openai-profile-b-ok",
					},
				},
			},
			"model": "gpt-4.1-mini",
		})
	}))
	defer openaiServer.Close()

	// DeepSeek server should NOT be called.
	deepseekCalled := false
	deepseekServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deepseekCalled = true
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "deepseek-fallback",
					},
				},
			},
			"model": "deepseek-chat",
		})
	}))
	defer deepseekServer.Close()

	authMgr := &mockAuthManager{
		profiles: []AuthSelection{
			{Provider: "openai-codex", ProfileID: "openai-codex:profile-a@test.com", Token: "token-profile-a"},
			{Provider: "openai-codex", ProfileID: "openai-codex:profile-b@test.com", Token: "token-profile-b"},
		},
	}

	primaryModel := &fallbackTestModel{
		name:     "gpt-4.1-mini",
		apiURL:   openaiServer.URL,
		provider: "openai-codex",
		apiMode:  APIModeChatCompletions,
	}
	secondaryModel := &fallbackTestModel{
		name:     "deepseek-chat",
		apiURL:   deepseekServer.URL,
		provider: "deepseek",
		apiMode:  APIModeChatCompletions,
	}

	cs := New("", primaryModel, Options{
		UseTool:        false,
		AuthManager:    authMgr,
		ModelFallbacks: []Modeler{secondaryModel},
	})

	resp, err := cs.Send(Messages{{Role: User, Content: "Halo"}})
	require.NoError(t, err)

	// Should have gotten the response from profile B, NOT DeepSeek.
	require.Equal(t, "openai-profile-b-ok", resp.Content)
	require.Equal(t, "gpt-4.1-mini", resp.Model)
	require.False(t, deepseekCalled, "DeepSeek should NOT have been called when second OpenAI profile succeeded")

	// Verify failure was recorded for profile A as FULL (quota exhausted).
	require.Len(t, authMgr.failures, 1)
	require.Equal(t, "openai-codex:profile-a@test.com", authMgr.failures[0].ProfileID)
	require.Equal(t, AuthFailureReasonFull, authMgr.failures[0].Reason)

	// Verify success was recorded for profile B.
	require.Len(t, authMgr.successes, 1)
	require.Equal(t, "openai-codex:profile-b@test.com", authMgr.successes[0].ProfileID)
}

// TestSendWithModel_AllProfilesFailThenFallbackToNextModel verifies that when
// ALL profiles for a provider fail, the system falls back to the next model
// candidate (DeepSeek).
func TestSendWithModel_AllProfilesFailThenFallbackToNextModel(t *testing.T) {
	// OpenAI server always returns 429.
	openaiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "rate limit exceeded"},
		})
	}))
	defer openaiServer.Close()

	deepseekServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "deepseek-ok",
					},
				},
			},
			"model": "deepseek-chat",
		})
	}))
	defer deepseekServer.Close()

	authMgr := &mockAuthManager{
		profiles: []AuthSelection{
			{Provider: "openai-codex", ProfileID: "openai-codex:a@test.com", Token: "token-a"},
			{Provider: "openai-codex", ProfileID: "openai-codex:b@test.com", Token: "token-b"},
		},
	}

	primaryModel := &fallbackTestModel{
		name:     "gpt-4.1-mini",
		apiURL:   openaiServer.URL,
		provider: "openai-codex",
		apiMode:  APIModeChatCompletions,
	}
	secondaryModel := &fallbackTestModel{
		name:     "deepseek-chat",
		apiURL:   deepseekServer.URL,
		provider: "deepseek",
		apiMode:  APIModeChatCompletions,
	}

	cs := New("deepseek-api-key", primaryModel, Options{
		UseTool:        false,
		AuthManager:    authMgr,
		ModelFallbacks: []Modeler{secondaryModel},
	})

	resp, err := cs.Send(Messages{{Role: User, Content: "Halo"}})
	require.NoError(t, err)

	// Both OpenAI profiles failed, should have fallen back to DeepSeek.
	require.Equal(t, "deepseek-ok", resp.Content)
	require.Equal(t, "deepseek-chat", resp.Model)

	// Both profiles should have been marked as failures.
	require.Len(t, authMgr.failures, 2)
}

// TestSendWithModel_RotateOnUnsupportedModel verifies that when one OpenAI
// profile returns 400 "model not supported" (e.g. free ChatGPT account for
// gpt-5.4), runtime retries the next OpenAI profile instead of failing
// immediately.
func TestSendWithModel_RotateOnUnsupportedModel(t *testing.T) {
	var requestCount int32

	openaiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(atomic.AddInt32(&requestCount, 1))
		authHeader := r.Header.Get("Authorization")

		if n == 1 {
			require.Equal(t, "Bearer token-free-account", authHeader)
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"detail": "The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account.",
			})
			return
		}

		require.Equal(t, "Bearer token-paid-account", authHeader)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "openai-paid-ok",
					},
				},
			},
			"model": "gpt-5.4",
		})
	}))
	defer openaiServer.Close()

	authMgr := &mockAuthManager{
		profiles: []AuthSelection{
			{Provider: "openai-codex", ProfileID: "openai-codex:free@test.com", Token: "token-free-account"},
			{Provider: "openai-codex", ProfileID: "openai-codex:paid@test.com", Token: "token-paid-account"},
		},
	}

	primaryModel := &fallbackTestModel{
		name:     "gpt-5.4",
		apiURL:   openaiServer.URL,
		provider: "openai-codex",
		apiMode:  APIModeChatCompletions,
	}

	cs := New("", primaryModel, Options{
		UseTool:     false,
		AuthManager: authMgr,
	})

	resp, err := cs.Send(Messages{{Role: User, Content: "Halo"}})
	require.NoError(t, err)
	require.Equal(t, "openai-paid-ok", resp.Content)
	require.Equal(t, "gpt-5.4", resp.Model)

	require.Len(t, authMgr.failures, 1)
	require.Equal(t, "openai-codex:free@test.com", authMgr.failures[0].ProfileID)
	require.Equal(t, AuthFailureReasonAuth, authMgr.failures[0].Reason)

	require.Len(t, authMgr.successes, 1)
	require.Equal(t, "openai-codex:paid@test.com", authMgr.successes[0].ProfileID)
}

// TestSendWithModel_RotateOnUnknownOpenAIError verifies policy "error apa pun"
// for OpenAI Codex profiles: even unknown/uncategorized errors should rotate
// to the next account before giving up.
func TestSendWithModel_RotateOnUnknownOpenAIError(t *testing.T) {
	var requestCount int32

	openaiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(atomic.AddInt32(&requestCount, 1))
		authHeader := r.Header.Get("Authorization")

		if n == 1 {
			require.Equal(t, "Bearer token-account-a", authHeader)
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"detail": "unexpected payload shape",
			})
			return
		}

		require.Equal(t, "Bearer token-account-b", authHeader)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "openai-account-b-ok",
					},
				},
			},
			"model": "gpt-5.4",
		})
	}))
	defer openaiServer.Close()

	authMgr := &mockAuthManager{
		profiles: []AuthSelection{
			{Provider: "openai-codex", ProfileID: "openai-codex:a@test.com", Token: "token-account-a"},
			{Provider: "openai-codex", ProfileID: "openai-codex:b@test.com", Token: "token-account-b"},
		},
	}

	primaryModel := &fallbackTestModel{
		name:     "gpt-5.4",
		apiURL:   openaiServer.URL,
		provider: "openai-codex",
		apiMode:  APIModeChatCompletions,
	}

	cs := New("", primaryModel, Options{
		UseTool:     false,
		AuthManager: authMgr,
	})

	resp, err := cs.Send(Messages{{Role: User, Content: "Halo"}})
	require.NoError(t, err)
	require.Equal(t, "openai-account-b-ok", resp.Content)
	require.Equal(t, "gpt-5.4", resp.Model)

	require.Len(t, authMgr.failures, 1)
	require.Equal(t, "openai-codex:a@test.com", authMgr.failures[0].ProfileID)
	require.Equal(t, AuthFailureReasonUnknown, authMgr.failures[0].Reason)

	require.Len(t, authMgr.successes, 1)
	require.Equal(t, "openai-codex:b@test.com", authMgr.successes[0].ProfileID)
}

// TestSendWithModel_AllUnknownOpenAIErrorsThenFallbackToDeepSeek verifies that
// after all OpenAI profiles are exhausted by unknown errors, runtime falls back
// to the next configured model (DeepSeek).
func TestSendWithModel_AllUnknownOpenAIErrorsThenFallbackToDeepSeek(t *testing.T) {
	openaiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"detail": "some random provider error",
		})
	}))
	defer openaiServer.Close()

	deepseekServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "deepseek-after-openai-exhausted",
					},
				},
			},
			"model": "deepseek-chat",
		})
	}))
	defer deepseekServer.Close()

	authMgr := &mockAuthManager{
		profiles: []AuthSelection{
			{Provider: "openai-codex", ProfileID: "openai-codex:a@test.com", Token: "token-account-a"},
			{Provider: "openai-codex", ProfileID: "openai-codex:b@test.com", Token: "token-account-b"},
		},
	}

	primaryModel := &fallbackTestModel{
		name:     "gpt-5.4",
		apiURL:   openaiServer.URL,
		provider: "openai-codex",
		apiMode:  APIModeChatCompletions,
	}
	secondaryModel := &fallbackTestModel{
		name:     "deepseek-chat",
		apiURL:   deepseekServer.URL,
		provider: "deepseek",
		apiMode:  APIModeChatCompletions,
	}

	cs := New("deepseek-api-key", primaryModel, Options{
		UseTool:        false,
		AuthManager:    authMgr,
		ModelFallbacks: []Modeler{secondaryModel},
	})

	resp, err := cs.Send(Messages{{Role: User, Content: "Halo"}})
	require.NoError(t, err)
	require.Equal(t, "deepseek-after-openai-exhausted", resp.Content)
	require.Equal(t, "deepseek-chat", resp.Model)

	require.Len(t, authMgr.failures, 2)
	require.Equal(t, AuthFailureReasonUnknown, authMgr.failures[0].Reason)
	require.Equal(t, AuthFailureReasonUnknown, authMgr.failures[1].Reason)
}

// TestSendWithModel_AllProfilesInCooldownSkipsToDeepSeek verifies the scenario
// from the user's log: when all OpenAI profiles are already in cooldown (from a
// previous request), the system should skip OpenAI entirely and fall back to
// DeepSeek using the static API key — NOT send the DeepSeek key to OpenAI.
func TestSendWithModel_AllProfilesInCooldownSkipsToDeepSeek(t *testing.T) {
	openaiCalled := false
	openaiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		openaiCalled = true
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "invalid api key"},
		})
	}))
	defer openaiServer.Close()

	deepseekServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the static API key is used (not an OAuth token).
		require.Equal(t, "Bearer deepseek-api-key", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "deepseek-cooldown-fallback",
					},
				},
			},
			"model": "deepseek-chat",
		})
	}))
	defer deepseekServer.Close()

	// AuthManager returns NO profiles (simulates all profiles in cooldown).
	authMgr := &mockAuthManager{
		profiles: []AuthSelection{}, // empty = all in cooldown
	}

	primaryModel := &fallbackTestModel{
		name:     "gpt-4.1-mini",
		apiURL:   openaiServer.URL,
		provider: "openai-codex",
		apiMode:  APIModeChatCompletions,
	}
	secondaryModel := &fallbackTestModel{
		name:     "deepseek-chat",
		apiURL:   deepseekServer.URL,
		provider: "deepseek",
		apiMode:  APIModeChatCompletions,
	}

	cs := New("deepseek-api-key", primaryModel, Options{
		UseTool:        false,
		AuthManager:    authMgr,
		ModelFallbacks: []Modeler{secondaryModel},
	})

	resp, err := cs.Send(Messages{{Role: User, Content: "Halo"}})
	require.NoError(t, err)

	// Should have skipped OpenAI entirely and used DeepSeek.
	require.False(t, openaiCalled, "OpenAI should NOT have been called when all profiles are in cooldown")
	require.Equal(t, "deepseek-cooldown-fallback", resp.Content)
	require.Equal(t, "deepseek-chat", resp.Model)
}
