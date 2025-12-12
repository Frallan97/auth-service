# K3s Deployment Guide

This guide explains how to deploy the auth service to your K3s cluster under `auth.w.viboholic.com`.

## Prerequisites

- K3s cluster with ArgoCD installed (see [k3s-infra/SETUP_GUIDE.md](../k3s-infra/SETUP_GUIDE.md))
- Traefik ingress controller (comes with k3s)
- cert-manager for TLS certificates
- GitHub Container Registry (GHCR) access configured

## Deployment Architecture

The auth service consists of three components:
1. **Backend** - Go API server (auth-service-backend)
2. **Frontend** - React SPA (auth-service-frontend)
3. **Database** - PostgreSQL StatefulSet

All components are deployed using a Helm chart and managed by ArgoCD.

## Step 1: Prepare Secrets

### 1.1 Generate JWT Keys

```bash
cd auth-service/backend
mkdir -p keys
openssl genrsa -out keys/private_key.pem 4096
openssl rsa -in keys/private_key.pem -pubout -out keys/public_key.pem
```

### 1.2 Encode Secrets

You need to create sealed secrets for:
- Google OAuth credentials
- PostgreSQL password
- JWT RSA keys
- Admin emails

```bash
# Install kubeseal if not already installed
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.18.0/controller.yaml

# Create sealed secret for backend
kubectl create secret generic auth-service-backend \
  --from-literal=GOOGLE_CLIENT_ID='your-client-id' \
  --from-literal=GOOGLE_CLIENT_SECRET='your-client-secret' \
  --from-literal=ADMIN_EMAILS='your-email@example.com' \
  --dry-run=client -o yaml | \
  kubeseal --format=yaml > charts/auth-service/templates/sealed-secret-backend.yaml

# Create sealed secret for PostgreSQL
kubectl create secret generic auth-service-postgres \
  --from-literal=POSTGRES_PASSWORD='secure-random-password' \
  --dry-run=client -o yaml | \
  kubeseal --format=yaml > charts/auth-service/templates/sealed-secret-postgres.yaml

# Create sealed secret for JWT keys
kubectl create secret generic auth-service-jwt-keys \
  --from-file=private_key.pem=backend/keys/private_key.pem \
  --from-file=public_key.pem=backend/keys/public_key.pem \
  --dry-run=client -o yaml | \
  kubeseal --format=yaml > charts/auth-service/templates/sealed-secret-jwt-keys.yaml
```

### 1.3 Update Secret Templates

Replace the placeholder `secret.yaml` with your sealed secrets, or manually update the secrets after deployment.

**IMPORTANT:** Never commit unencrypted secrets to git!

## Step 2: Configure Google OAuth

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Navigate to **APIs & Services** → **Credentials**
3. Update your OAuth 2.0 client:
   - **Authorized JavaScript origins**:
     - `https://auth.w.viboholic.com`
   - **Authorized redirect URIs**:
     - `https://auth.w.viboholic.com/api/auth/google/callback`

## Step 3: Update Values

Edit `charts/auth-service/values.yaml` if you need to customize:

```yaml
backend:
  image:
    repository: ghcr.io/YOUR_USERNAME/auth-service-backend
    tag: latest

frontend:
  image:
    repository: ghcr.io/YOUR_USERNAME/auth-service-frontend
    tag: latest

ingress:
  host: auth.w.viboholic.com  # Your domain

database:
  persistence:
    size: 2Gi  # Adjust based on needs
```

## Step 4: Build and Push Docker Images

### 4.1 Manual Build (Optional)

```bash
# Build backend
cd backend
docker build -t ghcr.io/YOUR_USERNAME/auth-service-backend:latest .
docker push ghcr.io/YOUR_USERNAME/auth-service-backend:latest

# Build frontend
cd ../frontend
docker build -t ghcr.io/YOUR_USERNAME/auth-service-frontend:latest .
docker push ghcr.io/YOUR_USERNAME/auth-service-frontend:latest
```

### 4.2 Automated with GitHub Actions

Push your code to GitHub, and the workflows will automatically:
1. Build Docker images
2. Push to GHCR
3. Tag with branch name and commit SHA

```bash
git add .
git commit -m "Add K3s deployment configuration"
git push origin main
```

## Step 5: Deploy with ArgoCD

### 5.1 Add to app-of-apps

Edit your k3s-infra app-of-apps configuration:

```yaml
# In k3s-infra/clusters/main/apps/app-of-apps.yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: app-of-apps
  namespace: argocd
spec:
  generators:
  - list:
      elements:
      # ... existing apps ...
      - name: auth-service
        repoURL: https://github.com/YOUR_USERNAME/auth-service.git
        targetRevision: main
        path: charts/auth-service
  template:
    metadata:
      name: '{{name}}'
      namespace: argocd
    spec:
      project: default
      source:
        repoURL: '{{repoURL}}'
        targetRevision: '{{targetRevision}}'
        path: '{{path}}'
      destination:
        server: https://kubernetes.default.svc
        namespace: '{{name}}'
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        syncOptions:
        - CreateNamespace=true
```

### 5.2 Apply the Configuration

```bash
cd k3s-infra
kubectl apply -f clusters/main/apps/app-of-apps.yaml
```

ArgoCD will automatically:
- Create the `auth-service` namespace
- Deploy all resources from the Helm chart
- Set up ingress with TLS certificate
- Run database migrations
- Start the backend and frontend pods

### 5.3 Monitor Deployment

```bash
# Watch ArgoCD sync status
argocd app get auth-service --refresh

# Watch pods
kubectl get pods -n auth-service -w

# Check logs
kubectl logs -n auth-service -l app.kubernetes.io/component=backend -f
kubectl logs -n auth-service -l app.kubernetes.io/component=frontend -f

# Check ingress
kubectl get ingress -n auth-service
```

## Step 6: Verify Deployment

### 6.1 DNS Configuration

Ensure your DNS points to your K3s cluster:

```bash
# Check current DNS
dig auth.w.viboholic.com

# Should return your K3s node IP
```

### 6.2 Test Endpoints

```bash
# Test backend health
curl https://auth.w.viboholic.com/health

# Test frontend
curl https://auth.w.viboholic.com/

# Test public key endpoint
curl https://auth.w.viboholic.com/api/public-key
```

### 6.3 Browser Test

1. Open https://auth.w.viboholic.com
2. Click "Sign in with Google"
3. Complete OAuth flow
4. Verify you're redirected to the dashboard
5. Test the admin panel at `/admin/users`

## Troubleshooting

### Pods Not Starting

```bash
# Check pod status
kubectl get pods -n auth-service

# Describe pod for events
kubectl describe pod -n auth-service <pod-name>

# Check logs
kubectl logs -n auth-service <pod-name>
```

### Database Connection Issues

```bash
# Check if postgres is running
kubectl get statefulset -n auth-service

# Test database connection from backend pod
kubectl exec -n auth-service -it <backend-pod> -- sh
# Inside pod:
nc -zv auth-service-postgres 5432
```

### Certificate Issues

```bash
# Check certificate status
kubectl get certificate -n auth-service

# Check cert-manager logs
kubectl logs -n cert-manager -l app=cert-manager -f

# Describe certificate for events
kubectl describe certificate -n auth-service auth-service-tls
```

### Image Pull Errors

Ensure GHCR credentials are configured:

```bash
# Create image pull secret (if not exists)
kubectl create secret docker-registry ghcr-pull-secret \
  --docker-server=ghcr.io \
  --docker-username=YOUR_USERNAME \
  --docker-password=YOUR_GITHUB_TOKEN \
  -n auth-service
```

### OAuth Redirect Issues

1. Verify Google OAuth redirect URI matches exactly:
   - `https://auth.w.viboholic.com/api/auth/google/callback`
2. Check backend logs for OAuth errors
3. Ensure CORS is configured correctly in values.yaml

## Updating the Deployment

### Update Code

```bash
# Make changes to code
git add .
git commit -m "Update feature"
git push

# GitHub Actions will build new images
# ArgoCD will automatically sync the changes
```

### Update Configuration

```bash
# Edit values.yaml or other Helm files
vim charts/auth-service/values.yaml

# Commit and push
git add charts/
git commit -m "Update configuration"
git push

# ArgoCD will detect changes and sync
```

### Manual Sync

```bash
# Force sync via ArgoCD
argocd app sync auth-service

# Or via kubectl
kubectl patch app auth-service -n argocd \
  -p '{"metadata": {"annotations": {"argocd.argoproj.io/refresh": "hard"}}}' \
  --type merge
```

## Rollback

```bash
# Via ArgoCD
argocd app rollback auth-service <revision>

# Or manually
kubectl rollout undo deployment/auth-service-backend -n auth-service
kubectl rollout undo deployment/auth-service-frontend -n auth-service
```

## Backup Database

```bash
# Exec into postgres pod
kubectl exec -n auth-service -it auth-service-postgres-0 -- sh

# Inside pod, create backup
pg_dump -U authuser authdb > /tmp/backup.sql

# Copy backup to local machine
kubectl cp auth-service/auth-service-postgres-0:/tmp/backup.sql ./backup.sql
```

## Monitoring

### Logs

```bash
# Stream all auth-service logs
kubectl logs -n auth-service -l app.kubernetes.io/name=auth-service -f --tail=100

# Backend logs only
kubectl logs -n auth-service -l app.kubernetes.io/component=backend -f

# Frontend logs only
kubectl logs -n auth-service -l app.kubernetes.io/component=frontend -f
```

### Resources

```bash
# Check resource usage
kubectl top pods -n auth-service

# Check PVC status
kubectl get pvc -n auth-service
```

## Security Considerations

1. **Secrets Management**
   - Use Sealed Secrets for sensitive data
   - Never commit plain secrets to git
   - Rotate credentials regularly

2. **Network Policies**
   - Consider adding NetworkPolicies to restrict traffic
   - Frontend should only talk to backend
   - Backend should only talk to database

3. **RBAC**
   - ArgoCD manages deployments
   - Limit access to the auth-service namespace

4. **TLS/SSL**
   - Certificates auto-renewed by cert-manager
   - Monitor certificate expiration

## Integration with Other Services

Once deployed, other services can integrate by:

1. Fetching the public key:
   ```bash
   curl https://auth.w.viboholic.com/api/public-key > public_key.pem
   ```

2. Validating JWT tokens locally

3. Using Casbin for authorization

See [INTEGRATION.md](./INTEGRATION.md) for complete integration guide.

## Production Checklist

- [ ] DNS configured and pointing to K3s cluster
- [ ] Google OAuth credentials configured with production URLs
- [ ] Sealed secrets created and applied
- [ ] JWT keys generated and stored securely
- [ ] Database persistent volume configured
- [ ] Resource limits tuned for production
- [ ] Backup strategy implemented
- [ ] Monitoring and alerting configured
- [ ] Load testing performed
- [ ] Security audit completed
