import { describe, expect, it } from "vitest";

import {
  createReasoningEffortMappingRow,
  normalizeReasoningEffortForPlatform,
  reasoningEffortMappingsToAPI,
  reasoningEffortMappingsToRows,
  reasoningEffortOptionsForPlatform,
  reasoningEffortSourceOptionsForPlatform,
  validateReasoningEffortMappings,
} from "../groupsReasoningEffort";

describe("groupsReasoningEffort", () => {
  it("provides fixed OpenAI choices without none", () => {
    expect(
      reasoningEffortOptionsForPlatform("openai").map((option) => option.value),
    ).toEqual([
      "minimal",
      "low",
      "medium",
      "high",
      "xhigh",
      "max",
    ]);
    for (const platform of [
      "anthropic",
      "gemini",
      "antigravity",
      "grok",
    ] as const) {
      expect(reasoningEffortOptionsForPlatform(platform)).toEqual([]);
    }
  });

  it("offers an omitted-value source only on the mapping input side", () => {
    expect(
      reasoningEffortSourceOptionsForPlatform("openai").map(
        (option) => option.value,
      ),
    ).toEqual([
      "default",
      "minimal",
      "low",
      "medium",
      "high",
      "xhigh",
      "max",
    ]);
    expect(
      reasoningEffortOptionsForPlatform("openai").map((option) => option.value),
    ).not.toContain("default");
  });

  it("hydrates supported rows and drops stale custom values", () => {
    const rows = reasoningEffortMappingsToRows(
      [
        { from: " max ", to: " xhigh " },
        { from: " default ", to: " low " },
        { from: "ultra", to: "high" },
      ],
      "openai",
    );

    expect(rows).toHaveLength(2);
    expect(reasoningEffortMappingsToAPI(rows)).toEqual([
      { from: "max", to: "xhigh" },
      { from: "default", to: "low" },
    ]);
  });

  it("clears values unsupported by OpenAI or used on another platform", () => {
    expect(normalizeReasoningEffortForPlatform("openai", " MAX ")).toBe("max");
    expect(normalizeReasoningEffortForPlatform("grok", "max")).toBe("");
    expect(normalizeReasoningEffortForPlatform("openai", "none")).toBe("");
  });

  it("requires both sides of every mapping", () => {
    const first = createReasoningEffortMappingRow({ to: "low" });
    const second = createReasoningEffortMappingRow({ from: "max" });

    expect(validateReasoningEffortMappings([first, second])).toEqual({
      [first.id]: { from: "fromRequired" },
      [second.id]: { to: "toRequired" },
    });
  });

  it("rejects duplicate source values case insensitively", () => {
    const first = createReasoningEffortMappingRow({ from: "MAX", to: "xhigh" });
    const second = createReasoningEffortMappingRow({ from: " max ", to: "high" });

    expect(validateReasoningEffortMappings([first, second])).toEqual({
      [first.id]: { from: "duplicateFrom" },
      [second.id]: { from: "duplicateFrom" },
    });
  });

  it("rejects custom mappings", () => {
    const row = createReasoningEffortMappingRow({ from: "ultra", to: "high" });
    expect(validateReasoningEffortMappings([row], "openai")).toEqual({
      [row.id]: { from: "unsupportedFrom" },
    });
  });

  it("rejects default as a mapping target", () => {
    const row = createReasoningEffortMappingRow({
      from: "low",
      to: "default",
    });
    expect(validateReasoningEffortMappings([row], "openai")).toEqual({
      [row.id]: { to: "unsupportedTo" },
    });
  });
});
