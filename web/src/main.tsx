import React, { useCallback, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { AreaChart, Bars, Gauge, ResponseChart, Ring, Sparkline } from "./charts";
import {
  availableTiles,
  changeDetail,
  resultDetail,
  agentName,
  agentSource,
  agentState,
  cockpitNetwork,
  cockpitServices,
  formatBytes,
  formatDuration,
  formatDateTime,
  hasDate,
  weekdayShort,
  formatMoney,
  formatMoneyValue,
  moneyRateNote,
  formatMoneyTotal,
  costMoney,
  formatPercent,
  formatQuotaPercent,
  formatRate,
  formatNetworkRate,
  formatNetworkAmount,
  formatNumber,
  formatTemperature,
  formatTokens,
  groupProcessSessions,
  matching,
  mountName,
  observationName,
  pace,
  quotaLabel,
  quotaMethod,
  quotaWindow,
  resetTime,
  result,
  sessionDisplayLabel,
  sessionProject,
  shortDate,
  total,
  value,
} from "./data";
import type {
	AppsView as AppsData,
	AppSample,
  Change,
  Cost,
  Money,
  Energy,
  Host,
  FleetHost,
  Overview,
  Point,
  ProbeDay,
  ProbeSummary,
  PublicProbe,
  Quota,
  Range,
  Result,
  Series,
  Session,
	SessionDetail,
	SpeedSummary,
  SubscriptionComparison,
	WorktreesView,
	TrafficTotal,
  Observations,
  Usage,
} from "./data";
import { browserLiveSource, connectLive } from "./live";
import { ProjectsView, StorageView, InsightsPanel } from "./observations";
import { defaultPreferences, hasPreferences, readPreferences, savePreferences, useCockpitLanguage, SettingsView, ShareView, Explain, formatAppCPU, t } from "./preferences";
import type { Preferences } from "./preferences";
import { activeLanguage, choiceLabel, navigationLabels, plural, sectionLabels, setLanguage, t as message } from "./i18n";
import { TerminalOverview, TerminalTable, TerminalFilter, TerminalKV, TerminalBar } from "./terminal";
import type { MessageKey } from "./i18n";
import "./tokens.css";
import "./style.css";

type View = "overview" | "fleet" | "agents" | "apps" | "projects" | "machine" | "storage" | "services" | "system" | "changes" | "costs" | "uptime" | "settings" | "share" | "glossary";
const views: View[] = ["overview", "fleet", "agents", "apps", "projects", "machine", "services", "system", "changes", "costs", "uptime"];
const terminalViews: View[] = ["overview", "agents", "apps", "projects", "machine", "services", "system", "changes", "costs", "uptime"];
function MonoMark() { return <svg className="terminal-logo" viewBox="0 0 64 64" aria-hidden="true"><rect x="9" y="9" width="28" height="28" rx="8" fill="currentColor" fillOpacity=".35"/><rect x="18" y="18" width="28" height="28" rx="8" fill="currentColor" fillOpacity=".62"/><rect x="27" y="27" width="28" height="28" rx="8" fill="currentColor"/></svg>; }
function currentView(): View {
  const p = location.pathname.slice(1) as View;
  return p === "storage" || p === "settings" || p === "share" || p === "glossary" || views.includes(p) ? p : "overview";
}
async function get<T>(path: string): Promise<T> {
  const r = await fetch(path, { credentials: "same-origin" });
  if (!r.ok)
    throw new Error(
      r.status === 401 ? "auth.required" : `request.failed:${r.status}`,
    );
  return r.json();
}
function displayError(code: string): string {
  if (code === "auth.required") return t("network.sign_in_required");
  if (code === "auth.invalid_token") return t("auth.invalid_token");
  if (code.startsWith("request.failed:")) return t("network.request_failed", {status:code.slice(15)});
  return t("network.unavailable");
}
function browserStorage(): Storage | null {
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}
function useTheme() {
  const [mode, setMode] = useState<"system" | "light" | "dark">(() => {
    try { const saved=browserStorage()?.getItem("fg-theme"); return saved==="light"||saved==="dark"?saved:"system"; } catch { return "system"; }
  });
  const [systemDark,setSystemDark]=useState(() => window.matchMedia("(prefers-color-scheme: dark)").matches);
  useEffect(() => { const media=window.matchMedia("(prefers-color-scheme: dark)"); const update=()=>setSystemDark(media.matches); media.addEventListener("change",update); return ()=>media.removeEventListener("change",update); },[]);
  const theme = mode === "system" ? (systemDark ? "dark" : "light") : mode;
  useEffect(() => {
    document.body.dataset.fg = theme;
    try { browserStorage()?.setItem("fg-theme", mode); } catch { /* Storage may be disabled. */ }
  }, [theme, mode]);
  return [theme, setMode, mode] as const;
}
function useCockpit(host: string, range: Range, authorized: boolean, currency: string) {
  const [overview, setOverview] = useState<Overview | null>(null),
    [agents, setAgents] = useState<Session[]>([]),
    [usage, setUsage] = useState<Usage[]>([]),
    [sessionUsage, setSessionUsage] = useState<Usage[]>([]),
    [models, setModels] = useState<Usage[]>([]),
    [quotas, setQuotas] = useState<Quota[]>([]),
    [costs, setCosts] = useState<Cost[]>([]),
    [modelCosts, setModelCosts] = useState<Cost[]>([]),
    [speed, setSpeed] = useState<SpeedSummary[]>([]),
    [subscriptions, setSubscriptions] = useState<SubscriptionComparison[]>([]),
    [worktrees,setWorktrees] = useState<WorktreesView>({worktrees:[],reclaimable_bytes:0,at:""}),
    [changes, setChanges] = useState<Change[]>([]),
    [energy, setEnergy] = useState<Energy | null>(null),
    [probes, setProbes] = useState<ProbeSummary[]>([]),
	[apps, setApps] = useState<AppsData>({apps:[],history:[],top_hours:[],range}),
	[traffic,setTraffic]=useState<TrafficTotal[]>([]),
	[observations,setObservations]=useState<Observations>({projects:[],storage:[],insights:[],heat:[]}),
    [series, setSeries] = useState<Series>({}),
    [error, setError] = useState(""),
    [last, setLast] = useState(0),
    [stream, setStream] = useState<"connecting" | "live" | "stale">(
      "connecting",
    );
  const requestId = useRef(0);
  const refresh = useCallback(async () => {
    if (!host || !authorized) return;
    const request = ++requestId.current;
    const base = `/api/v1/hosts/${encodeURIComponent(host)}`;
    try {
      const paths = [
        `${base}/overview`,
        `${base}/agents`,
        `${base}/ai/usage?range=${range}&by=project`,
        `${base}/ai/usage?range=${range}&by=model`,
        `${base}/ai/usage?range=${range}&by=session`,
        `${base}/ai/quotas`,
        `${base}/ai/costs?range=${range}&display_currency=${currency}`,
        `${base}/ai/costs?range=${range}&by=model&display_currency=${currency}`,
        `${base}/ai/speed?range=1h`,
        `${base}/ai/subscriptions?display_currency=${currency}`,
        `${base}/worktrees`,
        `${base}/changes?range=1y`,
        `${base}/energy?display_currency=${currency}`,
        `${base}/probes`,
		`${base}/apps?range=${range}`,
		`${base}/traffic`,
		`${base}/observations`,
      ];
      const [o, a, u, m, s, q, c, mc, sp, sub, wt, ch, en, pr, ap, tr, obs] = await Promise.all(
        paths.map((p) => get<unknown>(p)),
      );
      if (request !== requestId.current) return;
      setOverview(o as Overview);
      setAgents(a as Session[]);
      setUsage(u as Usage[]);
      setModels(m as Usage[]);
      setSessionUsage(s as Usage[]);
      setQuotas(q as Quota[]);
      setCosts(c as Cost[]);
      setModelCosts(mc as Cost[]);
      setSpeed(sp as SpeedSummary[]);
      setSubscriptions(sub as SubscriptionComparison[]);
      setWorktrees(wt as WorktreesView);
      setChanges(ch as Change[]);
      setEnergy(en as Energy);
      setProbes(pr as ProbeSummary[]);
	  setApps(ap as AppsData);
	  setTraffic(tr as TrafficTotal[]);
	  setObservations(obs as Observations);
      setLast(new Date((o as Overview).at).getTime());
      try { window.localStorage.setItem("fleetglass:last-data-at", (o as Overview).at); } catch { /* Storage may be disabled. */ }
      setError("");
    } catch (e) {
      if (request === requestId.current) setError((e as Error).message);
    }
  }, [host, range, authorized, currency]);
  useEffect(() => {
    requestId.current++;
    setOverview(null);
    setAgents([]);
    setUsage([]);
    setSessionUsage([]);
    setModels([]);
    setQuotas([]);
    setCosts([]);
    setModelCosts([]);
    setSpeed([]);
    setSubscriptions([]);
    setWorktrees({worktrees:[],reclaimable_bytes:0,at:""});
    setChanges([]);
    setEnergy(null);
    setProbes([]);
	setApps({apps:[],history:[],top_hours:[],range});
	setTraffic([]);
	setObservations({projects:[],storage:[],insights:[],heat:[]});
    setSeries({});
    setLast(0);
    setError("");
  }, [host]);
  useEffect(() => {
    void refresh();
  }, [refresh]);
  const sourceNames = [
    matching(overview?.results || [], "disk:").find(
      (x) => value(x, "used_bytes") !== null,
    )?.name || "",
    matching(overview?.results || [], "sensor:").find(
      (x) => value(x, "celsius") !== null,
    )?.name || "",
	matching(overview?.results || [], "network:").find(x => (value(x, "rx_bytes_per_second") || 0) + (value(x, "tx_bytes_per_second") || 0) > 0)?.name || matching(overview?.results || [], "network:")[0]?.name || "",
	matching(overview?.results || [], "drive:")[0]?.name || "",
	matching(overview?.results || [], "gpu:")[0]?.name || "",
  ].join("|");
  useEffect(() => {
    if (!host || !authorized) return;
    let cancelled = false;
	const [diskName, tempName, networkName, driveName, gpuName] = sourceNames.split("|");
    const base = `/api/v1/hosts/${encodeURIComponent(host)}/series?range=${range}`;
    const queries = [
      ["busy_percent", "busy_percent", "cpu"],
      ["watts", "watts", "cpu_package_power"],
      ["memory", "used_bytes", "memory"],
      ["disk", "used_bytes", diskName],
      ["celsius", "celsius", tempName],
      ["rx_bytes_per_second", "rx_bytes_per_second", networkName],
      ["tx_bytes_per_second", "tx_bytes_per_second", networkName],
	  ["drive_read", "read_bytes_per_second", driveName],
	  ["drive_write", "write_bytes_per_second", driveName],
	  ["gpu_busy", "busy_percent", gpuName],
    ].filter(([, , probe]) => probe);
    Promise.all([
      ...queries.map(
        async ([key, metric, probe]) =>
          [
            key,
            await get<Point[]>(
              `${base}&metric=${encodeURIComponent(metric)}&probe=${encodeURIComponent(probe)}`,
            ),
          ] as const,
      ),
      get<Point[]>(
        `/api/v1/hosts/${encodeURIComponent(host)}/ai/cost-series?range=${range}&display_currency=${currency}`,
      ).then((points) => ["cost", points.some(point => point.money && point.money.converted === null && point.money.currency !== point.money.display_currency) ? [] : points.map(point => ({...point,value:point.money?.converted ?? point.value}))] as const),
    ])
      .then((rows) => {
        if (!cancelled) setSeries(Object.fromEntries(rows));
      })
      .catch(() => {
        if (!cancelled) setSeries({});
      });
    return () => {
      cancelled = true;
    };
  }, [host, range, authorized, sourceNames, currency]);
  useEffect(() => {
    if (!host || !authorized) return;
    return connectLive(
      "/api/v1/live",
      browserLiveSource,
      (data) => {
        const o = data as Overview;
        if (o.host_id === host) {
          setOverview(o);
          setLast(new Date(o.at).getTime());
          void refresh();
        }
      },
      setStream,
    );
  }, [host, authorized, refresh]);
  return {
    host,
    range,
    overview,
    agents,
    usage,
    sessionUsage,
    models,
    quotas,
    costs,
    modelCosts,
    speed,
    subscriptions,
    worktrees,
    changes,
    energy,
    probes,
	apps,
	traffic,
	observations,
    series,
    error,
    last,
    stream,
    refresh,
  };
}
function Tile({
  title,
  source,
  children,
  className = "",
  id,
  explanationKey,
}: {
  title: string;
  source?: string;
  children: React.ReactNode;
  className?: string;
  id?: string;
  explanationKey?: MessageKey;
}) {
  const preferences = readPreferences();
  if (preferences.hiddenSections.some(x => { const key = sectionLabels[x as keyof typeof sectionLabels]; return key ? title.toLowerCase().includes(message(key, preferences.language).toLowerCase()) : false; })) return null;
  return (
    <section id={id} className={`g t ${className}`} aria-label={title}>
      <div className="h">
        <h2>{title}</h2><Explain title={title} language={preferences.language} messageKey={explanationKey}/>
        {source ? <span className="src">{source}</span> : null}
      </div>
      {children}
    </section>
  );
}
function Segmented<T extends string>({
  label,
  choices,
  value,
  onChange,
}: {
  label: string;
  choices: T[];
  value: T;
  onChange: (v: T) => void;
}) {
  return (
    <div className="seg" role="group" aria-label={label}>
      {choices.map((c) => (
        <button
          type="button"
          key={c}
          aria-pressed={value === c}
          onClick={() => onChange(c)}
        >
          {choiceLabel(c)}
        </button>
      ))}
    </div>
  );
}
function List({
  items,
}: {
  items: { name: string; detail?: string; end?: string; state?: string }[];
}) {
  if (!items.length) return <Empty>{t("ui.no.observations.reported.e045a")}</Empty>;
  return <TerminalTable heads={[t("terminal.label_name"),t("terminal.label_detail"),t("terminal.label_value")]} rows={items.map(i=>[
    <span className={i.state==="warn"?"terminal-warn":i.state==="crit"?"terminal-crit":undefined}>{i.name}{i.state==="stale"?" (stale)":""}</span>,
    i.detail ?? "", i.end ?? "",
  ])}/>;
}
function Table({
  heads,
  rows,
  rowIds,
  right = [],
}: {
  heads: string[];
  rows: React.ReactNode[][];
  rowIds?: string[];
  right?: number[];
}) {
  return <TerminalTable heads={heads} rows={rows} rowIds={rowIds} right={right}/>;
}
function Empty({ children }: { children: React.ReactNode }) {
  return <p className="empty">{children}</p>;
}
function Kpi({
  label,
  display,
  detail,
  trend,
  explanationKey,
}: {
  label: string;
  display: string;
  detail: string;
  trend?: number[];
  explanationKey?: MessageKey;
}) {
  const unit = display.match(/^(.*?)(\s?(?:%|W|°C|°F|GiB|MiB|KiB))$/);
  return (
    <Tile title={label} className="k" explanationKey={explanationKey}>
      <span className="num">
        {unit?.[1] || display}
        {unit ? <small>{unit[2]}</small> : null}
      </span>
      <span className="delta">{detail}</span>
      {trend && trend.length > 1 ? (
        <Sparkline values={trend} idLabel={label} />
      ) : null}
    </Tile>
  );
}
function Status({ o, agents }: { o: Overview; agents: Session[] }) {
  const checks = o.results,
    good = checks.filter((r) => r.state === "ok").length,
    issues = o.results.filter((r) => r.state === "warn" || r.state === "crit").sort((a,b)=>{const rank=(r:Result)=>r.state==="crit"?(/^(internal:|status:|backup:)/.test(r.name)?2:1):0;return rank(b)-rank(a)}),
	critical = issues.filter(r=>r.state==="crit").length,
	downChecks=issues.filter(r=>r.state==="crit"&&/^(internal:|status:|backup:)/.test(r.name)).length,
    unknown = o.results.filter((r) => r.state === "unknown").length,
    stale = o.results.filter((r) => r.state === "stale").length,
    notices = issues.length
      ? issues.slice(0, 2)
      : o.results
          .filter((r) => r.state === (stale ? "stale" : "unknown"))
          .slice(0, 1),
    failed = value(result(o.results, "systemd:failed"), "count"),
    containers = matching(o.results, "container:");
  return (
    <section className="g t hero" aria-label={t("ui.status.bae7d")}>
      <div className="hrow">
        <Ring good={good} total={checks.length} />
        <div>
          <p className="verdict">
            {downChecks ? plural(downChecks, "status.check_down.one", "status.check_down.many")
              : critical ? plural(critical, "status.critical.one", "status.critical.many")
              : issues.length ? plural(issues.length, "status.watch.one", "status.watch.many")
              : stale ? plural(stale, "status.stale.one", "status.stale.many")
              : unknown ? plural(unknown, "status.unknown.one", "status.unknown.many")
              : checks.length ? t("status.running_well") : t("status.unknown")}
          </p>
          <p className="sub">{t("status.summary", {reported:plural(checks.length, "status.reported.one", "status.reported.many"), unknown:plural(unknown, "status.unknown_count.one", "status.unknown_count.many"), stale:plural(stale, "status.stale_count.one", "status.stale_count.many"), agents:plural(agents.filter(a => a.state !== "ended").length, "status.agent_count.one", "status.agent_count.many")})}</p>
        </div>
      </div>
      {notices.map((i) => (
        <div className={`issue ${i.state}`} key={i.name}>
          <span className="tag">{i.state === "crit"
            ? t(/^(internal:|status:|backup:)/.test(i.name) ? "status.down" : "status.critical")
            : t(i.state === "warn" ? "status.watch" : i.state === "stale" ? "status.stale" : "status.unknown_tag")}</span>
          <b>{observationName(i.name, i.fields?.display_name)}</b>
          <span>
            {i.state === "stale" && i.at
              ? t("status.last_observed", {at:resetTime(i.at), detail:resultDetail(i)})
              : resultDetail(i)}
          </span>
        </div>
      ))}
      {!notices.length && checks.length ? (
        <p className="status-clear">{t("ui.nothing.needs.attention.b7acc")}</p>
      ) : null}
      <div className="cats">
        <div>
          <b>{failed === null ? "—" : formatNumber(failed)}</b>
          <span>{t("ui.failed.units.cc1be")}</span>
        </div>
        <div>
          <b>{containers.length ? formatNumber(containers.length) : "—"}</b>
          <span>{t("ui.containers.e040a")}</span>
        </div>
        <div>
          <b>{formatNumber(agents.filter((a) => a.state !== "ended").length)}</b>
          <span>{t("ui.agents.64acf")}</span>
        </div>
        <div>
          <b>{formatNumber(issues.length)}</b>
          <span>{t("ui.to.watch.f8978")}</span>
        </div>
      </div>
    </section>
  );
}
function AgentTable({
  agents,
  usage,
  compact = false,
  host = "",
}: {
  agents: Session[];
  usage: Usage[];
  compact?: boolean;
  host?: string;
}) {
  const [selected,setSelected] = useState(() => new URLSearchParams(location.search).get("session") || "");
  const [detail,setDetail] = useState<SessionDetail|null>(null);
  const currency = readPreferences().currency;
  useEffect(()=>{const sync=()=>setSelected(new URLSearchParams(location.search).get("session")||"");window.addEventListener("popstate",sync);return()=>window.removeEventListener("popstate",sync)},[]);
  useEffect(() => {
    if (!host || !selected || compact) return;
    let active = true;
    get<SessionDetail>(`/api/v1/hosts/${encodeURIComponent(host)}/agents/${encodeURIComponent(selected)}?display_currency=${currency}`).then(value=>{if(active)setDetail(value)}).catch(()=>{if(active)setDetail(null)});
    return () => {active=false};
  },[host,selected,compact,currency]);
  const choose = (id:string) => {setSelected(id); history.replaceState(null,"",`/agents?session=${encodeURIComponent(id)}`)};
  const tokens = new Map(usage.map((u) => [u.name, total(u.tokens)]));
  if (!agents.length) return <Empty>{t("ui.no.agent.sessions.reported.f9f1f")}</Empty>;
  return (
    <>

      {!compact && detail?<section className="g t session-detail" id="session-detail"><div className="h"><h2>{t("session.title",{agent:agentName(detail.session.kind)})}</h2><span className="src">{detail.session.model||t("value.unknown_model")}</span></div>
        <TerminalKV rows={[[t("session.start"),hasDate(detail.session.started_at)?formatDateTime(detail.session.started_at):t("value.unknown")],[t("session.end"),hasDate(detail.session.ended_at)?formatDateTime(detail.session.ended_at):t("session.running")],[t("session.duration"),hasDate(detail.session.started_at)?formatDuration(Math.max(0,(new Date(hasDate(detail.session.ended_at)?detail.session.ended_at:Date.now()).getTime()-new Date(detail.session.started_at).getTime())/1000)):t("value.unknown")],[t("ui.cpu.ff221"),detail.session.cpu_seconds?formatDuration(detail.session.cpu_seconds):t("value.unknown")],[t("ui.memory.89c8a"),detail.session.peak_rss_bytes?formatBytes(detail.session.peak_rss_bytes):t("value.unknown")],[t("ui.tokens.c38c6"),detail.usage_measured?formatTokens(total(detail.tokens)):t("value.unknown")],[t("ui.project.f6f4d"),sessionProject(detail.session)],["Session ID",detail.session.id]]}/>
        <h3>{t("session.spawned")}</h3>{detail.processes.length?<ul>{detail.processes.map(p=><li key={p.pid}>{p.name} · {p.pid} · {formatBytes(p.memory_bytes)}{p.left_behind?` · ${t("project.left_behind")}`:""}</li>)}</ul>:<p>{t("session.no_spawned")}</p>}
        <SpeedPanel speed={detail.speed} host={host} session={detail.session.id}/>
      </section>:null}
      <div className="agent-table">
        <Table
          rowIds={agents.map((a) => `session-${a.id}`)}
          right={[3,4,5]}
          heads={
            compact
              ? [t("ui.agent.5ce2e"), t("ui.state.a7250"), t("ui.cpu.ff221"), t("ui.memory.89c8a"), t("ui.tokens.c38c6")]
              : [
                  t("ui.agent.5ce2e"),
                  t("ui.project.worktree.cbe27"),
                  t("ui.state.reason.b5f5d"),
                  t("ui.cpu.by.process.f62e5"),
                  t("ui.memory.by.process.d76ce"),
                  t("ui.tokens.c38c6"),
                  t("ui.model.source.3a815"),
                ]
          }
          rows={agents.map((a) => {
            const first = <button type="button" className="terminal-link" onClick={()=>choose(a.id)}>{sessionDisplayLabel(a)}</button>;
            return compact
              ? [
                  first,
                  <span
                    className={
                      a.state === "waiting-for-input" ? "attention" : ""
                    }
                  >
                    {agentState(a.state)}
                  </span>,
                  a.pid ? (
                    <>{formatPercent(a.cpu_percent || 0)} <TerminalBar percent={a.cpu_percent || 0} width={8}/></>
                  ) : (
                    "Unknown"
                  ),
                  a.pid ? formatBytes(a.rss_bytes || 0) : t("ui.unknown.bc781"),
                  tokens.has(a.id)
                    ? formatTokens(tokens.get(a.id)!)
                    : t("ui.unknown.bc781"),
                ]
              : [
                  first,
                  `${sessionProject(a)} / ${a.worktree || t("value.unknown")}`,
                  <span className={a.state === "erroring" || a.state === "rate-limited" ? "attention" : ""}>{agentState(a.state)}</span>,
                  a.pid ? (
                    <>{formatPercent(a.cpu_percent || 0)} <TerminalBar percent={a.cpu_percent || 0} width={8}/></>
                  ) : (
                    "Unknown"
                  ),
                  a.pid ? formatBytes(a.rss_bytes || 0) : t("ui.unknown.bc781"),
                  tokens.has(a.id)
                    ? formatTokens(tokens.get(a.id)!)
                    : t("ui.unknown.bc781"),
                  `${a.model || t("value.unknown_model")} · ${agentSource(a.source)}`,
                ];
          })}
        />
      </div>
      <p className="foot">
        {" "}{t("ui.sessions.come.from.opentelemetry.and.local.sessi.9a18f")}{" "}</p>
    </>
  );
}
function Quotas({ quotas }: { quotas: Quota[] }) {
  if (!quotas.length) return <Empty>{t("ui.no.quota.observations.reported.42bea")}</Empty>;
  const windowMinutes=(x:Quota)=>x.window_minutes||((x.window==="week"||x.window==="secondary")?10080:300);
  const q = quotas[0],
    p = pace(q);
  return (
    <div className="qwrap">
      <Gauge
        used={q.used_percent}
        paceValue={p}
        label={`${agentName(q.kind)}, ${quotaWindow(q.window, q.window_minutes)}`}
      />
      <div className="qlist">
        <p className="quota-advice">{["codex","claude"].map(kind=>quotas.filter(x=>x.kind===kind).sort((a,b)=>windowMinutes(b)-windowMinutes(a))[0]).filter((x):x is Quota=>Boolean(x)).map(x=>hasDate(x.projected_at)?t(x.used_percent>pace(x)+5?"quota.advice_ahead":"quota.room",{agent:agentName(x.kind),window:quotaWindow(x.window,x.window_minutes),used:formatQuotaPercent(x.used_percent),projected:resetTime(x.projected_at)}):t("quota.no_projection",{agent:agentName(x.kind),window:quotaWindow(x.window,x.window_minutes),used:formatQuotaPercent(x.used_percent),pace:quotaLabel(x)})).join(" · ")}</p>
        {quotas.map((x) => {
          const paceValue = pace(x);
          return (
            <div className="qi" key={x.kind + x.window}>
              <div className="row">
                <span>
                  {agentName(x.kind)}, {quotaWindow(x.window, x.window_minutes)}
                </span>
                <b>{formatQuotaPercent(x.used_percent)}</b>
              </div>
              <div className="bar">
                <i
                  className={x.used_percent > paceValue + 5 ? "att" : ""}
                  style={{
                    width: `${Math.min(100, Math.round(x.used_percent))}%`,
                  }}
                />
                <s style={{ left: `${paceValue}%` }} />
              </div>
              <small>
                <b
                  className={x.used_percent > paceValue + 5 ? "attention" : ""}
                >
                  {quotaLabel(x)}
                </b>{" "}
                {" "}{t("ui.resets.5de1b")}{" "}{resetTime(x.resets_at)} ·{" "}
                {quotaMethod(x.method, x.kind)}
              </small>
            </div>
          );
        })}
      </div>
    </div>
  );
}
function SpeedPanel({speed,host,session}: {speed:SpeedSummary[];host?:string;session?:string}) {
  const [window,setWindow] = useState<"1h"|"24h">("1h");
  const [historical,setHistorical] = useState<SpeedSummary[]>([]);
  useEffect(()=>{if(window!=="24h"||!host)return;let active=true;get<SpeedSummary[]>(`/api/v1/hosts/${encodeURIComponent(host)}/ai/speed?range=24h${session?`&session=${encodeURIComponent(session)}`:""}`).then(rows=>{if(active)setHistorical(rows)}).catch(()=>{if(active)setHistorical([])});return()=>{active=false}},[window,host,session]);
  const visible=window==="24h"?historical:speed;
  return <><div className="speed-controls"><button aria-pressed={window==="1h"} onClick={()=>setWindow("1h")}>{t("speed.one_hour")}</button><button aria-pressed={window==="24h"} onClick={()=>setWindow("24h")}>{t("speed.day")}</button></div>{!visible.length?<Empty>{t("speed.no_data")}</Empty>:<TerminalTable heads={[t("terminal.label_model"),t("ui.method.88306"),t("terminal.label_tokens_second"),t("terminal.label_median"),t("terminal.label_p95"),t("terminal.label_tokens_minute"),t("terminal.label_requests_minute"),t("terminal.label_errors"),t("terminal.label_retries")]} right={[2,3,4,5,6,7,8]} rows={visible.map(s=>[`${agentName(s.kind)} · ${s.model||t("value.unknown_model")}`,s.method,s.live_tokens_per_second===null?"—":`${s.method==="estimated"?"~":""}${formatNumber(s.live_tokens_per_second,1)}`,s.median_tokens_per_second===null?"—":formatNumber(s.median_tokens_per_second,1),s.p95_tokens_per_second===null?"—":formatNumber(s.p95_tokens_per_second,1),formatNumber(s.tokens_per_minute,1),formatNumber(s.requests_per_minute,1),String(s.api_errors),String(s.retries)])}/>}</>;

}
function UsageTile({ usage }: { usage: Usage[] }) {
  const sorted = [...usage].sort((a, b) => total(b.tokens) - total(a.tokens)),
    sum = sorted.reduce((n, u) => n + total(u.tokens), 0),
    daily = new Map<string, number>();
  for (const u of usage)
    for (const b of u.daily)
      daily.set(b.at, (daily.get(b.at) || 0) + total(b.tokens));
  const bars = [...daily]
    .sort(([a], [b]) => a.localeCompare(b))
    .slice(-7)
    .map(([at, v]) => ({
      label: weekdayShort(at),
      value: v,
    }));
  return (
    <>
      <div className="num">
        {formatTokens(sum)}
        <small>{t("ui.tokens.33914")}</small>
      </div>
      {bars.length ? (
        <Bars values={bars} />
      ) : (
        <Empty>{t("ui.no.token.history.for.this.period.0e2da")}</Empty>
      )}
      <div className="proj">
        {sorted.slice(0, 7).map((u) => (
          <div key={u.name}>
            <span>{u.name}</span>
            <span>{formatTokens(total(u.tokens))}</span>
            <i>
              <b
                style={{
                  width: `${sum ? (100 * total(u.tokens)) / total(sorted[0].tokens) : 0}%`,
                }}
              />
            </i>
          </div>
        ))}
      </div>
    </>
  );
}
function RecentErrors({ results, compact = false }: { results: Result[]; compact?: boolean }) {
  const summary = result(results, "recent_errors");
  const groups = matching(results, "recent_error:").sort((a, b) => (value(b, "count") || 0) - (value(a, "count") || 0));
  const oom = matching(results, "oom:");
  const restarts = result(results, "recent_errors:container_restarts");
  const countText = (count: number | null, one: MessageKey, many: MessageKey) => count === null ? t("value.unknown") : plural(count, one, many);
  return <Tile title={t("ui.recent.errors.b0f52")} source={t("ui.journal.last.24.h.acc7b")} className={compact ? "system-compact" : "half"}>
    <p className="system-lead"><strong>{summary?.state === "unknown" ? t("ui.unknown.bc781") : countText(value(summary, "count"), "system.error.one", "system.error.many")}</strong> · {countText(value(summary, "oom_count"), "system.oom.one", "system.oom.many")} · {countText(value(restarts, "count"), "system.container_restart.one", "system.container_restart.many")}</p>
    <List items={groups.slice(0, compact ? 3 : 12).map(g => ({name:g.fields?.unit || t("system.unknown_unit"), detail:g.fields?.message || t("system.no_message"), end:`${value(g, "count") ?? 0}×${value(g, "last_unix_us") ? ` · ${shortDate((value(g,"last_unix_us") || 0)/1000)}` : ""}`, state:g.state}))} />
    {!compact && oom.length ? <List items={oom.map(x => ({name:x.fields?.process || t("system.unknown_process"), detail:t("system.oom_victim"), end:`${value(x,"count") ?? 0}×`, state:x.state}))} /> : null}
    {compact ? <p className="foot">{t("ui.priority.err.and.above.full.text.search.comes.la.7980f")}</p> : null}
  </Tile>;
}
function Security({ results, compact = false }: { results: Result[]; compact?: boolean }) {
  const updates=result(results,"security:updates"), ssh=result(results,"security:ssh_logins"), sudo=result(results,"security:sudo"), failed=result(results,"security:failed_logins"), files=result(results,"security:sensitive_files");
  const bound=matching(results,"port:").filter(x => x.name.endsWith(":any"));
  const unknown=t("value.unknown");
  const items=[
    {name:t("system.pending_updates"),detail:updates?.state === "unknown" ? unknown : t("system.updates_summary", {total:value(updates,"total") ?? unknown, security:value(updates,"security") ?? unknown}),state:updates?.state},
    {name:t("system.reboot_required"),detail:value(updates,"reboot_required") === null ? unknown : value(updates,"reboot_required") ? t("system.yes") : t("system.no"),state:value(updates,"reboot_required") ? "warn" : "ok"},
    {name:t("system.ssh_logins"),detail:ssh?.state === "unknown" ? unknown : t("system.ssh_last", {count:value(ssh,"count") ?? 0, user:ssh?.fields?.last_user || t("system.none"), source:ssh?.fields?.source_class || unknown}),state:ssh?.state},
    {name:t("system.sudo_use"),detail:sudo?.state === "unknown" ? unknown : t("system.in_24h", {count:value(sudo,"count") ?? 0}),state:sudo?.state},
    {name:t("system.failed_logins"),detail:failed?.state === "unknown" ? unknown : t("system.in_24h", {count:value(failed,"count") ?? 0}),state:failed?.state},
    {name:t("system.all_interface_ports"),detail:bound.length ? bound.map(x => x.name.split(":").at(-2)).join(", ") : t("system.none_reported"),state:bound.length ? "warn" : "ok"},
    {name:t("system.sensitive_changes"),detail:files?.state === "unknown" ? unknown : t("system.since_previous", {count:value(files,"changes") ?? 0}),state:files?.state},
  ];
  const compactNames=[t("system.pending_updates"),t("system.failed_logins"),t("system.all_interface_ports"),t("system.sensitive_changes")];
  return <Tile title={t("ui.security.f25ce")} source={t("ui.local.host.24.h.41ae3")} className={compact ? "system-compact" : "half"}>
    <List items={compact ? items.filter(x => compactNames.includes(x.name)) : items} />
    {!compact ? <>
      {matching(results,"security:sudo_user:").length ? <List items={matching(results,"security:sudo_user:").map(x => ({name:x.fields?.user || unknown,detail:t("system.sudo_user"),end:`${value(x,"count") ?? 0}×`}))} /> : null}
      {matching(results,"security:failed_source:").length ? <List items={matching(results,"security:failed_source:").map(x => ({name:x.fields?.source_class || unknown,detail:t("system.failed_source"),end:`${value(x,"count") ?? 0}×`,state:x.state}))} /> : null}
      {matching(results,"security:file_change:").length ? <List items={matching(results,"security:file_change:").map(x => ({name:x.fields?.path || unknown,detail:resultDetail(x),end:x.fields?.sha256?.slice(0,12),state:x.state}))} /> : null}
    </> : null}
  </Tile>;
}
function Backups({ results, compact = false }: { results: Result[]; compact?: boolean }) {
  const items=matching(results,"backup:");
  return <Tile title={t("ui.backups.530cc")} source={t("ui.configured.sets.latest.file.6a4e5")} className={compact ? "system-compact" : "half"}>
    <List items={items.map(x => ({name:x.name.slice(7),detail:x.state === "unknown" ? resultDetail(x) : t("system.backup_latest", {count:value(x,"count") ?? 0, age:formatDuration(value(x,"age_seconds"))}),end:x.state==="crit"?t("system.down"):value(x,"size_bytes") === null ? "" : formatBytes(value(x,"size_bytes")),state:x.state}))} />
  </Tile>;
}
function StatusFiles({results}:{results:Result[]}) {
  const files=matching(results,"status:");
  const probes=matching(results,"internal:");
  const markers=matching(results,"log_marker:");
  return <Tile title={t("ui.status.files.and.internal.probes.46db9")} source={t("ui.configured.checks.79704")} className="half">
    <List items={[...files,...markers,...probes].map(x => ({name:observationName(x.name,x.fields?.display_name),detail:resultDetail(x),end:x.state==="crit"?t("system.down"):x.name.startsWith("status:") || x.name.startsWith("log_marker:") ? formatDuration(value(x,"age_seconds")) : value(x,"latency_ms") !== null ? `${formatNumber(value(x,"latency_ms") || 0)} ms${value(x,"status_code") ? ` · HTTP ${value(x,"status_code")}` : ""}` : "",state:x.state}))} />
  </Tile>;
}
function Repositories({results}:{results:Result[]}) {
  return <Tile title={t("ui.repositories.c91e6")} source={t("ui.local.git.refs.no.fetch.e53ec")} className="half">
    <List items={matching(results,"repository:").map(x => ({name:x.fields?.repository || x.name.split(":").at(-1) || t("ui.repositories.c91e6"),detail:resultDetail(x),end:t("system.repo_commit", {age:formatDuration(value(x,"last_commit_age_seconds"))}),state:x.state}))} />
  </Tile>;
}
function SystemView({data}:{data:ReturnType<typeof useCockpit>}) {
  const r=data.overview?.results || [];
  return <><PageHead title={t("ui.system.bc079")} subtitle={t("ui.security.backups.and.local.service.checks.from.t.46d5c")} /><div className="bento system-view">{matching(r,"security:").length ? <Security results={r} /> : null}{result(r,"recent_errors") ? <RecentErrors results={r} /> : null}{matching(r,"backup:").length ? <Backups results={r} /> : null}{matching(r,"status:").length || matching(r,"log_marker:").length || matching(r,"internal:").length ? <StatusFiles results={r} /> : null}{matching(r,"repository:").length || matching(r,"repositories:").length ? <Repositories results={r} /> : null}</div></>;
}
export function OverviewView({
  data,
  range,
  setRange,
  go,
}: {
  data: ReturnType<typeof useCockpit>;
  range: Range;
  setRange: (r: Range) => void;
  go: (v: View) => void;
}) {
  const o = data.overview;
  if (!o)
    return <Empty>{data.error ? displayError(data.error) : t("network.waiting_sample")}</Empty>;
  const r = o.results,
    mem = result(r, "memory"),
    disk = matching(r, "disk:").find((x) => value(x, "used_bytes") !== null),
    power = result(r, "cpu_package_power"),
    temp = matching(r, "sensor:").find((x) => value(x, "celsius") !== null),
    cpu = result(r, "cpu"),
    network = cockpitNetwork(r),
    containers = matching(r, "container:"),
    services = matching(r, "systemd:").filter(
      (x) => x.name !== "systemd:failed",
    ),
    tls = matching(r, "tls:"),
    liveAgents = data.agents.filter((a) => a.pid && a.state !== "ended"),
    groupedAgents = groupProcessSessions(liveAgents).sort(
      (a, b) => (b.session_count || 1) - (a.session_count || 1),
    ),
    tiles = availableTiles(o, data.costs),
    bottomCount =
      Number(services.length > 0 || tls.length > 0) +
      Number(network.length > 0) +
      Number(containers.length > 0);
  const pct = (x: Result | undefined) => {
    const a = value(x, "used_bytes"),
      b = value(x, "total_bytes");
    return a !== null && b ? formatPercent((100 * a) / b) : t("ui.unknown.bc781");
  };
  const vals = (key: string) => data.series[key]?.map((x) => x.value);
  return (
    <div className="bento">
      <Status o={o} agents={data.agents} />
      <div className="kpis">
        {[
          [
            t("ui.cpu.ff221"),
            formatPercent(value(cpu, "busy_percent")),
            t("overview.busy_now"),
            vals("busy_percent"),
            "explain.cpu",
          ],
          [
            t("ui.memory.89c8a"),
            pct(mem),
            mem
              ? t("overview.of_total", {used:formatBytes(value(mem, "used_bytes")), total:formatBytes(value(mem, "total_bytes"))})
              : t("ui.no.sample.f797c"),
            vals("memory"),
            "explain.memory",
          ],
          [
            t("ui.disk.ef995"),
            pct(disk),
            disk ? mountName(disk.name) : t("ui.no.sample.f797c"),
            vals("disk"),
            undefined,
          ],
          [
            t("overview.cpu_package"),
            value(power, "watts") !== null
              ? `${formatNumber(value(power, "watts") || 0, 1)} W`
              : t("ui.unknown.bc781"),
            t("overview.package_not_wall"),
            vals("watts"),
            "explain.power",
          ],
          [
            t("overview.package_temp"),
            value(temp, "celsius") !== null
              ? formatTemperature(value(temp, "celsius"))
              : t("ui.unknown.bc781"),
            temp?.name.split(":").at(-1) || t("ui.no.sample.f797c"),
            vals("celsius"),
            undefined,
          ],
          [
            t("overview.ai_cost"),
            tiles.includes("cost") ? formatMoneyTotal(data.costs.filter(c => c.method === "api_equivalent").map(costMoney)) : t("ui.unknown.bc781"),
            t("ui.api.equivalent.3504a"),
            vals("cost"),
            "explain.api",
          ],
        ].map(([label, display, detail, trend, explanationKey]) => (
          <Kpi
            key={label as string}
            label={label as string}
            display={display as string}
            detail={detail as string}
            trend={trend as number[] | undefined}
            explanationKey={explanationKey as MessageKey | undefined}
          />
        ))}
      </div>
      <Tile
        title={t("ui.ai.agents.running.now.4d737")}
        source={t("ui.by.process.3b481")}
        className="agents"
      >
        <AgentTable
          agents={groupedAgents.slice(0, 4)}
          usage={data.sessionUsage}
          compact
        />
        {groupedAgents.length > 4 ? (
          <a
            className="tile-link"
            href="/agents"
            onClick={(e) => {
              e.preventDefault();
              go("agents");
            }}
          >
            {" "}{t("ui.view.all.931e1")}{" "}{formatNumber(liveAgents.length)}
          </a>
        ) : null}
      </Tile>
      <Tile
        title={t("ui.quotas.34ed5")}
        source={t("ui.pace.steady.use.until.reset.0fd4b")}
        className="quota"
      >
        <Quotas quotas={data.quotas} />
      </Tile>
      <Tile title={t("ui.cpu.and.package.power.688e3")} className="cpu" explanationKey="explain.cpu">
        <div className="chart-controls">
          <div className="legend">
            <span>
              <i className="brand-line" />
              {" "}{t("ui.cpu.a2ccb")}{" "}</span>
            <span>
              <i className="power-line" />
              {" "}{t("ui.package.w.right.axis.78a38")}{" "}</span>
          </div>
          <Segmented
            label={t("ui.period.170a2")}
            choices={["1h", "12h", "24h", "7d", "30d"]}
            value={range}
            onChange={setRange}
          />
        </div>
        <AreaChart
          primary={data.series.busy_percent || []}
          secondary={data.series.watts}
          label={t("ui.cpu.percent.and.package.power.56857")}
          period={range}
          changes={data.changes}
        />
        {data.energy?.today_seconds ? <p className="foot">{t("ui.cpu.package.only.5747c")}{" "}{formatNumber(data.energy.today_kwh, 3)} {" "}{t("ui.kwh.measured.today.1658a")}</p> : null}
      </Tile>
      <Tile title={t("ui.tokens.c38c6")} source={t("overview.by_project_range", {range})} className="tokens">
        <UsageTile usage={data.usage} />
      </Tile>
      <div className="overview-system-row">
      {data.probes.length ? <AvailabilityTile probes={data.probes} go={go} /> : null}
      {tiles.includes("recent_errors") ? <RecentErrors results={r} compact /> : null}
      {tiles.includes("security") ? <Security results={r} compact /> : null}
      {tiles.includes("backups") ? <Backups results={r} compact /> : null}
      {data.changes.filter(c=>Date.now()-new Date(c.at).getTime()<=({"1h":36e5,"12h":12*36e5,"24h":864e5,"7d":7*864e5,"30d":30*864e5}[range])).length ? <Tile title={t("ui.what.changed.7e4b9")} source={t("ui.selected.period.7cc59")} className="system-compact"><List items={data.changes.filter(c=>Date.now()-new Date(c.at).getTime()<=({"1h":36e5,"12h":12*36e5,"24h":864e5,"7d":7*864e5,"30d":30*864e5}[range])).slice(0,4).map(c => ({name:changeLabel(c.type),detail:changeDetail(c),end:shortDate(c.at)}))} /><a className="tile-link" href="/changes" onClick={e => {e.preventDefault();go("changes")}}>{t("ui.view.all.changes.cee9a")}</a></Tile> : null}
      </div>
      <div className={`overview-bottom-row count-${bottomCount}`}>
      {services.length || tls.length ? (
        <Tile
          title={t("ui.services.and.certificates.e34a8")}
          source={t("ui.systemd.tls.dfd8b")}
          className="third"
        >
          <List
            items={cockpitServices(services, tls).map((s) =>
              s.name.startsWith("tls:")
                ? {
                    name: s.name.slice(4),
                    detail: t("ui.certificate.735ad"),
                    end: resultDetail(s),
                    state: s.state,
                  }
                : { name: s.name.slice(8), detail: resultDetail(s), state: s.state },
            )}
          />
        </Tile>
      ) : null}
      {network.length ? (
        <Tile
          title={t("ui.network.53ebc")}
          source={t("ui.per.interface.now.39f4b")}
          className="third"
        >
          <List
            items={network.map((n) => ({
              name: n.name.slice(8),
              detail: t("machine.in_out", {inbound:formatNetworkRate(value(n, "rx_bytes_per_second")), outbound:formatNetworkRate(value(n, "tx_bytes_per_second"))}),
              end: t("overview.received", {amount:formatNetworkAmount(value(n, "rx_bytes"))}),
              state: n.state,
            }))}
          />
        </Tile>
      ) : null}
      {containers.length ? (
        <Tile
          title={t("ui.containers.e040a")}
          source={t("ui.docker.9f114")}
          className="third"
        >
          <List
            items={containers.slice(0, 8).map((c) => ({
              name: c.name.split(":").slice(2).join(":"),
              detail: resultDetail(c),
              state: c.state,
            }))}
          />
        </Tile>
      ) : null}
      </div>
      <InsightsPanel insights={data.observations.insights} compact />
      <Tile title={t("speed.title")} source={t("speed.last_hour")} className="full"><SpeedPanel speed={data.speed} host={data.host}/></Tile>
    </div>
  );
}
function changeLabel(type: Change["type"]): string {
  return t(({APT:"change.apt",Service:"change.service",Boot:"change.boot",Git:"change.git",Container:"change.container",Marker:"change.marker",Heat:"change.heat"} as const)[type]);
}
function PageHead({
  title,
  subtitle,
  range,
  setRange,
}: {
  title: string;
  subtitle: string;
  range?: Range;
  setRange?: (r: Range) => void;
}) {
  return (
    <div className="page-head">
      <div>
        <h1>{title}</h1>
        <p>{subtitle}</p>
      </div>
      {range && setRange ? (
        <Segmented
          label={t("ui.period.170a2")}
          choices={["1h", "12h", "24h", "7d", "30d"]}
          value={range}
          onChange={setRange}
        />
      ) : null}
    </div>
  );
}
function AgentsView({ data }: { data: ReturnType<typeof useCockpit> }) {
  const [kind, setKind] = useState("All"),
    [project, setProject] = useState("All"),
    [state, setState] = useState("All"),
    [query,setQuery] = useState("");
  const agents = data.agents,
    projects = ["All", ...new Set(agents.map(sessionProject))],
    filtered = agents.filter(
      (a) =>
        (kind === "All" || a.kind === kind) &&
        (project === "All" || sessionProject(a) === project) &&
        (state === "All" || a.state === state) &&
        `${a.id} ${a.label??""} ${a.project??""} ${a.model??""}`.toLowerCase().includes(query.toLowerCase()),
    );
  return (
    <>
      <PageHead
        title={t("ui.agents.64acf")}
        subtitle={t("ui.every.reported.session.including.unknown.and.ina.203c5")}
      />
      <div className="bento">
        <Tile
          id="sessions"
          title={t("ui.sessions.e11e3")}
          source={t("agents.filtered_count", {shown:filtered.length,total:agents.length})}
          className="full"
        >
          <div className="terminal-filters"><TerminalFilter label={t("ui.agent.5ce2e")} choices={["All",...new Set(agents.map(a=>a.kind))]} value={kind} onChange={setKind} display={v=>v==="All"?choiceLabel(v):agentName(v)}/><TerminalFilter label={t("ui.project.f6f4d")} choices={projects} value={project} onChange={setProject} display={v=>v==="All"?choiceLabel(v):v}/><TerminalFilter label={t("ui.state.a7250")} choices={["All",...new Set(agents.map(a=>a.state))]} value={state} onChange={setState} display={v=>v==="All"?choiceLabel(v):agentState(v)}/><label>{t("terminal.search")} <input type="search" value={query} onChange={e=>setQuery(e.target.value)} placeholder="/"/></label></div>
          <AgentTable agents={filtered} usage={data.sessionUsage} host={data.host} />
        </Tile>
        <Tile title={t("speed.title")} source={t("speed.last_hour")} className="full"><SpeedPanel speed={data.speed} host={data.host}/></Tile>
      </div>
    </>
  );
}
function MachineView({
  data,
  range,
  setRange,
  storage,
  onStorageChange,
}: {
  data: ReturnType<typeof useCockpit>;
  range: Range;
  setRange: (r: Range) => void;
  storage: boolean;
  onStorageChange: (storage: boolean) => void;
}) {
	const [machineSection,setMachineSection]=useState("Overview");
  const section = storage ? "Storage" : machineSection;
  const r = data.overview?.results || [],
    cpu = result(r, "cpu"),
    memory = result(r, "memory"),
    power = result(r, "cpu_package_power");
  return (
    <>
      <PageHead
        title={t("ui.machine.65bb1")}
        subtitle={t("ui.hardware.and.resource.measurements.from.the.sele.51456")}
        range={range}
        setRange={setRange}
      />
	  <div className="subsystem-nav"><Segmented label={t("ui.subsystem.b1a08")} choices={["Overview","CPU","Memory","Disk","Network",...(matching(r,"gpu:").length?["GPU"]:[]),"Containers","Storage"]} value={section} onChange={(choice) => { if (choice !== "Storage") setMachineSection(choice); onStorageChange(choice === "Storage"); }}/></div>
      {data.observations.heat.length?<section className="g observation-heat"><div className="h"><h2>{t("ui.heat.attribution.1afc5")}</h2><span className="src">{t("ui.at.threshold.crossing.top.cpu.apps.5da3e")}</span></div>{data.observations.heat.slice(0,3).map(x=><p key={x.id}><time dateTime={x.at}>{resetTime(x.at)}</time> · {x.text}</p>)}</section>:null}
	  {section==="Storage" ? <StorageView data={data.observations} /> : section==="Overview" ? (
      <div className="bento">
        <Tile
          title={t("ui.cpu.and.package.power.688e3")}
          explanationKey="explain.cpu"
          source={t("ui.cpu.package.w.0d73c")}
          className="half"
        >
          <div className="stat-line">
            {formatPercent(value(cpu, "busy_percent"))} <small>{t("ui.busy.now.20bdd")}</small>{" "}
            ·{" "}
            {value(power, "watts") !== null
              ? `${formatNumber(value(power, "watts") || 0, 1)} W`
              : t("ui.unknown.bc781")}
          </div>
          <AreaChart
            primary={data.series.busy_percent || []}
            secondary={data.series.watts}
            label={t("ui.cpu.and.package.power.688e3")}
            period={range}
            changes={data.changes}
          />
          {data.energy?.today_seconds ? <p className="foot">{t("ui.cpu.package.only.5747c")}{" "}{formatNumber(data.energy.today_kwh, 3)} {" "}{t("ui.kwh.measured.today.1658a")}</p> : null}
        </Tile>
        <Tile title={t("ui.memory.and.swap.3591a")} explanationKey="explain.memory" source={t("ui.bytes.daf52")} className="half">
          <div className="stat-grid">
            {[
              [t("machine.memory_used"), formatBytes(value(memory, "used_bytes"))],
              [t("machine.memory_total"), formatBytes(value(memory, "total_bytes"))],
              [t("machine.swap_used"), formatBytes(value(memory, "swap_used_bytes"))],
              [t("machine.swap_total"), formatBytes(value(memory, "swap_total_bytes"))],
            ].map(([k, v]) => (
              <div key={k}>
                <span>{k}</span>
                <b>{v}</b>
              </div>
            ))}
          </div>
        </Tile>
        <Tile title={t("ui.disks.and.inodes.d72dc")} className="half">
          <List
            items={matching(r, "disk:").map((d) => ({
              name: mountName(d.name),
              detail: `${formatBytes(value(d, "used_bytes"))} / ${formatBytes(value(d, "total_bytes"))}`,
              end: t("machine.inodes", {used:value(d, "inodes_used") ?? t("value.unknown"), total:value(d, "inodes_total") ?? t("value.unknown")}),
              state: d.state,
            }))}
          />
        </Tile>
        <Tile title={t("ui.temperatures.and.fans.014e2")} className="half">
          <List
            items={matching(r, "sensor:").map((s) => ({
              name: s.name.split(":").slice(1).join(" · "),
              detail:
                value(s, "celsius") !== null
                  ? formatTemperature(value(s, "celsius"), 1)
                  : value(s, "rpm") !== null
                    ? `${value(s, "rpm")} rpm`
                    : t("ui.unknown.bc781"),
              end: statusLabel(s.state),
              state: s.state,
            }))}
          />
        </Tile>
        <Tile title={t("ui.smart.disk.health.f1b19")} className="half">
          <List
            items={matching(r, "smart:").map((s) => ({
              name: s.name.slice(6),
              detail: [
                s.detail,
                value(s, "wear_percent") !== null
                  ? t("machine.wear", {percent:formatPercent(value(s, "wear_percent"))})
                  : "",
                value(s, "media_errors") !== null
                  ? t("machine.media_errors", {count:value(s, "media_errors") ?? 0})
                  : "",
              ]
                .filter(Boolean)
                .join(" · "),
              end: statusLabel(s.state),
              state: s.state,
            }))}
          />
        </Tile>
        <Tile
          title={t("ui.network.interfaces.ae625")}
          source={t("ui.rates.now.totals.since.boot.a9730")}
          className="half"
        >
          <List
            items={matching(r, "network:").map((n) => ({
              name: n.name.slice(8),
              detail: t("machine.in_out", {inbound:formatNetworkRate(value(n, "rx_bytes_per_second")), outbound:formatNetworkRate(value(n, "tx_bytes_per_second"))}),
              end: `${formatNetworkAmount(value(n, "rx_bytes"))} / ${formatBytes(value(n, "tx_bytes"))}`,
              state: n.state,
            }))}
          />
        </Tile>
      </div>
	  ) : <SubsystemPanel section={section} data={data} range={range}/>}
    </>
  );
}
function SubsystemPanel({section,data,range}:{section:string;data:ReturnType<typeof useCockpit>;range:Range}) {
	const r=data.overview?.results||[];
	const cpu=result(r,"cpu"),mem=result(r,"memory"),load=result(r,"load");
	const candidates=data.apps.apps.filter(a=>section==="GPU"?false:section==="Containers"?a.kind==="container":section==="Network"?a.network_rx_bytes_per_second!==undefined:true).sort((a,b)=>{
		const measure=(x:AppSample)=>section==="CPU"?x.cpu_percent:section==="Memory"?x.memory_bytes:section==="Disk"?x.disk_read_bytes_per_second+x.disk_write_bytes_per_second:section==="Network"?(x.network_rx_bytes_per_second||0)+(x.network_tx_bytes_per_second||0):x.cpu_percent;
		return measure(b)-measure(a);
	});
	const relevant=section==="Containers"?candidates.slice(0,6):[...candidates.filter(a=>a.kind!=="other").slice(0,5),...candidates.filter(a=>a.kind==="other")];
	const metric=section==="CPU"?"busy_percent":section==="Memory"?"memory":section==="Disk"?"drive_write":section==="Network"?"rx_bytes_per_second":section==="GPU"?"gpu_busy":"busy_percent";
	const metrics:Record<string,[string,string,MessageKey?][]>={
		CPU:[[t("machine.busy"),formatPercent(value(cpu,"busy_percent"))],[t("machine.user"),formatPercent(value(cpu,"user_percent"))],[t("machine.system"),formatPercent(value(cpu,"system_percent"))],[t("machine.iowait"),formatPercent(value(cpu,"iowait_percent")),"explain.iowait"],[t("machine.steal"),formatPercent(value(cpu,"steal_percent")),"explain.steal"],[t("machine.load"),["1m","5m","15m"].map(k=>value(load,k) === null ? t("value.unknown") : formatNumber(value(load,k)!, 2)).join(" / "),"explain.load"]],
		Memory:[[t("machine.used"),formatBytes(value(mem,"used_bytes"))],[t("machine.page_cache"),formatBytes(value(mem,"page_cache_bytes")),"explain.page_cache"],[t("machine.buffers"),formatBytes(value(mem,"buffers_bytes"))],[t("machine.shared"),formatBytes(value(mem,"shared_bytes"))],[t("machine.swap"),formatBytes(value(mem,"swap_used_bytes")),"explain.swap"],[t("machine.zram"),formatBytes(value(mem,"zram_memory_bytes")),"explain.zram"],[t("machine.pressure"),value(mem,"pressure_some_avg10")===null?t("ui.unknown.bc781"):formatPercent(value(mem,"pressure_some_avg10")),"explain.pressure"]],
	};
	const rows=section==="Disk"?matching(r,"drive:").map(d=>({name:d.name.slice(6),detail:t("machine.read_write", {read:formatRate(value(d,"read_bytes_per_second")), write:formatRate(value(d,"write_bytes_per_second"))}),end:t("machine.written_today", {size:formatBytes(value(d,"written_today_bytes"))})})):section==="Network"?matching(r,"network:").map(n=>({name:n.name.slice(8),detail:t("machine.in_out", {inbound:formatNetworkRate(value(n,"rx_bytes_per_second")), outbound:formatNetworkRate(value(n,"tx_bytes_per_second"))}),end:t("machine.since_boot", {amount:`${formatNetworkAmount(value(n,"rx_bytes"))} / ${formatNetworkAmount(value(n,"tx_bytes"))}`})})):section==="GPU"?matching(r,"gpu:").map(g=>({name:g.name.slice(4),detail:formatPercent(value(g,"busy_percent")),end:statusLabel(g.state)})):section==="Containers"?matching(r,"container:").map(c=>({name:c.name.split(":").slice(2).join(":"),detail:resultDetail(c),end:t("machine.restarts", {count:value(c,"restart_count")??0}),state:c.state})):[];
	return <div className="bento subsystem-grid">
		<Tile title={t("machine.now", {section:choiceLabel(section)})} source={t("ui.host.measurements.082dd")} className="half">
			{metrics[section]?<div className="stat-grid">{metrics[section].map(([label,v,explanationKey])=><div key={label}><span>{label}<Explain title={label} language={readPreferences().language} messageKey={explanationKey}/></span><b>{v}</b></div>)}</div>:rows.length?<List items={rows}/>:<Empty>{t("ui.no.measurements.reported.079d3")}</Empty>}
			{section==="CPU"?Object.entries(cpu?.values||{}).filter(([k])=>/^core_\d+_percent$/.test(k)).length?<div className="core-grid">{Object.entries(cpu?.values||{}).filter(([k])=>/^core_\d+_percent$/.test(k)).map(([k,v])=><div key={k}><span>{t("machine.core", {number:k.replace("core_", "").replace("_percent", "")})}</span><TerminalBar percent={v} width={12}/><b>{formatPercent(v)}</b></div>)}</div>:null:null}
		</Tile>
		<Tile title={t("ui.history.90ccd")} source={`${t(section === "Disk" ? "machine.drive_write" : section === "Network" ? "machine.receive" : section === "Memory" ? "machine.used_bytes" : section === "CPU" ? "ui.cpu.a2ccb" : section === "GPU" ? "machine.gpu_percent" : "machine.host_cpu")} · ${range}`} className="half"><AreaChart primary={data.series[metric]||[]} label={t("machine.history", {section:choiceLabel(section)})} period={range} changes={data.changes} unit={section==="Memory"?"bytes":section==="Disk"||section==="Network"?"rate":"percent"}/></Tile>
		{section==="Disk"?<Tile title={t("ui.volumes.39c0e")} className="half"><List items={matching(r,"disk:").map(d=>({name:mountName(d.name),detail:t("machine.disk_space", {used:formatBytes(value(d,"used_bytes")), free:formatBytes((value(d,"total_bytes")??0)-(value(d,"used_bytes")??0))}),end:formatBytes(value(d,"total_bytes")),state:d.state}))}/></Tile>:null}
		{section==="Memory"?<Tile title={t("ui.memory.pressure.ff290")} explanationKey="explain.pressure" className="half"><p className="foot">{t("ui.psi.reports.the.share.of.time.tasks.waited.for.m.0bf4c")}</p><div className="stat-grid">{["avg10","avg60","avg300"].map(k=><div key={k}><span>{k}</span><b>{value(mem,"pressure_some_"+k)===null?t("ui.unknown.bc781"):formatPercent(value(mem,"pressure_some_"+k))}</b></div>)}</div></Tile>:null}
		{section==="Network"?<Tile title={t("ui.traffic.by.interface.c13ca")} source={t("ui.measured.counter.changes.b99ac")} className="half"><List items={data.traffic.map(traffic=>({name:traffic.name.slice(8),detail:t("machine.traffic_today", {inbound:formatNetworkAmount(traffic.today_rx_bytes),outbound:formatNetworkAmount(traffic.today_tx_bytes)}),end:`7 d ${formatNetworkAmount(traffic.week_rx_bytes+traffic.week_tx_bytes)} · 30 d ${formatNetworkAmount(traffic.month_rx_bytes+traffic.month_tx_bytes)}`}))}/></Tile>:null}
		{section==="Containers"?<Tile title={t("ui.container.resources.9b1e1")} source={t("ui.per.container.289a4")} className="half"><TerminalTable heads={[t("terminal.label_name"),t("ui.cpu.ff221"),t("ui.memory.89c8a"),t("ui.network.53ebc"),t("ui.block.i.o.884a2"),t("ui.restarts.81727")]} right={[1,2,3,4,5]} rows={data.apps.apps.filter(a=>a.kind==="container").map(a=>[a.name,formatAppCPU(a.cpu_percent),formatBytes(a.memory_bytes),a.network_rx_bytes_per_second===undefined?t("ui.not.measured.6e95b"):formatNetworkRate(a.network_rx_bytes_per_second+(a.network_tx_bytes_per_second||0)),formatRate(a.disk_read_bytes_per_second+a.disk_write_bytes_per_second),String(value(matching(r,"container:").find(x=>x.name.endsWith(":"+a.name)),"restart_count")??t("value.unknown"))])}/></Tile>:null}
		{section==="CPU"||section==="GPU"?<Tile title={t("ui.measurement.notes.bdd57")} explanationKey="explain.coverage" source={t("ui.coverage.6dafb")} className="half measurement-notes"><p className="foot">{section === "CPU" ? t("machine.cpu_notes", {cpu:formatPercent(value(cpu,"busy_percent")), cores:value(cpu,"core_count") ?? t("value.unknown"), power:value(result(r,"cpu_package_power"),"watts") === null ? t("value.unknown") : formatNumber(value(result(r,"cpu_package_power"),"watts")!, 1)}) : t("machine.gpu_notes")}</p></Tile>:null}
		<Tile title={t("ui.apps.behind.this.figure.612b6")} source={t("ui.measured.now.7782e")} className="half">{section==="GPU"?<Empty>{t("ui.per.app.gpu.use.is.not.measured.by.this.driver.f2aef")}</Empty>:<List items={relevant.map(a=>({name:a.kind === "other" ? t("app.other") : a.name,detail:choiceLabel(a.kind[0].toUpperCase()+a.kind.slice(1)),end:section==="Memory"?formatBytes(a.memory_bytes):section==="Disk"?formatRate(a.disk_read_bytes_per_second+a.disk_write_bytes_per_second):section==="Network"?a.network_rx_bytes_per_second===undefined?t("ui.not.measured.6e95b"):formatNetworkRate(a.network_rx_bytes_per_second+(a.network_tx_bytes_per_second||0)):formatAppCPU(a.cpu_percent)}))}/>}</Tile>
	</div>;
}
function appMetric(app:AppSample, metric:string):number|null {
  if (metric === "Energy") return app.estimated_energy_watts ?? null;
  if (!app.rates_ready && metric !== "Memory") return null;
  switch (metric) {
    case "CPU": return app.cpu_percent;
    case "Memory": return app.memory_bytes;
    case "Disk": return app.disk_read_bytes_per_second + app.disk_write_bytes_per_second;
    case "Network": return app.network_rx_bytes_per_second === undefined ? null : app.network_rx_bytes_per_second + (app.network_tx_bytes_per_second || 0);
    default: return null;
  }
}
function appValue(app:AppSample, metric:string):string {
  const amount = appMetric(app, metric);
  if (amount === null) return app.rates_ready ? t("ui.not.measured.6e95b") : t("ui.waiting.33d30");
  if (metric === "CPU") return formatPercent(amount);
  if (metric === "Memory") return formatBytes(amount);
  if (metric === "Energy") return `${formatNumber(amount, 1)} W ${t("quota.estimated")}`;
  return formatRate(amount);
}
function appHourLabel(at:string):string { return resetTime(at); }
function AppCatalog({data,range,setRange}:{data:ReturnType<typeof useCockpit>;range:Range;setRange:(r:Range)=>void}) {
  const [metric,setMetric]=useState("CPU");
  const [query,setQuery]=useState("");
  const [kind,setKind]=useState("All types");
  const [selected,setSelected]=useState<string|null>(()=>new URLSearchParams(location.search).get("app"));
  const [show48,setShow48]=useState(false);
  const all=data.apps.apps;
  const other=all.filter(a=>a.kind==="other");
  const ranked=all.filter(a=>a.kind!=="other" && a.name.toLocaleLowerCase().includes(query.toLocaleLowerCase()) && (kind==="All types" || a.kind===kind.toLowerCase()))
    .sort((a,b)=>(appMetric(b,metric)??-1)-(appMetric(a,metric)??-1) || a.name.localeCompare(b.name));
  const apps=[...ranked,...other];
  const max=Math.max(1,...apps.map(a=>appMetric(a,metric)??0));
  const current=all.find(a=>`${a.kind}:${a.name}`===selected);
  const topHours=[...data.apps.top_hours].reverse();
  return <>
    <PageHead title={t("ui.apps.d9a36")} subtitle={t("ui.every.running.process.attributed.to.a.unit.conta.6b1d2")} range={range} setRange={setRange}/>
    <div className="app-intro g"><div><strong>{formatNumber(all.filter(a=>a.kind!=="other").length)}</strong><span>{t("ui.app.groups.measured.8135f")}</span></div><p>{t("ui.cpu.is.a.percentage.of.one.core.current.shares.a.69782")}</p></div>
    <div className="app-section-head"><h2>{t("ui.resource.owners.a20f8")}</h2><Segmented label={t("ui.rank.apps.by.bd839")} choices={["CPU","Memory","Disk","Network","Energy"]} value={metric} onChange={setMetric}/></div>
    <div className="app-filters terminal-filters"><label>{t("terminal.search")} <input type="search" value={query} onChange={e=>setQuery(e.target.value)} placeholder="/"/></label><TerminalFilter label={t("ui.type.3deb7")} choices={["All types","Unit","Container","Agent"]} value={kind} onChange={setKind}/><span>{plural(ranked.length, "app.matching.one", "app.matching.many")}</span></div>
    <section className="app-list g" aria-label={t("ui.ranked.apps.887df")}>
      <TerminalTable heads={[t("ui.app.fc4a6"),t("ui.cpu.ff221"),t("ui.memory.89c8a"),t("ui.disk.read.write.23868"),t("ui.network.in.out.e4044"),t("ui.energy.estimate.48ed6"),t("ui.share.09ca5")]} right={[1,2,3,4,5]} rows={apps.map(a=>{const id=`${a.kind}:${a.name}`;const amount=appMetric(a,metric);return [<button className="terminal-link" type="button" onClick={()=>setSelected(selected===id?null:id)} aria-expanded={selected===id}>{a.kind==="other"?t("app.other"):a.name}</button>,a.rates_ready?formatAppCPU(a.cpu_percent):t("ui.waiting.33d30"),formatBytes(a.memory_bytes),a.rates_ready?`${formatRate(a.disk_read_bytes_per_second)} / ${formatRate(a.disk_write_bytes_per_second)}`:"—",a.network_rx_bytes_per_second===undefined?t("ui.not.measured.6e95b"):`${formatNetworkRate(a.network_rx_bytes_per_second)} / ${formatNetworkRate(a.network_tx_bytes_per_second??0)}`,a.estimated_energy_watts===undefined?"—":`${formatNumber(a.estimated_energy_watts,1)} W`,<TerminalBar percent={amount===null?null:100*amount/max} width={10}/>]})}/>
      {ranked.length===0&&all.length>0?<p className="foot">{t("ui.no.apps.match.this.search.or.type.584a6")}</p>:null}
    </section>
    {current?<section className="app-detail g" aria-label={t("app.details", {name: current.name})}><div className="h"><h2>{current.name}</h2><button type="button" onClick={()=>setSelected(null)}>{t("ui.close.bbfa7")}</button></div><TerminalKV rows={[[t("ui.type.3deb7"),current.kind],[choiceLabel(metric),appValue(current,metric)],[t("ui.disk.today.6bb6e"),`${formatBytes(current.disk_read_bytes)} / ${formatBytes(current.disk_write_bytes)}`],[t("terminal.label_page_cache"),current.counter_source==="cgroup"?formatBytes(current.page_cache_bytes):t("ui.not.measured.6e95b")]]}/><TerminalTable heads={[t("ui.processes.ed730"),t("ui.pid.08323")]} right={[1]} rows={(current.processes??[]).map(p=>[p.name,String(p.pid)])}/></section>:null}
    <section className="app-hourly g"><div className="h"><h2>{t("ui.top.apps.hour.by.hour.367fc")}</h2><span className="src">{t("ui.last.48.hours.11ccc")}</span></div>{topHours.length?<><TerminalTable heads={[t("terminal.label_time"),t("ui.cpu.ff221"),t("ui.memory.89c8a"),t("ui.disk.ef995"),t("ui.network.53ebc")]} rows={topHours.slice(0,show48?48:12).map(h=>[appHourLabel(h.at),h.cpu||t("ui.not.measured.6e95b"),h.memory||t("ui.not.measured.6e95b"),h.disk||t("ui.not.measured.6e95b"),h.network||t("ui.not.measured.6e95b")])}/>{topHours.length>12?<button className="terminal-link" type="button" onClick={()=>setShow48(!show48)}>{t(show48?"app.show_12h":"app.show_48h")}</button>:null}</>:<Empty>{t("ui.hourly.leaders.appear.after.an.hour.of.measureme.092ce")}</Empty>}</section>

  </>;
}
function ServicesView({ data }: { data: ReturnType<typeof useCockpit> }) {
  const r = data.overview?.results || [];
  return (
    <>
      <PageHead
        title={t("ui.services.5cbd5")}
        subtitle={t("ui.system.units.containers.ports.and.certificates.388a0")}
      />
      <div className="bento">
        <Tile title={t("ui.systemd.units.dc52c")} source={t("ui.configured.units.616b0")} className="half">
          <List
            items={matching(r, "systemd:").map((s) => ({
              name: s.name.slice(8),
              detail: resultDetail(s),
              end: statusLabel(s.state),
              state: s.state,
            }))}
          />
        </Tile>
        <Tile title={t("ui.docker.containers.ba4cd")} source={t("ui.per.socket.807ef")} className="half">
          <List
            items={matching(r, "container:").map((s) => ({
              name: s.name.split(":").slice(2).join(":"),
              detail: `socket ${s.name.split(":")[1]} · ${s.detail}`,
              end: statusLabel(s.state),
              state: s.state,
            }))}
          />
        </Tile>
        <Tile
          title={t("ui.listening.ports.e2117")}
          source={t("ui.by.bound.address.9bdff")}
          className="half"
        >
          {matching(r, "port:").length ? (
            (["loopback", "specific", "any"] as const).map((group) => {
              const ports = matching(r, "port:").filter((s) =>
                s.name.endsWith(":" + group),
              );
              return ports.length ? (
                <div className="port-group" key={group}>
                  <h3>
                    {group === "any"
                      ? t("ui.all.interfaces.8d058")
                      : group === "specific"
                        ? "Specific address"
                        : "Loopback"}{" "}
                    <small>{formatNumber(ports.length)}</small>
                  </h3>
                  <List
                    items={ports.map((s) => ({
                      name: s.name.split(":").slice(1, -1).join(" · "),
                      detail: (()=>{const port=value(s,"port");const owner=data.observations.projects.find(p=>port!==null&&p.ports.includes(port));return owner?`${owner.project} / ${owner.name}`:resultDetail(s)})(),
                      end: statusLabel(s.state),
                      state: s.state,
                    }))}
                  />
                </div>
              ) : null;
            })
          ) : (
            <Empty>{t("ui.no.listening.ports.reported.bcaa5")}</Empty>
          )}
        </Tile>
        <Tile title={t("ui.tls.certificates.c50be")} className="half">
          <List
            items={matching(r, "tls:").map((s) => ({
              name: s.name.slice(4),
              detail: resultDetail(s),
              end: statusLabel(s.state),
              state: s.state,
            }))}
          />
        </Tile>
      </div>
    </>
  );
}
function ChangesView({data}:{data:ReturnType<typeof useCockpit>}) {
  const [kind,setKind]=useState("All");
  const [period,setPeriod]=useState("24h");
  const duration={"24h":864e5,"7d":7*864e5,"30d":30*864e5,"1y":365*864e5}[period] || 864e5;
  const rows=data.changes.filter(c=>(kind==="All"||c.type===kind)&&Date.now()-new Date(c.at).getTime()<=duration);
  return <><div className="page-head"><div><h1>{t("ui.changes.8aa57")}</h1><p>{t("ui.package.service.boot.merge.container.and.configu.fe439")}</p></div><Segmented label={t("ui.change.period.aaa89")} choices={["24h","7d","30d","1y"]} value={period} onChange={setPeriod}/></div>
    <div className="bento compact-rows"><Tile title={t("ui.change.journal.642e1")} source={t("change.local_history", {count:rows.length})} className="half">
      <div className="terminal-filters"><TerminalFilter label={t("ui.type.3deb7")} choices={["All","APT","Service","Boot","Git","Container","Marker","Heat"]} value={kind} onChange={setKind}/></div>
      {rows.length ? <List items={rows.map(c=>({name:changeLabel(c.type),detail:changeDetail(c),end:shortDate(c.at)}))} /> : <Empty>{t("ui.no.changes.in.this.period.88120")}</Empty>}
    </Tile></div></>;
}
function costMethodLabel(method: string): string {
  return method === "api_equivalent" ? t("ui.api.equivalent.3504a")
    : method === "cli_reported" ? t("cost.cli_reported")
    : method === "subscription_allocated" ? t("cost.subscription_allocated") : method;
}
function MoneyStat({money,amount,currency}: {money?:Money;amount:number;currency:string}) {
  if (!money) return <b>{formatMoney(amount,currency)}</b>;
  return <b>{formatMoneyValue(money)}</b>;
}
function CostsView({
  data,
  range,
  setRange,
}: {
  data: ReturnType<typeof useCockpit>;
  range: Range;
  setRange: (r: Range) => void;
}) {
  const converted = [...data.costs.map(costMoney), ...data.modelCosts.map(costMoney), ...data.subscriptions.flatMap(x=>[x.api_money,x.subscription_money].filter((m):m is Money=>Boolean(m))), ...[data.energy?.today_money,data.energy?.month_money,data.energy?.price_money].filter((m):m is Money=>Boolean(m))];
  const rateMoney = converted.find(m=>m.rate_source&&m.rate_date);
  const rateFootnote = rateMoney ? `${rateMoney.display_currency} at ${moneyRateNote(rateMoney)}` : "";
  return (
    <>
      <PageHead
        title={t("ui.costs.799b3")}
        subtitle={t("ui.ai.usage.and.measured.cpu.package.energy.84fbb")}
        range={range}
        setRange={setRange}
      />
      <div className="bento compact-rows">
        {rateFootnote?<p className="foot terminal-dim">{rateFootnote}</p>:null}
        <Tile title={t("subscription.title")} source={t("subscription.this_month")} className="full">
          <TerminalTable heads={[t("terminal.label_plan"),t("subscription.api_equivalent"),t("ui.month.cost.dedfe"),t("terminal.label_state")]} right={[1,2]} rows={data.subscriptions.map(s=>[agentName(s.kind),s.api_money?formatMoneyValue(s.api_money):formatMoney(s.api_equivalent_usd),s.subscription_usd===null?t("terminal.not_collected"):s.subscription_money?formatMoneyValue(s.subscription_money):formatMoney(s.subscription_amount??s.subscription_usd),s.coverage])}/>
        </Tile>
        {data.energy?.month_seconds ? <Tile title={t("ui.energy.437bc")} source={t("ui.cpu.package.only.56d4b")} className="half">
          <div className="stat-grid"><div><span>{t("ui.today.24345")}</span><b>{formatNumber(data.energy.today_kwh, 3)} {" "}{t("ui.kwh.72c28")}</b></div><div><span>{t("ui.this.month.1b478")}</span><b>{formatNumber(data.energy.month_kwh, 3)} {" "}{t("ui.kwh.72c28")}</b></div><div><span>{t("ui.today.cost.193fc")}</span><MoneyStat money={data.energy.today_money} amount={data.energy.today_cost} currency={data.energy.currency}/></div><div><span>{t("ui.month.cost.dedfe")}</span><MoneyStat money={data.energy.month_money} amount={data.energy.month_cost} currency={data.energy.currency}/></div></div>
          <p className="foot">{t("ui.at.08550")}{" "}{(data.energy.price_money ? formatMoneyValue(data.energy.price_money) : formatMoney(data.energy.price_per_kwh,data.energy.currency))}{t("ui.kwh.observed.intervals.only.cpu.package.only.not.148fe")}</p>
        </Tile> : null}
        <Tile
          title={t("ui.ai.cost.by.project.26d43")}
          source={t("ui.ai.usage.and.measured.cpu.package.energy.84fbb")}
          className="half"
        >
          {data.costs.length ? (
            <>
              <div className="cost-table">
                <Table
                  heads={[t("ui.project.f6f4d"), t("ui.method.88306"), t("ui.costs.799b3"), t("ui.coverage.80e13")]}
                  rows={data.costs.map((c) => [
                    c.project,
                    costMethodLabel(c.method),
                    formatMoneyValue(costMoney(c)),
                    c.partial
                      ? t("cost.partial", {tokens:formatTokens(c.unpriced_tokens || 0)})
                      : t("ui.reported.share.c6c3e"),
                  ])}
                />
              </div>
            </>
          ) : (
            <Empty>{t("ui.no.priced.usage.in.this.period.e9408")}</Empty>
          )}
          <p className="foot">
            {" "}{data.subscriptions.some(s=>s.subscription_amount!=null || s.subscription_usd!==null)?t("ui.api.equivalent.is.an.estimate.not.a.bill.subscri.101cc"):t("subscription.no_config_foot")}{" "}</p>
        </Tile>
        <Tile title="API equivalent by model" className="half"><TerminalTable heads={[t("terminal.label_model"),t("terminal.label_amount")]} right={[1]} rows={data.modelCosts.filter(c=>c.method==="api_equivalent").map(c=>[c.model??c.project,formatMoneyValue(costMoney(c))])}/></Tile>
        <Tile title={t("ui.tokens.by.model.92019")} source={t("ui.all.token.types.9ba49")} className={data.energy?.month_seconds ? "full content-height" : "half content-height"}>
          {data.models.length ? (
            <List
              items={[...data.models]
                .sort((a, b) => total(b.tokens) - total(a.tokens))
                .map((m) => ({
                  name: m.name,
                  end: formatTokens(total(m.tokens)),
                }))}
            />
          ) : (
            <Empty>{t("ui.no.model.usage.in.this.period.b130a")}</Empty>
          )}
        </Tile>
      </div>
    </>
  );
}
function FleetView({hosts, selectHost, go}: {hosts:FleetHost[];selectHost:(id:string)=>void;go:(v:View)=>void}) {
  const [label,setLabel]=useState("all"), [state,setState]=useState("all"), [sort,setSort]=useState("name");
  const labels=[...new Set(hosts.flatMap(h=>Object.entries(h.labels||{}).map(([k,v])=>`${k}=${v}`)))].sort();
  const visible=hosts.filter(h=>(label==="all"||Object.entries(h.labels||{}).some(([k,v])=>`${k}=${v}`===label))&&(state==="all"||h.liveness===state)).sort((a,b)=>sort==="state"?a.liveness.localeCompare(b.liveness)||a.host_id.localeCompare(b.host_id):sort==="agents"?b.agents-a.agents||a.host_id.localeCompare(b.host_id):a.host_id.localeCompare(b.host_id));
  const live=hosts.filter(h=>h.liveness==="live").length;
  const silent=hosts.filter(h=>h.liveness==="silent").length;
  const offline=hosts.length-live-silent;
  const moneyRows=hosts.flatMap(h=>h.costs.filter(c=>c.method==="api_equivalent").map(costMoney));
  const usage=hosts.reduce((n,h)=>n+h.usage.reduce((m,u)=>m+total(u.tokens),0),0);
  const projects=new Map<string,{tokens:number;costs:Cost[]}>();
  for (const h of hosts) {
    for (const u of h.usage) {const row=projects.get(u.name)||{tokens:0,costs:[]};row.tokens+=total(u.tokens);projects.set(u.name,row)}
    for (const c of h.costs.filter(c=>c.method==="api_equivalent")) {const row=projects.get(c.project)||{tokens:0,costs:[]};row.costs.push(c);projects.set(c.project,row)}
  }
  return <>
    <PageHead title={t("ui.fleet.e1991")} subtitle={t("fleet.summary", {hosts:hosts.length, live, silent, offline})} />
    <div className="fleet-summary bento">
      <Tile title={t("ui.hosts.53103")} className="fleet-stat"><strong>{formatNumber(hosts.length)}</strong><span>{formatNumber(live)} {" "}{t("ui.reporting.now.4fd91")}</span></Tile>
      <Tile title={t("ui.ai.usage.24.h.fae99")} className="fleet-stat"><strong>{formatTokens(usage)}</strong><span>{t("ui.tokens.across.all.hosts.f1666")}</span></Tile>
      <Tile title={t("ui.ai.cost.24.h.f1814")} className="fleet-stat"><strong>{formatMoneyTotal(moneyRows)}</strong><span>{t("ui.api.equivalent.estimate.6f668")}</span></Tile>
    </div>
    <div className="fleet-controls terminal-filters"><TerminalFilter label={t("ui.label.74341")} choices={["all",...labels]} value={label} onChange={setLabel}/><TerminalFilter label={t("ui.state.a7250")} choices={["all","live","silent","offline"]} value={state} onChange={setState}/><TerminalFilter label={t("ui.sort.adc4e")} choices={["name","state","agents"]} value={sort} onChange={setSort}/></div>
    <section className="g t"><div className="h"><h2>{t("ui.hosts.53103")}</h2></div><TerminalTable heads={[t("ui.host.3960e"),t("ui.state.a7250"),t("ui.cpu.ff221"),t("ui.memory.89c8a"),t("ui.disk.ef995"),t("ui.ai.agents.2b51b"),t("terminal.label_detail")]} right={[2,3,4,5]} rows={visible.map(h=>[<button className="terminal-link" onClick={()=>{selectHost(h.host_id);go("overview")}}>{h.host_id}</button>,h.liveness,formatPercent(h.cpu??null),formatPercent(h.memory??null),formatPercent(h.disk??null),String(h.agents),h.worst||"—"])}/></section>
    {visible.length===0?<Empty>{t("ui.no.hosts.match.these.filters.ab0a5")}</Empty>:null}
    <div className="fleet-usage bento"><Tile title={t("ui.ai.usage.by.host.24.h.09bbd")} className="half"><TerminalTable heads={[t("ui.host.3960e"),t("ui.tokens.c38c6"),t("ui.costs.799b3")]} right={[1,2]} rows={hosts.map(h=>[h.host_id,formatTokens(h.usage.reduce((n,u)=>n+total(u.tokens),0)),formatMoneyTotal(h.costs.filter(c=>c.method==="api_equivalent").map(costMoney))])}/></Tile><Tile title={t("ui.ai.usage.by.project.24.h.eb6fa")} className="half"><TerminalTable heads={[t("terminal.label_project"),t("ui.tokens.c38c6"),t("ui.costs.799b3")]} right={[1,2]} rows={[...projects].sort((a,b)=>b[1].tokens-a[1].tokens).map(([name,p])=>[name,formatTokens(p.tokens),formatMoneyTotal(p.costs.map(costMoney))])}/></Tile></div>
  </>;
}
function Palette({
  onClose,
  hosts,
  agents,
  go,
  selectHost,
}: {
  onClose: () => void;
  hosts: Host[];
  agents: Session[];
  go: (v: View) => void;
  selectHost: (v: string) => void;
}) {
  const [query, setQuery] = useState(""),
    [index, setIndex] = useState(0);
  const items = [
    ...views.filter(v=>v!=="fleet"||hosts.length>1).map((v) => ({
      label: t(navigationLabels[v as keyof typeof navigationLabels]),
      group: t("palette.view"),
      run: () => go(v),
    })),
    ...hosts.map((h) => ({
      label: `${h.host_id} · ${h.liveness}`,
      group: t("palette.host"),
      run: () => selectHost(h.host_id),
    })),
    ...agents.map((a) => ({
      label: sessionDisplayLabel(a),
      group: t("palette.agent"),
      run: () => {
        go("agents");
        history.replaceState(null,"",`/agents?session=${encodeURIComponent(a.id)}`);
        window.dispatchEvent(new PopStateEvent("popstate"));
        setTimeout(
          () =>
            document
              .getElementById(
                `${window.innerWidth < 640 ? "mobile-session" : "session"}-${a.id}`,
              )
              ?.scrollIntoView({ block: "center" }),
          50,
        );
      },
    })),
  ].filter((i) => i.label.toLowerCase().includes(query.toLowerCase()));
  return (
    <div className="palette-backdrop" onMouseDown={onClose}>
      <div
        className="palette g"
        role="dialog"
        aria-modal="true"
        aria-label={t("ui.command.palette.7b6b5")}
        onMouseDown={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === "Escape") onClose();
          if (e.key === "ArrowDown") {
            e.preventDefault();
            setIndex(Math.min(items.length - 1, index + 1));
          }
          if (e.key === "ArrowUp") {
            e.preventDefault();
            setIndex(Math.max(0, index - 1));
          }
          if (e.key === "Enter") {
            items[index]?.run();
            onClose();
          }
        }}
      >
        <input
          autoFocus
          placeholder={t("ui.search.views.hosts.or.agents.86df3")}
          aria-label={t("ui.search.commands.5ae3a")}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setIndex(0);
          }}
        />
        <div className="palette-results">
          {items.map((item, i) => (
            <button
              key={`${item.group}${item.label}${i}`}
              className={i === index ? "selected" : ""}
              onMouseEnter={() => setIndex(i)}
              onClick={() => {
                item.run();
                onClose();
              }}
            >
              <span>{item.label}</span>
              <small>{item.group}</small>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
function uptimeLabel(n: number | null): string { return formatPercent(n); }
function coverageLabel(coverage: number, periodSeconds: number, periodLabel: string): string | null {
  if (coverage >= 1) return null;
  const seconds = coverage * periodSeconds;
  const period = periodLabel === "day" ? t("coverage.day") : periodLabel;
  if (seconds === 0) return t("coverage.none", {period});
  const duration = seconds < 60 ? t("coverage.less_than_minute") : seconds < 3600 ? formatDuration(seconds) : seconds < 86400 ? formatDuration(seconds) : `${formatNumber(seconds / 86400, 1)} d`;
  return t("coverage.measured", {duration, period});
}
function AvailabilityValue({uptime,coverage,seconds,period}:{uptime:number|null;coverage:number;seconds:number;period:string}) {
  const note = coverageLabel(coverage,seconds,period);
  return <><b>{uptimeLabel(uptime)}</b>{note?<small className="coverage-note">{note}</small>:null}</>;
}
function statusLabel(state: string): string {
  return t(state === "ok" ? "state.operational" : state === "warn" ? "state.degraded" : state === "crit" ? "state.down" : "state.unknown");
}
function DailyBars({days}:{days: ProbeDay[]}) {
  return <div className="terminal-daily-bars" role="img" aria-label={t("ui.daily.availability.for.30.days.5072a")}>
    {days.map(d=>{
      const elapsed = Math.min(86400,Math.max(0,(Date.now()-Date.parse(`${d.date}T00:00:00Z`))/1000));
      const note = coverageLabel(d.coverage,elapsed,"day");
      return <span key={d.date} className={d.state==="crit"?"terminal-crit":d.state==="warn"?"terminal-warn":d.state==="unknown"?"terminal-dim":undefined} title={`${d.date}: ${uptimeLabel(d.uptime)}${note ? ` · ${note}` : ""}`}>{d.state==="unknown"?"░":"█"}</span>;
    })}
  </div>;
}
function AvailabilityTile({probes,go}:{probes:ProbeSummary[];go:(v:View)=>void}) {
  return <Tile title={t("ui.availability.681b5")} source={t("ui.30.days.7d427")} className="availability-tile half content-height">
    {probes.slice(0,4).map(p=><div className="uptime-row" key={p.name}><div><span>{p.display_name}</span><span className="uptime-row-value"><AvailabilityValue uptime={p.uptime_30d} coverage={p.coverage_30d} seconds={30*86400} period="30 d"/></span></div><DailyBars days={p.days}/></div>)}
    <a className="tile-link" href="/uptime" onClick={e=>{e.preventDefault();go("uptime")}}>{t("ui.view.all.probes.9f6c8")}</a>
  </Tile>;
}
function UptimeView({probes}:{probes:ProbeSummary[]}) {
  const [period,setPeriod]=useState<Range>("24h");
  const cutoff=Date.now()-({"1h":36e5,"12h":12*36e5,"24h":864e5,"7d":7*864e5,"30d":30*864e5}[period]);
  return <><PageHead title={t("ui.uptime.6aafa")} subtitle={t("ui.http.and.internal.probe.availability.with.unknow.d319a")} />
    {probes.length===0?<Empty>{t("ui.no.probes.configured.yet.e62a6")}</Empty>:<div className="bento compact-rows">
      {probes.map(p=>{
        const points=p.points.filter(x=>new Date(x.at).getTime()>=cutoff);
        const stats=p.percentiles?.[period];
        return <Tile key={p.name} title={p.display_name} source={`${p.kind.toUpperCase()} · ${statusLabel(p.state)}`} className="half content-height">
          <div className="uptime-stats"><div><small>{t("ui.24.hours.cb478")}</small><AvailabilityValue uptime={p.uptime_24h} coverage={p.coverage_24h} seconds={86400} period="24 h"/></div><div><small>{t("ui.7.days.0a8dd")}</small><AvailabilityValue uptime={p.uptime_7d} coverage={p.coverage_7d} seconds={7*86400} period="7 d"/></div><div><small>{t("ui.30.days.7d427")}</small><AvailabilityValue uptime={p.uptime_30d} coverage={p.coverage_30d} seconds={30*86400} period="30 d"/></div></div>
          <DailyBars days={p.days}/>
          <div className="uptime-metrics"><span>{t("ui.http.f40b2")}{" "}{p.status_code || "—"}</span><span>{t("ui.latest.decd7")}{" "}{p.latency_ms === undefined ? "—" : formatNumber(p.latency_ms)} {" "}{t("ui.ms.26cc3")}</span><span>{t("ui.dns.3ddc2")}{" "}{p.dns_ms === undefined ? "—" : formatNumber(p.dns_ms)} {" "}{t("ui.ms.26cc3")}</span><span>{t("ui.connect.b6546")}{" "}{p.connect_ms === undefined ? "—" : formatNumber(p.connect_ms)} {" "}{t("ui.ms.26cc3")}</span><span>{t("ui.tls.d91e1")}{" "}{p.tls_ms === undefined ? "—" : formatNumber(p.tls_ms)} {" "}{t("ui.ms.26cc3")}</span><span>{t("ui.first.byte.87d97")}{" "}{p.first_byte_ms === undefined ? "—" : formatNumber(p.first_byte_ms)} {" "}{t("ui.ms.26cc3")}</span>{p.certificate_days!==undefined?<span>{t("ui.certificate.10de4")}{" "}{p.certificate_days} {" "}{t("ui.days.5548a")}</span>:null}</div>
          <div className="chart-controls"><strong>{t("ui.response.time.e3f4e")}</strong><Segmented label={`${p.display_name} period`} choices={["1h","12h","24h","7d","30d"]} value={period} onChange={setPeriod}/></div>
          <ResponseChart points={points} label={p.display_name} period={period}/>
          <p className="foot response-legend"><span><i className="response-key"/>{t("ui.response.6e617")}</span><span>{t("ui.p50.42e31")}{" "}{stats?.p50_ms!==null&&stats?.p50_ms!==undefined?`${formatNumber(stats.p50_ms)} ms`:t("ui.unknown.bc781")}</span><span>{t("ui.p95.e375f")}{" "}{stats?.p95_ms!==null&&stats?.p95_ms!==undefined?`${formatNumber(stats.p95_ms)} ms`:t("ui.unknown.bc781")}</span><span><i className="failure-key"/>{t("ui.failure.16566")}</span></p>
        </Tile>;
      })}</div>}
  </>;
}
function PublicStatusPage() {
  const [theme,setTheme]=useTheme();
  useCockpitLanguage(activeLanguage());
  const [probes,setProbes]=useState<PublicProbe[]>([]);
  const [error,setError]=useState("");
  useEffect(()=>{const load=()=>get<PublicProbe[]>("/api/v1/status").then(setProbes).catch(()=>setError(t("public.unavailable")));void load();const id=setInterval(load,60000);return()=>clearInterval(id)},[]);
  const degraded=probes.filter(p=>p.state==="warn").length;
  const down=probes.filter(p=>p.state==="crit").length;
  const unknown=probes.filter(p=>p.state==="unknown"||p.state==="stale").length;
  const summary=down?plural(down,"public.down.one","public.down.many"):degraded?plural(degraded,"public.degraded.one","public.degraded.many"):unknown?plural(unknown,"public.unknown.one","public.unknown.many"):t("public.all_operational");
  return <div className="status-page"><header className="status-header"><a href="/status" className="logo"><MonoMark/>{t("ui.fleetglass.2e022")}</a><Segmented label={t("ui.theme.a797e")} choices={["Light","Dark"]} value={theme==="light"?"Light":"Dark"} onChange={x=>setTheme(x.toLowerCase() as "light"|"dark")}/></header>
    <main><div className="page-head"><div><h1>{t("ui.service.status.7efad")}</h1><p>{t("ui.availability.over.the.past.30.days.8e7a8")}</p></div></div>
    {error?<p role="alert">{error}</p>:null}
    {probes.length>0&&!error?<div className={`g status-banner ${down?"crit":degraded?"warn":unknown?"unknown":"ok"}`} role="status">{summary}</div>:null}
    {probes.length===0?<div className="g t status-empty">{t("ui.no.public.services.configured.854c9")}</div>:<div className="bento compact-rows">{probes.map((p,i)=><Tile key={`${p.display_name}-${i}`} title={p.display_name} source={statusLabel(p.state)} className="half content-height"><div className="uptime-stats"><div><small>{t("ui.30.day.uptime.84b41")}</small><AvailabilityValue uptime={p.uptime_30d} coverage={p.coverage_30d} seconds={30*86400} period="30 d"/></div><div><small>{t("ui.current.state.8e20a")}</small><b className={`state-text ${p.state}`}>{statusLabel(p.state)}</b></div>{p.latency_ms!==undefined?<div><small>{t("ui.response.6e617")}</small><b>{formatNumber(p.latency_ms)} {" "}{t("ui.ms.26cc3")}</b></div>:null}</div><DailyBars days={p.days}/></Tile>)}</div>}
    </main></div>;
}
function App() {
  const [theme, setTheme, themeMode] = useTheme(),
    [preferences, setPreferences] = useState<Preferences>(() => readPreferences()),
    [serverDefaults, setServerDefaults] = useState<Preferences>(defaultPreferences),
    [view, setView] = useState<View>(currentView),
    [hosts, setHosts] = useState<Host[]>([]),
    [fleet, setFleet] = useState<FleetHost[]>([]),
    [host, setHost] = useState(""),
    [range, setRange] = useState<Range>("24h"),
    [auth, setAuth] = useState<"checking" | "required" | "ready" | "offline">("checking"),
    [protectedServer, setProtectedServer] = useState(false),
    [token, setToken] = useState(""),
    [authError, setAuthError] = useState(""),
    [palette, setPalette] = useState(false),
    [release, setRelease] = useState<{version:string;commit?:string;latest:string;update_check_enabled:boolean}|null>(null),
    [tick, setTick] = useState(Date.now());
  const hadPreferences = useRef(hasPreferences());
  const hadCurrency = useRef((() => { try { return !!JSON.parse(localStorage.getItem("fleetglass.preferences.v1") || "null")?.currency; } catch { return false; } })());
  useCockpitLanguage(preferences.language);
  useEffect(() => { if (auth === "ready") get<{version:string;commit?:string;latest:string;update_check_enabled:boolean}>("/api/v1/version").then(setRelease).catch(()=>{}); }, [auth]);
  useEffect(() => {
    if (auth !== "ready") return;
    get<{language:string;temperature:string;network:string;cpu:string;currency:string}>("/api/v1/display-defaults").then(raw => {
      const defaults: Preferences = {language:"en",temperature:raw.temperature === "F" ? "F" : "C",network:raw.network === "bits" ? "bits" : "bytes",cpu:raw.cpu === "machine" ? "machine" : "core",currency:raw.currency || "USD",hiddenTabs:[],hiddenSections:[]};
      setServerDefaults(defaults);
      if (!hadPreferences.current) setPreferences(defaults);
      else if (!hadCurrency.current) setPreferences(current => ({...current,currency:defaults.currency}));
    }).catch(()=>{});
  }, [auth]);
  useEffect(() => { savePreferences(preferences); document.body.dataset.language = preferences.language; }, [preferences]);
  const changePreferences = (next: Preferences) => { setPreferences(next); savePreferences(next); };
  const data = useCockpit(host, range, auth === "ready", preferences.currency);
  useEffect(() => { document.body.dataset.cpuCores = String(value(result(data.overview?.results || [], "cpu"), "core_count") || 1); }, [data.overview]);
  useEffect(() => {
    if (data.error === "auth.required") setAuth("required");
  }, [data.error]);
  useEffect(() => {
    const id = setInterval(() => setTick(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);
  useEffect(() => {
    get<{ required: boolean }>("/api/v1/auth")
      .then(async (x) => {
        setProtectedServer(x.required);
        if (!x.required) {
          setAuth("ready");
          return;
        }
        try {
          await get<Host[]>("/api/v1/hosts");
          setAuth("ready");
        } catch {
          setAuth("required");
        }
      })
      .catch(() => setAuth("offline"));
  }, []);
  useEffect(() => {
    if (auth !== "ready") return;
    get<Host[]>("/api/v1/hosts")
      .then((h) => {
        setHosts(h);
        setHost((current) => current || h[0]?.host_id || "");
      })
      .catch((e) => {
        if ((e as Error).message === "auth.required") setAuth("required");
      });
  }, [auth]);
  useEffect(() => {
    if (auth!=="ready") return;
    let active=true;
    const load=()=>get<FleetHost[]>(`/api/v1/fleet?display_currency=${preferences.currency}`).then(rows=>{if(active){setFleet(rows);setHosts(rows);setHost(current=>current||rows[0]?.host_id||"")}}).catch(()=>{});
    void load();
    const id=setInterval(load,15000);
    return ()=>{active=false;clearInterval(id)};
  },[auth, preferences.currency]);
  useEffect(()=>{if(view==="fleet"&&hosts.length===1){history.replaceState(null,"","/");setView("overview")}},[view,hosts.length]);
  useEffect(() => {
    const pop = () => setView(currentView());
    const key = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPalette((x) => !x);
      } else if (e.key === "Escape") setPalette(false);
    };
    window.addEventListener("popstate", pop);
    window.addEventListener("keydown", key);
    return () => {
      window.removeEventListener("popstate", pop);
      window.removeEventListener("keydown", key);
    };
  }, []);
  const go = (v: View) => {
    history.pushState(null, "", `/${v === "overview" ? "" : v}`);
    setView(v);
    window.scrollTo(0, 0);
  };
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey || e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement || e.target instanceof HTMLSelectElement || (e.target instanceof HTMLElement && e.target.isContentEditable)) return;
      if (/^[1-9]$/.test(e.key) || e.key === "0") { e.preventDefault(); go(terminalViews[e.key === "0" ? 9 : Number(e.key)-1]); }
      else if (e.key.toLowerCase() === "r") { e.preventDefault(); const ranges: Range[]=["1h","24h","7d","30d"]; setRange(ranges[(ranges.indexOf(range)+1)%ranges.length]); }
      else if (e.key.toLowerCase() === "t") { e.preventDefault(); setTheme(themeMode === "system" ? "light" : themeMode === "light" ? "dark" : "system"); }
      else if (e.key === "?") { e.preventDefault(); go("glossary"); }
      else if (e.key === "/") { const filter=document.querySelector<HTMLInputElement>('main input[type="search"]'); if (filter) { e.preventDefault(); filter.focus(); } }
    };
    window.addEventListener("keydown",key);
    return () => window.removeEventListener("keydown",key);
  }, [range, themeMode]);
  const signIn = async (e: React.FormEvent) => {
    e.preventDefault();
    setAuthError("");
    try {
      const r = await fetch("/api/v1/auth", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token }),
      });
      if (!r.ok) throw new Error("auth.invalid_token");
      setToken("");
      setAuth("ready");
    } catch (e) {
      setAuthError((e as Error).message);
    }
  };
  const signOut = async () => {
    await fetch("/api/v1/auth/signout", { method: "POST" });
    setAuth("required");
    setHosts([]);
    setFleet([]);
    setHost("");
  };
  const age = data.last
      ? Math.max(0, Math.floor((tick - data.last) / 1000))
      : null,
    stale = data.stream === "stale" || (age !== null && age > 45);
  setLanguage(preferences.language);
  const displayVersion = release?.version?.includes("dev") && release.commit && release.commit !== "unknown" ? release.commit.slice(0, 7) : release?.version ?? "";
  return (
    <div className="shell">
      <div className="terminal-header">
      <div className="terminal-chrome" aria-label="Fleetglass window">
        <div className="terminal-lights" aria-hidden="true"><i/><i/><i/></div>
        <span className="terminal-window-title"><MonoMark/>{host || "fleetglass"} - fleetglass - {Math.max(40,Math.floor((typeof window !== "undefined" ? window.innerWidth : 1440)/7.5))}x{Math.max(20,Math.floor((typeof window !== "undefined" ? window.innerHeight : 900)/18))}</span>
      </div>
      <header className="top">
        <div className="terminal-status"><MonoMark/><b>fleetglass {displayVersion}</b><span className="terminal-separator">│</span><span className="host"><TerminalFilter label={t("ui.host.3960e")} choices={hosts.map(h=>h.host_id)} value={host} onChange={setHost}/></span><span className="terminal-separator">│</span><span>{t("terminal.status_up")} {value(result(data.overview?.results||[],"host"),"uptime_seconds")===null?t("terminal.not_collected"):(()=>{const minutes=Math.floor(value(result(data.overview?.results||[],"host"),"uptime_seconds")!/60);return `${Math.floor(minutes/1440)}d ${String(Math.floor(minutes%1440/60)).padStart(2,"0")}:${String(minutes%60).padStart(2,"0")}`})()}</span><span className="terminal-separator">│</span><span>{t("terminal.status_kernel")} {result(data.overview?.results||[],"host")?.fields?.kernel_release?`Linux ${result(data.overview?.results||[],"host")?.fields?.kernel_release} ${result(data.overview?.results||[],"host")?.fields?.arch??""}`:t("terminal.not_collected")}</span><span className="terminal-separator">│</span><span>{t("terminal.status_load")} {value(result(data.overview?.results||[],"load"),"1m")?.toFixed(2) ?? t("terminal.not_collected")}</span><span className="terminal-separator">│</span><span>{value(result(data.overview?.results||[],"cpu"),"core_count") ?? "—"} {t("terminal.label_threads")} · {value(result(data.overview?.results||[],"memory"),"total_bytes")===null?t("terminal.not_collected"):formatBytes(value(result(data.overview?.results||[],"memory"),"total_bytes"))}</span><span className="terminal-separator">│</span><span className="terminal-warn-count">{data.overview?.results.filter(x=>x.state==="warn").length ?? 0} WARN</span><span className="terminal-crit-count">{data.overview?.results.filter(x=>x.state==="crit").length ?? 0} CRIT</span><span>{data.stream === "stale" ? t("ui.connection.lost.62583") : stale ? t("ui.stale.data.2e635") : data.stream === "connecting" ? t("ui.connecting.c1f3b") : t("ui.live.65c82")}</span><span className="terminal-time">{new Date(tick).toLocaleString(preferences.language === "fr" ? "fr-FR" : "sv-SE",{dateStyle:"short",timeStyle:"medium",hour12:false})}</span>{protectedServer?<button type="button" className="quiet" onClick={signOut}>{t("ui.sign.out.dc164")}</button>:null}</div>
      </header>
      <div className="terminal-tabs"><nav className="tabs" aria-label={t("ui.views.24be6")}>{terminalViews.filter(v=>!preferences.hiddenTabs.includes(v)).map(v=>{const index=terminalViews.indexOf(v);return <button type="button" key={v} aria-pressed={view===v||(v==="machine"&&view==="storage")} onClick={()=>go(v)}>{index===9?0:index+1} {t(navigationLabels[v as keyof typeof navigationLabels], preferences.language).toLowerCase()}</button>})}<button type="button" className="terminal-search" onClick={()=>setPalette(true)} aria-label={t("ui.search.or.jump.to.commands.control.or.command.k.26860")} title={t("ui.search.or.jump.to.commands.ctrl.k.50228")}>{t("ui.k.64d86")}</button>{hosts.length>1?<button type="button" onClick={()=>go("fleet")}>{formatNumber(fleet.filter(h=>h.liveness==="live").length)} {t("ui.of.de04f")} {formatNumber(fleet.length)} {t("ui.hosts.reporting.57618")} · {formatNumber(fleet.filter(h=>h.liveness!=="live").length)} {t("ui.need.attention.c8806")} · {t("ui.view.fleet.8d8a0")}</button>:null}<button type="button" aria-pressed={view==="settings"} onClick={()=>go("settings")}>{t("ui.settings.c7f73",preferences.language).toLowerCase()}</button><button type="button" aria-pressed={view==="glossary"} onClick={()=>go("glossary")}>{t("terminal.glossary")}</button></nav><div className="terminal-range" role="group" aria-label={t("terminal.range")}>{t("terminal.range")} {(["1h","24h","7d","30d"] as Range[]).map(v=><button key={v} type="button" aria-pressed={range===v} onClick={()=>setRange(v)}>{v}</button>)}</div><div className="terminal-theme" role="group" aria-label={t("terminal.theme")}>{(["system","light","dark"] as const).map(mode=><button type="button" key={mode} aria-pressed={themeMode===mode} onClick={()=>setTheme(mode)}>{mode === "system" ? t("terminal.system") : mode === "light" ? t("terminal.light") : t("terminal.dark")}</button>)}</div></div>
      </div>
      {auth === "offline" ? (
        <main className="auth g"><h1>{t("pwa.offline")}</h1><p>{t("pwa.last_data", {at:(() => { try { const at = window.localStorage.getItem("fleetglass:last-data-at"); return at ? formatDateTime(at) : t("value.unknown"); } catch { return t("value.unknown"); } })()})}</p><button onClick={() => location.reload()}>{t("ui.retry.9f5cd")}</button></main>
      ) : auth === "checking" ? (
        <main className="auth g">{t("ui.connecting.fd3e7")}</main>
      ) : auth === "required" ? (
        <main className="auth g">
          <h1>{t("ui.sign.in.to.fleetglass.c7302")}</h1>
          <p>{t("ui.enter.the.server.read.token.to.view.the.cockpit.9cbd9")}</p>
          <form onSubmit={signIn}>
            <label>
              {" "}{t("ui.read.token.7180b")}{" "}
              <input
                type="password"
                autoComplete="current-password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                required
              />
            </label>
            <button type="submit">{t("ui.sign.in.ada2e")}</button>
          </form>
          {authError ? (
            <p className="attention" role="alert">
              {displayError(authError)}
            </p>
          ) : null}
        </main>
      ) : (
        <main>
          {data.error ? (
            <div className="notice" role="alert">
              {displayError(data.error)}{" "}
              <button onClick={() => void data.refresh()}>{t("ui.retry.9f5cd")}</button>
            </div>
          ) : null}
          {view === "glossary" ? (
            <div className="terminal-grid"><section className="terminal-box terminal-span-12" aria-label={t("terminal.glossary")}><h2>{t("terminal.glossary")}</h2><div className="terminal-box-body"><p>{t("terminal.glossary_intro")}</p><p>{t("terminal.glossary_bar")}</p><p>{t("terminal.glossary_unknown")}</p><p className="terminal-warn">{t("terminal.glossary_warn")}</p><p className="terminal-crit">{t("terminal.glossary_crit")}</p></div></section></div>
          ) : view === "settings" ? (
            <SettingsView preferences={preferences} onChange={changePreferences} onReset={() => changePreferences(serverDefaults)} />
          ) : !host ? (
            <div className="auth g">
              <h1>{t("ui.no.host.samples.yet.f109c")}</h1>
              <p>{t("ui.start.the.agent.to.populate.this.cockpit.8a383")}</p>
            </div>
          ) : view === "share" ? (
            <ShareView host={host} hosts={hosts} fleet={fleet} overview={data.overview} theme={theme} language={preferences.language} />
          ) : view === "fleet" && hosts.length>1 ? (
            <FleetView hosts={fleet} selectHost={setHost} go={go} />
          ) : view === "overview" ? (
            <TerminalOverview data={data} go={go} />
          ) : view === "agents" ? (
            <AgentsView data={data} />
		  ) : view === "apps" ? (
			<AppCatalog data={data} range={range} setRange={setRange}/>
          ) : view === "projects" ? (
            <ProjectsView data={data.observations} worktrees={data.worktrees} hostAt={data.overview?.at} />
          ) : view === "machine" || view === "storage" ? (
            <MachineView data={data} range={range} setRange={setRange} storage={view === "storage"} onStorageChange={(storage) => go(storage ? "storage" : "machine")} />
          ) : view === "services" ? (
            <ServicesView data={data} />
          ) : view === "system" ? (
            <SystemView data={data} />
          ) : view === "changes" ? (
            <ChangesView data={data} />
          ) : view === "uptime" ? (
            <UptimeView probes={data.probes} />
          ) : (
            <CostsView data={data} range={range} setRange={setRange} />
          )}
        </main>
      )}
      {palette ? (
        <Palette
          onClose={() => setPalette(false)}
          hosts={hosts}
          agents={data.agents}
          go={go}
          selectHost={setHost}
        />
      ) : null}
      {auth === "ready" ? <footer className="terminal-footer"><span><kbd>1-9/0</kbd>{t("terminal.views")}</span><span><kbd>r</kbd>{t("terminal.range")} {range}</span><span><kbd>/</kbd>{t("terminal.filter")}</span><span><kbd>?</kbd>{t("terminal.glossary")}</span><span><kbd>t</kbd>{t("terminal.theme")}</span><span className="terminal-footer-end">{release?t("release.version",{version:displayVersion}):""} · {release?.update_check_enabled&&release.latest&&release.latest!==release.version?t("release.update",{version:release.latest}):""} · {t("terminal.read_only")} · {data.stream} · {age===null?"—":t("live.seconds_ago",{seconds:age})}</span></footer> : null}
    </div>
  );
}

createRoot(document.getElementById("root")!).render(location.pathname==="/status"?<PublicStatusPage/>:<App />);
if ("serviceWorker" in navigator && location.pathname !== "/status") {
  window.addEventListener("load", () => { void navigator.serviceWorker.register("/sw.js"); });
}
