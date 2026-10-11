import { describe, it, expect } from "vitest";
import { setLanguage } from "./i18n";
import {
  availableTiles,
  agentName,
  agentState,
  cockpitNetwork,
  cockpitServices,
  formatBytes,
  formatDuration,
  formatMoney,
  formatConverted,
  formatMoneyTotalValue,
  formatPercent,
  formatRate,
  formatQuotaPercent,
  formatTokens,
  groupProcessSessions,
  mountName,
  observationName,
  pace,
  quotaLabel,
  quotaMethod,
  quotaWindow,
  resetTime,
  sessionDisplayLabel,
  sessionProject,
  reconnectDelay,
  saveTheme,
  themePreference,
  value,
} from "./data";
import type { Overview, Quota } from "./data";
describe("formatting", () => {
  setLanguage("en");
  it("keeps units and unknown states", () => {
    expect(formatBytes(1024)).toBe("1.0 KiB");
    expect(formatBytes(null)).toBe("Unknown");
    expect(formatRate(2048)).toBe("2.0 KiB/s");
    expect(formatDuration(3600)).toBe("1.0 h");
    expect(formatPercent(3.55)).toBe("3.6%");
    expect(formatMoney(2.7)).toBe("$2.70");
    expect(formatTokens(970000)).toBe("970 K");
    expect(formatTokens(1710000)).toBe("1.71 M");
    expect(formatBytes(47 * 1024 ** 3)).toBe("47 GiB");
    expect(mountName("disk:/")).toBe("Root filesystem");
  });
  it("uses French numeric separators, spacing and 24-hour time", () => {
    setLanguage("fr");
    try {
      expect(formatBytes(1536)).toBe("1,5 Kio");
      expect(formatPercent(3.55)).toMatch(/^3,6\s%$/);
      expect(formatDuration(5400)).toBe("1,5 h");
      expect(resetTime("2026-10-09T14:35:00Z")).toMatch(/\b\d{2}:35\b/);
      expect(resetTime("2026-10-09T14:35:00Z")).not.toMatch(/\b(?:AM|PM)\b/);
    } finally { setLanguage("en"); }
  });
  it("formats EUR in both languages and explains missing rates", () => {
    setLanguage("fr");
    expect(formatMoney(12.68,"EUR")).toMatch(/^12,68\s€$/);
    expect(formatConverted({original:12.68,currency:"EUR",converted:null,display_currency:"USD"})).toContain("taux indisponible");
    expect(formatConverted({original:12.68,currency:"USD",converted:10.93,display_currency:"EUR",rate_source:"ecb",rate_date:"2026-10-08"})).toContain("taux BCE du 8 oct.");
    setLanguage("en");
    expect(formatMoney(12.68,"EUR")).toBe("€12.68");
    expect(formatConverted({original:12.68,currency:"EUR",converted:null,display_currency:"USD"})).toContain("rate unavailable");
  });
  it("keeps rate details out of compact amounts and names unmatched sessions by location", () => {
    expect(formatMoneyTotalValue([{original:20.83,currency:"EUR",converted:20.83,display_currency:"EUR",rate_source:"ecb",rate_date:"2026-10-09"}])).toBe("€20.83");
    const session = {id:"abcdef123456",kind:"claude",project:"unmatched",worktree:"/tmp/coordination",at:"",state:"working",reason:"",source:"log"};
    expect(sessionProject(session)).toBe("—");
    expect(sessionDisplayLabel(session)).toBe("claude · coordination · abcdef12");
    expect(sessionDisplayLabel({...session,worktree:"",model:"claude-opus-4"})).toBe("claude · abcdef12");
  });
});
describe("observation titles", () => {
  it("uses display names and readable fallback titles in both languages", () => {
    for (const language of ["en", "fr"] as const) {
      setLanguage(language);
      expect(observationName("internal:closed_target", "Database relay")).toBe("Database relay");
      expect(observationName("internal:closed_target")).toBe(language === "en" ? "Closed target" : "Cible fermée");
      expect(observationName("oom:0")).toBe(language === "en" ? "Out-of-memory kill" : "Arrêt par manque de mémoire");
      expect(observationName("status:offsite_copy")).toBe(language === "en" ? "Offsite copy" : "Copie hors site");
      expect(observationName("security:updates")).toBe("Security updates");
    }
    setLanguage("en");
  });
});
describe("stale observations and ended sessions", () => {
  it("shows the backup tile only with configured backup results", () => {
    const overview: Overview = {host_id:"host", at:new Date().toISOString(), results:[], agents:[]};
    expect(availableTiles(overview, [])).not.toContain("backups");
    overview.results=[{name:"backup:db",state:"unknown",detail:"unreadable"}];
    expect(availableTiles(overview, [])).toContain("backups");
  });
  it("does not present a stale measurement as a current value", () => {
    expect(
      value(
        {
          name: "cpu",
          state: "stale",
          detail: "old",
          values: { busy_percent: 10 },
        },
        "busy_percent",
      ),
    ).toBeNull();
  });
  it("omits the agents tile when only ended sessions remain", () => {
    const overview: Overview = {
      host_id: "host",
      at: new Date().toISOString(),
      results: [],
      agents: [
        {
          id: "s",
          kind: "claude",
          at: new Date().toISOString(),
          state: "ended",
          reason: "inactive",
          source: "log",
        },
      ],
    };
    expect(availableTiles(overview, [])).not.toContain("agents");
  });
});
describe("quotas", () => {
  it("uses human-readable labels and consistent whole percentages", () => {
    expect(agentName("codex")).toBe("Codex");
    expect(agentState("waiting-for-input")).toBe("Waiting for input");
    expect(quotaWindow("secondary")).toBe("7 days");
    expect(quotaWindow("primary")).toBe("5 hours");
    expect(quotaMethod("cli_reported", "codex")).toBe("reported by Codex");
    expect(formatQuotaPercent(72.6)).toBe("73%");
    expect(resetTime("2026-10-14T15:44:23Z")).toMatch(
      /^Wed 14 Oct, \d{2}:\d{2}$/,
    );
  });
  it("compares use with elapsed window", () => {
    const now = new Date();
    const q: Quota = {
      kind: "codex",
      window: "week",
      used_percent: 42,
      resets_at: new Date(now.getTime() + 6 * 864e5).toISOString(),
      observed_at: now.toISOString(),
      method: "reported",
    };
    expect(pace(q)).toBeCloseTo(100 / 7);
    expect(quotaLabel(q)).toBe("Ahead of pace");
  });
  it("uses the actual Codex durations even when primary is weekly", () => {
    const weekly: Quota = {
      kind: "codex",
      window: "primary",
      window_minutes: 10080,
      used_percent: 64,
      resets_at: "2026-10-14T06:21:50Z",
      observed_at: "2026-10-08T14:28:38Z",
      method: "cli_reported",
    };
    const fiveHour: Quota = {
      ...weekly,
      window: "secondary",
      window_minutes: 300,
      used_percent: 28,
      resets_at: "2026-10-08T17:28:38Z",
    };
    expect(quotaWindow(weekly.window, weekly.window_minutes)).toBe(
      "7 days",
    );
    expect(quotaWindow("primary", 720)).toBe("12 h");
    expect(quotaWindow("primary", 2880)).toBe("2 d");
    expect(quotaWindow(fiveHour.window, fiveHour.window_minutes)).toBe(
      "5 hours",
    );
    expect(pace(weekly)).toBeGreaterThan(10);
    expect(pace(weekly)).toBeLessThan(20);
    expect(pace(fiveHour)).toBeCloseTo(40);
    expect(resetTime(weekly.resets_at)).toContain("Wed 14 Oct");
    expect(formatQuotaPercent(weekly.used_percent)).toBe("64%");
  });
});
describe("busy cockpit", () => {
  it("groups process-only sessions and sums process figures", () => {
    const base = {
      kind: "claude",
      project: "coordination",
      user: "root",
      at: "2026-10-08T14:00:00Z",
      state: "running",
      reason: "process alive",
      source: "process",
    };
    const rows = groupProcessSessions(
      [1, 2, 3, 4, 5, 6].map((i) => ({
        ...base,
        id: String(i),
        pid: i,
        cpu_percent: i,
        rss_bytes: i * 1000,
      })),
    );
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({
      session_count: 6,
      cpu_percent: 21,
      rss_bytes: 21000,
    });
  });
  it("prioritizes certificates and warnings and omits idle network interfaces", () => {
    const check = (name: string, state: "ok" | "warn", rx = 0) => ({
      name,
      state,
      detail: "",
      values: { rx_bytes: rx, tx_bytes: 0 },
    });
    const services = Array.from({ length: 10 }, (_, i) =>
      check(`systemd:${i}`, "ok"),
    );
    const cert = check("tls:expiring.test", "warn");
    expect(cockpitServices(services, [cert])[0].name).toBe(cert.name);
    expect(
      cockpitServices(
        [...services, check("systemd:broken", "warn")],
        [check("tls:valid.test", "ok")],
      )[0].name,
    ).toBe("systemd:broken");
    expect(
      cockpitNetwork([
        check("network:lo", "ok", 10),
        check("network:docker0", "ok"),
        check("network:eth0", "ok", 10),
      ]).map((x) => x.name),
    ).toEqual(["network:eth0"]);
  });
});
describe("theme", () => {
  it("uses storage when possible and falls back when blocked", () => {
    let stored = "dark";
    const storage = {
      getItem: () => stored,
      setItem: (_: string, v: string) => {
        stored = v;
      },
    };
    expect(themePreference(storage, false)).toBe("dark");
    saveTheme(storage, "light");
    expect(themePreference(storage, true)).toBe("light");
    expect(
      themePreference(
        {
          getItem: () => {
            throw Error();
          },
        },
        true,
      ),
    ).toBe("dark");
  });
});
describe("reconnect", () => {
  it("backs off with a ceiling", () => {
    expect([0, 1, 2, 10].map(reconnectDelay)).toEqual([
      1000, 2000, 4000, 30000,
    ]);
  });
});
describe("tiles", () => {
  it("hides absent data and never invents later-milestone tiles", () => {
    const o: Overview = {
      host_id: "test",
      at: new Date().toISOString(),
      results: [
        { name: "cpu", state: "ok", detail: "", values: { busy_percent: 5 } },
      ],
    };
    expect(availableTiles(o, [])).toEqual(["cpu"]);
  });
});
