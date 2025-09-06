# CS-AI Package

CS-AI adalah package Go yang menyediakan interface untuk AI conversation dengan berbagai storage backend yang fleksibel.

## 📑 Table of Contents

- [🚀 Fitur Utama](#-fitur-utama)
- [📦 Storage Providers](#-storage-providers)
- [🛠️ Installation](#️-installation)
- [⚡ Quick Reference Card](#-quick-reference-card)
- [🎯 Intent System](#-intent-system)
- [🔧 Middleware System](#-middleware-system)
- [📖 Quick Start](#-quick-start)
- [💡 Best Practices & Tips](#-best-practices--tips)
- [🎯 Use Cases](#-use-cases)
- [🧪 Testing](#-testing)
- [⚡ Performance Tips](#-performance-tips)
- [📊 Monitoring & Observability](#-monitoring--observability)
- [🔒 Security Best Practices](#-security-best-practices)
- [📚 Examples](#-examples)
- [🚧 Roadmap](#-roadmap)
- [📋 Changelog](#-changelog)
- [🔍 Troubleshooting](#-troubleshooting)
- [📖 API Reference](#-api-reference)
- [❓ FAQ](#-faq-frequently-asked-questions)
- [🆘 Common Issues & Solutions](#-common-issues--solutions)
- [🤝 Contributing](#-contributing)
- [🚀 Deployment & Production](#-deployment--production)
- [🌟 Community & Support](#-community--support)
- [📄 License](#-license)

## 🚀 Fitur Utama

- **Multiple Storage Backends**: Redis ✅, MongoDB ✅, DynamoDB 🚧, In-Memory ✅
- **Unified Interface**: API yang sama untuk semua storage
- **Backward Compatibility**: Kode lama tetap berfungsi
- **Easy Migration**: Ganti storage tanpa ubah kode
- **TTL Support**: Automatic session expiration
- **Health Monitoring**: Built-in health checks
- **Security Features**: Rate limiting, spam detection, security logging
- **Intent System**: Powerful intent-based conversation handling
- **Middleware Support**: Authentication, rate limiting, caching, logging
- **Tool Function Integration**: Seamless AI tool calling with intent handlers
- **Learning Data Storage**: AI model improvement dengan feedback system
- **Performance Optimization**: Automatic indexing dan connection pooling
- **Production Ready**: MongoDB dan Redis fully implemented untuk production use

## 📦 Storage Providers

### 1. Redis Storage (Fully Implemented)
- Production-ready dengan connection pooling
- Mendukung authentication dan database selection
- Configurable TTL dan timeout settings

### 2. In-Memory Storage (Fully Implemented)
- Fast local storage untuk development/testing
- Built-in cleanup dan statistics
- Tidak ada external dependencies

### 3. MongoDB Storage (Fully Implemented)
- **Production-ready** dengan connection pooling dan automatic indexing
- **TTL Support** dengan automatic session expiration
- **Learning Data Storage** untuk AI model improvement
- **Security Logs** untuk monitoring dan audit
- **Performance Optimization** dengan compound indexes
- **Flexible Schema** untuk complex data structures
- **Horizontal Scaling** support dengan MongoDB clusters
- **Configurable Collections** untuk different data types
- **Health Monitoring** dengan built-in health checks
- **Statistics & Analytics** untuk usage tracking

### 4. DynamoDB Storage (Stub Implementation)
- Interface ready, implementation pending
- Memerlukan AWS SDK dependency
- Planned features: auto-scaling, backup

## 🗄️ Storage Configuration Examples

### MongoDB Configuration

```go
import (
    "time"
    cs_ai "github.com/wirnat/cs-ai"
    "github.com/wirnat/cs-ai/model"
)

// Basic MongoDB setup
mongoConfig := cs_ai.StorageConfig{
    Type:            cs_ai.StorageTypeMongo,
    MongoURI:        "mongodb://localhost:27017",
    MongoDatabase:   "cs_ai",
    MongoCollection: "sessions",
    SessionTTL:      24 * time.Hour,
    Timeout:         10 * time.Second,
}

cs := cs_ai.New("your-api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: &mongoConfig,
    UseTool:       true,
    SessionTTL:    24 * time.Hour,
})

// Production MongoDB setup with authentication
mongoConfigProd := cs_ai.StorageConfig{
    Type:            cs_ai.StorageTypeMongo,
    MongoURI:        "mongodb://username:password@cluster.mongodb.net:27017",
    MongoDatabase:   "production_cs_ai",
    MongoCollection: "user_sessions",
    SessionTTL:      48 * time.Hour,
    Timeout:         15 * time.Second,
    MaxRetries:      3,
}

// MongoDB Atlas setup
mongoConfigAtlas := cs_ai.StorageConfig{
    Type:            cs_ai.StorageTypeMongo,
    MongoURI:        "mongodb+srv://username:password@cluster.mongodb.net/cs_ai?retryWrites=true&w=majority",
    MongoDatabase:   "cs_ai",
    MongoCollection: "sessions",
    SessionTTL:      12 * time.Hour,
    Timeout:         10 * time.Second,
}
```

### Redis Configuration

```go
// Basic Redis setup
redisConfig := cs_ai.StorageConfig{
    Type:          cs_ai.StorageTypeRedis,
    RedisAddress:  "localhost:6379",
    RedisPassword: "",
    RedisDB:       0,
    SessionTTL:    12 * time.Hour,
    Timeout:       5 * time.Second,
}

// Redis with authentication
redisConfigAuth := cs_ai.StorageConfig{
    Type:          cs_ai.StorageTypeRedis,
    RedisAddress:  "localhost:6379",
    RedisPassword: "your-redis-password",
    RedisDB:       1,
    SessionTTL:    12 * time.Hour,
    Timeout:       5 * time.Second,
}
```

### In-Memory Configuration

```go
// For development/testing
memoryConfig := cs_ai.StorageConfig{
    Type:       cs_ai.StorageTypeInMemory,
    SessionTTL: 1 * time.Hour,
    Timeout:    1 * time.Second,
}
```

## 🛠️ Installation

```bash
go get github.com/wirnat/cs-ai
```

### MongoDB Dependencies

Untuk menggunakan MongoDB storage, tambahkan dependency MongoDB driver:

```bash
go get go.mongodb.org/mongo-driver/mongo
```

### Redis Dependencies

Untuk menggunakan Redis storage, tambahkan dependency Redis client:

```bash
go get github.com/redis/go-redis/v9
```

## ⚡ Quick Reference Card

### Basic Setup
```go
// 1. Import
import cs_ai "github.com/wirnat/cs-ai"
import "github.com/wirnat/cs-ai/model"

// 2. Initialize
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    UseTool: true, // WAJIB untuk intent system
})

// 2a. Advanced LLM Configuration (Optional)
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    UseTool:          true,
    Temperature:      0.7,           // Lebih kreatif (0.0-2.0)
    TopP:            0.9,            // Lebih fokus (0.0-1.0)
    FrequencyPenalty: 0.5,           // Kurangi repetisi token (-2.0-2.0)
    PresencePenalty:  -0.5,          // Kurangi repetisi topik (-2.0-2.0)
})

// 3. Add Intent
cs.Add(YourIntent{})

// 4. Execute
response, err := cs.Exec(ctx, "session-id", cs_ai.UserMessage{
    Message: "user message",
})
```

### Intent Interface
```go
type Intent interface {
    Code() string
    Handle(ctx context.Context, req map[string]interface{}) (interface{}, error)
    Description() []string
    Param() interface{}
}
```

### Built-in Middleware
```go
// Authentication
cs.AddWithAuth(intents, "role")

// Rate Limiting
cs.AddWithRateLimit(intents, maxRequests, timeWindow)

// Caching
cs.AddWithCache(intents, ttl)

// Logging
cs.AddWithLogging(intents, logger)
```

### Storage Types
```go
cs_ai.StorageTypeRedis    // Redis
cs_ai.StorageTypeMongo    // MongoDB
cs_ai.StorageTypeDynamo   // DynamoDB
cs_ai.StorageTypeInMemory // In-Memory
```

## 🎯 Intent System

CS-AI menyediakan sistem intent yang powerful untuk menangani berbagai jenis percakapan AI. Intent adalah handler yang mendefinisikan bagaimana AI merespons permintaan spesifik dari user.

### Intent Interface

```go
type Intent interface {
    Code() string                                    // Unique identifier untuk intent
    Handle(ctx context.Context, req map[string]interface{}) (interface{}, error) // Handler function
    Description() []string                           // Deskripsi untuk AI model
    Param() interface{}                             // Parameter schema untuk validation
}
```

### Membuat Custom Intent

```go
package main

import (
    "context"
    cs_ai "github.com/wirnat/cs-ai"
)

// ProductSearch Intent
type ProductSearch struct{}

func (p ProductSearch) Code() string {
    return "product-search"
}

func (p ProductSearch) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    query, _ := params["query"].(string)
    category, _ := params["category"].(string)
    
    // Implementasi pencarian produk
    return map[string]interface{}{
        "products": []map[string]interface{}{
            {
                "name": "Product 1",
                "price": 100000,
                "category": category,
            },
        },
        "query": query,
    }, nil
}

func (p ProductSearch) Description() []string {
    return []string{
        "Mencari produk berdasarkan nama atau kategori",
        "Cari produk yang tersedia",
        "Lihat daftar produk",
    }
}

// Parameter schema untuk validation
type ProductSearchParams struct {
    Query    string `json:"query" validate:"required" description:"Kata kunci pencarian produk"`
    Category string `json:"category" description:"Kategori produk (optional)"`
}

func (p ProductSearch) Param() interface{} {
    return ProductSearchParams{}
}
```

### Menambahkan Intent ke CS-AI

```go
// Basic intent addition
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{})
cs.Add(ProductSearch{})

// Multiple intents sekaligus
intents := []cs_ai.Intent{
    ProductSearch{},
    BookingService{},
    GetUserProfile{},
}
cs.Adds(intents, nil) // tanpa middleware
```

## 🔧 Middleware System

CS-AI menyediakan sistem middleware yang powerful untuk menambahkan cross-cutting concerns seperti authentication, rate limiting, caching, dan logging.

### Middleware Interface

```go
type Middleware interface {
    Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error)
    Name() string        // Nama middleware untuk logging/debugging
    AppliesTo() []string // Intent codes yang diterapkan (kosong = global)
    Priority() int       // Prioritas eksekusi (lebih rendah = lebih awal)
}
```

### Built-in Middleware

#### 1. Authentication Middleware
```go
// Tambahkan intents dengan authentication
cs.AddWithAuth([]cs_ai.Intent{adminIntent1, adminIntent2}, "admin")
cs.AddWithAuth([]cs_ai.Intent{userIntent1, userIntent2}, "user")
```

#### 2. Rate Limiting Middleware
```go
// Rate limiting: 10 requests per minute
cs.AddWithRateLimit([]cs_ai.Intent{userIntents}, 10, time.Minute)

// Rate limiting: 5 requests per hour untuk sensitive operations
cs.AddWithRateLimit([]cs_ai.Intent{deleteAccountIntent}, 5, time.Hour)
```

#### 3. Caching Middleware
```go
// Cache hasil intent selama 5 menit
cs.AddWithCache([]cs_ai.Intent{searchIntent}, 5*time.Minute)

// Cache untuk search dan product intents
cs.AddWithCache([]cs_ai.Intent{searchIntent, productIntent}, 10*time.Minute)
```

#### 4. Logging Middleware
```go
// Logging untuk semua intents
logger := log.New(os.Stdout, "[AI] ", log.LstdFlags)
cs.AddWithLogging([]cs_ai.Intent{allIntents}, logger)

// Logging khusus untuk admin intents
adminLogger := log.New(os.Stdout, "[ADMIN] ", log.LstdFlags)
cs.AddWithLogging([]cs_ai.Intent{adminIntents}, adminLogger)
```

### Custom Middleware

```go
// Custom validation middleware
cs.AddGlobalMiddleware("parameter_validator", 20, 
    func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
        // Validasi parameter
        if len(mctx.Parameters) == 0 {
            return nil, fmt.Errorf("parameters required for %s", mctx.IntentCode)
        }
        
        // Lanjut ke middleware/intent berikutnya
        return next(ctx, mctx)
    })

// Custom error handling middleware
cs.AddGlobalMiddleware("error_handler", 1,
    func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
        result, err := next(ctx, mctx)
        if err != nil {
            // Log error atau kirim ke error tracking service
            log.Printf("Error in %s: %v", mctx.IntentCode, err)
        }
        return result, err
    })
```

### Advanced Middleware Combinations

```go
// Kombinasi multiple middleware untuk intents yang sama
cs.AddsWithMiddleware([]cs_ai.Intent{userIntents}, []cs_ai.Middleware{
    NewLoggingMiddleware(logger),
    NewRateLimitMiddleware(5, time.Minute),
    NewCacheMiddleware(5*time.Minute, []string{}),
})

// Function-based middleware untuk intents spesifik
cs.AddsWithFunc([]cs_ai.Intent{adminIntents}, "admin_validator", 15,
    func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
        // Validasi khusus admin
        if mctx.Metadata["user_role"] != "admin" {
            return nil, fmt.Errorf("admin access required")
        }
        return next(ctx, mctx)
    })
```

## 📖 Quick Start

### Complete Example: Setup Client, System Prompt, dan Intent

Berikut adalah contoh lengkap untuk setup CS-AI dengan intent system:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    cs_ai "github.com/wirnat/cs-ai"
    "github.com/wirnat/cs-ai/model"
)

// 1. Define Custom Intents
type ProductSearch struct{}

func (p ProductSearch) Code() string {
    return "product-search"
}

func (p ProductSearch) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    query, _ := params["query"].(string)
    category, _ := params["category"].(string)
    
    // Simulasi pencarian produk
    products := []map[string]interface{}{
        {
            "id":       1,
            "name":     "Hairnerd Pomade",
            "price":    70000,
            "category": "pomade",
            "stock":    10,
        },
        {
            "id":       2,
            "name":     "Hairnerd Shampoo",
            "price":    40000,
            "category": "shampoo",
            "stock":    15,
        },
    }
    
    // Filter berdasarkan query dan category
    filteredProducts := []map[string]interface{}{}
    for _, product := range products {
        if query != "" && !contains(product["name"].(string), query) {
            continue
        }
        if category != "" && product["category"] != category {
            continue
        }
        filteredProducts = append(filteredProducts, product)
    }
    
    return map[string]interface{}{
        "products": filteredProducts,
        "total":    len(filteredProducts),
        "query":    query,
    }, nil
}

func (p ProductSearch) Description() []string {
    return []string{
        "Mencari produk berdasarkan nama atau kategori",
        "Cari produk yang tersedia di toko",
        "Lihat daftar produk dengan filter",
    }
}

type ProductSearchParams struct {
    Query    string `json:"query" description:"Kata kunci pencarian produk"`
    Category string `json:"category" description:"Kategori produk (pomade, shampoo, dll)"`
}

func (p ProductSearch) Param() interface{} {
    return ProductSearchParams{}
}

// Booking Service Intent
type BookingService struct{}

func (b BookingService) Code() string {
    return "booking-service"
}

func (b BookingService) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    service, _ := params["service"].(string)
    date, _ := params["date"].(string)
    time, _ := params["time"].(string)
    
    // Simulasi booking
    bookingCode := fmt.Sprintf("BK%04d", time.Now().Unix()%10000)
    
    return map[string]interface{}{
        "booking_code": bookingCode,
        "service":      service,
        "date":         date,
        "time":         time,
        "status":       "confirmed",
        "message":      "Booking berhasil dibuat",
    }, nil
}

func (b BookingService) Description() []string {
    return []string{
        "Booking layanan seperti potong rambut, cuci rambut",
        "Reservasi jadwal untuk layanan salon",
        "Membuat appointment untuk layanan",
    }
}

type BookingParams struct {
    Service string `json:"service" validate:"required" description:"Nama layanan yang akan dibooking"`
    Date    string `json:"date" validate:"required" description:"Tanggal booking (format: YYYY-MM-DD)"`
    Time    string `json:"time" validate:"required" description:"Waktu booking (format: HH:MM)"`
}

func (b BookingService) Param() interface{} {
    return BookingParams{}
}

// Helper function
func contains(s, substr string) bool {
    return len(s) >= len(substr) && s[:len(substr)] == substr
}

func main() {
    // 2. Setup Storage Configuration
    config := &cs_ai.StorageConfig{
        Type:        cs_ai.StorageTypeInMemory,
        SessionTTL:  1 * time.Hour,
        Timeout:     5 * time.Second,
    }

    // 3. Initialize CS-AI Client
    cs := cs_ai.New("your-deepseek-api-key", model.NewDeepSeekChat(), cs_ai.Options{
        StorageConfig: config,
        SessionTTL:    1 * time.Hour,
        UseTool:       true, // Enable tool calling untuk intents
        LogChatFile:   true, // Enable logging
    })

    // 4. Add Intents dengan Middleware
    logger := log.New(log.Writer(), "[HAIRKATZ] ", log.LstdFlags)
    
    // Add intents dengan logging middleware
    cs.AddWithLogging([]cs_ai.Intent{
        ProductSearch{},
        BookingService{},
    }, logger)

    // Add rate limiting untuk booking intent
    cs.AddWithRateLimit([]cs_ai.Intent{BookingService{}}, 5, time.Minute)

    // 5. Define System Prompts
    systemPrompts := []string{
        "Kamu adalah Hairo, asisten AI untuk salon Hairkatz.",
        "Kamu membantu pelanggan dengan pencarian produk dan booking layanan.",
        "Selalu ramah dan profesional dalam berkomunikasi.",
        "Jika pelanggan ingin mencari produk, gunakan fungsi product-search.",
        "Jika pelanggan ingin booking layanan, gunakan fungsi booking-service.",
        "Pastikan untuk meminta informasi yang diperlukan sebelum melakukan booking.",
    }

    // 6. Simulasi Percakapan
    ctx := context.Background()
    sessionID := "user-123"

    // Test 1: Pencarian Produk
    fmt.Println("=== Test 1: Pencarian Produk ===")
    response1, err := cs.Exec(ctx, sessionID, cs_ai.UserMessage{
        Message:         "Saya ingin cari pomade yang bagus",
        ParticipantName: "John",
    }, systemPrompts...)
    
    if err != nil {
        log.Printf("Error: %v", err)
    } else {
        fmt.Printf("AI Response: %s\n\n", response1.Content)
    }

    // Test 2: Booking Layanan
    fmt.Println("=== Test 2: Booking Layanan ===")
    response2, err := cs.Exec(ctx, sessionID, cs_ai.UserMessage{
        Message:         "Saya ingin booking potong rambut hari ini jam 2 siang",
        ParticipantName: "John",
    }, systemPrompts...)
    
    if err != nil {
        log.Printf("Error: %v", err)
    } else {
        fmt.Printf("AI Response: %s\n\n", response2.Content)
    }

    // Test 3: Pencarian dengan Filter
    fmt.Println("=== Test 3: Pencarian dengan Filter ===")
    response3, err := cs.Exec(ctx, sessionID, cs_ai.UserMessage{
        Message:         "Tunjukkan semua produk shampoo yang tersedia",
        ParticipantName: "John",
    }, systemPrompts...)
    
    if err != nil {
        log.Printf("Error: %v", err)
    } else {
        fmt.Printf("AI Response: %s\n\n", response3.Content)
    }
}
```

### Output yang Diharapkan:

```
=== Test 1: Pencarian Produk ===
AI Response: Saya menemukan produk pomade yang bagus untuk Anda! Berikut adalah produk pomade yang tersedia:

- Hairnerd Pomade
  Harga: Rp 70,000
  Stok: 10 unit
  Kategori: pomade

Apakah ada yang ingin Anda tanyakan tentang produk ini?

=== Test 2: Booking Layanan ===
AI Response: Baik, saya akan membantu Anda booking layanan potong rambut. 

Booking berhasil dibuat dengan detail:
- Kode Booking: BK1234
- Layanan: potong rambut
- Tanggal: 2024-01-15
- Waktu: 14:00
- Status: confirmed

Silakan datang tepat waktu untuk layanan Anda. Terima kasih!

=== Test 3: Pencarian dengan Filter ===
AI Response: Berikut adalah semua produk shampoo yang tersedia:

- Hairnerd Shampoo
  Harga: Rp 40,000
  Stok: 15 unit
  Kategori: shampoo

Total ditemukan: 1 produk shampoo. Apakah ada yang ingin Anda tanyakan?
```

### Langkah-langkah Setup Detail

#### 1. **Setup Dependencies**
```bash
# Install CS-AI package
go get github.com/wirnat/cs-ai

# Install model dependencies (jika menggunakan DeepSeek)
go get github.com/wirnat/cs-ai/model
```

#### 2. **Konfigurasi Storage**
```go
// Pilih storage type sesuai kebutuhan
config := &cs_ai.StorageConfig{
    Type:        cs_ai.StorageTypeInMemory, // atau Redis, MongoDB, DynamoDB
    SessionTTL:  1 * time.Hour,             // Session expiration
    Timeout:     5 * time.Second,           // Operation timeout
}
```

#### 3. **Inisialisasi Client**
```go
cs := cs_ai.New("your-api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: config,
    SessionTTL:    1 * time.Hour,
    UseTool:       true,  // WAJIB true untuk intent system
    LogChatFile:   true,  // Optional: untuk debugging
})
```

#### 4. **Definisi System Prompts**
```go
systemPrompts := []string{
    "Kamu adalah asisten AI yang membantu pelanggan.",
    "Gunakan fungsi yang tersedia untuk membantu pelanggan.",
    "Selalu ramah dan profesional.",
    // Tambahkan prompt sesuai kebutuhan bisnis
}
```

#### 5. **Implementasi Intent**
```go
// Implementasi interface Intent
type YourIntent struct{}

func (y YourIntent) Code() string {
    return "your-intent-code"
}

func (y YourIntent) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    // Business logic di sini
    return result, nil
}

func (y YourIntent) Description() []string {
    return []string{
        "Deskripsi untuk AI model",
        "Apa yang intent ini lakukan",
    }
}

func (y YourIntent) Param() interface{} {
    return YourParamStruct{}
}
```

#### 6. **Registrasi Intent dengan Middleware**
```go
// Basic registration
cs.Add(YourIntent{})

// Dengan middleware
cs.AddWithLogging([]cs_ai.Intent{YourIntent{}}, logger)
cs.AddWithRateLimit([]cs_ai.Intent{YourIntent{}}, 10, time.Minute)
cs.AddWithCache([]cs_ai.Intent{YourIntent{}}, 5*time.Minute)
```

#### 7. **Eksekusi Percakapan**
```go
response, err := cs.Exec(ctx, sessionID, cs_ai.UserMessage{
    Message:         "User message",
    ParticipantName: "User Name",
}, systemPrompts...)
```

### 💡 Best Practices & Tips

#### **Intent Design**
- **Gunakan nama intent yang deskriptif**: `product-search`, `booking-service`, `user-profile`
- **Deskripsi yang jelas**: Berikan deskripsi yang membantu AI memahami kapan menggunakan intent
- **Parameter validation**: Selalu validasi parameter yang masuk ke handler
- **Error handling**: Handle error dengan baik dan berikan pesan yang informatif

#### **System Prompts**
- **Jelaskan role AI**: "Kamu adalah asisten untuk salon Hairkatz"
- **Instruksi penggunaan fungsi**: "Gunakan fungsi product-search untuk mencari produk"

#### **LLM Parameter Tuning**
- **Temperature (0.0-2.0)**: 
  - `0.0-0.3`: Deterministic, konsisten
  - `0.4-0.7`: Balanced creativity
  - `0.8-2.0`: Sangat kreatif, variatif
- **TopP (0.0-1.0)**:
  - `0.1-0.3`: Sangat fokus pada token terbaik
  - `0.4-0.7`: Balanced sampling
  - `0.8-1.0`: Lebih variatif
- **FrequencyPenalty (-2.0-2.0)**:
  - `0.0`: Tidak ada penalty (default)
  - `0.1-1.0`: Kurangi repetisi token
  - `1.0-2.0`: Sangat mengurangi repetisi
- **PresencePenalty (-2.0-2.0)**:
  - `-2.0`: Sangat repetitif topik
  - `0.0`: Tidak ada penalty
  - `0.1-1.0`: Kurangi repetisi topik
  - `1.0-2.0`: Sangat mengurangi repetisi topik

#### **Middleware Strategy**
```go
// Urutan middleware yang disarankan:
// 1. Authentication (priority: 1-5)
// 2. Rate limiting (priority: 5-10)  
// 3. Validation (priority: 10-15)
// 4. Business logic (priority: 15-20)
// 5. Caching (priority: 20-25)
// 6. Logging (priority: 25-30)

// Contoh setup yang baik:
cs.AddWithAuth(adminIntents, "admin")           // Priority: 1
cs.AddWithRateLimit(userIntents, 10, time.Minute) // Priority: 5
cs.AddWithCache(searchIntents, 5*time.Minute)   // Priority: 20
cs.AddWithLogging(allIntents, logger)           // Priority: 25
```

#### **Session Management**
- **Gunakan session ID yang unik**: UUID atau kombinasi user ID + timestamp
- **Set TTL yang sesuai**: 1-24 jam tergantung use case
- **Cleanup session**: Implementasi cleanup untuk session yang expired

#### **Error Handling**
```go
func (p ProductSearch) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    // Validasi parameter
    query, ok := params["query"].(string)
    if !ok || query == "" {
        return nil, fmt.Errorf("query parameter is required")
    }
    
    // Business logic dengan error handling
    products, err := searchProducts(query)
    if err != nil {
        return nil, fmt.Errorf("failed to search products: %w", err)
    }
    
    return map[string]interface{}{
        "products": products,
        "total":    len(products),
    }, nil
}
```

#### **Testing Strategy**
```go
func TestProductSearch(t *testing.T) {
    // Setup test
    cs := cs_ai.New("test-key", model.NewDeepSeekChat(), cs_ai.Options{})
    cs.Add(ProductSearch{})
    
    // Test cases
    testCases := []struct {
        name     string
        message  string
        expected string
    }{
        {
            name:     "search pomade",
            message:  "cari pomade",
            expected: "pomade",
        },
        {
            name:     "search shampoo", 
            message:  "tunjukkan shampoo",
            expected: "shampoo",
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            response, err := cs.Exec(context.Background(), "test-session", cs_ai.UserMessage{
                Message: tc.message,
            })
            
            assert.NoError(t, err)
            assert.Contains(t, response.Content, tc.expected)
        })
    }
}
```

### Basic Usage dengan In-Memory Storage (Simple Version)

```go
package main

import (
    "context"
    "time"
    cs_ai "github.com/wirnat/cs-ai"
    "github.com/wirnat/cs-ai/model"
)

func main() {
    // Buat storage configuration
    config := &cs_ai.StorageConfig{
        Type:        cs_ai.StorageTypeInMemory,
        SessionTTL:  1 * time.Hour,
        Timeout:     5 * time.Second,
    }

    // Buat CsAI dengan storage
    cs := cs_ai.New("your-api-key", model.NewDeepSeekChat(), cs_ai.Options{
        StorageConfig: config,
        SessionTTL:   1 * time.Hour,
    })

    // Gunakan seperti biasa - storage dihandle otomatis
    response, err := cs.Exec(context.Background(), "session-123", cs_ai.UserMessage{
        Message: "Hello, how are you?",
    })
}
```

### Redis Storage

```go
// Redis configuration
redisConfig := &cs_ai.StorageConfig{
    Type:          cs_ai.StorageTypeRedis,
    RedisAddress:  "localhost:6379",
    RedisPassword: "your-password",
    RedisDB:       0,
    SessionTTL:    12 * time.Hour,
    Timeout:       5 * time.Second,
}

// Buat CsAI dengan Redis
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: redisConfig,
})
```

### MongoDB Storage (Fully Implemented)

```go
// MongoDB configuration
mongoConfig := &cs_ai.StorageConfig{
    Type:             cs_ai.StorageTypeMongo,
    MongoURI:         "mongodb://localhost:27017",
    MongoDatabase:    "cs_ai",
    MongoCollection:  "sessions",
    SessionTTL:       24 * time.Hour,
    Timeout:          10 * time.Second,
}

// Buat CsAI dengan MongoDB
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: mongoConfig,
})

// Note: MongoDB storage provider memerlukan driver dependency
// go get go.mongodb.org/mongo-driver/mongo
```

### DynamoDB Storage (Stub)

```go
// DynamoDB configuration
dynamoConfig := &cs_ai.StorageConfig{
    Type:         cs_ai.StorageTypeDynamo,
    AWSRegion:    "us-east-1",
    DynamoTable:  "cs_ai_sessions",
    SessionTTL:   24 * time.Hour,
    Timeout:      10 * time.Second,
}

// Buat CsAI dengan DynamoDB
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: dynamoConfig,
})

// Note: DynamoDB storage provider memerlukan AWS SDK dependency
// go get github.com/aws/aws-sdk-go-v2
```

## 🔧 Configuration Options

### StorageConfig Fields

```go
type StorageConfig struct {
    Type StorageType // Storage type (redis, mongo, dynamo, memory)
    
    // Redis specific
    RedisAddress  string // Redis server address
    RedisPassword string // Redis password
    RedisDB       int    // Redis database number
    
    // MongoDB specific
    MongoURI        string // MongoDB connection string
    MongoDatabase   string // Database name
    MongoCollection string // Collection name
    
    // DynamoDB specific
    AWSRegion   string // AWS region
    DynamoTable string // Table name
    
    // Common configuration
    SessionTTL time.Duration // Session expiration time
    MaxRetries int           // Maximum retry attempts
    Timeout    time.Duration // Operation timeout
}
```

### Storage Types

```go
const (
    StorageTypeRedis    StorageType = "redis"
    StorageTypeMongo    StorageType = "mongo"
    StorageTypeDynamo   StorageType = "dynamo"
    StorageTypeInMemory StorageType = "memory"
)
```

## 🔄 Migration Guide

### Dari Redis ke MongoDB

1. **Update Configuration**
```go
// Sebelum (Redis)
config := &cs_ai.StorageConfig{
    Type:          cs_ai.StorageTypeRedis,
    RedisAddress:  "localhost:6379",
    SessionTTL:    12 * time.Hour,
}

// Sesudah (MongoDB)
config := &cs_ai.StorageConfig{
    Type:             cs_ai.StorageTypeMongo,
    MongoURI:         "mongodb://localhost:27017",
    MongoDatabase:    "cs_ai",
    MongoCollection:  "sessions",
    SessionTTL:       24 * time.Hour,
}
```

2. **Tidak Ada Perubahan Kode**
   - Semua method calls tetap sama
   - Storage abstraction handle perbedaannya
   - Session data otomatis migrate

### Dari Redis ke In-Memory

```go
// Untuk development/testing
config := &cs_ai.StorageConfig{
    Type:        cs_ai.StorageTypeInMemory,
    SessionTTL:  1 * time.Hour,
}

cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: config,
})
```

## 📊 Health Monitoring

### Health Checks

```go
// Check storage health
if err := cs.options.StorageProvider.HealthCheck(); err != nil {
    log.Printf("Storage health check failed: %v", err)
} else {
    log.Println("Storage health check passed")
}
```

### Statistics (In-Memory Only)

```go
if inMemory, ok := cs.options.StorageProvider.(*cs_ai.InMemoryStorageProvider); ok {
    stats := inMemory.GetStorageStats()
    log.Printf("Active sessions: %d", stats.ActiveSessions)
    log.Printf("Total sessions: %d", stats.TotalSessions)
    log.Printf("Memory usage: %d bytes", stats.MemoryUsage)
}
```

## 🎯 Use Cases

### E-commerce Customer Service
```go
// Setup untuk e-commerce dengan multiple intents
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    UseTool: true,
})

// Product-related intents
cs.AddWithCache([]cs_ai.Intent{
    ProductSearch{},
    ProductCatalog{},
    StockCheck{},
}, 5*time.Minute)

// Order-related intents dengan authentication
cs.AddWithAuth([]cs_ai.Intent{
    OrderStatus{},
    OrderHistory{},
    CancelOrder{},
}, "customer")

// Support intents dengan rate limiting
cs.AddWithRateLimit([]cs_ai.Intent{
    ContactSupport{},
    ReportIssue{},
}, 5, time.Hour)

systemPrompts := []string{
    "Kamu adalah asisten customer service untuk toko online",
    "Gunakan fungsi yang tersedia untuk membantu pelanggan",
    "Selalu ramah dan informatif",
}
```

### Healthcare Appointment System
```go
// Setup untuk sistem appointment kesehatan
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    UseTool: true,
})

// Public intents
cs.AddWithLogging([]cs_ai.Intent{
    DoctorList{},
    ServiceList{},
    ClinicInfo{},
}, logger)

// Patient intents dengan authentication
cs.AddWithAuth([]cs_ai.Intent{
    BookAppointment{},
    ViewAppointment{},
    RescheduleAppointment{},
    CancelAppointment{},
}, "patient")

// Admin intents dengan strict authentication
cs.AddWithAuth([]cs_ai.Intent{
    ViewAllAppointments{},
    ManageSchedule{},
    GenerateReport{},
}, "admin")

systemPrompts := []string{
    "Kamu adalah asisten untuk klinik kesehatan",
    "Bantu pasien dengan appointment dan informasi medis",
    "Selalu profesional dan menjaga privasi",
}
```

### Financial Services Bot
```go
// Setup untuk layanan finansial dengan security tinggi
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    UseTool: true,
})

// Public information intents
cs.AddWithCache([]cs_ai.Intent{
    InterestRate{},
    ProductInfo{},
    BranchLocator{},
}, 1*time.Hour)

// Customer intents dengan strict authentication
cs.AddWithAuth([]cs_ai.Intent{
    AccountBalance{},
    TransactionHistory{},
    TransferMoney{},
    BillPayment{},
}, "customer")

// Rate limiting untuk sensitive operations
cs.AddWithRateLimit([]cs_ai.Intent{
    TransferMoney{},
    BillPayment{},
}, 3, time.Hour)

systemPrompts := []string{
    "Kamu adalah asisten untuk bank digital",
    "Selalu verifikasi identitas sebelum transaksi",
    "Jaga keamanan dan privasi data pelanggan",
}
```

### Development & Testing
```go
// Gunakan in-memory storage untuk development
config := &cs_ai.StorageConfig{
    Type:        cs_ai.StorageTypeInMemory,
    SessionTTL:  1 * time.Hour,
}
```

### Production
```go
// Gunakan Redis untuk production
config := &cs_ai.StorageConfig{
    Type:          cs_ai.StorageTypeRedis,
    RedisAddress:  "redis.production.com:6379",
    SessionTTL:    24 * time.Hour,
}
```

### Analytics
```go
// Gunakan MongoDB untuk analytics
config := &cs_ai.StorageConfig{
    Type:             cs_ai.StorageTypeMongo,
    MongoURI:         "mongodb://analytics.db.com:27017",
    SessionTTL:       7 * 24 * time.Hour, // 7 hari
}
```

### Cloud (AWS)
```go
// Gunakan DynamoDB untuk AWS ecosystem
config := &cs_ai.StorageConfig{
    Type:         cs_ai.StorageTypeDynamo,
    AWSRegion:    "ap-southeast-1",
    DynamoTable:  "cs_ai_production",
    SessionTTL:   24 * time.Hour,
}
```

## 🧪 Testing

```bash
# Test semua storage providers
go test -v

# Test khusus storage
go test -v -run "TestStorage|TestInMemory"

# Test dengan coverage
go test -v -cover
```

### Testing Intent System

```go
func TestIntentSystem(t *testing.T) {
    // Setup test environment
    cs := cs_ai.New("test-key", model.NewDeepSeekChat(), cs_ai.Options{
        UseTool: true,
    })
    
    // Add test intents
    cs.Add(ProductSearch{})
    cs.Add(BookingService{})
    
    // Test cases
    testCases := []struct {
        name        string
        message     string
        expectIntent string
        expectError bool
    }{
        {
            name:        "product search intent",
            message:     "cari pomade yang bagus",
            expectIntent: "product-search",
            expectError: false,
        },
        {
            name:        "booking intent",
            message:     "saya ingin booking potong rambut",
            expectIntent: "booking-service",
            expectError: false,
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            response, err := cs.Exec(context.Background(), "test-session", cs_ai.UserMessage{
                Message: tc.message,
            })
            
            if tc.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.NotEmpty(t, response.Content)
            }
        })
    }
}
```

## ⚡ Performance Tips

### Intent Optimization
```go
// ✅ Efficient intent dengan caching
cs.AddWithCache([]cs_ai.Intent{
    ProductSearch{},
    ServiceList{},
}, 5*time.Minute)

// ✅ Rate limiting untuk expensive operations
cs.AddWithRateLimit([]cs_ai.Intent{
    GenerateReport{},
    BulkOperation{},
}, 2, time.Hour)

// ✅ Async processing untuk heavy operations
func (r GenerateReport) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    // Start async job
    jobID := startAsyncReportGeneration(params)
    
    return map[string]interface{}{
        "job_id": jobID,
        "status": "processing",
        "message": "Report generation started",
    }, nil
}
```

### Session Management
```go
// ✅ Efficient session cleanup
func cleanupExpiredSessions() {
    // Implementasi cleanup untuk session yang expired
    // Jalankan sebagai background job
}

// ✅ Session pooling untuk high traffic
func getSessionPool() *sync.Pool {
    return &sync.Pool{
        New: func() interface{} {
            return &SessionData{}
        },
    }
}
```

### Memory Management
```go
// ✅ Limit session data size
type SessionData struct {
    Messages []Message `json:"messages"`
    Metadata map[string]interface{} `json:"metadata"`
}

func (s *SessionData) TrimMessages(maxMessages int) {
    if len(s.Messages) > maxMessages {
        s.Messages = s.Messages[len(s.Messages)-maxMessages:]
    }
}
```

## 🍃 MongoDB-Specific Features

### Automatic Indexing

MongoDB storage provider secara otomatis membuat indexes untuk performa optimal:

```go
// Indexes yang dibuat otomatis:
// 1. session_id (unique)
// 2. expires_at (TTL index)
// 3. user_id + timestamp (compound index)
```

### Learning Data Management

```go
// Simpan learning data untuk AI improvement
learningData := cs_ai.LearningData{
    Query:     "user query",
    Response:  "ai response",
    Tools:     []string{"tool1", "tool2"},
    Context:   map[string]interface{}{"key": "value"},
    Feedback:  1, // 1 = positive, -1 = negative, 0 = neutral
}

// Data akan disimpan di collection "learning_data"
err := cs.options.StorageProvider.SaveLearningData(ctx, learningData)
```

### Security Logging

```go
// Security logs otomatis disimpan untuk monitoring
securityLog := cs_ai.SecurityLog{
    SessionID:   "session-123",
    UserID:      "user-456",
    MessageHash: "hash-of-message",
    Timestamp:   time.Now(),
    SpamScore:   0.2,
    Allowed:     true,
    Error:       "",
}

// Data akan disimpan di collection "security_logs"
err := cs.options.StorageProvider.SaveSecurityLog(ctx, securityLog)
```

### Storage Statistics

```go
// Get MongoDB storage statistics
if mongoStorage, ok := cs.options.StorageProvider.(*cs_ai.MongoStorageProvider); ok {
    stats := mongoStorage.GetStorageStats()
    fmt.Printf("Total sessions: %v\n", stats["total_sessions"])
    fmt.Printf("Total learning data: %v\n", stats["total_learning_data"])
    fmt.Printf("Total security logs: %v\n", stats["total_security_logs"])
    fmt.Printf("Data size (MB): %v\n", stats["data_size_mb"])
    fmt.Printf("Storage size (MB): %v\n", stats["storage_size_mb"])
}
```

### Health Monitoring

```go
// Check MongoDB connection health
err := cs.options.StorageProvider.HealthCheck()
if err != nil {
    log.Printf("MongoDB health check failed: %v", err)
}
```

### Production Best Practices

```go
// Production MongoDB configuration
mongoConfig := cs_ai.StorageConfig{
    Type:            cs_ai.StorageTypeMongo,
    MongoURI:        "mongodb://user:pass@cluster1,cluster2,cluster3/db?replicaSet=rs0",
    MongoDatabase:   "production_cs_ai",
    MongoCollection: "sessions",
    SessionTTL:      24 * time.Hour,
    Timeout:         10 * time.Second,
    MaxRetries:      3,
}

// Enable security features
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: &mongoConfig,
    UseTool:       true,
    SessionTTL:    24 * time.Hour,
    SecurityOptions: &cs_ai.SecurityOptions{
        MaxRequestsPerMinute:  60,
        MaxRequestsPerHour:    1000,
        MaxRequestsPerDay:     10000,
        SpamThreshold:         0.7,
        EnableSecurityLogging: true,
        UserIDField:           "ParticipantName",
    },
})
```

## 📊 Monitoring & Observability

### Health Checks
```go
// Health check endpoint
func healthCheck(cs *cs_ai.CsAI) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Check storage health
        if err := cs.options.StorageProvider.HealthCheck(); err != nil {
            http.Error(w, "Storage unhealthy", http.StatusServiceUnavailable)
            return
        }
        
        // Check intent system
        if len(cs.intents) == 0 {
            http.Error(w, "No intents registered", http.StatusServiceUnavailable)
            return
        }
        
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    }
}
```

### Metrics Collection
```go
// Custom metrics middleware
func metricsMiddleware() cs_ai.Middleware {
    return &MetricsMiddleware{
        requestCount:    prometheus.NewCounterVec(...),
        requestDuration: prometheus.NewHistogramVec(...),
        errorCount:      prometheus.NewCounterVec(...),
    }
}

type MetricsMiddleware struct {
    requestCount    *prometheus.CounterVec
    requestDuration *prometheus.HistogramVec
    errorCount      *prometheus.CounterVec
}

func (m *MetricsMiddleware) Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
    start := time.Now()
    
    result, err := next(ctx, mctx)
    
    duration := time.Since(start)
    
    // Record metrics
    m.requestCount.WithLabelValues(mctx.IntentCode).Inc()
    m.requestDuration.WithLabelValues(mctx.IntentCode).Observe(duration.Seconds())
    
    if err != nil {
        m.errorCount.WithLabelValues(mctx.IntentCode, err.Error()).Inc()
    }
    
    return result, err
}
```

### Logging Strategy
```go
// Structured logging
func setupLogging() *logrus.Logger {
    logger := logrus.New()
    logger.SetFormatter(&logrus.JSONFormatter{})
    logger.SetLevel(logrus.InfoLevel)
    
    return logger
}

// Log middleware
func loggingMiddleware(logger *logrus.Logger) cs_ai.Middleware {
    return &LoggingMiddleware{logger: logger}
}

func (l *LoggingMiddleware) Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
    start := time.Now()
    
    logger := l.logger.WithFields(logrus.Fields{
        "session_id":   mctx.SessionID,
        "intent_code":  mctx.IntentCode,
        "user_message": mctx.UserMessage.Message,
    })
    
    logger.Info("Intent execution started")
    
    result, err := next(ctx, mctx)
    
    duration := time.Since(start)
    logger = logger.WithField("duration_ms", duration.Milliseconds())
    
    if err != nil {
        logger.WithError(err).Error("Intent execution failed")
    } else {
        logger.Info("Intent execution completed")
    }
    
    return result, err
}
```

## 📚 Examples

### File Examples yang Tersedia

Lihat direktori `example/` untuk contoh lengkap:

- **`example/main.go`** - Complete working example dengan Echo server
- **`example/intents/`** - Koleksi intent examples:
  - `product_catalog.go` - Intent untuk menampilkan katalog produk
  - `list_service.go` - Intent untuk menampilkan layanan yang tersedia
  - `booking_capster.go` - Intent untuk booking layanan dengan parameter
  - `brand_info.go` - Intent untuk informasi brand
  - `report.go` - Intent untuk generate report
  - `availability_capster.go` - Intent untuk cek ketersediaan

### Storage Examples

Lihat file `storage_examples.go` untuk contoh lengkap penggunaan:

- Redis storage setup
- MongoDB storage setup (fully implemented)
- In-memory storage setup
- Custom storage provider
- Storage migration examples
- Health checking
- Statistics

### Middleware Examples

Lihat file `middleware_examples.go` untuk contoh middleware:

- Logging middleware
- Authentication middleware
- Rate limiting middleware
- Caching middleware
- Custom middleware implementation
- Real-world usage scenarios

### Quick Start dengan Examples

```bash
# Clone repository
git clone https://github.com/wirnat/cs-ai.git
cd cs-ai

# Run example
go run example/main.go

# Test dengan curl
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Saya ingin cari pomade", "participant_name": "John"}'
```

## 🚧 Roadmap

### Phase 1 (Current) ✅
- Redis storage provider
- In-memory storage provider
- Storage interface abstraction
- Backward compatibility
- Intent system dengan middleware support
- Built-in middleware (auth, rate limiting, caching, logging)

### Phase 2 (Next) 🔄
- MongoDB storage provider (full implementation)
- DynamoDB storage provider (full implementation)
- Connection pooling improvements
- Performance optimizations
- Advanced middleware (metrics, tracing)
- Intent versioning dan migration

### Phase 3 (Future) 📋
- Storage encryption
- Backup and restore
- Multi-region support
- Advanced analytics
- Streaming response support
- Intent marketplace
- Visual intent builder

## 📋 Changelog

### v1.2.0 (Current)
- ✨ **NEW**: Intent system dengan middleware support
- ✨ **NEW**: Built-in middleware (authentication, rate limiting, caching, logging)
- ✨ **NEW**: Advanced storage configuration
- ✨ **NEW**: Comprehensive examples dan documentation
- 🔧 **IMPROVED**: Better error handling dan logging
- 🔧 **IMPROVED**: Performance optimizations
- 🐛 **FIXED**: Session management issues
- 🐛 **FIXED**: Memory leaks dalam in-memory storage

### v1.1.0
- ✨ **NEW**: Multiple storage backends (Redis, MongoDB, DynamoDB, In-Memory)
- ✨ **NEW**: Storage interface abstraction
- ✨ **NEW**: Health monitoring
- 🔧 **IMPROVED**: Backward compatibility
- 🔧 **IMPROVED**: Configuration management
- 🐛 **FIXED**: Connection pooling issues

### v1.0.0
- 🎉 **INITIAL**: First release
- ✨ **NEW**: Basic CS-AI functionality
- ✨ **NEW**: Redis storage support
- ✨ **NEW**: Session management
- ✨ **NEW**: Tool calling support

## 🔍 Troubleshooting

### Common Issues

1. **Storage Provider Not Found**
   - Check `StorageType` constant spelling
   - Ensure required dependencies are installed
   - Verify configuration parameters

2. **Connection Timeouts**
   - Increase `Timeout` value
   - Check network connectivity
   - Verify server addresses

3. **Memory Issues (In-Memory)**
   - Monitor session count
   - Implement cleanup strategies
   - Consider switching to persistent storage

### Intent System Issues

4. **Intent Not Triggered**
   - Check `UseTool: true` dalam Options
   - Verify intent description yang jelas dan spesifik
   - Pastikan system prompt menginstruksikan AI untuk menggunakan fungsi
   - Test dengan message yang lebih eksplisit

5. **Parameter Parsing Error**
   - Validasi parameter schema di `Param()` method
   - Pastikan parameter yang dikirim sesuai dengan schema
   - Gunakan type assertion yang aman di handler

6. **Middleware Not Executing**
   - Check priority order (lower number = earlier execution)
   - Verify `AppliesTo()` method returns correct intent codes
   - Pastikan middleware terdaftar sebelum intent digunakan

7. **AI Tidak Menggunakan Intent**
   ```go
   // ❌ System prompt yang kurang jelas
   systemPrompts := []string{
       "Kamu adalah asisten AI",
   }
   
   // ✅ System prompt yang jelas
   systemPrompts := []string{
       "Kamu adalah asisten AI untuk salon Hairkatz",
       "Gunakan fungsi product-search untuk mencari produk",
       "Gunakan fungsi booking-service untuk booking layanan",
       "Selalu gunakan fungsi yang tersedia untuk membantu pelanggan",
   }
   ```

8. **Intent Handler Error**
   ```go
   // ❌ Error handling yang buruk
   func (p ProductSearch) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
       query := params["query"].(string) // Panic jika nil
       return searchProducts(query), nil
   }
   
   // ✅ Error handling yang baik
   func (p ProductSearch) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
       query, ok := params["query"].(string)
       if !ok || query == "" {
           return nil, fmt.Errorf("query parameter is required")
       }
       
       products, err := searchProducts(query)
       if err != nil {
           return nil, fmt.Errorf("failed to search products: %w", err)
       }
       
       return map[string]interface{}{
           "products": products,
           "total":    len(products),
       }, nil
   }
   ```

### Debug Mode

```go
// Enable debug logging
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: config,
    LogChatFile:   true, // Log semua operations
})
```

## 🔒 Security Best Practices

### Input Validation
```go
// ✅ Validasi input di intent handler
func (p ProductSearch) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    // Sanitize input
    query, ok := params["query"].(string)
    if !ok {
        return nil, fmt.Errorf("invalid query parameter")
    }
    
    // Validate length
    if len(query) > 100 {
        return nil, fmt.Errorf("query too long")
    }
    
    // Sanitize untuk mencegah injection
    query = strings.TrimSpace(query)
    query = html.EscapeString(query)
    
    return searchProducts(query), nil
}
```

### Authentication & Authorization
```go
// ✅ Strong authentication middleware
func authMiddleware(requiredRole string) cs_ai.Middleware {
    return &AuthenticationMiddleware{
        requiredRole: requiredRole,
        tokenValidator: validateJWT,
    }
}

func (a *AuthenticationMiddleware) Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
    // Extract token dari metadata
    token, ok := mctx.Metadata["auth_token"].(string)
    if !ok {
        return nil, fmt.Errorf("authentication required")
    }
    
    // Validate token
    claims, err := a.tokenValidator(token)
    if err != nil {
        return nil, fmt.Errorf("invalid token: %w", err)
    }
    
    // Check role
    if claims.Role != a.requiredRole {
        return nil, fmt.Errorf("insufficient permissions")
    }
    
    // Add user info ke metadata
    mctx.Metadata["user_id"] = claims.UserID
    mctx.Metadata["user_role"] = claims.Role
    
    return next(ctx, mctx)
}
```

### Rate Limiting
```go
// ✅ Advanced rate limiting
func advancedRateLimitMiddleware(maxRequests int, timeWindow time.Duration) cs_ai.Middleware {
    return &AdvancedRateLimitMiddleware{
        limiter: rate.NewLimiter(rate.Every(timeWindow/time.Duration(maxRequests)), maxRequests),
        userLimits: make(map[string]*rate.Limiter),
    }
}

func (r *AdvancedRateLimitMiddleware) Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
    userID := mctx.Metadata["user_id"].(string)
    
    // Get user-specific limiter
    limiter := r.getUserLimiter(userID)
    
    if !limiter.Allow() {
        return nil, fmt.Errorf("rate limit exceeded for user %s", userID)
    }
    
    return next(ctx, mctx)
}
```

### Data Encryption
```go
// ✅ Encrypt sensitive data
func encryptSensitiveData(data map[string]interface{}) (map[string]interface{}, error) {
    encrypted := make(map[string]interface{})
    
    for key, value := range data {
        if isSensitiveField(key) {
            encryptedValue, err := encrypt(value.(string))
            if err != nil {
                return nil, err
            }
            encrypted[key] = encryptedValue
        } else {
            encrypted[key] = value
        }
    }
    
    return encrypted, nil
}

func isSensitiveField(field string) bool {
    sensitiveFields := []string{"password", "token", "ssn", "credit_card"}
    for _, sensitive := range sensitiveFields {
        if strings.Contains(strings.ToLower(field), sensitive) {
            return true
        }
    }
    return false
}
```

### Audit Logging
```go
// ✅ Audit logging middleware
func auditMiddleware() cs_ai.Middleware {
    return &AuditMiddleware{
        auditLogger: setupAuditLogger(),
    }
}

func (a *AuditMiddleware) Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
    start := time.Now()
    
    // Log request
    a.auditLogger.Info("Intent request", map[string]interface{}{
        "session_id":   mctx.SessionID,
        "intent_code":  mctx.IntentCode,
        "user_id":      mctx.Metadata["user_id"],
        "user_role":    mctx.Metadata["user_role"],
        "parameters":   sanitizeForLogging(mctx.Parameters),
        "timestamp":    start,
    })
    
    result, err := next(ctx, mctx)
    
    // Log response
    a.auditLogger.Info("Intent response", map[string]interface{}{
        "session_id":   mctx.SessionID,
        "intent_code":  mctx.IntentCode,
        "success":      err == nil,
        "duration_ms":  time.Since(start).Milliseconds(),
        "timestamp":    time.Now(),
    })
    
    return result, err
}
```

### Secure Configuration
```go
// ✅ Secure configuration management
type SecureConfig struct {
    APIKey        string `json:"api_key" validate:"required"`
    DatabaseURL   string `json:"database_url" validate:"required"`
    RedisPassword string `json:"redis_password"`
    JWTSecret     string `json:"jwt_secret" validate:"required,min=32"`
}

func loadSecureConfig() (*SecureConfig, error) {
    // Load dari environment variables
    config := &SecureConfig{
        APIKey:        os.Getenv("CS_AI_API_KEY"),
        DatabaseURL:   os.Getenv("DATABASE_URL"),
        RedisPassword: os.Getenv("REDIS_PASSWORD"),
        JWTSecret:     os.Getenv("JWT_SECRET"),
    }
    
    // Validate configuration
    if err := validate.Struct(config); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }
    
    return config, nil
}
```

## 📖 API Reference

### Core Methods

#### **Constructor**
```go
func New(apiKey string, model Model, options Options) *CsAI
```

#### **Execution**
```go
func (c *CsAI) Exec(ctx context.Context, sessionID string, userMessage UserMessage, systemPrompts ...string) (Message, error)
```

#### **Intent Management**
```go
// Add single intent
func (c *CsAI) Add(intent Intent)

// Add multiple intents
func (c *CsAI) Adds(intents []Intent, middleware Middleware)

// Add intents with middleware combinations
func (c *CsAI) AddsWithMiddleware(intents []Intent, middlewares []Middleware)
func (c *CsAI) AddsWithFunc(intents []Intent, middlewareName string, priority int, handler func(...) (interface{}, error))
```

#### **Built-in Intent Helpers**
```go
// Add with authentication
func (c *CsAI) AddWithAuth(intents []Intent, requiredRole string)

// Add with rate limiting
func (c *CsAI) AddWithRateLimit(intents []Intent, maxRequests int, timeWindow time.Duration)

// Add with caching
func (c *CsAI) AddWithCache(intents []Intent, ttl time.Duration)

// Add with logging
func (c *CsAI) AddWithLogging(intents []Intent, logger *log.Logger)
```

#### **Middleware Management**
```go
// Add middleware
func (c *CsAI) AddMiddleware(middleware Middleware)

// Add function-based middleware
func (c *CsAI) AddMiddlewareFunc(name string, appliesTo []string, priority int, handler func(...) (interface{}, error))

// Add global middleware
func (c *CsAI) AddGlobalMiddleware(name string, priority int, handler func(...) (interface{}, error))

// Add middleware to specific intents
func (c *CsAI) AddMiddlewareToIntents(intentCodes []string, middleware Middleware)
func (c *CsAI) AddMiddlewareFuncToIntents(intentCodes []string, middlewareName string, priority int, handler func(...) (interface{}, error))
```

#### **Session Management**
```go
// Generate report for session
func (c *CsAI) Report(sessionID string) error

// Clear session
func (c *CsAI) ClearSession(sessionID string) error
```

### Data Structures

#### **Intent Interface**
```go
type Intent interface {
    Code() string
    Handle(ctx context.Context, req map[string]interface{}) (interface{}, error)
    Description() []string
    Param() interface{}
}
```

#### **Middleware Interface**
```go
type Middleware interface {
    Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error)
    Name() string
    AppliesTo() []string
    Priority() int
}
```

#### **MiddlewareContext**
```go
type MiddlewareContext struct {
    SessionID       string
    IntentCode      string
    UserMessage     UserMessage
    Parameters      map[string]interface{}
    StartTime       time.Time
    Metadata        map[string]interface{}
    PreviousResults []interface{}
}
```

#### **UserMessage**
```go
type UserMessage struct {
    Message         string `json:"message"`
    ParticipantName string `json:"participant_name"`
}
```

#### **Message**
```go
type Message struct {
    Content    string `json:"content"`
    Name       string `json:"name"`
    Role       string `json:"role"`
    ToolCallID string `json:"tool_call_id,omitempty"`
}
```

#### **StorageConfig**
```go
type StorageConfig struct {
    Type StorageType
    
    // Redis specific
    RedisAddress  string
    RedisPassword string
    RedisDB       int
    
    // MongoDB specific
    MongoURI        string
    MongoDatabase   string
    MongoCollection string
    
    // DynamoDB specific
    AWSRegion   string
    DynamoTable string
    
    // Common configuration
    SessionTTL time.Duration
    MaxRetries int
    Timeout    time.Duration
}
```

#### **Options**
```go
type Options struct {
    StorageConfig     *StorageConfig
    SessionTTL        time.Duration
    UseTool           bool
    LogChatFile       bool
    Temperature       float32 // Kontrol kreativitas output LLM (0.0-2.0, default 0.2)
    TopP              float32 // Kontrol probabilitas sampling LLM (0.0-1.0, default 0.7)
    FrequencyPenalty  float32 // Kontrol repetisi token (-2.0-2.0, default 0.0)
    PresencePenalty   float32 // Kontrol repetisi topik (-2.0-2.0, default -1.5)
    Redis             *redis.Client // Legacy support
}
```

## 📖 Documentation

- `STORAGE_GUIDE.md` - Panduan lengkap storage system
- `STORAGE_README.md` - Dokumentasi teknis storage
- `storage_examples.go` - Contoh penggunaan praktis
- `middleware_examples.go` - Contoh middleware implementation
- `example/` - Complete working examples

## ❓ FAQ (Frequently Asked Questions)

### **Q: Bagaimana cara memulai dengan CS-AI?**
A: Mulai dengan contoh lengkap di section "Complete Example" di atas. Setup storage, buat intent sederhana, dan test dengan system prompt yang jelas.

### **Q: Apakah CS-AI mendukung model AI selain DeepSeek?**
A: Ya, CS-AI dirancang untuk mendukung berbagai model AI. Implementasi interface `Model` untuk model yang berbeda.

### **Q: Bagaimana cara menangani error di intent handler?**
A: Selalu return error yang informatif dari intent handler. Gunakan error wrapping untuk memberikan context yang jelas.

### **Q: Apakah session data aman?**
A: Session data disimpan sesuai dengan storage provider yang dipilih. Untuk production, gunakan Redis dengan authentication atau database yang aman.

### **Q: Bagaimana cara mengoptimalkan performance?**
A: Gunakan caching middleware, rate limiting, dan pilih storage yang sesuai. Lihat section "Performance Tips" untuk detail.

### **Q: Apakah CS-AI thread-safe?**
A: Ya, CS-AI dirancang untuk thread-safe. Setiap session memiliki context yang terpisah.

### **Q: Bagaimana cara menambahkan custom middleware?**
A: Implementasi interface `Middleware` atau gunakan `AddMiddlewareFunc` untuk function-based middleware.

### **Q: Apakah ada limit untuk jumlah intent?**
A: Tidak ada limit hard-coded, tapi pertimbangkan performance dan memory usage untuk jumlah intent yang sangat besar.

### **Q: Bagaimana cara migrate dari Redis ke MongoDB?**
A: Ganti `StorageType` dan konfigurasi yang sesuai. Session data akan otomatis migrate.

### **Q: Apakah CS-AI mendukung streaming response?**
A: Saat ini belum, tapi bisa diimplementasikan dengan custom middleware atau modifikasi response handling.

## 🆘 Common Issues & Solutions

### **Issue: Intent tidak ter-trigger**
```go
// ❌ Problem: UseTool tidak diaktifkan
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{})

// ✅ Solution: Aktifkan UseTool
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    UseTool: true, // WAJIB untuk intent system
})
```

### **Issue: Parameter parsing error**
```go
// ❌ Problem: Type assertion yang tidak aman
func (p ProductSearch) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    query := params["query"].(string) // Panic jika nil
    return searchProducts(query), nil
}

// ✅ Solution: Type assertion yang aman
func (p ProductSearch) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    query, ok := params["query"].(string)
    if !ok || query == "" {
        return nil, fmt.Errorf("query parameter is required")
    }
    return searchProducts(query), nil
}
```

### **Issue: Middleware tidak dieksekusi**
```go
// ❌ Problem: Priority yang salah
cs.AddMiddlewareFunc("validator", []string{}, 100, handler) // Priority terlalu tinggi

// ✅ Solution: Priority yang tepat
cs.AddMiddlewareFunc("validator", []string{}, 10, handler) // Priority rendah = eksekusi awal
```

### **Issue: Session tidak persist**
```go
// ❌ Problem: Storage tidak dikonfigurasi
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{})

// ✅ Solution: Konfigurasi storage
config := &cs_ai.StorageConfig{
    Type: cs_ai.StorageTypeRedis,
    RedisAddress: "localhost:6379",
}
cs := cs_ai.New("api-key", model.NewDeepSeekChat(), cs_ai.Options{
    StorageConfig: config,
})
```

### **Issue: AI tidak menggunakan intent**
```go
// ❌ Problem: System prompt tidak jelas
systemPrompts := []string{
    "Kamu adalah asisten AI",
}

// ✅ Solution: System prompt yang spesifik
systemPrompts := []string{
    "Kamu adalah asisten AI untuk salon Hairkatz",
    "Gunakan fungsi product-search untuk mencari produk",
    "Gunakan fungsi booking-service untuk booking layanan",
    "Selalu gunakan fungsi yang tersedia untuk membantu pelanggan",
}
```

## 🤝 Contributing

1. Fork repository
2. Create feature branch
3. Implement changes
4. Add tests
5. Submit pull request

### Development Setup
```bash
# Clone repository
git clone https://github.com/wirnat/cs-ai.git
cd cs-ai

# Install dependencies
go mod download

# Run tests
go test -v

# Run example
go run example/main.go
```

### Code Style
- Follow Go standard formatting (`gofmt`)
- Add tests for new features
- Update documentation
- Use meaningful commit messages

## 🚀 Deployment & Production

### Docker Deployment
```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main example/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/example/assets ./assets

EXPOSE 8080
CMD ["./main"]
```

### Docker Compose
```yaml
# docker-compose.yml
version: '3.8'

services:
  cs-ai-app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - CS_AI_API_KEY=${CS_AI_API_KEY}
      - REDIS_URL=redis://redis:6379
    depends_on:
      - redis
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    restart: unless-stopped

volumes:
  redis_data:
```

### Kubernetes Deployment
```yaml
# k8s-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cs-ai-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: cs-ai-app
  template:
    metadata:
      labels:
        app: cs-ai-app
    spec:
      containers:
      - name: cs-ai-app
        image: your-registry/cs-ai-app:latest
        ports:
        - containerPort: 8080
        env:
        - name: CS_AI_API_KEY
          valueFrom:
            secretKeyRef:
              name: cs-ai-secrets
              key: api-key
        - name: REDIS_URL
          value: "redis://redis-service:6379"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: cs-ai-service
spec:
  selector:
    app: cs-ai-app
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

### Environment Configuration
```bash
# .env.production
CS_AI_API_KEY=your-production-api-key
REDIS_URL=redis://redis.production.com:6379
REDIS_PASSWORD=your-redis-password
JWT_SECRET=your-jwt-secret-key
LOG_LEVEL=info
ENVIRONMENT=production
```

### Production Checklist

#### **Security**
- [ ] API keys stored in environment variables
- [ ] JWT secrets are strong and rotated regularly
- [ ] Input validation implemented
- [ ] Rate limiting configured
- [ ] HTTPS enabled
- [ ] CORS configured properly

#### **Performance**
- [ ] Redis connection pooling configured
- [ ] Session TTL optimized
- [ ] Caching strategy implemented
- [ ] Database indexes created
- [ ] Monitoring and alerting setup

#### **Reliability**
- [ ] Health checks implemented
- [ ] Graceful shutdown handling
- [ ] Error handling and logging
- [ ] Backup strategy for sessions
- [ ] Load balancing configured

#### **Monitoring**
- [ ] Application metrics exposed
- [ ] Log aggregation setup
- [ ] Error tracking configured
- [ ] Performance monitoring
- [ ] Uptime monitoring

### Scaling Considerations

#### **Horizontal Scaling**
```go
// Session affinity untuk horizontal scaling
func getSessionAffinity(sessionID string) string {
    hash := sha256.Sum256([]byte(sessionID))
    return fmt.Sprintf("node-%d", hash[0]%3) // 3 nodes
}
```

#### **Database Scaling**
```go
// Read replicas untuk database scaling
type DatabaseConfig struct {
    MasterURL string
    ReadReplicas []string
}

func (db *Database) getConnection(readOnly bool) *sql.DB {
    if readOnly && len(db.ReadReplicas) > 0 {
        // Use read replica
        replica := db.ReadReplicas[rand.Intn(len(db.ReadReplicas))]
        return connect(replica)
    }
    return connect(db.MasterURL)
}
```

#### **Cache Scaling**
```go
// Redis cluster support
func setupRedisCluster() *redis.ClusterClient {
    return redis.NewClusterClient(&redis.ClusterOptions{
        Addrs: []string{
            "redis-node-1:6379",
            "redis-node-2:6379",
            "redis-node-3:6379",
        },
        Password: os.Getenv("REDIS_PASSWORD"),
        PoolSize: 100,
    })
}
```

## 🌟 Community & Support

### Getting Help
- 📖 **Documentation**: Baca dokumentasi lengkap di repository ini
- 🐛 **Issues**: Laporkan bug atau request fitur di [GitHub Issues](https://github.com/wirnat/cs-ai/issues)
- 💬 **Discussions**: Diskusi umum di [GitHub Discussions](https://github.com/wirnat/cs-ai/discussions)
- 📧 **Email**: Kontak langsung untuk support enterprise

### Community Resources
- 🎥 **Tutorials**: Video tutorial dan walkthrough
- 📝 **Blog Posts**: Artikel dan best practices
- 🏆 **Showcase**: Contoh implementasi dari komunitas
- 🤝 **Contributors**: Daftar kontributor dan maintainer

### Enterprise Support
Untuk kebutuhan enterprise dengan SLA dan support khusus:
- 📧 Email: enterprise@aksaratech.cloud
- 💼 Custom implementation
- 🏢 On-premise deployment
- 🔒 Security audit dan compliance

## 📊 Statistics

![GitHub stars](https://img.shields.io/github/stars/wirnat/cs-ai?style=social)
![GitHub forks](https://img.shields.io/github/forks/wirnat/cs-ai?style=social)
![GitHub issues](https://img.shields.io/github/issues/wirnat/cs-ai)
![GitHub pull requests](https://img.shields.io/github/issues-pr/wirnat/cs-ai)
![GitHub license](https://img.shields.io/github/license/wirnat/cs-ai)

## 🏆 Acknowledgments

Terima kasih kepada:
- **Contributors** yang telah berkontribusi pada project ini
- **Community** yang memberikan feedback dan suggestions
- **Open Source** libraries yang digunakan dalam project ini
- **AksaraTech** untuk support dan infrastructure

## 📄 License

MIT License - lihat file LICENSE untuk detail.

---

**Note**: Storage system ini dirancang untuk backward compatible. Kode lama yang menggunakan Redis akan tetap berfungsi tanpa modifikasi.

**Made with ❤️ by [AksaraTech](https://aksaratech.cloud)**
