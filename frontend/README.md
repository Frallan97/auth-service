# Auth Service Frontend

React frontend for the authentication service with Google OAuth integration.

## Prerequisites

- Bun 1.0+
- Node.js 18+ (alternative to Bun)

## Setup

### 1. Install Dependencies

```bash
bun install
```

### 2. Configure Environment

Copy `.env.example` to `.env`:

```bash
cp .env.example .env
```

Update the `VITE_API_URL` to point to your backend:

```env
VITE_API_URL=http://localhost:8080
```

### 3. Run Development Server

```bash
bun dev
```

The app will be available at `http://localhost:5173`

## Build for Production

```bash
bun run build
```

The production build will be in the `dist/` directory.

## Preview Production Build

```bash
bun run preview
```

## Tech Stack

- **React 18+** - UI framework
- **TypeScript** - Type safety
- **Bun** - Fast JavaScript runtime and package manager
- **Vite** - Build tool
- **Tailwind CSS** - Styling
- **React Router** - Routing
- **Axios** - HTTP client

## Features

- Google OAuth authentication
- JWT token management
- Automatic token refresh
- Protected routes
- User dashboard
- Admin user management
- Responsive design
- Dark mode support

## Project Structure

```
frontend/
├── src/
│   ├── components/
│   │   ├── Auth/
│   │   │   └── ProtectedRoute.tsx
│   │   ├── Layout/
│   │   │   ├── Header.tsx
│   │   │   └── Layout.tsx
│   │   └── UI/
│   ├── contexts/
│   │   └── AuthContext.tsx
│   ├── pages/
│   │   ├── Admin/
│   │   │   └── Users.tsx
│   │   ├── AuthCallback.tsx
│   │   ├── Dashboard.tsx
│   │   └── Login.tsx
│   ├── services/
│   │   └── api.ts
│   ├── App.tsx
│   ├── main.tsx
│   └── index.css
├── index.html
├── tailwind.config.js
├── vite.config.ts
└── package.json
```

## Authentication Flow

1. User clicks "Sign in with Google"
2. Redirected to backend `/api/auth/google/login`
3. Backend redirects to Google OAuth consent
4. User approves
5. Google redirects to backend `/api/auth/google/callback`
6. Backend generates JWT and redirects to `/auth/callback?access_token=...`
7. Frontend extracts token and stores in sessionStorage
8. Frontend redirects to dashboard

## Token Management

- **Access Token**: Stored in sessionStorage, short-lived (15 min)
- **Refresh Token**: HTTP-only cookie, long-lived (7 days)
- Automatic refresh on 401 responses
- Cleared on logout

## Available Routes

- `/login` - Login page
- `/dashboard` - User dashboard (protected)
- `/admin/users` - User management (protected)
- `/auth/callback` - OAuth callback handler

## API Integration

The frontend communicates with the backend through the API client in `src/services/api.ts`.

All API calls automatically:
- Include JWT token in Authorization header
- Handle token refresh on 401 errors
- Redirect to login on auth failures

## Development Tips

- Use the browser DevTools to inspect JWT tokens
- Check sessionStorage for the access_token
- Monitor Network tab for API calls
- Refresh tokens are handled automatically

## Deployment

### Static Hosting (Netlify, Vercel, etc.)

1. Build the project: `bun run build`
2. Deploy the `dist/` directory
3. Configure environment variables in hosting platform
4. Set up redirects for SPA routing

### Docker

```dockerfile
FROM node:18-alpine as build
WORKDIR /app
COPY package.json bun.lockb ./
RUN npm install -g bun && bun install
COPY . .
RUN bun run build

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

## Troubleshooting

### CORS Errors
- Ensure backend ALLOWED_ORIGINS includes frontend URL
- Check that credentials are being sent with requests

### Token Not Refreshing
- Verify refresh token cookie is being set by backend
- Check cookie SameSite and Secure settings

### OAuth Redirect Issues
- Ensure Google OAuth redirect URL matches backend callback
- Verify GOOGLE_CLIENT_ID is correct in backend
