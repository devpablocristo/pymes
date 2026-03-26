/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_DEV_WOWDASH_OPEN?: string;
}
/// <reference types="vitest/globals" />
/// <reference types="@testing-library/jest-dom" />

declare module '#wowdash/App' {
  import type { FC } from 'react';
  const App: FC;
  export default App;
}

declare module 'bootstrap/dist/js/bootstrap.bundle.min.js';
