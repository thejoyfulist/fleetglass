import {describe, expect, it} from "vitest";
import {attentionLabel, blockBar, formatTextTable, measuredSpeed, truncateName, truncateSessionLabel} from "./terminal-format";

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
  it("truncates long names without splitting Unicode characters", () => {
    expect(truncateName("Other / not attributed", 12)).toBe("Other / not…");
    expect(truncateName("short", 12)).toBe("short");
    expect(truncateName("🚀🚀🚀", 2)).toBe("🚀…");
  });
  it("preserves the short session ID while shortening only the middle", () => {
    expect(truncateSessionLabel("claude · session · 0b82eee0")).toBe("claude · ses… · 0b82eee0");
    expect(truncateSessionLabel("claude · 0b82eee0")).toBe("claude · 0b82eee0");
    expect(truncateSessionLabel("codex · very-long-project · branch-name · abcdef12").endsWith(" · abcdef12")).toBe(true);
  });
  it("names attention families from structured fields", () => {
    expect(attentionLabel({name:"recent_error:0",state:"warn",detail:"journal error group",values:{count:2},fields:{unit:"demo.service"}})).toBe("journal errors: demo.service · 2 in 24 h");
    expect(attentionLabel({name:"repository:0:atlas",state:"warn",detail:"4 uncommitted files",values:{dirty_count:4},fields:{repository:"atlas"}})).toBe("atlas: 4 uncommitted files");
    expect(attentionLabel({name:"recent_errors",state:"warn",detail:"journal, last 24 h",values:{count:3}})).toBe("journal errors: 3 in 24 h");
  });
  it("keeps one positive speed measurement per model", () => {
    const speed = (model:string, rate:number) => ({kind:"codex",model,method:"estimated",window:"1h",live_tokens_per_second:rate,median_tokens_per_second:rate,p95_tokens_per_second:null,median_duration_ms:null,median_first_token_ms:null,tokens_per_minute:0,requests_per_minute:0,api_errors:0,retries:0});
    expect(measuredSpeed([speed("fast",12),speed("idle",0),speed("fast",8)]).map(x=>[x.model,x.rate])).toEqual([["fast",12]]);
  });
});

describe("session source labels", () => {
  it("shows all feed combinations and a translated legend", async () => {
    const {sessionSourceTag, sessionSourceLegend} = await import("./data");
    const {setLanguage} = await import("./i18n");
    expect(sessionSourceTag({source:"otlp"})).toBe("otel");
    expect(sessionSourceTag({source:"log"})).toBe("log");
    expect(sessionSourceTag({source:"otlp",sources:["otlp","log","process"]})).toBe("otel+log");
    expect(sessionSourceTag({source:"process"})).toBe("proc");
    setLanguage("en");
    expect(sessionSourceLegend()).toBe("otel = OpenTelemetry · log = local logs · proc = process · est. = estimated");
    setLanguage("fr");
    expect(sessionSourceLegend()).toContain("journaux locaux");
    setLanguage("en");
  });
});
