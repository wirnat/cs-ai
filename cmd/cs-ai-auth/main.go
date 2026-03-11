package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	cs_ai "github.com/wirnat/cs-ai"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	flags := parseGlobalFlags(os.Args[1:])
	if len(flags.CommandArgs) == 0 {
		printUsage()
		os.Exit(1)
	}

	manager, err := createAuthStore(flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gagal inisialisasi auth store: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := manager.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: gagal close auth store: %v\n", closeErr)
		}
	}()

	switch flags.CommandArgs[0] {
	case "login":
		if err := runLogin(manager, flags.CommandArgs[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
			os.Exit(1)
		}
	case "profiles":
		if err := runProfiles(manager, flags.CommandArgs[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "profiles command failed: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := runStatus(manager); err != nil {
			fmt.Fprintf(os.Stderr, "status failed: %v\n", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func runLogin(manager authStore, args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	provider := fs.String("provider", "openai-codex", "provider id")
	redirectURI := fs.String("redirect-uri", "http://localhost:1455/auth/callback", "oauth redirect uri")
	scope := fs.String("scope", "openid profile email offline_access", "oauth scope")
	clientIDFlag := fs.String("client-id", "", "oauth client id (default: CS_AI_OAUTH_CLIENT_ID or built-in codex client)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*provider) != "openai-codex" {
		return fmt.Errorf("provider %q not supported yet", *provider)
	}

	clientID := strings.TrimSpace(cs_ai.ResolveOpenAICodexClientID(*clientIDFlag))

	verifier, err := generateCodeVerifier()
	if err != nil {
		return err
	}
	challenge := generateCodeChallenge(verifier)
	state, err := generateOAuthState()
	if err != nil {
		return err
	}

	authURL := buildAuthorizeURL(clientID, *redirectURI, *scope, challenge, state)
	fmt.Printf("Buka URL login berikut:\n\n%s\n\n", authURL)

	code, returnedState, waitErr := waitForCallbackOrPaste(*redirectURI, authURL)
	if waitErr != nil {
		return waitErr
	}
	if strings.TrimSpace(state) != strings.TrimSpace(returnedState) {
		return fmt.Errorf("oauth state mismatch")
	}

	tokenResp, err := exchangeToken(code, verifier, clientID, *redirectURI)
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli()
	email := extractEmailFromAnyToken(tokenResp.AccessToken, tokenResp.IDToken)
	profileID, err := manager.UpsertOAuthProfile("openai-codex", cs_ai.OAuthProfileInput{
		Access:  tokenResp.AccessToken,
		Refresh: tokenResp.RefreshToken,
		Expires: expiresAt,
		Email:   email,
	})
	if err != nil {
		return err
	}

	fmt.Printf("OAuth login sukses.\nProfile: %s\nExpires: %s\nStore: %s\n", profileID, time.UnixMilli(expiresAt).Format(time.RFC3339), manager.StorePath())
	return nil
}

func runProfiles(manager authStore, args []string) error {
	if len(args) == 0 {
		return runProfilesList(manager, nil)
	}

	switch args[0] {
	case "list":
		return runProfilesList(manager, args[1:])
	case "order":
		if len(args) >= 2 && args[1] == "set" {
			return runProfilesOrderSet(manager, args[2:])
		}
		return fmt.Errorf("unknown profiles order command")
	default:
		return fmt.Errorf("unknown profiles subcommand: %s", args[0])
	}
}

func runProfilesList(manager authStore, args []string) error {
	fs := flag.NewFlagSet("profiles list", flag.ContinueOnError)
	provider := fs.String("provider", "", "filter provider")
	if err := fs.Parse(args); err != nil {
		return err
	}

	profiles, err := manager.ListProfiles(*provider)
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		fmt.Println("Tidak ada profil auth.")
		return nil
	}

	now := time.Now().UnixMilli()
	fmt.Println("PROFILE\tPROVIDER\tEMAIL\tSTATUS\tEXPIRES")
	for _, profile := range profiles {
		status := "active"
		switch {
		case profile.DisabledUntil > now:
			status = "disabled"
		case profile.CooldownUntil > now:
			status = "cooldown"
		case profile.Expires > 0 && profile.Expires <= now:
			status = "expired"
		}
		expires := "-"
		if profile.Expires > 0 {
			expires = time.UnixMilli(profile.Expires).Format(time.RFC3339)
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", profile.ProfileID, profile.Provider, profile.Email, status, expires)
	}
	return nil
}

func runProfilesOrderSet(manager authStore, args []string) error {
	fs := flag.NewFlagSet("profiles order set", flag.ContinueOnError)
	provider := fs.String("provider", "openai-codex", "provider id")
	profilesCSV := fs.String("profiles", "", "comma-separated profile order")
	if err := fs.Parse(args); err != nil {
		return err
	}

	csvValue := strings.TrimSpace(*profilesCSV)
	if csvValue == "" {
		rest := fs.Args()
		if len(rest) > 0 {
			csvValue = rest[0]
		}
	}
	if csvValue == "" {
		return fmt.Errorf("profiles order tidak boleh kosong")
	}

	parts := strings.Split(csvValue, ",")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	if len(clean) == 0 {
		return fmt.Errorf("profiles order tidak valid")
	}

	if err := manager.SetOrder(*provider, clean); err != nil {
		return err
	}
	fmt.Printf("Order profil untuk provider %s tersimpan: %s\n", *provider, strings.Join(clean, ", "))
	return nil
}

func runStatus(manager authStore) error {
	store, err := manager.LoadStore()
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("Store: %s\n", manager.StorePath())
	fmt.Println(string(payload))
	return nil
}

func buildAuthorizeURL(clientID, redirectURI, scope, challenge, state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", scope)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	q.Set("originator", "pi")
	return "https://auth.openai.com/oauth/authorize?" + q.Encode()
}

func waitForCallbackOrPaste(redirectURI, authURL string) (code string, state string, err error) {
	callbackURL, err := url.Parse(redirectURI)
	if err != nil {
		return "", "", err
	}

	resultCh := make(chan struct {
		code  string
		state string
		err   error
	}, 1)

	listener, listenErr := net.Listen("tcp", callbackURL.Host)
	if listenErr == nil {
		mux := http.NewServeMux()
		mux.HandleFunc(callbackURL.Path, func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			code := strings.TrimSpace(q.Get("code"))
			state := strings.TrimSpace(q.Get("state"))
			if code == "" || state == "" {
				http.Error(w, "missing code/state", http.StatusBadRequest)
				resultCh <- struct {
					code  string
					state string
					err   error
				}{err: fmt.Errorf("missing code/state")}
				return
			}
			_, _ = w.Write([]byte("Login berhasil. Silakan kembali ke terminal."))
			resultCh <- struct {
				code  string
				state string
				err   error
			}{code: code, state: state}
		})

		srv := &http.Server{Handler: mux}
		go func() {
			_ = srv.Serve(listener)
		}()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
		}()
	}

	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Gagal membuka browser otomatis: %v\n", err)
		fmt.Println("Silakan buka URL login secara manual dari output di atas.")
	}

	if listenErr == nil {
		select {
		case res := <-resultCh:
			return res.code, res.state, res.err
		case <-time.After(3 * time.Minute):
			fmt.Println("Timeout menunggu callback otomatis. Lanjut mode paste URL.")
		}
	}

	fmt.Print("Paste redirect URL callback: ")
	reader := bufio.NewReader(os.Stdin)
	line, readErr := reader.ReadString('\n')
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", "", readErr
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", fmt.Errorf("callback URL kosong")
	}

	parsed, parseErr := url.Parse(line)
	if parseErr != nil {
		return "", "", parseErr
	}
	q := parsed.Query()
	code = strings.TrimSpace(q.Get("code"))
	state = strings.TrimSpace(q.Get("state"))
	if code == "" || state == "" {
		return "", "", fmt.Errorf("callback URL tidak memiliki code/state")
	}
	return code, state, nil
}

type tokenExchangeResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	IDToken      string `json:"id_token"`
}

func exchangeToken(code, verifier, clientID, redirectURI string) (*tokenExchangeResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("client_id", clientID)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest(http.MethodPost, "https://auth.openai.com/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("oauth token exchange gagal status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	out := &tokenExchangeResponse{}
	if err = json.Unmarshal(body, out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return nil, fmt.Errorf("response token tidak memiliki access_token")
	}
	if out.ExpiresIn <= 0 {
		out.ExpiresIn = int64((30 * 24 * time.Hour).Seconds())
	}
	return out, nil
}

func openBrowser(target string) error {
	if strings.TrimSpace(target) == "" {
		return nil
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

func printUsage() {
	fmt.Println("cs-ai-auth commands:")
	fmt.Println("  login --provider openai-codex [--client-id ...]")
	fmt.Println("  profiles list [--provider openai-codex]")
	fmt.Println("  profiles order set --provider openai-codex --profiles id1,id2")
	fmt.Println("  status")
	fmt.Println()
	fmt.Println("global flags:")
	fmt.Println("  --store-path <path>  explicit auth store file path")
	fmt.Println("                       default: <project-root>/.cs-ai/auth-profiles.json")
	fmt.Println("                       (project root = nearest dir with go.mod)")
	fmt.Println("                       fallback: ~/.cs-ai/auth-profiles.json")
	fmt.Println("                       env override: CS_AI_AUTH_STORE_PATH")
	fmt.Println("  --mongo-uri <uri>          gunakan MongoDB untuk auth profile store")
	fmt.Println("  --mongo-database <name>    default: cs_ai")
	fmt.Println("  --mongo-collection <name>  default: auth_profiles")
	fmt.Println("                             env fallback: CS_AI_AUTH_MONGO_*, CS_AI_MONGO_*, MONGODB_*, MONGO_*")
}

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
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		parts := strings.Split(token, ".")
		if len(parts) < 2 {
			continue
		}
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			continue
		}
		claims := map[string]interface{}{}
		if err = json.Unmarshal(payload, &claims); err != nil {
			continue
		}
		email, _ := claims["email"].(string)
		email = strings.TrimSpace(strings.ToLower(email))
		if email != "" {
			return email
		}
	}
	return ""
}
