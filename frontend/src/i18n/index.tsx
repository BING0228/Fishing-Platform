"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import messages from "./messages.json";
import {
  DEFAULT_LOCALE,
  getLocale,
  isLocale,
  resolveLocale,
  type LocaleCode,
} from "./locales";

type Dict = Record<string, unknown>;

const STORAGE_KEY = "spearhook-locale";

function getByPath(obj: Dict, path: string): unknown {
  return path.split(".").reduce<unknown>((acc, key) => {
    if (acc && typeof acc === "object" && key in (acc as Dict)) {
      return (acc as Dict)[key];
    }
    return undefined;
  }, obj);
}

function interpolate(template: string, vars?: Record<string, string | number>) {
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (_, key: string) =>
    vars[key] !== undefined ? String(vars[key]) : `{${key}}`,
  );
}

function readStoredLocale(): string | null {
  try {
    return window.localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

function detectLocale(): LocaleCode {
  const stored = readStoredLocale();
  if (stored) return resolveLocale([stored]);
  const languages =
    Array.isArray(navigator.languages) && navigator.languages.length > 0
      ? navigator.languages
      : [navigator.language];
  return resolveLocale(languages);
}

type I18nContextValue = {
  locale: LocaleCode;
  setLocale: (locale: LocaleCode) => void;
  t: (key: string, vars?: Record<string, string | number>) => string;
  dateLocale: string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  // Render English first so SSR, static export, and the first client paint are
  // deterministic. The effect below then applies the stored/browser locale.
  const [locale, setLocaleState] = useState<LocaleCode>(DEFAULT_LOCALE);

  useEffect(() => {
    setLocaleState(detectLocale());
  }, []);

  const setLocale = useCallback((next: LocaleCode) => {
    if (!isLocale(next)) return;
    setLocaleState(next);
    try {
      window.localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Private browsing / storage-disabled contexts still switch for this tab.
    }
  }, []);

  const meta = getLocale(locale);

  useEffect(() => {
    document.documentElement.lang = meta.htmlLang;
    document.documentElement.dir = "ltr";
  }, [meta.htmlLang]);

  const t = useCallback(
    (key: string, vars?: Record<string, string | number>) => {
      const table = (messages as Record<string, Dict>)[locale];
      const value = table ? getByPath(table, key) : undefined;
      if (typeof value === "string") return interpolate(value, vars);
      // English is the canonical source and final fallback.
      const fallback = getByPath(messages.en as Dict, key);
      if (typeof fallback === "string") return interpolate(fallback, vars);
      return key;
    },
    [locale],
  );

  const value = useMemo(
    () => ({ locale, setLocale, t, dateLocale: meta.dateLocale }),
    [locale, setLocale, t, meta.dateLocale],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used within I18nProvider");
  return ctx;
}
