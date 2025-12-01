# lib-common

Shared Go library for Niaga Platform microservices. Provides common utilities for database connections, logging, authentication, middleware, and more.

## Installation

```bash
go get github.com/niaga-platform/lib-common
```

## Packages

### 📦 database

PostgreSQL and Redis connection helpers with health checks.

```go
import "github.com/niaga-platform/lib-common/database"

// PostgreSQL
dbConfig := database.PostgresConfig{
    Host:     "localhost",
    Port:     "5432",
    User:     "niaga",
    Password: "password",
    Database: "niaga",
    SSLMode:  "disable",
}
db, err := database.Connect(dbConfig, logger)

// Redis
redisConfig := database.RedisConfig{
    Host: "localhost",
    Port: "6379",
    DB:   0,
}
client, err := database.ConnectRedis(redisConfig, logger)
```

### 📝 logger

Structured logging with Zap.

```go
import "github.com/niaga-platform/lib-common/logger"

logger, err := logger.NewLogger("development") // or "production"
logger.Info("Service started", zap.String("port", "8001"))
```

### 📤 response

Standardized HTTP response helpers for Gin.

```go
import "github.com/niaga-platform/lib-common/response"

// Success response
response.OK(c, "User created successfully", user)

// Error responses
response.BadRequest(c, "Invalid input", validationErrors)
response.Unauthorized(c, "Invalid credentials")
response.NotFound(c, "User not found")
response.InternalServerError(c, "Something went wrong")

// With pagination
meta := &response.Meta{
    Page:       1,
    Limit:      20,
    TotalPages: 5,
    TotalCount: 100,
}
response.SuccessWithMeta(c, 200, "Products retrieved", products, meta)
```

### 🔐 auth

JWT token generation and validation.

```go
import (
    "github.com/niaga-platform/lib-common/auth"
    "time"
)

jwtManager := auth.NewJWTManager(
    "your-secret-key",
    15*time.Minute,  // access token TTL
    168*time.Hour,   // refresh token TTL
)

// Generate tokens
tokens, err := jwtManager.GenerateTokenPair(userID, email, role)

// Validate token
claims, err := jwtManager.ValidateToken(tokenString)
```

### 🛡️ middleware

Reusable Gin middleware.

```go
import "github.com/niaga-platform/lib-common/middleware"

router := gin.Default()

// Logger middleware
router.Use(middleware.LoggerMiddleware(logger))

// Recovery middleware
router.Use(middleware.RecoveryMiddleware(logger))

// CORS middleware
router.Use(middleware.CORSMiddleware())

// Protected routes
protected := router.Group("/api")
protected.Use(middleware.AuthMiddleware(jwtManager))
{
    protected.GET("/profile", getProfile)
}

// Admin-only routes
admin := protected.Group("/admin")
admin.Use(middleware.RoleMiddleware("admin", "super_admin"))
{
    admin.GET("/users", listUsers)
}
```

### ⚙️ config

Configuration loader with Viper.

```go
import "github.com/niaga-platform/lib-common/config"

// Load from file and environment variables
cfg, err := config.LoadConfig("config.yaml")

// Access config values
dbHost := cfg.Database.Host
jwtSecret := cfg.JWT.Secret
```

### ✅ validator

Custom validation helpers.

```go
import "github.com/niaga-platform/lib-common/validator"

v := validator.NewValidator()

type CreateUserRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
    Phone    string `json:"phone" validate:"required,phone"`
}

var req CreateUserRequest
if err := v.Struct(req); err != nil {
    errors := validator.FormatValidationErrors(err)
    response.BadRequest(c, "Validation failed", errors)
    return
}
```

## Example Service Setup

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/niaga-platform/lib-common/auth"
    "github.com/niaga-platform/lib-common/config"
    "github.com/niaga-platform/lib-common/database"
    "github.com/niaga-platform/lib-common/logger"
    "github.com/niaga-platform/lib-common/middleware"
    "time"
)

func main() {
    // Load config
    cfg, _ := config.LoadConfig("")
    
    // Setup logger
    log, _ := logger.NewLogger(cfg.App.Env)
    defer log.Sync()
    
    // Connect to database
    db, _ := database.Connect(database.PostgresConfig{
        Host:     cfg.Database.Host,
        Port:     cfg.Database.Port,
        User:     cfg.Database.User,
        Password: cfg.Database.Password,
        Database: cfg.Database.Name,
        SSLMode:  cfg.Database.SSLMode,
    }, log)
    
    // Connect to Redis
    redis, _ := database.ConnectRedis(database.RedisConfig{
        Host: cfg.Redis.Host,
        Port: cfg.Redis.Port,
        DB:   cfg.Redis.DB,
    }, log)
    
    // Setup JWT manager
    jwtManager := auth.NewJWTManager(
        cfg.JWT.Secret,
        15*time.Minute,
        168*time.Hour,
    )
    
    // Setup router
    router := gin.Default()
    router.Use(middleware.LoggerMiddleware(log))
    router.Use(middleware.RecoveryMiddleware(log))
    router.Use(middleware.CORSMiddleware())
    
    // Routes
    api := router.Group("/api/v1")
    api.Use(middleware.AuthMiddleware(jwtManager))
    {
        api.GET("/protected", handler)
    }
    
    // Start server
    router.Run(":" + cfg.App.Port)
}
```

## Development

```bash
# Install dependencies
cd lib-common
go mod download

# Run tests
go test ./...

# Update dependencies
go get -u ./...
go mod tidy
```

## License

MIT
