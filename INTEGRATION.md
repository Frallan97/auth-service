# Integration Guide for Other Services

This guide explains how to integrate your microservices with the Auth Service for authentication and authorization using Casbin.

## Overview

The Auth Service provides:
- **Authentication**: Google OAuth login with JWT tokens
- **Token Generation**: RS256-signed JWT access tokens
- **Token Validation**: Public key distribution for other services

Your services are responsible for:
- **Token Validation**: Verify JWT signatures using the public key
- **Authorization**: Use Casbin for RBAC/ABAC policies

## Architecture Pattern

```
User Request → Your Service → Validate JWT → Casbin Check → Handle Request
                    ↑              ↑
                    |              |
            Auth Service    Local Policy DB
         (Public Key Once)
```

## Integration Steps

### Step 1: Fetch the Public Key

Fetch the JWT public key from the auth service once during startup:

```bash
curl http://localhost:8080/api/public-key > public_key.pem
```

Or programmatically in your service:

```go
package main

import (
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
    "fmt"
    "io"
    "net/http"
)

func fetchPublicKey(authServiceURL string) (*rsa.PublicKey, error) {
    resp, err := http.Get(authServiceURL + "/api/public-key")
    if err != nil {
        return nil, fmt.Errorf("failed to fetch public key: %w", err)
    }
    defer resp.Body.Close()

    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read public key: %w", err)
    }

    block, _ := pem.Decode(data)
    if block == nil {
        return nil, fmt.Errorf("failed to decode PEM block")
    }

    pub, err := x509.ParsePKIXPublicKey(block.Bytes)
    if err != nil {
        return nil, fmt.Errorf("failed to parse public key: %w", err)
    }

    rsaPub, ok := pub.(*rsa.PublicKey)
    if !ok {
        return nil, fmt.Errorf("not an RSA public key")
    }

    return rsaPub, nil
}
```

### Step 2: Create JWT Validation Middleware

```go
package middleware

import (
    "context"
    "crypto/rsa"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
    UserIDKey contextKey = "userID"
    EmailKey  contextKey = "email"
    NameKey   contextKey = "name"
)

type Claims struct {
    UserID string `json:"sub"`
    Email  string `json:"email"`
    Name   string `json:"name"`
    jwt.RegisteredClaims
}

func AuthMiddleware(publicKey *rsa.PublicKey) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract token from Authorization header
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Missing authorization header", http.StatusUnauthorized)
                return
            }

            parts := strings.SplitN(authHeader, " ", 2)
            if len(parts) != 2 || parts[0] != "Bearer" {
                http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
                return
            }

            tokenString := parts[1]

            // Parse and validate token
            token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
                if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
                    return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
                }
                return publicKey, nil
            })

            if err != nil {
                http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
                return
            }

            claims, ok := token.Claims.(*Claims)
            if !ok || !token.Valid {
                http.Error(w, "Invalid token claims", http.StatusUnauthorized)
                return
            }

            // Add user info to context
            ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
            ctx = context.WithValue(ctx, EmailKey, claims.Email)
            ctx = context.WithValue(ctx, NameKey, claims.Name)

            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Step 3: Set Up Casbin

#### Install Casbin

```bash
go get github.com/casbin/casbin/v2
go get github.com/casbin/gorm-adapter/v3
```

#### Create Casbin Model (model.conf)

For RBAC (Role-Based Access Control):

```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```

For ABAC (Attribute-Based Access Control):

```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub_rule, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = eval(p.sub_rule) && r.obj == p.obj && r.act == p.act
```

#### Initialize Casbin Enforcer

```go
package main

import (
    "github.com/casbin/casbin/v2"
    gormadapter "github.com/casbin/gorm-adapter/v3"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func initCasbin(dbURL string) (*casbin.Enforcer, error) {
    // Connect to database
    db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
    if err != nil {
        return nil, err
    }

    // Initialize adapter (stores policies in database)
    adapter, err := gormadapter.NewAdapterByDB(db)
    if err != nil {
        return nil, err
    }

    // Create enforcer
    enforcer, err := casbin.NewEnforcer("model.conf", adapter)
    if err != nil {
        return nil, err
    }

    // Load policies from database
    err = enforcer.LoadPolicy()
    if err != nil {
        return nil, err
    }

    return enforcer, nil
}
```

### Step 4: Create Authorization Middleware

```go
package middleware

import (
    "net/http"

    "github.com/casbin/casbin/v2"
)

func CasbinMiddleware(enforcer *casbin.Enforcer, obj, act string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Get user ID from context (set by AuthMiddleware)
            userID, ok := r.Context().Value(UserIDKey).(string)
            if !ok {
                http.Error(w, "User ID not found in context", http.StatusUnauthorized)
                return
            }

            // Check permission
            allowed, err := enforcer.Enforce(userID, obj, act)
            if err != nil {
                http.Error(w, "Permission check failed", http.StatusInternalServerError)
                return
            }

            if !allowed {
                http.Error(w, "Forbidden", http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### Step 5: Apply Middleware to Routes

```go
package main

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "your-service/middleware"
)

func main() {
    // Fetch public key
    publicKey, err := fetchPublicKey("http://localhost:8080")
    if err != nil {
        panic(err)
    }

    // Initialize Casbin
    enforcer, err := initCasbin("postgresql://user:pass@localhost:5432/yourdb")
    if err != nil {
        panic(err)
    }

    // Create router
    r := chi.NewRouter()

    // Public routes
    r.Get("/health", healthHandler)

    // Protected routes
    r.Group(func(r chi.Router) {
        // Apply JWT validation middleware
        r.Use(middleware.AuthMiddleware(publicKey))

        // Routes with different permissions
        r.Group(func(r chi.Router) {
            r.Use(middleware.CasbinMiddleware(enforcer, "tickets", "read"))
            r.Get("/tickets", listTicketsHandler)
            r.Get("/tickets/{id}", getTicketHandler)
        })

        r.Group(func(r chi.Router) {
            r.Use(middleware.CasbinMiddleware(enforcer, "tickets", "write"))
            r.Post("/tickets", createTicketHandler)
            r.Put("/tickets/{id}", updateTicketHandler)
        })

        r.Group(func(r chi.Router) {
            r.Use(middleware.CasbinMiddleware(enforcer, "tickets", "delete"))
            r.Delete("/tickets/{id}", deleteTicketHandler)
        })
    })

    http.ListenAndServe(":8081", r)
}
```

## Managing Casbin Policies

### Add Policy Programmatically

```go
// Add a permission policy
enforcer.AddPolicy("admin", "tickets", "read")
enforcer.AddPolicy("admin", "tickets", "write")
enforcer.AddPolicy("admin", "tickets", "delete")
enforcer.AddPolicy("user", "tickets", "read")

// Assign role to user
enforcer.AddGroupingPolicy("user-uuid-123", "admin")
enforcer.AddGroupingPolicy("user-uuid-456", "user")

// Save policies to database
enforcer.SavePolicy()
```

### Load Initial Policies

```go
func seedPolicies(enforcer *casbin.Enforcer) error {
    // Define roles and permissions
    policies := [][]string{
        {"admin", "tickets", "read"},
        {"admin", "tickets", "write"},
        {"admin", "tickets", "delete"},
        {"admin", "users", "read"},
        {"admin", "users", "write"},
        {"user", "tickets", "read"},
        {"user", "tickets", "write"},
    }

    for _, policy := range policies {
        _, err := enforcer.AddPolicy(policy)
        if err != nil {
            return err
        }
    }

    return enforcer.SavePolicy()
}
```

### Query User Permissions

```go
// Get all permissions for a user
permissions := enforcer.GetPermissionsForUser("user-uuid-123")

// Get all roles for a user
roles := enforcer.GetRolesForUser("user-uuid-123")

// Check if user has specific permission
hasPermission, _ := enforcer.Enforce("user-uuid-123", "tickets", "write")
```

## Frontend Integration

### Send JWT with Requests

```typescript
// In your API client
import axios from 'axios'

const api = axios.create({
  baseURL: 'http://localhost:8081',
  withCredentials: true, // Include cookies if needed
})

// Add JWT token to all requests
api.interceptors.request.use((config) => {
  const token = sessionStorage.getItem('access_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Handle 401 responses (refresh token if needed)
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401) {
      // Redirect to login or try to refresh token
      window.location.href = 'http://localhost:3000/login'
    }
    return Promise.reject(error)
  }
)

export default api
```

## Example: Complete Service Integration

Here's a complete example of a ticket service integrated with auth:

```go
package main

import (
    "context"
    "crypto/rsa"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strings"

    "github.com/casbin/casbin/v2"
    gormadapter "github.com/casbin/gorm-adapter/v3"
    "github.com/go-chi/chi/v5"
    chiMiddleware "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"
    "github.com/golang-jwt/jwt/v5"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

type contextKey string

const UserIDKey contextKey = "userID"

type Claims struct {
    UserID string `json:"sub"`
    Email  string `json:"email"`
    Name   string `json:"name"`
    jwt.RegisteredClaims
}

func main() {
    // Fetch public key from auth service
    publicKey, err := fetchPublicKey("http://localhost:8080")
    if err != nil {
        log.Fatal("Failed to fetch public key:", err)
    }

    // Initialize Casbin
    enforcer, err := initCasbin("postgresql://user:pass@localhost:5432/ticketdb")
    if err != nil {
        log.Fatal("Failed to initialize Casbin:", err)
    }

    // Seed initial policies
    seedPolicies(enforcer)

    // Create router
    r := chi.NewRouter()

    // Middleware
    r.Use(chiMiddleware.Logger)
    r.Use(chiMiddleware.Recoverer)
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{"http://localhost:3000"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
        AllowCredentials: true,
    }))

    // Health check
    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    })

    // Protected routes
    r.Group(func(r chi.Router) {
        r.Use(authMiddleware(publicKey))

        r.Get("/tickets", casbinMiddleware(enforcer, "tickets", "read")(http.HandlerFunc(listTickets)).ServeHTTP)
        r.Post("/tickets", casbinMiddleware(enforcer, "tickets", "write")(http.HandlerFunc(createTicket)).ServeHTTP)
        r.Delete("/tickets/{id}", casbinMiddleware(enforcer, "tickets", "delete")(http.HandlerFunc(deleteTicket)).ServeHTTP)
    })

    log.Println("Ticket service starting on :8081")
    http.ListenAndServe(":8081", r)
}

func fetchPublicKey(authServiceURL string) (*rsa.PublicKey, error) {
    // Implementation from Step 1
    return nil, nil
}

func initCasbin(dbURL string) (*casbin.Enforcer, error) {
    // Implementation from Step 3
    return nil, nil
}

func seedPolicies(enforcer *casbin.Enforcer) {
    // Add default policies
    enforcer.AddPolicy("admin", "tickets", "read")
    enforcer.AddPolicy("admin", "tickets", "write")
    enforcer.AddPolicy("admin", "tickets", "delete")
    enforcer.AddPolicy("user", "tickets", "read")
    enforcer.SavePolicy()
}

func authMiddleware(publicKey *rsa.PublicKey) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Missing authorization", http.StatusUnauthorized)
                return
            }

            parts := strings.SplitN(authHeader, " ", 2)
            if len(parts) != 2 || parts[0] != "Bearer" {
                http.Error(w, "Invalid authorization", http.StatusUnauthorized)
                return
            }

            token, err := jwt.ParseWithClaims(parts[1], &Claims{}, func(token *jwt.Token) (interface{}, error) {
                return publicKey, nil
            })

            if err != nil || !token.Valid {
                http.Error(w, "Invalid token", http.StatusUnauthorized)
                return
            }

            claims := token.Claims.(*Claims)
            ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func casbinMiddleware(enforcer *casbin.Enforcer, obj, act string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userID := r.Context().Value(UserIDKey).(string)

            allowed, _ := enforcer.Enforce(userID, obj, act)
            if !allowed {
                http.Error(w, "Forbidden", http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

func listTickets(w http.ResponseWriter, r *http.Request) {
    // Handler implementation
    json.NewEncoder(w).Encode([]string{"ticket1", "ticket2"})
}

func createTicket(w http.ResponseWriter, r *http.Request) {
    // Handler implementation
    json.NewEncoder(w).Encode(map[string]string{"message": "Ticket created"})
}

func deleteTicket(w http.ResponseWriter, r *http.Request) {
    // Handler implementation
    json.NewEncoder(w).Encode(map[string]string{"message": "Ticket deleted"})
}
```

## Testing Your Integration

### 1. Get a JWT Token

```bash
# Login through the auth service frontend
# Or use this to test directly:
curl -c cookies.txt http://localhost:8080/api/auth/google/login

# After OAuth flow, extract the access token
```

### 2. Test Your Service

```bash
# Test protected endpoint
curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
     http://localhost:8081/tickets

# Test forbidden endpoint (no permission)
curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
     -X DELETE \
     http://localhost:8081/tickets/123
```

### 3. Assign Permissions

```go
// In your service's admin endpoint or CLI tool
enforcer.AddGroupingPolicy("user-uuid-from-jwt", "admin")
enforcer.SavePolicy()
```

## Best Practices

1. **Cache the Public Key**: Fetch once at startup, refresh periodically (e.g., every 24 hours)
2. **Use Middleware**: Apply auth and authorization as middleware for clean code
3. **Store Policies in Database**: Use Casbin adapters to persist policies
4. **Provide Admin UI**: Create endpoints/UI to manage roles and permissions
5. **Log Authorization Events**: Track who accessed what for audit purposes
6. **Handle Token Expiry**: Implement proper error handling for expired tokens
7. **Test Permissions**: Write tests for your authorization logic
8. **Document Permissions**: Maintain clear documentation of required permissions

## Troubleshooting

### Token Validation Fails
- Verify you're using the correct public key
- Check token hasn't expired (15-minute default)
- Ensure RS256 algorithm is expected

### Casbin Always Denies
- Check policies are loaded: `enforcer.GetPolicy()`
- Verify user ID matches exactly (UUID format)
- Check model.conf syntax is correct

### CORS Errors
- Add your frontend URL to ALLOWED_ORIGINS
- Ensure credentials are allowed if using cookies

## Additional Resources

- [Casbin Documentation](https://casbin.org/docs/overview)
- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)
- [Go Chi Router](https://github.com/go-chi/chi)
- [Auth Service API Reference](README.md#api-endpoints)
