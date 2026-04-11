package cs_ai

import (
	"context"
	"strings"
)

type httpLogMetadataContextKey struct{}

func WithHTTPLogMetadata(ctx context.Context, meta HTTPLogMetadata) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	current := extractHTTPLogMetadata(ctx)
	if trimmed := strings.TrimSpace(meta.SessionID); trimmed != "" {
		current.SessionID = trimmed
	}
	if trimmed := strings.TrimSpace(meta.TurnID); trimmed != "" {
		current.TurnID = trimmed
	}
	if trimmed := strings.TrimSpace(meta.Stage); trimmed != "" {
		current.Stage = trimmed
	}
	if trimmed := strings.TrimSpace(meta.RequestKind); trimmed != "" {
		current.RequestKind = trimmed
	}
	if meta.Hop > 0 || current.Hop == 0 {
		current.Hop = meta.Hop
	}
	if trimmed := strings.TrimSpace(meta.ProviderName); trimmed != "" {
		current.ProviderName = trimmed
	}
	return context.WithValue(ctx, httpLogMetadataContextKey{}, current)
}

func extractHTTPLogMetadata(ctx context.Context) HTTPLogMetadata {
	if ctx == nil {
		return HTTPLogMetadata{}
	}
	meta, _ := ctx.Value(httpLogMetadataContextKey{}).(HTTPLogMetadata)
	return meta
}

func resolveHTTPLogMetadata(ctx context.Context) HTTPLogMetadata {
	meta := extractHTTPLogMetadata(ctx)
	if rt := extractStreamRuntime(ctx); rt != nil && strings.TrimSpace(meta.TurnID) == "" {
		meta.TurnID = strings.TrimSpace(rt.turnID)
	}
	if stageCtx, ok := extractStageStreaming(ctx); ok && strings.TrimSpace(meta.Stage) == "" {
		meta.Stage = stageName(stageCtx.Stage)
	}
	if strings.TrimSpace(meta.RequestKind) == "" {
		meta.RequestKind = strings.TrimSpace(meta.Stage)
	}
	return meta
}
