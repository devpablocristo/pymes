export type LanguageCode = 'es' | 'en';

export type TranslationVariables = Record<string, string | number>;

export type FlatMessages = Record<string, string>;

export type TranslationsByLanguage = Record<LanguageCode, FlatMessages>;
