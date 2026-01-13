# ArgoCD Image Updater Setup

This document describes the automated deployment setup using ArgoCD Image Updater for the auth-service.

## Overview

The auth-service now uses **ArgoCD Image Updater** to automatically detect and deploy new Docker images without requiring GitHub Actions to push changes back to the repository. This simplifies the deployment pipeline and eliminates the permission issues associated with the bot pushing commits.

## How It Works

1. **GitHub Actions** builds Docker images and pushes them to `ghcr.io/frallan97/auth-service-{backend,frontend}` with tags:
   - `latest` - always points to the most recent main branch build
   - `main-{sha}` - commit-specific tags for traceability

2. **ArgoCD Image Updater** monitors the container registry for new images and automatically updates the Application spec in ArgoCD when new images are detected.

3. **ArgoCD** syncs the changes to the Kubernetes cluster, deploying the new images.

## Configuration

### GitHub Actions Workflows

The workflows (`.github/workflows/build-backend.yml` and `.github/workflows/build-frontend.yml`) have been simplified to:
- Build Docker images
- Push to GitHub Container Registry (ghcr.io)
- **No longer push changes back to the repository**

### Helm Chart Values

The `charts/auth-service/values.yaml` contains an `argocd.imageUpdater` section with the recommended annotations:

```yaml
argocd:
  imageUpdater:
    enabled: true
    annotations:
      # Image list to track
      argocd-image-updater.argoproj.io/image-list: backend=ghcr.io/frallan97/auth-service-backend,frontend=ghcr.io/frallan97/auth-service-frontend

      # Update strategy
      argocd-image-updater.argoproj.io/backend.update-strategy: latest
      argocd-image-updater.argoproj.io/frontend.update-strategy: latest

      # Force update even if tag is latest
      argocd-image-updater.argoproj.io/backend.force-update: "true"
      argocd-image-updater.argoproj.io/frontend.force-update: "true"

      # Helm parameter paths
      argocd-image-updater.argoproj.io/backend.helm.image-name: backend.image.repository
      argocd-image-updater.argoproj.io/backend.helm.image-tag: backend.image.tag
      argocd-image-updater.argoproj.io/frontend.helm.image-name: frontend.image.repository
      argocd-image-updater.argoproj.io/frontend.helm.image-tag: frontend.image.tag

      # Pull secrets
      argocd-image-updater.argoproj.io/backend.pull-secret: pullsecret:auth-service/ghcr-pull-secret
      argocd-image-updater.argoproj.io/frontend.pull-secret: pullsecret:auth-service/ghcr-pull-secret

      # Write method: update Application spec directly
      argocd-image-updater.argoproj.io/write-back-method: argocd
```

### ArgoCD Application

These annotations must be added to the ArgoCD Application resource in the `k3s-infra` repository.

**Location**: `k3s-infra/clusters/main/apps/app-of-apps.yaml`

**Update the auth-service entry to include metadata annotations**:

```yaml
- name: auth-service
  repoURL: https://github.com/Frallan97/auth-service.git
  targetRevision: main
  path: charts/auth-service
  # Add these annotations
  metadata:
    annotations:
      argocd-image-updater.argoproj.io/image-list: backend=ghcr.io/frallan97/auth-service-backend,frontend=ghcr.io/frallan97/auth-service-frontend
      argocd-image-updater.argoproj.io/backend.update-strategy: latest
      argocd-image-updater.argoproj.io/frontend.update-strategy: latest
      argocd-image-updater.argoproj.io/backend.force-update: "true"
      argocd-image-updater.argoproj.io/frontend.force-update: "true"
      argocd-image-updater.argoproj.io/backend.helm.image-name: backend.image.repository
      argocd-image-updater.argoproj.io/backend.helm.image-tag: backend.image.tag
      argocd-image-updater.argoproj.io/frontend.helm.image-name: frontend.image.repository
      argocd-image-updater.argoproj.io/frontend.helm.image-tag: frontend.image.tag
      argocd-image-updater.argoproj.io/backend.pull-secret: pullsecret:auth-service/ghcr-pull-secret
      argocd-image-updater.argoproj.io/frontend.pull-secret: pullsecret:auth-service/ghcr-pull-secret
      argocd-image-updater.argoproj.io/write-back-method: argocd
```

## Applying the Configuration

### Step 1: Ensure ArgoCD Image Updater is Installed

Check if ArgoCD Image Updater is running in your cluster:

```bash
kubectl get pods -n argocd | grep image-updater
```

If not installed, install it:

```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-image-updater/stable/manifests/install.yaml
```

### Step 2: Configure Registry Access

Ensure the image updater can access ghcr.io (it should use the existing pull secret):

```bash
# Verify the pull secret exists
kubectl get secret ghcr-pull-secret -n auth-service
```

### Step 3: Update ArgoCD Application

Update the `k3s-infra` repository with the annotations shown above, then commit and push:

```bash
cd /path/to/k3s-infra
# Edit clusters/main/apps/app-of-apps.yaml
git add clusters/main/apps/app-of-apps.yaml
git commit -m "feat: enable ArgoCD Image Updater for auth-service"
git push origin main
```

### Step 4: Verify Image Updater is Working

Check the image updater logs:

```bash
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-image-updater -f
```

You should see logs indicating it's scanning the registry and updating images.

### Step 5: Trigger a Deployment

Push a change to the auth-service repository to trigger the GitHub Actions workflows. The image updater should detect the new image within a few minutes and update the deployment automatically.

## Monitoring

### Check Application Status

```bash
# Via kubectl
kubectl get application auth-service -n argocd -o yaml

# Via ArgoCD UI
# Visit https://argocd.vibeoholic.com
# Look for the auth-service application
```

### Check Image Updater Status

View annotations added by the image updater:

```bash
kubectl get application auth-service -n argocd -o jsonpath='{.metadata.annotations}'
```

Look for annotations like:
- `argocd-image-updater.argoproj.io/backend.image-name` - The latest detected image
- `argocd-image-updater.argoproj.io/write-back-status` - Update status

### Troubleshooting

**Images not updating:**
1. Check image updater logs for errors
2. Verify the annotations are correctly applied to the Application
3. Ensure the pull secret has access to ghcr.io
4. Verify images are actually being pushed to the registry

**Rate limiting issues:**
If you hit GitHub Container Registry rate limits, you may need to configure registry credentials:

```bash
kubectl create secret generic argocd-image-updater-secret \
  -n argocd \
  --from-literal=registries.conf="
registries:
- name: GitHub Container Registry
  api_url: https://ghcr.io
  prefix: ghcr.io
  credentials: pullsecret:auth-service/ghcr-pull-secret
"
```

## Benefits of This Approach

1. **No Repository Commits**: GitHub Actions no longer needs write access to push values.yaml changes
2. **Simpler Workflows**: Workflows only build and push images
3. **Faster Updates**: Image detection happens automatically without git operations
4. **Better GitOps**: True GitOps pattern where git remains the source of truth for configuration, not image tags
5. **Scalable**: Easy to add more images or applications without modifying workflows

## Update Strategies

The current configuration uses the `latest` tag strategy with force updates. Alternative strategies:

### Semver Strategy
Use semantic versioning for more controlled releases:

```yaml
argocd-image-updater.argoproj.io/backend.update-strategy: semver
argocd-image-updater.argoproj.io/backend.allow-tags: regexp:^v[0-9]+\.[0-9]+\.[0-9]+$
```

### Digest Strategy
Pin to specific image digests for maximum reproducibility:

```yaml
argocd-image-updater.argoproj.io/backend.update-strategy: digest
```

## References

- [ArgoCD Image Updater Documentation](https://argocd-image-updater.readthedocs.io/)
- [GitHub Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
