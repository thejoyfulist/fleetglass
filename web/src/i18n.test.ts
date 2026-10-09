import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { messages, plural, setLanguage, t, unchangedTechnicalTerms, unchangedFrenchCognates } from "./i18n";
import { agentReason, changeDetail, resultDetail, quotaWindow } from "./data";
import { insightDetail } from "./observations";
import type { Insight, Session } from "./data";

describe("message catalog", () => {
  it("has a French message for every English message", () => {
    for (const [key, [en, fr]] of Object.entries(messages)) {
      expect(en.length, key).toBeGreaterThan(0);
      expect(fr.length, key).toBeGreaterThan(0);
      const parameters = (message: string) => [...message.matchAll(/\{([a-zA-Z][a-zA-Z0-9_]*)\}/g)].map(match => match[1]).sort();
      expect(parameters(fr), key).toEqual(parameters(en));
      if (en === fr && /[\p{L}]/u.test(en.replace(/\{[^}]+\}/g, ""))) expect([...unchangedTechnicalTerms, ...unchangedFrenchCognates], key).toContain(en);
    }
  });
  it("contains no unused keys", () => {
    const source = ["main.tsx", "observations.tsx", "charts.tsx", "preferences.tsx", "data.ts"]
      .map(file => readFileSync(new URL(file, import.meta.url), "utf8")).join("\n");
    const mappings = readFileSync(new URL("i18n.ts", import.meta.url), "utf8")
      .split("export type MessageKey =")[1];
    for (const key of Object.keys(messages)) {
      expect(source.includes(`"${key}"`) || mappings.includes(`"${key}"`), key).toBe(true);
    }
  });
  it("formats parameters and one or several correctly", () => {
    setLanguage("fr");
    expect(plural(1, "status.check_down.one", "status.check_down.many")).toBe("1 contrôle en échec");
    expect(plural(2, "status.check_down.one", "status.check_down.many")).toBe("2 contrôles en échec");
    expect(t("status.last_observed", {at: "10:30", detail: "raw message"})).toContain("raw message");
    expect(agentReason({reason_code: "quota_reached", reason: "quota reached"} as Session)).toBe("Quota atteint");
    expect(agentReason({reason: "Raw provider message"} as Session)).toBe("Raw provider message");
    expect(insightDetail({kind: "future_code", detail: "Raw observation"} as Insight)).toBe("Raw observation");
    expect(resultDetail({name:"tls:example.test",state:"warn",detail:"expires in 20.7 days",detail_code:"certificate_expires",values:{days_remaining:20.7}})).toBe("Expire dans 21 jours");
    expect(resultDetail({name:"container:local:web",state:"ok",detail:"running; healthy",detail_code:"container_status",detail_params:{state:"running",health:"healthy"}})).toBe("en cours ; sain");
    expect(changeDetail({id:"x",at:"",type:"APT",text:"Upgraded 3 packages: curl",change_code:"apt_upgraded",change_params:{count:"3",packages:"curl"}})).toBe("3 paquets mis à jour : curl");
    expect(quotaWindow("secondary")).toBe("fenêtre de 7 jours");
    setLanguage("en");
  });
});
