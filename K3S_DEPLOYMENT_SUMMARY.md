# K3s Deployment Summary

## What Was Created

Your auth service is now fully prepared for K3s deployment with the following components:

### 1. Helm Chart (`charts/auth-service/`)

Complete Helm chart with templates for:
- ✅ Backend Deployment (Go API)
- ✅ Frontend Deployment (React SPA)
- ✅ PostgreSQL StatefulSet (with persistent storage)
- ✅ Services (backend, frontend, database)
- ✅ Ingress (Traefik with TLS)
- ✅ ConfigMaps (application configuration)
- ✅ Secrets (Google OAuth, PostgreSQL, JWT keys)
- ✅ Database Migration Job (runs automatically)

### 2. GitHub Actions Workflows (`.github/workflows/`)

CI/CD pipelines for:
- ✅ `build-backend.yml` - Builds and pushes backend Docker image to GHCR
- ✅ `build-frontend.yml` - Builds and pushes frontend Docker image to GHCR
- Auto-triggers on push to main branch
- Caches layers for faster builds
- Tags with branch name, commit SHA, and latest

### 3. Documentation

- ✅ `K3S_DEPLOYMENT.md` - Complete deployment guide (150+ lines)
- ✅ `scripts/prepare-k3s-deployment.sh` - Automated setup script
- ✅ Updated `README.md` with K3s deployment section

### 4. Architecture

```
Internet (HTTPS)
        ↓
   Traefik Ingress (TLS via cert-manager)
        ↓
   auth.w.viboholic.com
        ↓
   ┌────────────────────┬─────────────────────┐
   │                    │                     │
Frontend Service   Backend Service   Database Service
   (React)           (Go API)         (PostgreSQL)
   Port 80           Port 8080        Port 5432
   │                    │                     │
Frontend Pod       Backend Pod       PostgreSQL Pod
   │                    │                     │
   └────────────────────┴─────────────────────┘
                        │
                 Persistent Volume
                   (Database Data)
```

## Deployment Pattern (Following Your Infrastructure)

Based on your existing `k3s-infra` and `frans-cv` setups:

### ArgoCD GitOps
- Applications defined in `app-of-apps.yaml`
- Auto-sync enabled
- Self-healing enabled
- Namespace auto-creation

### Traefik Ingress
- Automatic routing based on hostname
- TLS termination with Let's Encrypt
- HTTP → HTTPS redirect
- WebSocket support (if needed)

### Sealed Secrets
- Secrets encrypted at rest
- Only k3s cluster can decrypt
- Safe to commit to git

### Container Registry
- GitHub Container Registry (GHCR)
- Private images with image pull secrets
- Automatic builds on push

## Next Steps to Deploy

### 1. Prepare Secrets (One-time Setup)

```bash
cd /home/frans-sjostrom/Documents/hobby/sandbox/auth-service
./scripts/prepare-k3s-deployment.sh
```

This script will:
- Generate JWT RSA keys (if not exists)
- Prompt for Google OAuth credentials
- Generate secure PostgreSQL password
- Create sealed secrets for K3s
- Update image repositories with your GitHub username

### 2. Configure Google OAuth

Update your Google OAuth app:
- **Authorized JavaScript origins**: `https://auth.w.viboholic.com`
- **Authorized redirect URIs**: `https://auth.w.viboholic.com/api/auth/google/callback`

### 3. Push to GitHub

```bash
git add .
git commit -m "Add K3s deployment configuration"
git push origin main
```

GitHub Actions will automatically build and push Docker images.

### 4. Add to ArgoCD

Edit `k3s-infra/clusters/main/apps/app-of-apps.yaml`:

```yaml
elements:
  # ... existing apps ...
  - name: auth-service
    repoURL: https://github.com/YOUR_USERNAME/auth-service.git
    targetRevision: main
    path: charts/auth-service
```

Apply the configuration:

```bash
cd ../k3s-infra
kubectl apply -f clusters/main/apps/app-of-apps.yaml
```

### 5. Monitor Deployment

```bash
# Watch ArgoCD sync
argocd app get auth-service --refresh

# Watch pods starting
kubectl get pods -n auth-service -w

# Check logs
kubectl logs -n auth-service -l app.kubernetes.io/component=backend -f
```

### 6. Verify

Once deployed, test:

```bash
# Backend health
curl https://auth.w.viboholic.com/health

# Frontend
curl https://auth.w.viboholic.com/

# Public key
curl https://auth.w.viboholic.com/api/public-key
```

Open in browser: **https://auth.w.viboholic.com**

## File Structure Created

```
auth-service/
├── charts/
│   └── auth-service/
│       ├── Chart.yaml                           # Helm chart metadata
│       ├── values.yaml                          # Configuration values
│       └── templates/
│           ├── _helpers.tpl                     # Template helpers
│           ├── configmap.yaml                   # App configuration
│           ├── configmap-migrations.yaml        # Database migrations
│           ├── deployment-backend.yaml          # Backend deployment
│           ├── deployment-frontend.yaml         # Frontend deployment
│           ├── ingress.yaml                     # Traefik ingress + TLS
│           ├── job-migration.yaml               # DB migration job
│           ├── secret.yaml                      # Secret placeholders
│           ├── service-backend.yaml             # Backend service
│           ├── service-frontend.yaml            # Frontend service
│           ├── service-postgres.yaml            # Database service
│           └── statefulset-postgres.yaml        # PostgreSQL database
│
├── .github/
│   └── workflows/
│       ├── build-backend.yml                    # Backend CI/CD
│       └── build-frontend.yml                   # Frontend CI/CD
│
├── scripts/
│   └── prepare-k3s-deployment.sh               # Setup script
│
├── K3S_DEPLOYMENT.md                            # Full deployment guide
├── K3S_DEPLOYMENT_SUMMARY.md                   # This file
└── README.md                                    # Updated with K3s section
```

## Configuration Values

### Key Configuration in `values.yaml`

```yaml
backend:
  image:
    repository: ghcr.io/frallan97/auth-service-backend
    tag: latest

frontend:
  image:
    repository: ghcr.io/frallan97/auth-service-frontend
    tag: latest

database:
  enabled: true
  persistence:
    enabled: true
    size: 2Gi

ingress:
  enabled: true
  host: auth.w.viboholic.com
  className: traefik
  tls:
    enabled: true
```

### Secrets Required

1. **Google OAuth** (`auth-service-backend`)
   - GOOGLE_CLIENT_ID
   - GOOGLE_CLIENT_SECRET
   - ADMIN_EMAILS

2. **Database** (`auth-service-postgres`)
   - POSTGRES_PASSWORD

3. **JWT Keys** (`auth-service-jwt-keys`)
   - private_key.pem
   - public_key.pem

## Resource Requirements

Per pod:

- **Backend**: 250m CPU, 256Mi RAM (request) / 500m CPU, 512Mi RAM (limit)
- **Frontend**: 100m CPU, 128Mi RAM (request) / 200m CPU, 256Mi RAM (limit)
- **Database**: 250m CPU, 256Mi RAM (request) / 500m CPU, 512Mi RAM (limit)
- **Storage**: 2Gi persistent volume for PostgreSQL

Total cluster requirements (minimum):
- **CPU**: ~600m (0.6 cores)
- **RAM**: ~640Mi
- **Storage**: 2Gi

## Troubleshooting Quick Reference

| Issue | Command | Solution |
|-------|---------|----------|
| Pods not starting | `kubectl describe pod -n auth-service <pod>` | Check events and logs |
| Image pull error | `kubectl get pods -n auth-service` | Ensure GHCR credentials exist |
| Database connection | `kubectl logs -n auth-service <backend-pod>` | Check DATABASE_URL in configmap |
| Certificate not issuing | `kubectl describe certificate -n auth-service` | Check cert-manager logs |
| 404 on URL | `kubectl get ingress -n auth-service` | Verify DNS and ingress rules |
| OAuth redirect error | Check Google Console | Ensure redirect URI matches exactly |

## Differences from Local Development

| Feature | Local (Docker Compose) | K3s (Production) |
|---------|----------------------|------------------|
| Deployment | Manual `docker-compose up` | GitOps with ArgoCD |
| Secrets | .env files | Sealed Secrets |
| TLS/SSL | None | Automatic with cert-manager |
| Domain | localhost:3000, :8080 | auth.w.viboholic.com |
| Database | Ephemeral container | Persistent StatefulSet |
| Scaling | Manual | Automatic (HPA ready) |
| Health checks | None | Liveness + Readiness |
| Updates | Manual rebuild | Git push → Auto-deploy |
| Monitoring | Docker logs | Kubernetes logs + metrics |
| Backup | Manual | Automated (via k3s) |

## Integration with Your Existing Services

Once deployed, your other services (like `frans-cv`, `go-api`, etc.) can integrate:

### 1. Fetch Public Key

```bash
# One-time fetch
curl https://auth.w.viboholic.com/api/public-key > public_key.pem
```

### 2. Store as Secret in K3s

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: auth-service-public-key
  namespace: your-service
type: Opaque
stringData:
  public_key.pem: |
    -----BEGIN PUBLIC KEY-----
    ...
    -----END PUBLIC KEY-----
```

### 3. Mount in Your Deployment

```yaml
volumeMounts:
- name: auth-public-key
  mountPath: /etc/auth
  readOnly: true

volumes:
- name: auth-public-key
  secret:
    secretName: auth-service-public-key
```

### 4. Validate Tokens

See [INTEGRATION.md](INTEGRATION.md) for complete Go code examples.

## Advantages of This Setup

✅ **Infrastructure as Code** - Everything in git
✅ **Declarative** - Desired state defined, k3s ensures it
✅ **Automatic** - Push to git → deployed to cluster
✅ **Secure** - Sealed secrets, TLS, network policies
✅ **Scalable** - Ready for HPA (Horizontal Pod Autoscaler)
✅ **Observable** - Built-in health checks and logs
✅ **Recoverable** - Self-healing, rollback support
✅ **Consistent** - Same pattern as your other services

## Production Checklist

Before going live:

- [ ] DNS A record points to K3s cluster IP
- [ ] Google OAuth configured with production URLs
- [ ] Sealed secrets created and applied
- [ ] Resource limits tuned for your workload
- [ ] Database backup strategy implemented
- [ ] Monitoring and alerting configured
- [ ] Load testing performed
- [ ] Security audit completed
- [ ] Documentation reviewed

## Support

For questions or issues:
1. Check [K3S_DEPLOYMENT.md](K3S_DEPLOYMENT.md) for detailed instructions
2. Review Kubernetes logs: `kubectl logs -n auth-service <pod>`
3. Check ArgoCD UI for sync status
4. Verify DNS and Google OAuth configuration

---

**Ready to deploy? Run:** `./scripts/prepare-k3s-deployment.sh`
