import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  CHAT_ROOT_CLASS,
  groupSessions,
  relativeTime,
  hasNegativeMarginBleed,
} from "./chatLayout";

const here = dirname(fileURLToPath(import.meta.url));

describe("CHAT_ROOT_CLASS", () => {
  it("fills the shell main without negative-margin bleed", () => {
    assert.equal(hasNegativeMarginBleed(CHAT_ROOT_CLASS), false);
    assert.match(CHAT_ROOT_CLASS, /\bflex-1\b/);
    assert.match(CHAT_ROOT_CLASS, /\bmin-h-0\b/);
    assert.doesNotMatch(CHAT_ROOT_CLASS, /-m-/);
  });

  it("is what ChatPage mounts as its root chrome", () => {
    const page = readFileSync(
      join(here, "../spa/pages/ChatPage.tsx"),
      "utf8",
    );
    assert.match(page, /CHAT_ROOT_CLASS/);
    assert.match(page, /className=\{CHAT_ROOT_CLASS\}/);
    assert.equal(hasNegativeMarginBleed(page), false);
  });
});

describe("hasNegativeMarginBleed", () => {
  it("detects Tailwind negative margin utilities", () => {
    assert.equal(hasNegativeMarginBleed("-m-4 flex"), true);
    assert.equal(hasNegativeMarginBleed("md:-m-6 overflow-hidden"), true);
    assert.equal(hasNegativeMarginBleed("-mt-2"), true);
    assert.equal(hasNegativeMarginBleed("flex min-h-0 flex-1"), false);
    assert.equal(hasNegativeMarginBleed("m-4"), false);
  });
});

describe("groupSessions", () => {
  const labels = { today: "Today", yesterday: "Yesterday", older: "Older" };
  // Local calendar dates avoid UTC/local day-boundary flakiness.
  const now = new Date(2026, 9, 15, 12, 0, 0);

  it("buckets by today / yesterday / older relative to now", () => {
    const groups = groupSessions(
      [
        {
          id: 1,
          title: "a",
          updated_at: new Date(2026, 9, 15, 8, 0, 0).toISOString(),
        },
        {
          id: 2,
          title: "b",
          updated_at: new Date(2026, 9, 14, 18, 0, 0).toISOString(),
        },
        {
          id: 3,
          title: "c",
          updated_at: new Date(2026, 9, 1, 0, 0, 0).toISOString(),
        },
      ],
      labels,
      now,
    );
    assert.deepEqual(
      groups.map((g) => ({ key: g.key, ids: g.items.map((i) => i.id) })),
      [
        { key: "today", ids: [1] },
        { key: "yesterday", ids: [2] },
        { key: "older", ids: [3] },
      ],
    );
  });

  it("omits empty groups", () => {
    const groups = groupSessions(
      [
        {
          id: 9,
          title: "only",
          updated_at: new Date(2026, 9, 15, 1, 0, 0).toISOString(),
        },
      ],
      labels,
      now,
    );
    assert.equal(groups.length, 1);
    assert.equal(groups[0].key, "today");
  });
});

describe("relativeTime", () => {
  const now = Date.parse("2026-10-15T12:00:00.000Z");

  it("formats relative times in every supported locale", () => {
    const justNow = { justNow: "just now" };
    assert.equal(
      relativeTime("2026-10-15T11:59:30.000Z", "en", now, justNow),
      "just now",
    );
    assert.equal(
      relativeTime("2026-10-15T11:30:00.000Z", "en", now),
      "30m ago",
    );
    assert.equal(
      relativeTime("2026-10-15T11:30:00.000Z", "zh-CN", now),
      "30分钟前",
    );
    assert.equal(
      relativeTime("2026-10-15T11:30:00.000Z", "zh-TW", now),
      "30 分鐘前",
    );
    assert.equal(
      relativeTime("2026-10-15T11:30:00.000Z", "ja", now),
      "30分前",
    );
    assert.equal(
      relativeTime("2026-10-15T11:30:00.000Z", "ko", now),
      "30분 전",
    );
    assert.equal(
      relativeTime("2026-10-14T12:00:00.000Z", "en", now),
      "yesterday",
    );
  });
});
