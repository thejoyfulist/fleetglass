import { plural, t } from "./i18n";
import type { MessageKey } from "./i18n";
import { useState } from "react";
import { formatAppCPU } from "./preferences";
import { agentName, formatBytes, formatDateTime, formatDuration, hasDate, resetTime } from "./data";
import type { Insight, Observations, ProjectProcess, WorktreesView } from "./data";

const insightMessages: Record<string, MessageKey> = {
  cpu: "insight.cpu", disk: "insight.disk", network: "insight.network",
  memory: "insight.memory", idle_dev_server: "insight.idle_dev_server",
  runaway_dev_server: "insight.runaway_dev_server",
};
export function insightDetail(insight: Insight): string {
  const key = insightMessages[insight.detail_code || insight.kind];
  return key ? t(key) : insight.detail;
}
function appLink(insight: Insight) {
  return insight.app_kind === "project"
    ? "/projects"
    : `/apps?app=${encodeURIComponent(`${insight.app_kind}:${insight.app}`)}`;
}
export function InsightsPanel({
  insights,
  compact = false,
}: {
  insights: Insight[];
  compact?: boolean;
}) {
  const rows = compact ? insights.slice(0, 4) : insights;
  return (
    <section
      className={`g t observation-insights ${compact ? "observation-insights-compact" : ""}`}
      aria-label={t("ui.what.stands.out.691fa")}
    >
      <div className="h">
        <h2>{t("ui.what.stands.out.691fa")}</h2>
        <span className="src">{t("ui.fixed.rules.no.ai.f3669")}</span>
      </div>
      {rows.length ? (
        <ul className="observation-list">
          {rows.map((x, i) => (
            <li key={`${x.kind}:${x.app}:${i}`}>
              <span className="observation-mark" aria-hidden="true" />
              <div>
                <a href={appLink(x)}>{x.app}</a>
                <small>{insightDetail(x)}</small>
              </div>
            </li>
          ))}
        </ul>
      ) : (
        <p className="foot">{t("ui.no.rule.has.crossed.its.duration.threshold.43f0c")}</p>
      )}
      {!compact ? (
        <p className="foot">
          {" "}{t("ui.cpu.gt.80.for.10.min.memory.100.mib.for.1.h.with.145d4")}{" "}</p>
      ) : null}
    </section>
  );
}
function processLabel(p: ProjectProcess) {
  return p.kind === "dev_server"
    ? t("ui.dev.server.df14a")
    : p.kind === "agent"
      ? t("ui.ai.agent.ab15e")
      : t("ui.process.12f4c");
}
export function ProjectsView({
  data,
  worktrees,
  hostAt,
}: {
  data: Observations;
  worktrees: WorktreesView;
  hostAt?: string;
}) {
  const [copied, setCopied] = useState<number | null>(null);
  const groups = new Map<string, ProjectProcess[]>();
  for (const p of data.projects) {
    const key = p.project + (p.worktree ? ` / ${p.worktree}` : "");
    groups.set(key, [...(groups.get(key) || []), p]);
  }
  const idle = data.projects.filter((p) => p.idle && p.kind === "dev_server");
  const leftBehind = data.projects.filter(p => p.left_behind);
  const stale = !hostAt || Date.now() - new Date(hostAt).getTime() > 60_000;
  const bytes = idle.reduce((n, p) => n + p.memory_bytes, 0);
  const ports = [...new Set(idle.flatMap((p) => p.ports))].sort(
    (a, b) => a - b,
  );
  async function copy(p: ProjectProcess) {
    try {
      await navigator.clipboard.writeText(p.stop_command);
      setCopied(p.pid);
    } catch {
      setCopied(null);
    }
  }
  return (
    <>
      <div className="page-head">
        <div>
          <h1>{t("ui.projects.53e89")}</h1>
          <p>
            {" "}{t("ui.agents.and.long.running.processes.under.configur.14995")}{" "}</p>
        </div>
      </div>
      <div className="bento observation-bento">
        <section className="g t observation-summary full"><div className="h"><h2>{t("project.left_behind")}</h2><span className="src">{t("project.witnessed_ancestry")}</span></div><strong>{leftBehind.length}</strong><p>{t("project.freeable",{memory:formatBytes(leftBehind.reduce((sum,p)=>sum+p.memory_bytes,0))})}</p><small>{t("project.copy_only")}</small></section>
        <section className="g t observation-summary">
          <div className="h">
            <h2>{t("ui.idle.dev.servers.ad057")}</h2>
            <span className="src">
              {" "}{t("ui.15.min.without.cpu.above.1.or.a.new.connection.f9521")}{" "}</span>
          </div>
          <p>
            {stale ? t("ui.unknown.bc781") : plural(idle.length, "project.idle.one", "project.idle.many")}</p>
          {stale ? (
            <p>{t("ui.latest.process.inventory.is.stale.5912a")}</p>
          ) : (
            <p>
              {" "}{t("ui.stopping.these.would.free.e4751")}{" "}<b>{formatBytes(bytes)}</b> {" "}{t("ui.of.process.memory.and.ports.38762")}{" "}<b>{ports.length ? ports.join(", ") : t("project.none")}</b>
              .
            </p>
          )}
          <small>
            {" "}{t("ui.estimate.from.current.process.rss.copy.a.command.58c52")}{" "}</small>
        </section>
        <InsightsPanel insights={data.insights} />
      </div>
      {groups.size ? (
        [...groups].map(([key, processes]) => (
          <section className="g observation-project" key={key}>
            <div className="h">
              <h2>{key}</h2>
              <span className="src">
                {plural(processes.length, "project.process.one", "project.process.many")}
              </span>
            </div>
            <div className="observation-processes">
              {processes.map((p) => (
                <div className="observation-process" key={p.pid}>
                  <div>
                    <strong>{p.name}</strong>
                    <small>
                      {processLabel(p)} {" "}{t("ui.pid.08323")}{" "}{p.pid}
                      {p.left_behind ? ` · ${t("project.left_by",{agent:p.agent_kind?agentName(p.agent_kind):t("value.unknown"),session:(p.agent_session_id||"").slice(0,8)})}` : ""}
                      {hasDate(p.started_at) ? ` · ${formatDuration(Math.max(0,(Date.now()-new Date(p.started_at).getTime())/1000))}` : ""}
                      {p.agent_session_id?<a href={`/agents?session=${encodeURIComponent(p.agent_session_id)}`}>{t("session.open")}</a>:null}
                      {p.idle
                        ? ` · ${t("project.idle_age", {minutes: Math.floor(p.idle_minutes)})}`
                        : ""}
                    </small>
                  </div>
                  <div>
                    <b>{formatAppCPU(p.cpu_percent)} {" "}{t("ui.cpu.ff221")}</b>
                    <small>{formatBytes(p.memory_bytes)} {" "}{t("ui.ram.ac698")}</small>
                  </div>
                  <div>
                    <b>
                      {!p.ports_measured
                        ? t("ui.ports.not.measured.d4001")
                        : p.ports.length
                          ? p.ports.map((x) => `:${x}`).join(", ")
                          : t("ui.no.listening.port.b2a83")}
                    </b>
                    <small>{p.worktree || p.project}</small>
                  </div>
                  <button
                    type="button"
                    onClick={() => void copy(p)}
                    aria-label={t("project.copy_command_for", {name: p.name, pid: p.pid})}
                  >
                    {copied === p.pid ? t("ui.copied.8e3df") : t("ui.copy.stop.command.e9e3a")}
                  </button>
                </div>
              ))}
            </div>
          </section>
        ))
      ) : (
        <section className="g t">
          <h2>{t("ui.no.project.processes.found.c00d6")}</h2>
          <p className="foot">
            {" "}{t("ui.configure.project.roots.and.worktree.roots.in.th.edda4")}{" "}</p>
        </section>
      )}
      <section className="g t observation-project"><div className="h"><h2>{t("worktree.title")}</h2><span className="src">{worktrees.at?formatDateTime(worktrees.at):t("value.unknown")}</span></div><p>{t("worktree.reclaimable",{size:formatBytes(worktrees.reclaimable_bytes)})}</p>
        {worktrees.worktrees.length?<div className="worktree-list"><div className="worktree-row worktree-heading"><span>{t("worktree.title")}</span><span>{t("ui.size.b7152")}</span><span>{t("worktree.age_label")}</span><span>{t("worktree.last_commit")}</span><span>{t("worktree.merge_status")}</span><span>{t("ui.agents.64acf")}</span><span>{t("ui.state.a7250")}</span></div>{worktrees.worktrees.map(tree=><div className="worktree-row" key={tree.path}><strong>{tree.project} / {tree.name}</strong><span>{tree.size_complete?formatBytes(tree.size_bytes):t("worktree.incomplete")}</span><span>{hasDate(tree.created_at)?t("worktree.age",{age:formatDuration(Math.max(0,(Date.now()-new Date(tree.created_at).getTime())/1000))}):t("value.unknown")}</span><span>{hasDate(tree.last_commit_at)?formatDateTime(tree.last_commit_at):t("value.unknown")}</span><span>{tree.merged===undefined?t("worktree.merge_unknown"):tree.merged?t("worktree.merged"):t("worktree.unmerged")}</span><span>{t("worktree.processes",{count:tree.agent_processes})}</span><span>{tree.stale?t("worktree.stale"):tree.reclaimable?t("worktree.reclaimable_tag"):""}</span></div>)}</div>:<p>{t("worktree.no_data")}</p>}
      </section>
    </>
  );
}
export function StorageView({ data }: { data: Observations }) {
  const roots = [...new Set(data.storage.map((x) => x.root))];
  return (
    <>
      <div className="page-head">
        <div>
          <h1>{t("ui.storage.map.ec0f7")}</h1>
          <p>
            {" "}{t("ui.largest.directories.under.configured.roots.to.tw.53937")}{" "}</p>
        </div>
      </div>
      {roots.length ? (
        roots.map((root) => {
          const entries = data.storage.filter((x) => x.root === root);
          const max = Math.max(1, ...entries.map((x) => x.size_bytes));
          return (
            <section className="g observation-storage" key={root}>
              <div className="h">
                <h2>{root}</h2>
                <span className="src">
                  {" "}{t("ui.hourly.scan.b64e7")}{" "}
                  {entries[0] ? resetTime(entries[0].at) : t("ui.waiting.33d30")}
                </span>
              </div>
              <div className="storage-heading">
                <span>{t("ui.directory.4b892")}</span>
                <span>{t("ui.size.b7152")}</span>
                <span>{t("ui.7.d.growth.36d43")}</span>
              </div>
              {entries.map((e) => (
                <div className="storage-row" key={e.path}>
                  <div
                    className="storage-path"
                    style={{ paddingInlineStart: `${e.depth * 12}px` }}
                  >
                    <strong>{e.path}</strong>
                    <i style={{ width: `${(100 * e.size_bytes) / max}%` }} />
                  </div>
                  <span>
                    {e.complete
                      ? formatBytes(e.size_bytes)
                      : t("storage.partial")}
                  </span>
                  <span>
                    {e.growth_7d_bytes === undefined
                      ? t("storage.waiting_growth")
                      : `${e.growth_7d_bytes >= 0 ? "+" : "−"}${formatBytes(Math.abs(e.growth_7d_bytes))}`}
                  </span>
                </div>
              ))}
              <p className="foot">
                {" "}{t("ui.directory.totals.include.descendants.symbolic.li.373ca")}{" "}</p>
            </section>
          );
        })
      ) : (
        <section className="g t">
          <h2>{t("ui.waiting.for.storage.scan.699ef")}</h2>
          <p className="foot">
            {" "}{t("ui.the.agent.scans.configured.roots.once.per.hour.2fffc")}{" "}</p>
        </section>
      )}
    </>
  );
}
