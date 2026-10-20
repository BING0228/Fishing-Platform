# Localization

English (`en`) is the canonical source language. Every other locale is a
translation, and English is the final fallback for a missing key.

## Adding or changing copy

1. Add the key to `messages.json` under `en` first. Keep the English short,
   specific, and free of unexplained jargon.
2. Add the same key to every locale in `messages.json`.
3. Use `t("section.key")` in components. Do not branch on a locale code for
   copy or date formats.
4. Use `dateLocale` from `useI18n()` for `toLocaleString`, `toLocaleDateString`,
   and number formatting. For date-fns calendars, map through
   `DATE_FNS_LOCALES` in `AuditLogsPage.tsx`.

## Adding a locale

1. Add an entry to `LOCALES` in `locales.ts` (`nativeName`, `englishName`,
   `htmlLang`, `dateLocale`).
2. Add the dictionary to `messages.json`.
3. Add the date-fns locale where a calendar is rendered.

`I18nProvider` detects `localStorage` first, then `navigator.languages`, and
falls back to English. It sets `<html lang>` and persists explicit choices.
