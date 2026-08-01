/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string;
  readonly VITE_CLERK_PUBLISHABLE_KEY?: string;
  readonly VITE_ALLOW_INSECURE_LOCAL_AUTH?: string;
  readonly VITE_USE_FAKE_API?: string;
  readonly VITE_PYMES_ORGANIZATION_ID?: string;
  readonly VITE_PYMES_ORGANIZATION_SLUG?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
