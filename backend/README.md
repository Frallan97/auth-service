# Auth Service Backend

Go backend for the authentication service providing Google OAuth and JWT token management.

## Prerequisites

- Go 1.21+
- PostgreSQL 15+
- golang-migrate CLI (for database migrations)
- OpenSSL (for generating RSA keys)

## Setup

### 1. Install golang-migrate

```bash
# macOS
brew install golang-migrate

# Linux
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/
```

### 2. Generate RSA Keys

```bash
make keys
```

This will create `keys/private_key.pem` and `keys/public_key.pem` for JWT signing.

### 3. Configure Environment

Copy `.env.example` to `.env` and update with your values:

```bash
cp .env.example .env
```

Required values:
- `DATABASE_URL`: PostgreSQL connection string
- `GOOGLE_CLIENT_ID`: From Google Cloud Console
- `GOOGLE_CLIENT_SECRET`: From Google Cloud Console
- `ADMIN_EMAILS`: Comma-separated list of admin emails

### 4. Set up Google OAuth

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing
3. Enable Google+ API
4. Create OAuth 2.0 credentials
5. Add authorized redirect URI: `http://localhost:8080/api/auth/google/callback`
6. Copy Client ID and Client Secret to `.env`

### 5. Run Database Migrations

```bash
export DATABASE_URL='postgresql://authuser:authpass@localhost:5432/authdb?sslmode=disable'
make migrate-up
```

### 6. Run the Server

```bash
make run
```

Server will start on `http://localhost:8080`

## API Endpoints

### Public Endpoints

- `GET /health` - Health check
- `GET /api/public-key` - Get JWT public key (for other services)
- `GET /api/auth/google/login` - Initiate Google OAuth
- `GET /api/auth/google/callback` - OAuth callback
- `POST /api/auth/refresh` - Refresh access token
- `POST /api/auth/logout` - Logout user

### Protected Endpoints

Requires `Authorization: Bearer <access_token>` header

- `GET /api/auth/me` - Get current user
- `GET /api/users` - List users (paginated)
- `GET /api/users/:id` - Get user by ID
- `PUT /api/users/:id` - Update user
- `DELETE /api/users/:id` - Soft delete user
- `POST /api/users/:id/activate` - Activate user
- `POST /api/users/:id/deactivate` - Deactivate user

## Development

### Run Tests

```bash
make test
```

### Create New Migration

```bash
make migrate-create NAME=add_new_table
```

### Rollback Migrations

```bash
make migrate-down
```

## Project Structure

```
backend/
├── cmd/
│   └── api/
│       └── main.go          # Application entry point
├── internal/
│   ├── auth/
│   │   └── service.go       # Auth business logic
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── database/
│   │   └── database.go      # Database connection
│   ├── handlers/
│   │   ├── handlers.go      # Handler setup
│   │   ├── auth.go          # Auth endpoints
│   │   └── users.go         # User management endpoints
│   ├── middleware/
│   │   └── auth.go          # JWT validation middleware
│   └── models/
│       └── models.go        # Data models
├── pkg/
│   └── jwt/
│       └── jwt.go           # JWT utilities
├── migrations/              # Database migrations
├── keys/                    # RSA keys (gitignored)
├── .env                     # Environment variables (gitignored)
├── .env.example             # Example environment variables
├── Makefile                 # Build and run commands
└── README.md
```

## Integration with Other Services

Other services can validate JWTs without calling the auth service:

1. Fetch the public key once from `/api/public-key`
2. Cache the public key
3. Validate incoming JWT tokens using the public key
4. Extract user ID from token claims
5. Use Casbin for authorization

Example integration code available in the main plan.md.
