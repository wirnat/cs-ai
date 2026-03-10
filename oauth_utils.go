package cs_ai

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func generateCodeVerifier() (string, error) {
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func generateOAuthState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func extractEmailFromAnyToken(tokens ...string) string {
	for _, token := range tokens {
		if email := extractEmailFromJWT(token); email != "" {
			return strings.ToLower(email)
		}
	}
	return ""
}

func extractEmailFromJWT(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	claims := map[string]interface{}{}
	if err = json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	if email, ok := claims["email"].(string); ok {
		return strings.TrimSpace(email)
	}
	return ""
}

func extractOpenAIChatGPTAccountIDFromJWT(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	claims := map[string]interface{}{}
	if err = json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	authClaims, ok := claims["https://api.openai.com/auth"].(map[string]interface{})
	if !ok {
		return ""
	}
	if accountID, ok := authClaims["chatgpt_account_id"].(string); ok {
		return strings.TrimSpace(accountID)
	}
	return ""
}

func parseOAuthCallbackURL(input string) (code string, state string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", fmt.Errorf("empty callback input")
	}
	u, parseErr := url.Parse(input)
	if parseErr != nil {
		return "", "", parseErr
	}
	query := u.Query()
	code = strings.TrimSpace(query.Get("code"))
	state = strings.TrimSpace(query.Get("state"))
	if code == "" {
		return "", "", fmt.Errorf("missing code in callback URL")
	}
	if state == "" {
		return "", "", fmt.Errorf("missing state in callback URL")
	}
	return code, state, nil
}
