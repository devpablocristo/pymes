import eslint from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import globals from 'globals';
import tseslint from 'typescript-eslint';

/**
 * ESLint flat config (ESLint 9 + typescript-eslint).
 * Sin reglas type-aware para mantener el lint rápido en CI local/Docker.
 */
export default tseslint.config(
  {
    ignores: ['dist/**', 'node_modules/**', 'coverage/**', 'scripts/**', '*.config.js', '*.config.ts'],
  },
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      parserOptions: {
        ecmaFeatures: { jsx: true },
      },
      globals: {
        ...globals.browser,
      },
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // Warn para detectar deps faltantes sin romper el build; fix gradual
      'react-hooks/exhaustive-deps': 'warn',
      // Warn para detectar archivos mixtos; allowConstantExport para configs CRUD/i18n
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
      // Warn para detectar nuevos usos de any; los existentes en CRUD configs están justificados
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': [
        'warn',
        {
          argsIgnorePattern: '^_',
          varsIgnorePattern: '^_',
        },
      ],
    },
  },
);
