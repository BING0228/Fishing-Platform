import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { getLocale, isLocale, resolveLocale } from "./locales";

describe("locale registry", () => {
  it("recognizes supported locale codes", () => {
    assert.equal(isLocale("en"), true);
    assert.equal(isLocale("zh-CN"), true);
    assert.equal(isLocale("zh-TW"), true);
    assert.equal(isLocale("ja"), true);
    assert.equal(isLocale("ko"), true);
    assert.equal(isLocale("fr"), false);
  });

  it("maps browser tags to the closest supported locale", () => {
    assert.equal(resolveLocale(["en-GB"]), "en");
    assert.equal(resolveLocale(["zh"]), "zh-CN");
    assert.equal(resolveLocale(["zh_CN"]), "zh-CN");
    assert.equal(resolveLocale(["zh-Hans-CN"]), "zh-CN");
    assert.equal(resolveLocale(["zh-Hant-HK"]), "zh-TW");
    assert.equal(resolveLocale(["zh-TW"]), "zh-TW");
    assert.equal(resolveLocale(["ja-JP"]), "ja");
    assert.equal(resolveLocale(["ko-KR"]), "ko");
    assert.equal(resolveLocale(["fr-FR", "de-DE"]), "en");
  });

  it("exposes a locale-specific date format", () => {
    assert.equal(getLocale("en").dateLocale, "en-US");
    assert.equal(getLocale("zh-TW").dateLocale, "zh-TW");
    assert.equal(getLocale("ja").dateLocale, "ja-JP");
  });
});
