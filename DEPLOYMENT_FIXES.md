# Auth Service Deployment Fixes

This document outlines immediate fixes needed for the auth-service deployment based on lessons from option-platform.

## Issue 1: Localhost Redirect in Frontend

**Problem:** Frontend redirects to `http://localhost:8080` instead of `https://auth.vibeoholic.com`

**Fix:** Update frontend Dockerfile to accept VITE_API_URL as build arg

**Files to change:**
1. `frontend/Dockerfile` - Add ARG and ENV
2. `.github/workflows/build-frontend.yml` - Pass build arg
3. Rebuild and redeploy frontend

**Commands:**
```bash
# Update Dockerfile (see below)
# Then rebuild:
cd frontend
docker build --build-arg VITE_API_URL=https://auth.vibeoholic.com \
  -t ghcr.io/frallan97/auth-service-frontend:latest .
docker push ghcr.io/frallan97/auth-service-frontend:latest

# Restart frontend
kubectl rollout restart deployment/auth-service-frontend -n auth-service
```

**Updated `frontend/Dockerfile`:**
```dockerfile
# Build stage
FROM oven/bun:1 AS builder

# Build argument for API URL
ARG VITE_API_URL=https://auth.vibeoholic.com

WORKDIR /app

# Copy package files
COPY package.json bun.lock ./

# Install dependencies
RUN bun install --frozen-lockfile

# Copy source code
COPY . .

# Build the application with environment variable
ENV VITE_API_URL=$VITE_API_URL
RUN bun run build

# Production stage
FROM nginx:alpine

# Copy built assets from builder
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Expose port
EXPOSE 3000

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
```

**Updated `.github/workflows/build-frontend.yml`:**
```yaml
- name: Build and push Docker image
  uses: docker/build-push-action@v5
  with:
    context: ./frontend
    file: ./frontend/Dockerfile
    push: true
    tags: ${{ steps.meta.outputs.tags }}
    labels: ${{ steps.meta.outputs.labels }}
    cache-from: type=gha
    cache-to: type=gha,mode=max
    platforms: linux/amd64
    build-args: |
      VITE_API_URL=https://auth.vibeoholic.com
```

---

## Issue 2: Migrations Should Run in Backend Startup

**Problem:** Separate Kubernetes Job for migrations (currently stuck, adds complexity)

**Solution:** Run migrations in backend startup like option-platform does

### Changes Required

#### 1. Install golang-migrate in backend

**Add to `backend/go.mod`:**
```go
require (
    // ... existing dependencies ...
    github.com/golang-migrate/migrate/v4 v4.17.0
)
```

#### 2. Update `backend/internal/database/database.go`

Add migration function:
```go
package database

import (
    "fmt"
    "log"
    "os"

    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/jackc/pgx/v5/pgxpool"
)

// ... existing code ...

// RunMigrations runs database migrations on startup
func RunMigrations(dbURL string) error {
    // Get migrations path from environment or use default
    migrationsPath := os.Getenv("MIGRATIONS_PATH")
    if migrationsPath == "" {
        // Default path in container
        migrationsPath = "file:///app/migrations"
    }

    m, err := migrate.New(migrationsPath, dbURL)
    if err != nil {
        return fmt.Errorf("failed to create migrate instance: %w", err)
    }
    defer func() {
        _, closeErr := m.Close()
        if closeErr != nil {
            log.Printf("Warning: failed to close migrate instance: %v", closeErr)
        }
    }()

    // Run migrations
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("failed to run migrations: %w", err)
    }

    log.Println("Database migrations completed successfully")
    return nil
}
```

#### 3. Update `backend/cmd/api/main.go`

Add migration call before connecting to database:

```go
func main() {
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Run migrations BEFORE connecting
    log.Println("Running database migrations...")
    if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
        log.Fatalf("Failed to run migrations: %v", err)
    }

    // Connect to database
    db, err := database.Connect(cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer db.Close()

    log.Println("Database connected successfully")

    // ... rest of main ...
}
```

#### 4. Update `backend/Dockerfile`

Copy migrations into the container:

```dockerfile
# ... existing build stages ...

# Copy migrations
COPY migrations /app/migrations

# ... rest of Dockerfile ...
```

#### 5. Remove migration job from Helm chart

**Delete these files:**
- `charts/auth-service/templates/job-migration.yaml`
- `charts/auth-service/templates/configmap-migrations.yaml`

#### 6. Delete stuck migration job

```bash
kubectl delete job -n auth-service --all
```

### Benefits
- ✅ Simpler deployment (one less resource to manage)
- ✅ Migrations run automatically on every backend restart
- ✅ Idempotent (safe to run multiple times)
- ✅ Consistent with option-platform approach
- ✅ No risk of Helm hook failures

---

## Issue 3: Automatic Deployment Not Working

**Problem:** Code is pushed → images are built → but pods don't restart with new images

**Root Cause:** ArgoCD watches Git repo, not image registry. Since `tag: latest` never changes in Git, ArgoCD sees no update.

### Solution A: Use ArgoCD Image Updater (Recommended)

**Install Image Updater:**
```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-image-updater/stable/manifests/install.yaml
```

**Annotate auth-service Application:**
```bash
kubectl patch application auth-service -n argocd --type merge -p '{
  "metadata": {
    "annotations": {
      "argocd-image-updater.argoproj.io/image-list": "backend=ghcr.io/frallan97/auth-service-backend:latest,frontend=ghcr.io/frallan97/auth-service-frontend:latest",
      "argocd-image-updater.argoproj.io/backend.pull-secret": "pullsecret:auth-service/ghcr-pull-secret",
      "argocd-image-updater.argoproj.io/frontend.pull-secret": "pullsecret:auth-service/ghcr-pull-secret",
      "argocd-image-updater.argoproj.io/write-back-method": "argocd"
    }
  }
}'
```

This will:
1. Watch ghcr.io for new images
2. Automatically trigger ArgoCD sync when images are updated
3. Restart pods with new images

### Solution B: Use Commit SHA Tags (Alternative)

**Update `.github/workflows/build-backend.yml` and `build-frontend.yml`:**
```yaml
- name: Extract metadata (tags, labels) for Docker
  id: meta
  uses: docker/metadata-action@v5
  with:
    images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
    tags: |
      type=sha,format=short
      type=raw,value=latest,enable={{is_default_branch}}
```

This creates tags like: `sha-abc1234` and `latest`

**Then either:**
- Manually update `values.yaml` with new SHA after each push
- Use Image Updater to track SHA tags

### Solution C: Manual Restart (Quick Fix)

Add to your workflow or run manually:
```bash
# After images are pushed
kubectl rollout restart deployment/auth-service-backend -n auth-service
kubectl rollout restart deployment/auth-service-frontend -n auth-service
```

---

## Implementation Order

### Phase 1: Immediate Fixes (Do Today)
1. ✅ Fix localhost redirect (update Dockerfile + rebuild frontend)
2. ✅ Delete stuck migration job
3. ✅ Install ArgoCD Image Updater OR add manual restart to workflow

### Phase 2: Migration to Backend Startup (Do This Week)
1. Update backend code to run migrations on startup
2. Test locally with `docker-compose up`
3. Deploy to k8s
4. Verify migrations run successfully
5. Remove migration job files from Helm chart
6. Commit and push

### Phase 3: Multi-Solution Support (Follow MULTI_SOLUTION_PLAN.md)
1. Database schema changes
2. Backend API updates
3. Frontend updates
4. Integration with existing solutions

---

## Testing Checklist

After implementing fixes:

- [ ] Frontend loads at https://auth.vibeoholic.com
- [ ] Login redirects to Google OAuth (NOT localhost)
- [ ] After login, redirected back to https://auth.vibeoholic.com
- [ ] Backend logs show "Database migrations completed successfully"
- [ ] No migration job in `kubectl get jobs -n auth-service`
- [ ] Push code change → new image built → pods automatically restart
- [ ] Database schema is correct after backend restart

---

## Rollback Plan

If anything goes wrong:

**Frontend issue:**
```bash
# Revert to previous image
kubectl set image deployment/auth-service-frontend -n auth-service \
  frontend=ghcr.io/frallan97/auth-service-frontend:<previous-tag>
```

**Backend migration issue:**
```bash
# Backend will fail to start if migrations fail
# Check logs:
kubectl logs -n auth-service -l app.kubernetes.io/component=backend

# If needed, manually fix database and restart
kubectl rollout restart deployment/auth-service-backend -n auth-service
```

**Complete rollback:**
```bash
# Revert Git commit
git revert HEAD
git push

# Force ArgoCD sync
kubectl patch app auth-service -n argocd \
  -p '{"metadata": {"annotations": {"argocd.argoproj.io/refresh": "hard"}}}' \
  --type merge
```

---

## Current State vs Desired State

| Aspect | Current | Desired |
|--------|---------|---------|
| Frontend API URL | localhost:8080 | auth.vibeoholic.com |
| Migrations | Separate K8s Job (stuck) | Backend startup |
| Auto-deployment | Manual restart needed | Automatic on push |
| Complexity | High (job, hooks, manual) | Low (simple, automatic) |

---

## Questions?

1. **Will migrations run on every pod restart?**
   - Yes, but `migrate.Up()` is idempotent - it only applies new migrations

2. **What if a migration fails?**
   - Backend pod will crash and restart
   - K8s will keep trying
   - Fix the migration and push new code

3. **Can I roll back a migration?**
   - Yes, but you need to implement it manually
   - Add `migrate.Down()` functionality if needed
   - Generally, write forward-only migrations

4. **How do I add a new migration?**
   ```bash
   # Create migration files
   migrate create -ext sql -dir backend/migrations -seq add_new_feature

   # Edit the .up.sql and .down.sql files
   # Commit and push
   # Backend will auto-apply on next restart
   ```
