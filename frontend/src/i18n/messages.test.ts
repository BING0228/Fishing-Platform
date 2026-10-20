import assert from "node:assert/strict";
import { describe, it } from "node:test";
import messages from "./messages.json";

type Dict = Record<string, unknown>;

function flatten(value: Dict, prefix = ""): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [key, child] of Object.entries(value)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (child && typeof child === "object" && !Array.isArray(child)) {
      Object.assign(out, flatten(child as Dict, path));
    } else if (typeof child === "string") {
      out[path] = child;
    }
  }
  return out;
}

function placeholders(value: string): string[] {
  return [...value.matchAll(/\{\{[^{}]+\}\}|\{[^{}]+\}/g)]
    .map((match) => match[0])
    .sort();
}

const english = flatten(messages.en as Dict);

describe("message catalog", () => {
  it("keeps every locale key-aligned with English", () => {
    for (const [locale, table] of Object.entries(messages)) {
      const flat = flatten(table as Dict);
      assert.deepEqual(
        Object.keys(flat).sort(),
        Object.keys(english).sort(),
        `${locale} keys differ from English`,
      );
    }
  });

  it("preserves interpolation placeholders in every locale", () => {
    for (const [locale, table] of Object.entries(messages)) {
      const flat = flatten(table as Dict);
      for (const [key, value] of Object.entries(flat)) {
        assert.deepEqual(
          placeholders(value),
          placeholders(english[key] ?? ""),
          `${locale}:${key} placeholders differ`,
        );
        assert.ok(!value.includes("__PH"), `${locale}:${key} has a leftover token`);
      }
    }
  });

  it("keeps the navigation label for mail templates distinct", () => {
    assert.equal(flatten(messages.en as Dict)["nav.mailTemplates"], "Mail Templates");
  });
});
