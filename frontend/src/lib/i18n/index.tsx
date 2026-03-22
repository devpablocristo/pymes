import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from 'react';
import { createBrowserStorageNamespace } from '@devpablocristo/core-browser/storage';
import { vocab } from '../vocabulary';
import { apiKeysMessages } from './messages/apiKeys';
import { authMessages } from './messages/auth';
import { billingMessages } from './messages/billing';
import { calendarMessages } from './messages/calendar';
import { commonMessages } from './messages/common';
import { crudMessages } from './messages/crud';
import { dashboardMessages } from './messages/dashboard';
import { moduleMessages } from './messages/module';
import { profileMessages } from './messages/profile';
import { shellMessages } from './messages/shell';
import { translateLegacyText } from './legacy';
import type { FlatMessages, LanguageCode, TranslationVariables, TranslationsByLanguage } from './types';

const STORAGE_KEY = 'pymes:language';
const DEFAULT_LANGUAGE: LanguageCode = 'es';
const storage = createBrowserStorageNamespace({ namespace: 'pymes-ui', hostAware: false });

const supportedLanguages = [
  { code: 'es' as const, labelKey: 'common.language.es' },
  { code: 'en' as const, labelKey: 'common.language.en' },
];

function mergeMessages(...sources: TranslationsByLanguage[]): Record<LanguageCode, FlatMessages> {
  return {
    es: Object.assign({}, ...sources.map((source) => source.es)),
    en: Object.assign({}, ...sources.map((source) => source.en)),
  };
}

const messages = mergeMessages(
  commonMessages,
  shellMessages,
  crudMessages,
  moduleMessages,
  profileMessages,
  billingMessages,
  authMessages,
  calendarMessages,
  apiKeysMessages,
  dashboardMessages,
);

function hasLettersOrDigits(token: string): boolean {
  return /[\p{L}\p{N}]/u.test(token);
}

function isUppercaseAcronym(token: string): boolean {
  const alphanumeric = token.replace(/[^\p{L}\p{N}]/gu, '');
  return alphanumeric.length >= 2 && /^[A-Z0-9]+$/u.test(alphanumeric);
}

function capitalizeFirstLetter(token: string): string {
  return token.replace(/\p{L}/u, (char) => char.toLocaleUpperCase());
}

export function toSentenceCase(text: string): string {
  let seenFirstWord = false;

  return text
    .split(/(\s+)/)
    .map((token) => {
      if (/^\s+$/u.test(token) || !hasLettersOrDigits(token)) {
        return token;
      }

      if (isUppercaseAcronym(token)) {
        seenFirstWord = true;
        return token;
      }

      const normalized = token.toLocaleLowerCase();
      if (!seenFirstWord) {
        seenFirstWord = true;
        return capitalizeFirstLetter(normalized);
      }

      return normalized;
    })
    .join('');
}

function interpolate(template: string, variables?: TranslationVariables): string {
  if (!variables) {
    return template;
  }
  return template.replace(/\{\{(\w+)\}\}/g, (_match, key: string) => String(variables[key] ?? ''));
}

function getMessage(language: LanguageCode, key: string, variables?: TranslationVariables): string {
  const template = messages[language][key] ?? messages[DEFAULT_LANGUAGE][key] ?? key;
  return interpolate(template, variables);
}

function readStoredLanguage(): LanguageCode {
  if (typeof window === 'undefined') {
    return DEFAULT_LANGUAGE;
  }
  const stored = storage.getString(STORAGE_KEY);
  return stored === 'en' || stored === 'es' ? stored : DEFAULT_LANGUAGE;
}

function applyDocumentLanguage(language: LanguageCode): void {
  document.documentElement.lang = language;
}

type I18nContextValue = {
  language: LanguageCode;
  setLanguage: (language: LanguageCode) => void;
  t: (key: string, variables?: TranslationVariables) => string;
  localizeText: (text: string) => string;
  sentenceCase: (text: string) => string;
  localizeUiText: (text: string) => string;
  options: typeof supportedLanguages;
};

const defaultContext: I18nContextValue = {
  language: DEFAULT_LANGUAGE,
  setLanguage: () => undefined,
  t: (key, variables) => getMessage(DEFAULT_LANGUAGE, key, variables),
  localizeText: (text) => translateLegacyText(vocab(text), DEFAULT_LANGUAGE),
  sentenceCase: toSentenceCase,
  localizeUiText: (text) => toSentenceCase(translateLegacyText(vocab(text), DEFAULT_LANGUAGE)),
  options: supportedLanguages,
};

const I18nContext = createContext<I18nContextValue>(defaultContext);

export function LanguageProvider({
  children,
  initialLanguage,
}: PropsWithChildren<{ initialLanguage?: LanguageCode }>) {
  const [language, setLanguageState] = useState<LanguageCode>(() => initialLanguage ?? readStoredLanguage());

  useEffect(() => {
    storage.setString(STORAGE_KEY, language);
    applyDocumentLanguage(language);
  }, [language]);

  const value = useMemo<I18nContextValue>(() => ({
    language,
    setLanguage: setLanguageState,
    t: (key, variables) => getMessage(language, key, variables),
    localizeText: (text) => translateLegacyText(vocab(text), language),
    sentenceCase: toSentenceCase,
    localizeUiText: (text) => toSentenceCase(translateLegacyText(vocab(text), language)),
    options: supportedLanguages,
  }), [language]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  return useContext(I18nContext);
}

export type { LanguageCode };
