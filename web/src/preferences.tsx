import { useEffect, useState } from "react";
import { t, navigationLabels, sectionLabels } from "./i18n";
import type { MessageKey } from "./i18n";
import type { Language } from "./i18n";
import type { Host, FleetHost, Overview } from "./data";
import { formatBytes, formatDateTime, formatNumber, formatPercent, formatMoneyTotal, costMoney, value, result } from "./data";
import { TerminalFilter } from "./terminal";

export type { Language } from "./i18n";
export type Preferences = {
  language: Language;
  temperature: "C" | "F";
  network: "bytes" | "bits";
  cpu: "core" | "machine";
  currency: string;
  hiddenTabs: string[];
  hiddenSections: string[];
};
export const defaultPreferences: Preferences = {
  language: "en",
  temperature: "C",
  network: "bytes",
  cpu: "core",
  currency: "USD",
  hiddenTabs: [],
  hiddenSections: [],
};
const key = "fleetglass.preferences.v1";
export function hasPreferences() {
  try {
    return localStorage.getItem(key) !== null;
  } catch {
    return false;
  }
}
export function readPreferences(
  defaults: Preferences = defaultPreferences,
): Preferences {
  try {
    const raw = JSON.parse(localStorage.getItem(key) || "null");
    if (!raw || typeof raw !== "object") return defaults;
    return {
      language: raw.language === "fr" ? "fr" : "en",
      temperature: raw.temperature === "F" ? "F" : "C",
      network: raw.network === "bits" ? "bits" : "bytes",
      cpu: raw.cpu === "machine" ? "machine" : "core",
      currency: typeof raw.currency === "string" && /^[A-Z]{3}$/.test(raw.currency) ? raw.currency : defaults.currency,
      hiddenTabs: Array.isArray(raw.hiddenTabs)
        ? raw.hiddenTabs.filter((x: unknown) => typeof x === "string")
        : [],
      hiddenSections: Array.isArray(raw.hiddenSections)
        ? raw.hiddenSections.filter((x: unknown) => typeof x === "string")
        : [],
    };
  } catch {
    return defaults;
  }
}
export function savePreferences(p: Preferences) {
  try {
    localStorage.setItem(key, JSON.stringify(p));
  } catch {
    /* Storage may be disabled. */
  }
}
export function formatAppCPU(percent: number) {
  const cores = Number(document.body.dataset.cpuCores || "1");
  const divisor = readPreferences().cpu === "machine" && cores > 0 ? cores : 1;
  return formatPercent(percent / divisor);
}

export { t } from "./i18n";
export function useCockpitLanguage(language: Language) {
  useEffect(() => { document.documentElement.lang = language; }, [language]);
}
export function Explain({
  title,
  language,
  messageKey,
}: {
  title: string;
  language: Language;
  messageKey?: MessageKey;
}) {
  const [open, setOpen] = useState(false);
  const [left, setLeft] = useState(0);
  const copy = messageKey ? t(messageKey, language) : null;
  if (!copy) return null;
  return (
    <span className="explain">
      <button
        type="button"
        aria-label={`${t("ui.what.this.means.a4bcc",language)}: ${title}`}
        aria-expanded={open}
        onClick={(e) => {
          const x = e.currentTarget.getBoundingClientRect().left;
          const width = Math.min(310, window.innerWidth - 40);
          setLeft(
            Math.min(Math.max(x, 20), window.innerWidth - width - 20) - x,
          );
          setOpen(!open);
        }}
      >
        ?
      </button>
      {open ? (
        <span role="note" className="explain-popover" style={{ left }}>
          {copy}
        </span>
      ) : null}
    </span>
  );
}
const allTabs = [
  "overview",
  "fleet",
  "agents",
  "apps",
  "projects",
  "machine",
  "services",
  "system",
  "changes",
  "costs",
  "uptime",
];
const allSections = [
  "CPU",
  "Memory",
  "Disk",
  "Network",
  "Power",
  "Temperature",
  "Agents",
  "Services",
  "Costs",
  "Backups",
  "Security",
];
export function SettingsView({
  preferences,
  onChange,
  onReset,
}: {
  preferences: Preferences;
  onChange: (p: Preferences) => void;
  onReset: () => void;
}) {
  const lang = preferences.language;
  const update = (
    field: keyof Preferences,
    v: Preferences[keyof Preferences],
  ) => onChange({ ...preferences, [field]: v });
  const toggle = (field: "hiddenTabs" | "hiddenSections", name: string) =>
    update(
      field,
      preferences[field].includes(name)
        ? preferences[field].filter((x) => x !== name)
        : [...preferences[field], name],
    );
  return (
    <>
      <div className="page-head">
        <div>
          <h1>{t("ui.settings.c7f73", lang)}</h1>
          <p>
            {t("ui.preferences.saved.in.this.browser.8c56a",lang)}
          </p>
        </div>
      </div>
      <div className="settings-grid">
        <section className="g t">
          <div className="h">
            <h2>{t("ui.display.574ff",lang)}</h2>
          </div>
          <TerminalFilter label={t("ui.language.89b86",lang)} choices={["en","fr"]} value={lang} onChange={v=>update("language",v as Language)}/>
          <TerminalFilter label={lang==="fr"?"Devise des coûts":"Cost currency"} choices={["USD","EUR"]} value={preferences.currency} onChange={v=>update("currency",v)}/>
          <TerminalFilter label={t("ui.temperature.0a906",lang)} choices={["C","F"]} value={preferences.temperature} onChange={v=>update("temperature",v as "C"|"F")}/>
          <TerminalFilter label={t("ui.network.rate.e2691",lang)} choices={["bytes","bits"]} value={preferences.network} onChange={v=>update("network",v as "bytes"|"bits")}/>
          <TerminalFilter label={t("ui.cpu.usage.d52b4",lang)} choices={["core","machine"]} value={preferences.cpu} onChange={v=>update("cpu",v as "core"|"machine")}/>
        </section>
        <section className="g t">
          <div className="h">
            <h2>{t("ui.navigation.cf03c",lang)}</h2>
          </div>
          <p className="foot">
            {t("ui.choose.visible.tabs.settings.always.remains.avai.6bae1",lang)}
          </p>
          <div className="settings-checks">
            {allTabs.map((name) => (
              <label key={name}>
                <input
                  type="checkbox"
                  checked={!preferences.hiddenTabs.includes(name)}
                  onChange={() => toggle("hiddenTabs", name)}
                />
                {t(navigationLabels[name as keyof typeof navigationLabels], lang)}
              </label>
            ))}
          </div>
        </section>
        <section className="g t">
          <div className="h">
            <h2>{t("ui.sections.7ff5a",lang)}</h2>
          </div>
          <p className="foot">
            {t("ui.hide.cards.you.do.not.need.6eab6",lang)}
          </p>
          <div className="settings-checks">
            {allSections.map((name) => (
              <label key={name}>
                <input
                  type="checkbox"
                  checked={!preferences.hiddenSections.includes(name)}
                  onChange={() => toggle("hiddenSections", name)}
                />
                {t(sectionLabels[name as keyof typeof sectionLabels], lang)}
              </label>
            ))}
          </div>
        </section>
        <section className="g t">
          <div className="h">
            <h2>{t("ui.reset.44c57",lang)}</h2>
          </div>
          <p>
            {t("ui.restore.server.defaults.and.show.all.tabs.and.se.be9b1",lang)}
          </p>
          <button type="button" className="quiet" onClick={onReset}>
            {t("ui.reset.settings.9ba8d",lang)}
          </button>
        </section>
      </div>
    </>
  );
}
export function ShareView({
  host,
  hosts,
  fleet,
  overview,
  theme,
  language,
}: {
  host: string;
  hosts: Host[];
  fleet: FleetHost[];
  overview: Overview | null;
  theme: "light" | "dark";
  language: Language;
}) {
  const [scope, setScope] = useState<"host" | "fleet">("host");
  const [hide, setHide] = useState(false);
  const [cardTheme, setCardTheme] = useState(theme);
  const [url, setUrl] = useState("");
  useEffect(() => {
    if (!host && scope === "host") return;
    const canvas = document.createElement("canvas");
    canvas.width = 1200;
    canvas.height = 630;
    const c = canvas.getContext("2d");
    if (!c) return;
    const dark = cardTheme === "dark";
    const bg = dark ? "#1E1E1E" : "#FFFFFF",
      ink = dark ? "#E5E5E5" : "#000000",
      muted = dark ? "#8C8C8C" : "#666666",
      line = dark ? "#444444" : "#BFBFBF";
    c.fillStyle = bg;
    c.fillRect(0, 0, 1200, 630);
    c.fillStyle = dark ? "#2B2B2B" : "#E8E8E8";
    c.fillRect(0, 0, 1200, 42);
    for (const [index,color] of ["#FF5F57","#FEBC2E","#28C840"].entries()) {
      c.beginPath(); c.arc(22+index*23,21,7,0,Math.PI*2); c.fillStyle=color; c.fill();
    }
    c.fillStyle = ink;
    c.font = "22px Menlo, Monaco, monospace";
    c.textAlign = "center";
    c.fillText("fleetglass - 120x63", 600, 29);
    c.textAlign = "left";
    c.font = "bold 24px Menlo, Monaco, monospace";
    c.fillText("fleetglass", 40, 94);
    c.fillStyle = muted;
    c.font = "20px Menlo, Monaco, monospace";
    c.fillText(
      scope === "fleet"
        ? t("ui.fleet.summary.52086",language)
        : hide
          ? t("ui.private.host.fd00f",language)
          : host,
      40,
      135,
    );
    const boxes: [string, string][] =
      scope === "fleet"
        ? [
            [t("ui.hosts.53103",language), formatNumber(hosts.length)],
            [
              t("ui.reporting.c9e57",language),
              formatNumber(fleet.filter((x) => x.liveness === "live").length),
            ],
            [
              t("ui.agents.64acf",language),
              formatNumber(fleet.reduce((n, x) => n + x.agents, 0)),
            ],
          ]
        : [
            [
              t("ui.cpu.ff221",language),
              overview && value(result(overview.results, "cpu"), "busy_percent") !== null
                ? formatPercent(value(result(overview.results, "cpu"), "busy_percent")!)
                : "—",
            ],
            [
              t("ui.memory.89c8a",language),
              overview
                ? formatBytes(
                    value(result(overview.results, "memory"), "used_bytes"),
                  )
                : "—",
            ],
            [
              t("ui.agents.64acf",language),
              formatNumber(
                overview?.agents?.filter((x) => x.state !== "ended").length ||
                  0,
              ),
            ],
          ];
    boxes.forEach(([label, num], i) => {
      const x = 40+i*373;
      c.strokeStyle = line;
      c.lineWidth = 1;
      c.strokeRect(x, 185, 350, 250);
      c.fillStyle = muted;
      c.font = "19px Menlo, Monaco, monospace";
      c.fillText(label, x+22, 235);
      c.fillStyle = ink;
      c.font = "bold 48px Menlo, Monaco, monospace";
      c.fillText(num, x+22, 335);
    });
    c.fillStyle = muted;
    c.font = "18px Menlo, Monaco, monospace";
    c.fillText(
      `${t("ui.measured.95503",language)} ${formatDateTime(overview?.at || Date.now(), language)}`,
      40,
      520,
    );
    c.fillText("fleetglass", 40, 575);
    const costs = (scope === "fleet" ? fleet : fleet.filter(x => x.host_id === host)).flatMap(x => x.costs.filter(cost => cost.method === "api_equivalent").map(costMoney));
    c.textAlign = "right";
    c.fillText(`${language === "fr" ? "Équivalent API" : "API equivalent"} · ${formatMoneyTotal(costs)}`, 1160, 575);
    c.textAlign = "left";
    setUrl(canvas.toDataURL("image/png"));
  }, [host, hosts, fleet, overview, scope, hide, cardTheme, language]);
  const download = () => {
    if (!url) return;
    const a = document.createElement("a");
    a.href = url;
    a.download = `fleetglass-${scope}.png`;
    a.click();
  };
  return (
    <>
      <div className="page-head">
        <div>
          <h1>{t("ui.share.09ca5", language)}</h1>
          <p>
            {t("ui.export.a.1200.630.pixel.summary.8c4bb",language)}
          </p>
        </div>
      </div>
      <div className="share-layout">
        <section className="g t share-controls">
          <div className="h">
            <h2>{t("ui.card.4d4ce",language)}</h2>
          </div>
          <TerminalFilter label={t("ui.content.4f9be",language)} choices={["host","fleet"]} value={scope} onChange={v=>setScope(v as "host"|"fleet")}/>
          <TerminalFilter label={t("ui.theme.a797e",language)} choices={["light","dark"]} value={cardTheme} onChange={v=>setCardTheme(v as "light"|"dark")}/>
          <label className="check">
            <input
              type="checkbox"
              checked={hide}
              onChange={(e) => setHide(e.target.checked)}
            />
            {t("ui.hide.host.and.project.names.9f067",language)}
          </label>
          <p className="foot">
            {t("ui.the.card.is.drawn.in.this.browser.no.project.nam.00d8c",language)}
          </p>
          <button type="button" onClick={download}>
            {t("ui.download.png.0c830",language)}
          </button>
        </section>
        <section className="g t share-preview">
          <div className="h">
            <h2>{t("ui.preview.f1fbb",language)}</h2>
            <span className="src">1200 × 630</span>
          </div>
          {url ? (
            <img
              src={url}
              alt={
                t("ui.fleetglass.card.preview.8484c",language)
              }
            />
          ) : null}
        </section>
      </div>
    </>
  );
}
