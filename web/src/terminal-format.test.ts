import {describe, expect, it} from "vitest";
import {blockBar, formatTextTable} from "./terminal-format";

describe("terminal text rendering", () => {
  it("aligns columns and preserves markup tones", () => {
    const lines = formatTextTable(["name", "rss", "state"], [
      ["atlas", "8M", {text:"WARN", tone:"warn"}],
      ["orbit/build", "120M", "ok"],
    ], [1]);
    expect(lines[0].map(cell => cell.text).join("  ")).toBe("name         rss   state");
    expect(lines[1].map(cell => cell.text).join("  ")).toBe("atlas          8M  WARN ");
    expect(lines[1][2].tone).toBe("warn");
    expect(lines[2][1].text).toBe("120M");
  });
  it("fills a bar and puts the pace marker in the track", () => {
    expect(blockBar(38, 10, 60).map(cell => cell.text).join("")).toBe("████░░│░░░");
    expect(blockBar(150, 4).map(cell => cell.text).join("")).toBe("████");
    expect(blockBar(-2, 4).map(cell => cell.text).join("")).toBe("░░░░");
  });
});
