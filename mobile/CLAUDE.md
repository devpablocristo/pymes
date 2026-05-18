# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
npx expo start          # Start dev server (Expo Go)
npx expo start --ios    # Open in iOS simulator
npx expo start --android
npx expo lint           # Run ESLint
```

No test suite is configured yet.

## Folder Structure

```
mobile/
├── app/                        # Expo Router file-based routes
│   ├── _layout.tsx             # Root layout — NavThemeProvider
│   ├── index.tsx               # Entry point — redirects based on auth
│   ├── (auth)/                 # Unauthenticated stack
│   │   ├── _layout.tsx
│   │   └── login.tsx
│   ├── (tabs)/                 # Authenticated stack (tab navigator)
│   │   ├── _layout.tsx
│   │   ├── index.tsx
│   │   └── explore.tsx
│   └── modal.tsx
├── components/
│   └── ui/                     # Design System (DS) components
│       ├── button.tsx          # DSButton
│       ├── layout.tsx          # DSLayout (background + padding + optional scroll)
│       ├── stack.tsx           # DSVStack / DSHStack
│       ├── text.tsx            # DSText (variant + color)
│       └── text-input.tsx      # DSTextInput
├── constants/
│   └── theme.ts                # Colors, Spacing, FontSize, FontWeight, Fonts
│   └── translations.ts         # All user-facing strings (Spanish only)
├── hooks/
│   ├── use-color-scheme.ts     # Reads from useThemeStore (not OS)
│   ├── use-color-scheme.web.ts # Web variant
│   └── use-theme-color.ts
├── services/                   # API layer
│   ├── api.ts                  # Base axios client (BASE_URL, auth header, error handling)
│   ├── types.ts                # Shared API types (SessionAuth, UserProfile, etc.)
│   ├── auth-service.ts         # POST /v1/auth/login — stores JWT via token.ts
│   ├── token.ts                # JWT helpers: get/set/clear via expo-secure-store
│   ├── session-service.ts      # GET /v1/session
│   └── user-service.ts         # GET /v1/users/me, PATCH /v1/users/me/profile
└── stores/                     # Zustand global state
    ├── auth-store.ts           # isAuthenticated, isInitializing, initialize, login, logout
    ├── session-store.ts        # session (SessionAuth), hydrate, clear
    └── theme-store.ts          # colorScheme (default: 'dark'), setColorScheme, toggleColorScheme
```

## Architecture

**Expo SDK 54, React Native 0.81, Expo Router v6, New Architecture enabled, React Compiler enabled.**

### State management — Zustand

Global state lives in `stores/`. No React Context or Providers needed — stores are singletons.

```ts
// Reading state (granular subscription)
const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
const colorScheme = useThemeStore((state) => state.colorScheme);

// Actions
const login = useAuthStore((state) => state.login);       // (email, password) => Promise<void>
const logout = useAuthStore((state) => state.logout);
const toggleColorScheme = useThemeStore((state) => state.toggleColorScheme);
```

New stores go in `stores/<feature>-store.ts`.

### Auth flow

```
app/_layout.tsx  →  useAuthStore.initialize()  →  checks JWT in SecureStore
                                                  ↓
                                     isAuthenticated ? (tabs) : (auth)
```

- `_layout.tsx` observa `isSignedIn` de Clerk via `useEffect` y llama `router.replace('/(drawer)')` o `router.replace('/(auth)/login')` según el estado.
- El Stack en `_layout.tsx` incluye todas las rutas (`(drawer)`, `(auth)`, `modal`). El redirect maneja la navegación.
- **Token storage**: `services/token.ts` expone los helpers de JWT registrados desde Clerk.

### Theming

The theme is user-controlled (not OS-driven). Default is `'dark'`. `useColorScheme()` reads from `useThemeStore`, not from the device.

```ts
const colorScheme = useColorScheme(); // 'light' | 'dark' — never null
const colors = Colors[colorScheme];
```

All color/spacing/font tokens are in `constants/theme.ts`.

### Design System components

All DS components are prefixed with `DS`. They handle theming internally — consumers never need to pass colors manually.

| Component | Description |
|---|---|
| `DSText` | `variant`: title / label / paragraph — `color`: default / muted |
| `DSTextInput` | Themed input with border, background, placeholder color |
| `DSButton` | Pressable with loading state, uses `colors.tint` |
| `DSVStack` | Vertical flex — `gap` accepts spacing token (`'md'`, `'lg'`, etc.) |
| `DSHStack` | Horizontal flex — same API as DSVStack |
| `DSLayout` | Root screen wrapper — background color + `paddingHorizontal: Spacing.lg`. Props: `scrollable`, `disablePadding`, `contentStyle` |

### Path aliases

`@/*` maps to the root. Always use aliases — never relative imports.

### Platform-specific files

Use `.ios.tsx` / `.android.tsx` suffixes for platform variants. Example: `components/ui/icon-symbol.ios.tsx`.

## Conventions

- **Styles**: Always use `StyleSheet.create`. Never inline style objects in JSX. Dynamic values merged via arrays: `[styles.foo, { color: colors.text }]`.
- **Colors**: Never hardcode hex literals. Always use `Colors[colorScheme].<token>` from `constants/theme.ts`. Never pass raw strings like `'#FF0000'` or `'red'` in JSX or StyleSheet.
- **Text color**: Use `DSText` `color` prop (`color="primary"`, `color="muted"`, etc.) instead of inline `{ color: colors.xxx }` styles.
- **Font sizes**: Use `FontSize` tokens (`FontSize.md`). Never hardcode numbers.
- **Font weights**: Use `FontWeight` tokens (`FontWeight.semibold`). Never hardcode strings.
- **Spacing**: Use `Spacing` tokens (`Spacing.lg`). Never hardcode numbers. Pass as string token to DS components (`gap="md"`).
- **Icon sizes**: Use `IconSize` string tokens (`size="md"`) on `IconSymbol` and `DSIconButton`. Never hardcode numbers.
- **Text strings**: All user-facing text must come from `constants/translations.ts`. Never hardcode strings in JSX.
- **Icons**: `IconSymbol` with SF Symbol names on iOS.
- **Safe area**: `DSLayout scrollable` or `<ScrollView contentInsetAdjustmentBehavior="automatic" />`. Never `SafeAreaView`.
- **No**: inline styles, hardcoded colors/sizes/strings, literal hex values, `Platform.OS`, `SafeAreaView` from RN, `expo-av`, `AsyncStorage` from RN, `expo-permissions`, React Context for global state (use Zustand).
