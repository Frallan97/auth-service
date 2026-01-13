# Google OAuth Setup Guide - Fix "OAuth client was not found" Error

## Current Problem

**Error:** `Access blocked: Authorization Error - The OAuth client was not found. Error 401: invalid_client`

**Root Cause:** The auth-service is configured with placeholder OAuth credentials:
```
GOOGLE_CLIENT_ID: placeholder-google-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET: (placeholder)
```

**Required Redirect URI:** `https://auth.vibeoholic.com/api/auth/google/callback`

---

## Solution: Configure Real Google OAuth Credentials

### Option 1: Use Existing OAuth Client (If You Have One)

If you already have an OAuth client ID for auth-service in Google Cloud Console:

#### Step 1: Find Your OAuth Client

```bash
# Open Google Cloud Console OAuth page
open "https://console.cloud.google.com/apis/credentials?project=auth-service-481021"

# Or check other projects:
# oauth-project-454218
# sharon-prod-1766061524
# hackaton-vittsjo
```

#### Step 2: Update the Redirect URI

1. Click on your OAuth client in the list
2. Under **"Authorized redirect URIs"**, add:
   ```
   https://auth.vibeoholic.com/api/auth/google/callback
   ```
3. Click **Save**

#### Step 3: Get the Credentials

1. Copy the **Client ID** (looks like: `xxxxx.apps.googleusercontent.com`)
2. Copy the **Client Secret**

#### Step 4: Update Kubernetes Secret

```bash
# Update the secret with real credentials
ssh root@37.27.40.86 "kubectl create secret generic auth-service-backend \
  --from-literal=GOOGLE_CLIENT_ID='YOUR_CLIENT_ID_HERE.apps.googleusercontent.com' \
  --from-literal=GOOGLE_CLIENT_SECRET='YOUR_CLIENT_SECRET_HERE' \
  --dry-run=client -o yaml | kubectl apply -f - -n auth-service"

# Restart backend to pick up new credentials
ssh root@37.27.40.86 "kubectl rollout restart deployment/auth-service-backend -n auth-service"
```

---

### Option 2: Create New OAuth Client (If You Don't Have One)

If you need to create a new OAuth client from scratch:

#### Step 1: Open Google Cloud Console

```bash
# Set the correct project
gcloud config set project auth-service-481021

# Open credentials page
open "https://console.cloud.google.com/apis/credentials?project=auth-service-481021"
```

Or visit: https://console.cloud.google.com/apis/credentials

#### Step 2: Configure OAuth Consent Screen (If Not Done)

1. Click **"OAuth consent screen"** in the left menu
2. Choose **"External"** (unless you have a Google Workspace)
3. Fill in required fields:
   - **App name:** Auth Service
   - **User support email:** Your email
   - **Developer contact:** Your email
4. Click **Save and Continue**
5. **Scopes:** Skip for now (default scopes are fine)
6. **Test users:** Add your email (franssjos@gmail.com)
7. Click **Save and Continue**

#### Step 3: Create OAuth Client ID

1. Go back to **"Credentials"**
2. Click **"+ CREATE CREDENTIALS"**
3. Select **"OAuth client ID"**
4. **Application type:** Web application
5. **Name:** auth-service-production
6. Under **"Authorized JavaScript origins"**, add:
   ```
   https://auth.vibeoholic.com
   ```
7. Under **"Authorized redirect URIs"**, add:
   ```
   https://auth.vibeoholic.com/api/auth/google/callback
   ```
8. Click **Create**

#### Step 4: Copy Credentials

A popup will show your credentials:
- **Client ID:** Copy this (e.g., `123456789-abc.apps.googleusercontent.com`)
- **Client Secret:** Copy this (e.g., `GOCSPX-xxxxxxxxxxxxx`)

**Important:** Save these somewhere secure!

#### Step 5: Update Kubernetes Secret

```bash
# Replace with your actual credentials
export GOOGLE_CLIENT_ID="YOUR_CLIENT_ID_HERE.apps.googleusercontent.com"
export GOOGLE_CLIENT_SECRET="YOUR_CLIENT_SECRET_HERE"

# Update the secret
ssh root@37.27.40.86 "kubectl create secret generic auth-service-backend \
  --from-literal=GOOGLE_CLIENT_ID='$GOOGLE_CLIENT_ID' \
  --from-literal=GOOGLE_CLIENT_SECRET='$GOOGLE_CLIENT_SECRET' \
  --dry-run=client -o yaml | kubectl apply -f - -n auth-service"

# Restart backend to pick up new credentials
ssh root@37.27.40.86 "kubectl rollout restart deployment/auth-service-backend -n auth-service"
```

---

## Verification Steps

### 1. Check Secret Was Updated

```bash
ssh root@37.27.40.86 "kubectl get secret -n auth-service auth-service-backend -o jsonpath='{.data.GOOGLE_CLIENT_ID}' | base64 -d"
```

**Expected:** Should show your real client ID (not "placeholder-google-client-id")

### 2. Wait for Backend to Restart

```bash
ssh root@37.27.40.86 "kubectl get pods -n auth-service -l app.kubernetes.io/component=backend -w"
```

Wait until the new pod is **Running** and **Ready: 1/1**

### 3. Test OAuth Flow

1. Open https://auth.vibeoholic.com (clear cache if needed)
2. Click **"Sign in with Google"**
3. Should redirect to Google login page (no error!)
4. Sign in with your Google account
5. You'll see a consent screen (first time only)
6. Click **"Allow"**
7. Should redirect back to https://auth.vibeoholic.com/dashboard
8. You should be logged in!

---

## Common Issues & Solutions

### Issue 1: "This app isn't verified"

**Cause:** Your app is in testing mode with external users

**Solution:**
- Click **"Advanced"** → **"Go to Auth Service (unsafe)"**
- OR publish your app (if you want public access)
- OR add test users to your OAuth consent screen

### Issue 2: "Redirect URI mismatch"

**Error:** `redirect_uri_mismatch`

**Cause:** The redirect URI in Google Cloud Console doesn't match

**Solution:**
1. Go to Google Cloud Console
2. Edit your OAuth client
3. Make sure you have EXACTLY: `https://auth.vibeoholic.com/api/auth/google/callback`
4. No trailing slash, no extra parameters

### Issue 3: Still seeing "OAuth client was not found"

**Possible causes:**
- Wrong project selected in Google Cloud Console
- Client was deleted
- Using Client ID from wrong project

**Solution:**
- Double-check the project: `auth-service-481021`
- Verify the Client ID exists in that project
- Make sure you copied the full Client ID including `.apps.googleusercontent.com`

### Issue 4: Secret not updating in pod

**Cause:** Kubernetes doesn't automatically restart pods when secrets change

**Solution:**
```bash
# Force restart
ssh root@37.27.40.86 "kubectl rollout restart deployment/auth-service-backend -n auth-service"

# Verify new pod has new credentials
ssh root@37.27.40.86 "kubectl exec -n auth-service deployment/auth-service-backend -- sh -c 'echo \$GOOGLE_CLIENT_ID'"
```

---

## Quick Command Reference

```bash
# Check current credentials (returns placeholder)
ssh root@37.27.40.86 "kubectl get secret -n auth-service auth-service-backend -o jsonpath='{.data.GOOGLE_CLIENT_ID}' | base64 -d"

# Update credentials
ssh root@37.27.40.86 "kubectl create secret generic auth-service-backend \
  --from-literal=GOOGLE_CLIENT_ID='YOUR_ID_HERE' \
  --from-literal=GOOGLE_CLIENT_SECRET='YOUR_SECRET_HERE' \
  --dry-run=client -o yaml | kubectl apply -f - -n auth-service"

# Restart backend
ssh root@37.27.40.86 "kubectl rollout restart deployment/auth-service-backend -n auth-service"

# Watch pods restart
ssh root@37.27.40.86 "kubectl get pods -n auth-service -w"

# Check logs
ssh root@37.27.40.86 "kubectl logs -n auth-service -l app.kubernetes.io/component=backend -f"
```

---

## Security Best Practices

### ✅ DO:
- Use a separate OAuth client for production vs development
- Keep client secrets in Kubernetes secrets (never commit to git)
- Limit authorized redirect URIs to only what you need
- Add only necessary test users during development
- Regularly rotate client secrets

### ❌ DON'T:
- Share client secrets publicly
- Use the same OAuth client across multiple environments
- Add wildcard redirect URIs
- Leave your app in testing mode forever (if it's public)

---

## What's in the Kubernetes Secret?

The secret should contain:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: auth-service-backend
  namespace: auth-service
type: Opaque
data:
  GOOGLE_CLIENT_ID: <base64-encoded-client-id>
  GOOGLE_CLIENT_SECRET: <base64-encoded-client-secret>
```

The backend deployment reads these as environment variables.

---

## Alternative: Use Google Cloud Secret Manager (Advanced)

Instead of Kubernetes secrets, you could use Google Secret Manager:

```bash
# Store secret in Google Secret Manager
echo -n "YOUR_CLIENT_SECRET" | gcloud secrets create auth-service-google-client-secret \
  --data-file=- \
  --project=auth-service-481021

# Use Workload Identity to access from Kubernetes
# (Requires additional setup)
```

This is more secure but requires Workload Identity configuration.

---

## Next Steps After OAuth Works

Once OAuth is working:

1. ✅ Test login/logout flow thoroughly
2. ✅ Test with multiple users
3. 📝 Document your OAuth setup for team
4. 🔒 Store backup of Client ID/Secret securely
5. 📊 Monitor auth metrics in Google Cloud Console
6. 🚀 Begin Phase 1 of multi-solution features (see MULTI_SOLUTION_PLAN.md)

---

## Need Help?

If you're stuck:

1. Check the error in browser console (F12)
2. Check backend logs: `kubectl logs -n auth-service -l app.kubernetes.io/component=backend`
3. Verify redirect URI matches EXACTLY (including https://)
4. Make sure you're in the right Google Cloud project

**Google OAuth Documentation:**
- https://developers.google.com/identity/protocols/oauth2
- https://console.cloud.google.com/apis/credentials
