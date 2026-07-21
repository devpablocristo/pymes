import { createContext, type PropsWithChildren, useContext, useEffect, useMemo, useState } from "react";

type Locale = "es" | "en";

const copy = {
  es: {
    navigation: "Navegación principal",
    runtime: "Runtime",
    principles: "Principios",
    language: "Idioma",
    darkTheme: "Usar tema oscuro",
    lightTheme: "Usar tema claro",
    title: "Una base limpia para construir.",
    description: "API, datos y web avanzan como componentes independientes sobre contratos publicados.",
    statusLabel: "Componentes del runtime",
    principlesTitle: "Horizontal desde el primer día",
    principlesDescription: "Este shell contiene solamente infraestructura. Identidad y capacidades de negocio llegarán en entregas separadas.",
    foundation: "Runtime técnico",
    noLegacy: "Sin dependencias de v1",
  },
  en: {
    navigation: "Primary navigation",
    runtime: "Runtime",
    principles: "Principles",
    language: "Language",
    darkTheme: "Use dark theme",
    lightTheme: "Use light theme",
    title: "A clean foundation to build on.",
    description: "API, data, and web evolve as independent components over published contracts.",
    statusLabel: "Runtime components",
    principlesTitle: "Horizontal from day one",
    principlesDescription: "This shell contains infrastructure only. Identity and business capabilities will arrive in separate deliveries.",
    foundation: "Technical runtime",
    noLegacy: "No v1 dependencies",
  },
} as const;

type I18nContextValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  text: (typeof copy)[Locale];
};

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: PropsWithChildren) {
  const [locale, setLocale] = useState<Locale>("es");

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const value = useMemo(() => ({ locale, setLocale, text: copy[locale] }), [locale]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const value = useContext(I18nContext);
  if (!value) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return value;
}
