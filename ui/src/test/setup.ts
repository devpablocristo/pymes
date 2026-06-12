import { cleanup } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import * as React from 'react';

globalThis.React = React;

afterEach(() => {
  cleanup();
});
