# Session TTL Configuration Examples

## Overview
CS-AI sekarang mendukung konfigurasi TTL (Time To Live) untuk session messages melalui `Options.SessionTTL`.

## Default Behavior
- **Default TTL**: 12 jam (sesuai untuk barbershop shift pagi-sore)
- **Auto-expiration**: Session messages otomatis expired setelah TTL
- **Memory efficient**: Mencegah memory Redis penuh

## Basic Usage

### 1. Default TTL (12 jam)
```go
import (
    "time"
    "github.com/your-repo/pkg/cs-ai"
)

// Gunakan default TTL 12 jam
cs := cs_ai.New(apiKey, model)
```

### 2. Custom TTL untuk Barbershop
```go
// TTL 8 jam untuk shift pagi
options := &cs_ai.Options{
    SessionTTL: 8 * time.Hour,
    Redis:      redisClient,
}
cs := cs_ai.New(apiKey, model, options)
```

### 3. TTL untuk Salon (24 jam)
```go
// TTL 24 jam untuk salon yang buka full day
options := &cs_ai.Options{
    SessionTTL: 24 * time.Hour,
    Redis:      redisClient,
}
cs := cs_ai.New(apiKey, model, options)
```

### 4. TTL untuk Weekend Coverage
```go
// TTL 72 jam untuk cover weekend
options := &cs_ai.Options{
    SessionTTL: 72 * time.Hour,
    Redis:      redisClient,
}
cs := cs_ai.New(apiKey, model, options)
```

### 5. TTL untuk Testing/Development
```go
// TTL 1 jam untuk testing
options := &cs_ai.Options{
    SessionTTL: 1 * time.Hour,
    Redis:      redisClient,
}
cs := cs_ai.New(apiKey, model, options)
```

## TTL Recommendations by Business Type

| Business Type | Recommended TTL | Reason |
|---------------|-----------------|---------|
| **Barbershop Shift** | 8-12 jam | Covers 1 shift kerja |
| **Salon Full Day** | 24 jam | Covers 1 hari kerja |
| **Weekend Business** | 72 jam | Covers weekend |
| **High Volume** | 6-8 jam | Memory management |
| **Low Volume** | 24-48 jam | Better UX |

## Implementation Details

### Options Struct
```go
type Options struct {
    // ... existing fields ...
    
    // === Cache & Session Options ===
    SessionTTL time.Duration // TTL untuk session messages (default: 12 jam)
    
    // ... other fields ...
}
```

### Constructor Logic
```go
// Set default SessionTTL jika tidak diatur
if cs.options.SessionTTL == 0 {
    cs.options.SessionTTL = 12 * time.Hour // Default 12 jam
}
```

### Session Save Logic
```go
// Set TTL dari Options atau default 12 jam
var ttl time.Duration
if c.options.SessionTTL > 0 {
    ttl = c.options.SessionTTL
} else {
    ttl = 12 * time.Hour // Default TTL 12 jam
}
err = c.options.Redis.Set(c.options.Redis.Context(), key, data, ttl).Err()
```

## Benefits

1. **Flexible Configuration**: Set TTL sesuai kebutuhan bisnis
2. **Memory Management**: Auto-cleanup session lama
3. **Cost Effective**: Hemat storage Redis
4. **Business Logic**: TTL sesuai siklus operasional
5. **Backward Compatible**: Default behavior tidak berubah

## Migration Notes

- **Existing code**: Tidak perlu diubah, akan menggunakan default 12 jam
- **New code**: Bisa set custom TTL sesuai kebutuhan
- **Redis key format**: Tetap sama `ai:session:{sessionID}`
- **TTL behavior**: Session akan otomatis expired setelah TTL
