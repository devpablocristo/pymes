# Pymes App

Mobile app for the Pymes platform, built with Expo + React Native.

## Stack

- **Expo SDK 54** + React Native 0.81
- **Expo Router v6** — file-based navigation
- **Clerk** — authentication (sign-in/sign-up, JWT)
- **Zustand** — global state management
- **TypeScript** — strict mode enabled
- New Architecture + React Compiler enabled

## Requirements

- Node.js 18+
- Expo Go (simulator) or Xcode/Android Studio (native builds)
- A [Clerk](https://clerk.com) account with an app configured
- Pymes backend running (Docker or published URL)

## Setup

```bash
# 1. Clone and install dependencies
git clone git@github.com:santiagorobra/pymes-app.git
cd pymes-app
npm install

# 2. Create the environment file
cp .env.example .env
```

Fill in `.env` with real values:

```env
EXPO_PUBLIC_API_URL=https://api.pymes.app      # Published backend URL
EXPO_PUBLIC_CLERK_PUBLISHABLE_KEY=pk_test_...  # Clerk Dashboard → API Keys
```

> On a physical device, replace `localhost` with your machine's local IP address.

## Development

```bash
npx expo start          # Expo Go (scan QR)
npx expo start --ios    # iOS simulator
npx expo start --android
npx expo lint           # ESLint
```

## Authentication

Login is handled by **Clerk** directly on the client — there is no login endpoint on the backend. The flow is:

1. User signs in via Clerk (`useSignIn`)
2. Clerk returns a JWT
3. The app calls `GET /v1/session` with the JWT to hydrate global state
4. All backend requests include `Authorization: Bearer <jwt>`

## Project Structure

```
app/          Routes (Expo Router)
components/   Design System (DS*) + general components
constants/    Design tokens (Colors, Spacing, FontSize, FontWeight) + translations
hooks/        Reusable hooks
services/     API layer (api.ts as base client + per-resource services)
stores/       Global state with Zustand (auth, session, theme)
```

See [CLAUDE.md](./CLAUDE.md) for code conventions and detailed architecture.
