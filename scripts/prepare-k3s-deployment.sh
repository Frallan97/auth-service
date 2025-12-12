#!/bin/bash

# Script to prepare secrets and configurations for K3s deployment
set -e

echo "🔐 Preparing auth-service for K3s deployment..."

# Check if running from the correct directory
if [ ! -f "charts/auth-service/Chart.yaml" ]; then
    echo "❌ Error: Must run from auth-service root directory"
    exit 1
fi

# Check prerequisites
command -v kubectl >/dev/null 2>&1 || { echo "❌ kubectl is required but not installed."; exit 1; }
command -v kubeseal >/dev/null 2>&1 || { echo "❌ kubeseal is required but not installed."; exit 1; }
command -v openssl >/dev/null 2>&1 || { echo "❌ openssl is required but not installed."; exit 1; }

echo ""
echo "📋 Step 1: Generate JWT RSA Keys"
echo "================================"

if [ -f "backend/keys/private_key.pem" ] && [ -f "backend/keys/public_key.pem" ]; then
    echo "✅ JWT keys already exist"
    read -p "Do you want to regenerate them? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -f backend/keys/*.pem
    fi
fi

if [ ! -f "backend/keys/private_key.pem" ]; then
    echo "🔑 Generating RSA key pair..."
    mkdir -p backend/keys
    openssl genrsa -out backend/keys/private_key.pem 4096 2>/dev/null
    openssl rsa -in backend/keys/private_key.pem -pubout -out backend/keys/public_key.pem 2>/dev/null
    echo "✅ JWT keys generated"
fi

echo ""
echo "📋 Step 2: Collect Configuration"
echo "================================"

# Get Google OAuth credentials
echo ""
read -p "Enter Google OAuth Client ID: " GOOGLE_CLIENT_ID
read -p "Enter Google OAuth Client Secret: " GOOGLE_CLIENT_SECRET
read -p "Enter Admin Email(s) (comma-separated): " ADMIN_EMAILS

# Generate secure PostgreSQL password
POSTGRES_PASSWORD=$(openssl rand -base64 32)
echo "✅ Generated secure PostgreSQL password"

echo ""
echo "📋 Step 3: Create Sealed Secrets"
echo "================================"

NAMESPACE="auth-service"

# Create sealed secret for backend
echo "🔒 Creating sealed secret for backend..."
kubectl create secret generic auth-service-backend \
  --from-literal=GOOGLE_CLIENT_ID="$GOOGLE_CLIENT_ID" \
  --from-literal=GOOGLE_CLIENT_SECRET="$GOOGLE_CLIENT_SECRET" \
  --from-literal=ADMIN_EMAILS="$ADMIN_EMAILS" \
  --namespace=$NAMESPACE \
  --dry-run=client -o yaml | \
  kubeseal --controller-namespace=sealed-secrets --format=yaml > \
  charts/auth-service/templates/sealed-secret-backend.yaml

echo "✅ Backend sealed secret created"

# Create sealed secret for PostgreSQL
echo "🔒 Creating sealed secret for PostgreSQL..."
kubectl create secret generic auth-service-postgres \
  --from-literal=POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
  --namespace=$NAMESPACE \
  --dry-run=client -o yaml | \
  kubeseal --controller-namespace=sealed-secrets --format=yaml > \
  charts/auth-service/templates/sealed-secret-postgres.yaml

echo "✅ PostgreSQL sealed secret created"

# Create sealed secret for JWT keys
echo "🔒 Creating sealed secret for JWT keys..."
kubectl create secret generic auth-service-jwt-keys \
  --from-file=private_key.pem=backend/keys/private_key.pem \
  --from-file=public_key.pem=backend/keys/public_key.pem \
  --namespace=$NAMESPACE \
  --dry-run=client -o yaml | \
  kubeseal --controller-namespace=sealed-secrets --format=yaml > \
  charts/auth-service/templates/sealed-secret-jwt-keys.yaml

echo "✅ JWT keys sealed secret created"

# Remove the plain secret.yaml template
if [ -f "charts/auth-service/templates/secret.yaml" ]; then
    rm charts/auth-service/templates/secret.yaml
    echo "✅ Removed plain secret template"
fi

echo ""
echo "📋 Step 4: Update Values"
echo "========================"

# Update image repository in values.yaml if needed
read -p "Enter your GitHub username (default: frallan97): " GITHUB_USER
GITHUB_USER=${GITHUB_USER:-frallan97}

sed -i "s|ghcr.io/frallan97/|ghcr.io/${GITHUB_USER}/|g" charts/auth-service/values.yaml
echo "✅ Updated image repositories to use: ghcr.io/${GITHUB_USER}/"

echo ""
echo "✨ Preparation Complete!"
echo "======================="
echo ""
echo "Next steps:"
echo "1. Review the sealed secrets in charts/auth-service/templates/"
echo "2. Configure Google OAuth redirect URI: https://auth.w.viboholic.com/api/auth/google/callback"
echo "3. Commit and push changes to your repository"
echo "4. Add auth-service to your k3s-infra app-of-apps.yaml"
echo "5. ArgoCD will automatically deploy the application"
echo ""
echo "For detailed instructions, see K3S_DEPLOYMENT.md"
