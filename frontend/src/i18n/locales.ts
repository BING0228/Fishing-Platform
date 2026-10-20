export type LocaleCode = "en" | "zh-CN" | "zh-TW" | "ja" | "ko";

export type LocaleDefinition = {
  code: LocaleCode;
  /** Name shown in the language picker, written in that language. */
  nativeName: string;
  /** English name, useful for screen readers and logs. */
  englishName: string;
  htmlLang: string;
  dateLocale: string;
};

/**
 * English is the source language and the final fallback for every other
 * locale. Add a language here and add its dictionary to messages.json.
 */
export const LOCALES: readonly LocaleDefinition[] = [
  {
    code: "en",
    nativeName: "English",
    englishName: "English",
    htmlLang: "en",
    dateLocale: "en-US",
  },
  {
    code: "zh-CN",
    nativeName: "简体中文",
    englishName: "Simplified Chinese",
    htmlLang: "zh-CN",
    dateLocale: "zh-CN",
  },
  {
    code: "zh-TW",
    nativeName: "繁體中文",
    englishName: "Traditional Chinese",
    htmlLang: "zh-TW",
    dateLocale: "zh-TW",
  },
  {
    code: "ja",
    nativeName: "日本語",
    englishName: "Japanese",
    htmlLang: "ja",
    dateLocale: "ja-JP",
  },
  {
    code: "ko",
    nativeName: "한국어",
    englishName: "Korean",
    htmlLang: "ko",
    dateLocale: "ko-KR",
  },
] as const;

export const DEFAULT_LOCALE: LocaleCode = "en";

export function isLocale(value: unknown): value is LocaleCode {
  return LOCALES.some((locale) => locale.code === value);
}

export function getLocale(code: LocaleCode): LocaleDefinition {
  return LOCALES.find((locale) => locale.code === code) ?? LOCALES[0];
}

/**
 * Map a BCP 47 tag or browser preference to one of the supported locales.
 * English is returned when nothing matches.
 */
export function resolveLocale(candidates: readonly string[]): LocaleCode {
  for (const candidate of candidates) {
    const tag = candidate.trim().replace(/_/g, "-").toLowerCase();
    if (!tag) continue;

    if (
      tag === "zh-tw" ||
      tag === "zh-hk" ||
      tag === "zh-mo" ||
      tag.startsWith("zh-hant")
    ) {
      return "zh-TW";
    }
    if (
      tag === "zh" ||
      tag === "zh-cn" ||
      tag === "zh-sg" ||
      tag.startsWith("zh-hans")
    ) {
      return "zh-CN";
    }
    if (tag === "ja" || tag.startsWith("ja-")) return "ja";
    if (tag === "ko" || tag.startsWith("ko-")) return "ko";
    if (tag === "en" || tag.startsWith("en-")) return "en";
  }
  return DEFAULT_LOCALE;
}
