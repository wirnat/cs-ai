package cs_ai

import (
	"context"
	"fmt"
	"strings"
)

const (
	defaultGroundingRepairMaxAttempts = 1
)

type normalizedGroundingRepairOptions struct {
	Enabled     bool
	MaxAttempts int
}

type groundingVerifierResult struct {
	NeedsRepair bool   `json:"needs_repair"`
	Reason      string `json:"reason"`
}

func normalizeGroundingRepairOptions(raw *GroundingRepairOptions) normalizedGroundingRepairOptions {
	options := normalizedGroundingRepairOptions{
		Enabled:     false,
		MaxAttempts: defaultGroundingRepairMaxAttempts,
	}
	if raw == nil {
		return options
	}
	options.Enabled = raw.Enabled
	if raw.MaxAttempts > 0 {
		options.MaxAttempts = raw.MaxAttempts
	}
	return options
}

func shouldRunGroundingVerifier(userText string, assistantText string, availableToolCodes []string) bool {
	if len(availableToolCodes) == 0 {
		return false
	}
	userText = strings.ToLower(strings.TrimSpace(userText))
	assistantText = strings.ToLower(strings.TrimSpace(assistantText))
	if userText == "" || assistantText == "" {
		return false
	}

	riskySignals := []string{
		"harga", "price", "pricelist", "tarif", "biaya",
		"jam", "slot", "tersedia", "available", "ketersediaan",
		"durasi", "antrian", "queue", "promo", "diskon", "poin", "point",
		"booking", "reservasi", "jadwal",
	}
	for _, signal := range riskySignals {
		if strings.Contains(userText, signal) || strings.Contains(assistantText, signal) {
			return true
		}
	}
	return false
}

func fallbackGroundingRepairHeuristic(userText string, assistantText string) (bool, string) {
	if shouldRunGroundingVerifier(userText, assistantText, []string{"_dummy"}) {
		return true, "high_risk_factual_turn"
	}
	return false, ""
}

func (c *CsAI) evaluateGroundingRepairNeed(
	ctx context.Context,
	userText string,
	assistantText string,
	availableToolCodes []string,
) (bool, string) {
	if c == nil {
		return false, ""
	}
	if !shouldRunGroundingVerifier(userText, assistantText, availableToolCodes) {
		return false, ""
	}

	verifierSystem := strings.Join([]string{
		"Kamu adalah grounding verifier internal.",
		"Tugasmu hanya memutuskan apakah jawaban asisten perlu di-repair dengan tool call tambahan.",
		"Balas HANYA JSON valid tanpa markdown: {\"needs_repair\":true|false,\"reason\":\"...\"}",
		"Set needs_repair=true jika draft jawaban memuat fakta numerik/operasional yang berisiko halusinasi tanpa evidence tool di turn ini.",
		"Set needs_repair=false jika draft jawaban aman tanpa tool atau sudah jelas bukan fakta operasional.",
	}, "\n")

	verifierUser := strings.Join([]string{
		"Pesan user:",
		strings.TrimSpace(userText),
		"",
		"Draft jawaban asisten:",
		strings.TrimSpace(assistantText),
		"",
		"Tool yang tersedia:",
		strings.Join(normalizeAllowedToolCodes(availableToolCodes), ", "),
		"",
		"Evidence tool pada turn ini:",
		"(kosong)",
	}, "\n")

	roleMessage := []map[string]interface{}{
		{
			"role":    "system",
			"content": verifierSystem,
		},
		{
			"role":    "user",
			"content": verifierUser,
		},
	}

	verifierMessage, err := c.sendWithModelCandidates(ctx, "", roleMessage, nil)
	if err != nil {
		return fallbackGroundingRepairHeuristic(userText, assistantText)
	}

	result := groundingVerifierResult{}
	if err := decodeJSONObjectStrict(verifierMessage.Content, &result); err != nil {
		return fallbackGroundingRepairHeuristic(userText, assistantText)
	}
	if result.NeedsRepair {
		return true, strings.TrimSpace(result.Reason)
	}
	return false, strings.TrimSpace(result.Reason)
}

func buildGroundingRepairInstruction(reason string, availableToolCodes []string, attempt int) string {
	tools := strings.Join(normalizeAllowedToolCodes(availableToolCodes), ", ")
	if tools == "" {
		tools = "(tidak ada)"
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "possible_ungrounded_facts"
	}
	if attempt <= 0 {
		attempt = 1
	}

	return fmt.Sprintf(
		"GROUNDING REPAIR PASS #%d: Draft jawaban berisiko tidak grounded (%s). Tetap LLM-driven: pilih tool hanya jika perlu, namun untuk fakta harga/jadwal/slot/promo/poin/booking wajib gunakan evidence tool terbaru sebelum menyebut angka/fakta operasional. Jika evidence tidak tersedia, katakan belum ada data dan minta klarifikasi. Tool tersedia: %s",
		attempt,
		reason,
		tools,
	)
}
