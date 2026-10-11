import { activeLanguage, t } from "./i18n";
import type { Language } from "./i18n";
export type State = "ok" | "warn" | "crit" | "unknown" | "stale";
export interface Result {
  name: string;
  state: State;
  detail: string;
  detail_code?: string;
  detail_params?: Record<string, string>;
  values?: Record<string, number>;
  fields?: Record<string, string>;
  at?: string;
  cadence_seconds?: number;
}
export interface Session {
  id: string;
  label?: string;
  branch?: string;
  api_equivalent_usd?: number;
  api_money?: Money;
  usage_measured?: boolean;
  speed_tokens_per_second?: number;
  speed_method?: string;
  kind: string;
  model?: string;
  project?: string;
  worktree?: string;
  user?: string;
  pid?: number;
  cpu_percent?: number;
  rss_bytes?: number;
  at: string;
  started_at?:string; ended_at?:string; cpu_seconds?:number; peak_rss_bytes?:number;
  state: string;
  reason: string;
  reason_code?: string;
  source: string;
  session_count?: number;
}
export function sessionProject(session: Session): string { return session.project && session.project !== "unmatched" ? session.project : "—"; }
export function sessionDisplayLabel(session: Session): string {
  const location = session.worktree?.split("/").filter(Boolean).at(-1) ||
    (session.project && session.project !== "unmatched" ? session.project.split("/").filter(Boolean).at(-1) : "");
  if (location && session.project !== "unmatched") return session.label || `${session.kind} · ${location} · ${session.id.slice(0,8)}`;
  return [session.kind, location, session.id.slice(0,8)].filter(Boolean).join(" · ");
}
export interface Money { original:number; currency:string; converted:number|null; display_currency:string; rate?:number; rate_source?:string; rate_date?:string }
export interface SessionDetail {session:Session;tokens:Tokens;usage_measured:boolean;tokens_per_minute:number;api_equivalent_usd:number;cli_reported_usd:number;api_money?:Money;cli_money?:Money;unpriced_tokens:number;processes:ProjectProcess[];speed:SpeedSummary[]}
export interface Overview {
  host_id: string;
  at: string;
  results: Result[];
  agents?: Session[];
}
export interface Host {
  host_id: string;
  at: string;
  labels?: Record<string,string>;
  liveness: "live" | "silent" | "offline";
}
export interface FleetHost extends Host {
  normal: number; checks: number; cpu?: number; memory?: number; disk?: number;
  agents: number; worst?: string; usage: Usage[]; costs: Cost[];
  worst_state?: State;
}
export interface Point {
  at: string;
  value: number;
  money?: Money;
}
export interface Tokens {
  input: number;
  output: number;
  cache_write: number;
  cache_read: number;
}
export interface Bucket {
  at: string;
  tokens: Tokens;
}
export interface Usage {
  name: string;
  tokens: Tokens;
  hourly: Bucket[];
  daily: Bucket[];
}
export interface Quota {
  kind: string;
  window: string;
  window_minutes?: number;
  used_percent: number;
  resets_at: string;
  projected_at?: string;
  method: string;
  observed_at: string;
}
export interface Cost {
  project: string;
  model?: string;
  method: string;
  usd?: number;
  amount?: number; currency?: string; money?: Money;
  partial?: boolean;
  unpriced_tokens?: number;
  unpriced_models?: string[];
}
export interface SpeedSummary {
  kind:string; model:string; method:string; window:string; session?:string;
  live_tokens_per_second:number|null; median_tokens_per_second:number|null; p95_tokens_per_second:number|null;
  median_duration_ms:number|null; median_first_token_ms:number|null;
  tokens_per_minute:number; requests_per_minute:number; api_errors:number; retries:number;
}
export interface SubscriptionComparison {
  kind:string; month:string; api_equivalent_usd:number; subscription_usd:number|null; coverage:string; unpriced_tokens:number;
  subscription_amount?:number|null; subscription_currency?:string; api_money?:Money; subscription_money?:Money|null;
}
export interface WorktreeEntry {root:string;project:string;name:string;path:string;size_bytes:number;size_complete:boolean;created_at?:string;last_commit_at?:string;merged?:boolean;agent_processes:number;processes:number;stale:boolean;reclaimable:boolean}
export interface WorktreesView {worktrees:WorktreeEntry[];reclaimable_bytes:number;at:string}
export interface Change {
  id: string;
  at: string;
  type: "APT" | "Service" | "Boot" | "Git" | "Container" | "Marker" | "Heat";
  text: string;
  change_code?: string;
  change_params?: Record<string, string>;
}
export function resultDetail(result: Result): string {
  const stateLabels: Record<string, string> = {
    running: t("result.running"), healthy: t("result.healthy"),
    unhealthy: t("result.unhealthy"), starting: t("result.starting"),
  };
  if (result.detail_code === "systemd_active") return t("result.systemd_active", {substate: stateLabels[result.detail_params?.substate || ""] || result.detail_params?.substate || ""});
  if (result.detail_code === "container_status") return t("result.container_status", {state: stateLabels[result.detail_params?.state || ""] || result.detail_params?.state || "", health: stateLabels[result.detail_params?.health || ""] || result.detail_params?.health || ""});
  if (result.detail_code === "certificate_expires") return t("result.certificate_expires", {days: Math.max(0, Math.ceil(result.values?.days_remaining ?? 0))});
  if (result.detail_code === "certificate_expired") return t("result.certificate_expired", {days: Math.ceil(Math.abs(result.values?.days_remaining ?? 0))});
  if (result.detail_code === "active") return t("result.active");
  if (result.detail_code === "healthy") return t("result.healthy");
  if (result.detail_code === "observed") return t("result.observed");
  if (result.detail_code === "configured_status_value") return t("result.configured_status_value");
  if (result.detail_code === "recent_timestamp") return t("result.recent_timestamp");
  if (result.detail_code === "log_entry_observed") return t("result.log_entry_observed");
  if (result.detail_code === "tcp_connect_succeeded") return t("result.tcp_connect_succeeded");
  return result.detail;
}
export function changeDetail(change: Change): string {
  const params = change.change_params || {};
  switch (change.change_code) {
    case "apt_upgraded": return t(params.count === "1" ? "change.apt_upgraded_one" : "change.apt_upgraded", params);
    case "service_restarted": return t("change.service_restarted", params);
    case "service_failed": return t("change.service_failed", params);
    case "machine_restarted": return t("change.machine_restarted");
    case "container_image_changed": return t("change.container_image_changed", params);
    case "marker": return t("change.marker_name", params);
    default: return change.text;
  }
}
export interface Energy {
  today_kwh: number;
  month_kwh: number;
  today_cost: number;
  month_cost: number;
  today_seconds: number;
  month_seconds: number;
  price_per_kwh: number;
  currency: string;
  label: string;
  today_money?:Money; month_money?:Money; price_money?:Money;
}
export type Range = "1h" | "12h" | "24h" | "7d" | "30d";
export type ProbeRange = Range;

export interface AppSample {
  processes?: {name:string;pid:number}[];
  name:string;kind:"agent"|"container"|"unit"|"other";
  rates_ready:boolean;
  counter_source?:"cgroup"|"proc";
  cpu_percent:number;memory_bytes:number;page_cache_bytes:number;
  disk_read_bytes_per_second:number;disk_write_bytes_per_second:number;
  disk_read_bytes:number;disk_write_bytes:number;
  network_rx_bytes_per_second?:number;network_tx_bytes_per_second?:number;
  estimated_energy_watts?:number;
}
export interface AppPoint extends AppSample {at:string}
export interface AppTopHour {at:string;cpu:string;memory:string;disk:string;network:string}
export interface AppsView {apps:AppSample[];history:AppPoint[];top_hours:AppTopHour[];range:Range}
export interface ProjectProcess {pid:number;name:string;project:string;worktree:string;kind:"agent"|"dev_server"|"process";cpu_percent:number;memory_bytes:number;ports:number[];ports_measured:boolean;connections_measured:boolean;idle_minutes:number;connection_idle_minutes:number;idle:boolean;stop_command:string;agent_session_id?:string;agent_kind?:string;left_behind?:boolean;started_at?:string}
export interface StorageSize {root:string;path:string;depth:number;size_bytes:number;complete:boolean;growth_7d_bytes?:number;at:string}
export interface Insight {kind:string;app:string;app_kind:string;detail:string;detail_code?:string;since:string}
export interface Observations {projects:ProjectProcess[];storage:StorageSize[];insights:Insight[];heat:Change[]}
export interface TrafficTotal {name:string;today_rx_bytes:number;today_tx_bytes:number;week_rx_bytes:number;week_tx_bytes:number;month_rx_bytes:number;month_tx_bytes:number}
export type Series = Record<string, Point[]>;
export interface ProbeDay { date: string; state: State; uptime: number | null; coverage: number }
export interface ProbePoint { at: string; latency_ms: number; state: State }
export interface ProbeSummary {
  name: string; display_name: string; kind: string; state: State;
  uptime_24h: number | null; uptime_7d: number | null; uptime_30d: number | null;
  coverage_24h: number; coverage_7d: number; coverage_30d: number;
  days: ProbeDay[]; p50_ms: number | null; p95_ms: number | null;
  percentiles: Record<ProbeRange, {p50_ms:number|null;p95_ms:number|null}>;
  status_code?: number; latency_ms?: number; dns_ms?: number;
  connect_ms?: number; tls_ms?: number; first_byte_ms?: number;
  certificate_days?: number; points: ProbePoint[];
}
export interface PublicProbe { display_name: string; state: State; uptime_30d: number | null; coverage_30d: number; days: ProbeDay[]; latency_ms?: number }
export function shortDate(at: string | number | Date): string {
  return resetTime(new Date(at).toISOString());
}
export const metrics = [
  "busy_percent",
  "used_bytes",
  "total_bytes",
  "watts",
  "celsius",
  "rx_bytes_per_second",
  "tx_bytes_per_second",
];
export function total(t?: Tokens) {
  return t ? t.input + t.output + t.cache_write + t.cache_read : 0;
}
export function result(results: Result[], name: string) {
  return results.find((r) => r.name === name);
}
export function value(r: Result | undefined, key: string) {
  if (r?.state === "stale") return null;
  const v = r?.values?.[key];
  return v !== undefined && Number.isFinite(v) ? v : null;
}
export function matching(results: Result[], prefix: string) {
  return results.filter((r) => r.name.startsWith(prefix));
}
export function present(r: Result | undefined, key: string) {
  return value(r, key) !== null;
}
export function availableTiles(o: Overview | null, costs: Cost[]) {
  if (!o) return [];
  const r = o.results;
  return [
    ["cpu", present(result(r, "cpu"), "busy_percent")],
    ["memory", present(result(r, "memory"), "used_bytes")],
    ["disk", matching(r, "disk:").some((x) => present(x, "used_bytes"))],
    ["power", present(result(r, "cpu_package_power"), "watts")],
    ["temperature", matching(r, "sensor:").some((x) => present(x, "celsius"))],
    ["cost", costs.some((x) => x.method === "api_equivalent")],
    ["agents", !!o.agents?.some((a) => a.state !== "ended")],
    [
      "services",
      matching(r, "systemd:").length > 0 || matching(r, "tls:").length > 0,
    ],
    ["network", matching(r, "network:").length > 0],
    ["containers", matching(r, "container:").length > 0],
    ["recent_errors", !!result(r, "recent_errors")],
    ["security", matching(r, "security:").length > 0],
    ["backups", matching(r, "backup:").length > 0],
  ]
    .filter(([, ok]) => ok)
    .map(([name]) => name as string);
}
export function pace(q: Quota) {
  const end = new Date(q.resets_at).getTime(),
    at = new Date(q.observed_at).getTime();
  const duration =
    q.window_minutes && q.window_minutes > 0
      ? q.window_minutes * 60_000
      : q.window.includes("week") ||
          q.window.includes("7") ||
          q.window === "secondary"
        ? 7 * 864e5
        : q.window.includes("5") || q.window === "primary"
          ? 5 * 36e5
          : 864e5;
  return Math.max(0, Math.min(100, 100 * (1 - (end - at) / duration)));
}
export function quotaLabel(q: Quota) {
  const p = pace(q);
  return q.used_percent >= 100 ? t("quota.limit_reached") : q.used_percent > p + 5 ? t("quota.ahead") : t("quota.within");
}
export function groupProcessSessions(agents: Session[]) {
  const groups = new Map<string, Session>();
  const rows: Session[] = [];
  for (const agent of agents) {
    if (agent.source !== "process") {
      rows.push(agent);
      continue;
    }
    const key = JSON.stringify([
      agent.kind,
      agent.project || "",
      agent.user || "",
    ]);
    const group = groups.get(key);
    if (group) {
      group.session_count = (group.session_count || 1) + 1;
      group.cpu_percent = (group.cpu_percent || 0) + (agent.cpu_percent || 0);
      group.rss_bytes = (group.rss_bytes || 0) + (agent.rss_bytes || 0);
    } else {
      const first = { ...agent, session_count: 1 };
      groups.set(key, first);
      rows.push(first);
    }
  }
  return rows;
}
export function cockpitNetwork(results: Result[]) {
  return matching(results, "network:").filter(
    (r) =>
      r.name !== "network:lo" &&
      ((value(r, "rx_bytes") || 0) > 0 || (value(r, "tx_bytes") || 0) > 0),
  );
}
export function cockpitServices(services: Result[], certificates: Result[]) {
  const priority = (r: Result) =>
    r.state === "crit"
      ? 0
      : r.state === "warn" || r.state === "stale" || r.state === "unknown"
        ? 1
        : 2;
  const sorted = [...certificates, ...services].sort(
    (a, b) =>
      priority(a) - priority(b) ||
      Number(b.name.startsWith("tls:")) - Number(a.name.startsWith("tls:")),
  );
  const mustShow = sorted.filter(
    (r) => priority(r) < 2 || r.name.startsWith("tls:"),
  );
  return [
    ...mustShow,
    ...sorted
      .filter((r) => priority(r) === 2 && !r.name.startsWith("tls:"))
      .slice(0, Math.max(0, 8 - mustShow.length)),
  ];
}
export function formatNumber(n: number, digits = 0): string {
  return new Intl.NumberFormat(activeLanguage() === "fr" ? "fr-FR" : "en-GB", {
    minimumFractionDigits: digits, maximumFractionDigits: digits,
  }).format(n);
}
function unknown(): string { return t("value.unknown"); }
export function formatBytes(n: number | null): string {
  if (n === null) return unknown();
  const fr = activeLanguage() === "fr";
  const units = fr ? ["o", "Kio", "Mio", "Gio", "Tio"] : ["B", "KiB", "MiB", "GiB", "TiB"];
  let i = 0;
  while (Math.abs(n) >= 1024 && i < units.length - 1) { n /= 1024; i++; }
  return `${formatNumber(n, i > 0 && Math.abs(n) < 10 ? 1 : 0)} ${units[i]}`;
}
export function formatRate(n: number | null): string {
  return n === null ? unknown() : `${formatBytes(n)}/s`;
}
function networkBits(): boolean {
  try { return JSON.parse(localStorage.getItem("fleetglass.preferences.v1") || "{}").network === "bits"; }
  catch { return false; }
}
function formatBits(n: number | null, rate: boolean): string {
  if (n === null) return unknown();
  const units = rate ? ["bit/s", "kbit/s", "Mbit/s", "Gbit/s", "Tbit/s"] : ["bit", "kbit", "Mbit", "Gbit", "Tbit"];
  let amount = n * 8, i = 0;
  while (Math.abs(amount) >= 1000 && i < units.length - 1) { amount /= 1000; i++; }
  return `${formatNumber(amount, i > 0 && Math.abs(amount) < 10 ? 1 : 0)} ${units[i]}`;
}
export function formatNetworkRate(n: number | null): string { return networkBits() ? formatBits(n, true) : formatRate(n); }
export function formatNetworkAmount(n: number | null): string { return networkBits() ? formatBits(n, false) : formatBytes(n); }
export function formatTemperature(celsius: number | null, digits = 0): string {
  if (celsius === null) return unknown();
  let fahrenheit = false;
  try { fahrenheit = JSON.parse(localStorage.getItem("fleetglass.preferences.v1") || "{}").temperature === "F"; }
  catch { /* Browser storage is optional. */ }
  return `${formatNumber(fahrenheit ? celsius * 9 / 5 + 32 : celsius, digits)} ${fahrenheit ? "°F" : "°C"}`;
}
export function formatDuration(seconds: number | null): string {
  if (seconds === null) return unknown();
  if (seconds < 60) return `${formatNumber(Math.round(seconds))} s`;
  if (seconds < 3600) return `${formatNumber(Math.round(seconds / 60))} min`;
  return `${formatNumber(seconds / 3600, 1)} h`;
}
export function formatPercent(n: number | null): string {
  return n === null ? unknown() : new Intl.NumberFormat(activeLanguage() === "fr" ? "fr-FR" : "en-GB", {
    style: "percent", minimumFractionDigits: 1, maximumFractionDigits: 1,
  }).format(n / 100);
}
export function formatQuotaPercent(n: number): string {
  return new Intl.NumberFormat(activeLanguage() === "fr" ? "fr-FR" : "en-GB", {style: "percent", maximumFractionDigits: 0}).format(n / 100);
}
export function agentName(kind: string) {
  return kind === "codex" ? "Codex" : kind === "claude" ? "Claude" : kind === "unknown" ? t("agent.unknown") : kind;
}
const agentStates = {
  working: "agent.state.working", running: "agent.state.running", "waiting-for-input": "agent.state.waiting_for_input",
  "rate-limited": "agent.state.rate_limited", erroring: "agent.state.erroring", ended: "agent.state.ended",
  unknown: "agent.state.unknown", "stale-worktree": "agent.state.stale_worktree",
} as const;
export function agentState(state: string) {
  return t(agentStates[state as keyof typeof agentStates] || "agent.state.unknown");
}
const reasonCodes = {
  process_alive: "agent.reason.process_alive", inactive_ended: "agent.reason.inactive_ended",
  quota_reached: "agent.reason.quota_reached", awaiting_input: "agent.reason.awaiting_input",
  turn_ended: "agent.reason.turn_ended", recent_activity: "agent.reason.recent_activity",
  inactive_unknown: "agent.reason.inactive_unknown", stale_worktree: "agent.reason.stale_worktree",
} as const;
export function agentReason(session: Session): string {
  const key = reasonCodes[session.reason_code as keyof typeof reasonCodes];
  return key ? t(key) : session.reason || t("value.no_reason");
}
export function agentSource(source: string) {
  return source === "otlp" || source === "otel" ? t("agent.source.otlp") : source === "log" || source === "local log" ? t("agent.source.log") : source === "process" ? t("agent.source.process") : source === "worktree" ? t("agent.source.worktree") : source;
}
export function quotaWindow(window: string, minutes?: number) {
  if (minutes === 300) return t("quota.five_hour");
  if (minutes === 10080) return t("quota.seven_day");
  if (minutes && minutes > 0) {
    if (minutes % 1440 === 0) return t("quota.days", {days:minutes / 1440});
    if (minutes % 60 === 0) return t("quota.hours", {hours:minutes / 60});
    return t("quota.minutes", {minutes});
  }
  if (window === "secondary" || window === "week" || window.includes("7")) return t("quota.seven_day");
  if (window === "primary" || window.includes("5")) return t("quota.five_hour");
  return window;
}
export function quotaMethod(method: string, kind: string) {
  if (method === "cli_reported" || method === "reported") return t("quota.reported", {agent: agentName(kind)});
  if (method === "estimated") return t("quota.estimated");
  return method;
}
export function resetTime(at: string, language: Language = activeLanguage()) {
  const parts = new Intl.DateTimeFormat(language === "fr" ? "fr-FR" : "en-GB", {
    weekday: "short",
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).formatToParts(new Date(at));
  const part = (name: string) =>
    parts.find((x) => x.type === name)?.value || "";
  return `${part("weekday")} ${part("day")} ${part("month")}, ${part("hour")}:${part("minute")}`;
}
export function formatDateTime(at: string | number | Date, language: Language = activeLanguage()): string {
  return new Intl.DateTimeFormat(language === "fr" ? "fr-FR" : "en-GB", {
    dateStyle: "medium", timeStyle: "short", hourCycle: "h23",
  }).format(new Date(at));
}
export function hasDate(at?: string): at is string { return Boolean(at && !at.startsWith("0001-")); }
export function weekdayShort(at: string | number | Date, language: Language = activeLanguage()): string {
  return new Intl.DateTimeFormat(language === "fr" ? "fr-FR" : "en-GB", {weekday: "short"}).format(new Date(at));
}
export function mountName(name: string) {
  const mount = name.startsWith("disk:") ? name.slice(5) : name;
  return mount === "/" ? t("observation.root") : mount;
}
export function observationName(name: string, displayName?: string) {
  if (displayName?.trim()) return displayName.trim();
  if (name.startsWith("oom:")) return t("observation.oom");
  if (name === "internal:closed_target") return t("observation.closed_target");
  if (name === "status:offsite_copy") return t("observation.offsite_copy");
  if (/^(log_marker|internal|status):/.test(name)) {
    const label = name.split(":").slice(1).join(" ").replaceAll("_", " ").trim();
    return label ? label.charAt(0).toUpperCase() + label.slice(1) : t("observation.check");
  }
  if (name.startsWith("disk:")) return mountName(name);
  if (name === "cpu_package_power") return t("observation.cpu_package");
  if (name === "cpu") return t("observation.cpu");
  if (name === "memory") return t("observation.memory");
  if (name.startsWith("sensor:")) return t("observation.sensor", {name: name.slice(7)});
  if (name.startsWith("systemd:")) return t("observation.service", {name: name.slice(8)});
  if (name.startsWith("container:")) return t("observation.container", {name: name.split(":").slice(2).join(":")});
  if (name.startsWith("network:")) return t("observation.network", {name: name.slice(8)});
  if (name.startsWith("tls:")) return t("observation.certificate", {name: name.slice(4)});
  if (name === "recent_errors") return t("ui.recent.errors.b0f52");
  const label = name.replaceAll(":", " ").replaceAll("_", " ").trim();
  return label ? label.charAt(0).toUpperCase() + label.slice(1) : t("observation.check");
}
export function formatMoney(n: number | null, currency = "USD"): string {
  return n === null ? unknown() : new Intl.NumberFormat(activeLanguage() === "fr" ? "fr-FR" : "en-US", {
    style: "currency", currency, minimumFractionDigits: 2,
    maximumFractionDigits: n < 0.01 ? 4 : 2,
  }).format(n);
}
export function formatConverted(m: Money | undefined, fallback = 0): string {
  if (!m) return formatMoney(fallback);
  const note = moneyRateNote(m);
  return note ? `${formatMoneyValue(m)} (${note})` : formatMoneyValue(m);
}
export function formatMoneyValue(m: Money): string {
  return formatMoney(m.converted ?? m.original, m.converted === null ? m.currency : m.display_currency);
}
export function moneyRateNote(m: Money): string {
  if (m.converted === null && m.currency !== m.display_currency) return activeLanguage() === "fr" ? "taux indisponible" : "rate unavailable";
  return m.rate_source && m.rate_date ? rateDescription(m.rate_source,m.rate_date) : "";
}
function rateDescription(source: string, date: string): string {
  const french = activeLanguage() === "fr";
  const name = source === "ecb" ? french ? "BCE" : "ECB" : source;
  const day = new Intl.DateTimeFormat(french ? "fr-FR" : "en-US", {day:"numeric",month:"short",timeZone:"UTC"}).format(new Date(date + "T12:00:00Z"));
  return french ? `taux ${name} du ${day}` : `${name} rate ${day}`;
}
export function formatMoneyTotal(rows: Money[]): string {
  if (!rows.length) return formatMoney(0, readDisplayCurrency());
  if (rows.some(m => m.converted === null)) return rows.map(m => formatConverted(m)).join(" + ");
  const first = rows[0];
  const amount = rows.reduce((sum,m) => sum + (m.converted ?? 0),0);
  const label = formatMoney(amount, first.display_currency);
  const rates = [...new Set(rows.filter(m => m.rate_source && m.rate_date).map(m => rateDescription(m.rate_source!,m.rate_date!)))];
  return rates.length ? `${label} (${rates.join(", ")})` : label;
}
export function formatMoneyTotalValue(rows: Money[]): string {
  if (!rows.length) return formatMoney(0, readDisplayCurrency());
  if (rows.some(m => m.converted === null)) return rows.map(formatMoneyValue).join(" + ");
  return formatMoney(rows.reduce((sum,m) => sum + (m.converted ?? 0),0),rows[0].display_currency);
}
export function costMoney(cost: Cost): Money {
  if (cost.money) return cost.money;
  const currency = cost.currency || "USD";
  const amount = cost.amount ?? cost.usd ?? 0;
  const display = readDisplayCurrency();
  return {original:amount,currency,converted:currency === display ? amount : null,display_currency:display};
}
function readDisplayCurrency(): string {
  try { const value = JSON.parse(localStorage.getItem("fleetglass.preferences.v1") || "null")?.currency; return /^[A-Z]{3}$/.test(value) ? value : "USD"; } catch { return "USD"; }
}
export function formatTokens(n: number | null): string {
  if (n === null) return unknown();
  const compact = (amount: number, digits: number) => new Intl.NumberFormat(activeLanguage() === "fr" ? "fr-FR" : "en-GB", {
    maximumFractionDigits: digits,
  }).format(amount);
  if (n >= 1e9) return `${compact(n / 1e9, 2)} ${activeLanguage() === "fr" ? "Md" : "B"}`;
  if (n >= 1e6) return `${compact(n / 1e6, n >= 1e7 ? 0 : 2)} M`;
  if (n >= 1e3) return `${compact(n / 1e3, n >= 1e5 ? 0 : 1)} ${activeLanguage() === "fr" ? "k" : "K"}`;
  return formatNumber(n);
}
export function themePreference(
  storage: Pick<Storage, "getItem"> | null,
  dark: boolean,
): "light" | "dark" {
  try {
    const v = storage?.getItem("fg-theme");
    if (v === "light" || v === "dark") return v;
  } catch {}
  return dark ? "dark" : "light";
}
export function saveTheme(
  storage: Pick<Storage, "setItem"> | null,
  theme: "light" | "dark",
) {
  try {
    storage?.setItem("fg-theme", theme);
  } catch {}
}
export function reconnectDelay(attempt: number) {
  return Math.min(30000, 1000 * 2 ** Math.min(attempt, 5));
}
