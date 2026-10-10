export type TextCell = string | { text: string; tone?: "dim" | "warn" | "crit" | "bright" };
export type FormattedCell = { text: string; tone?: "dim" | "warn" | "crit" | "bright" };
import type {Result, SpeedSummary} from "./data";
import {activeLanguage} from "./i18n";

export function truncateName(name: string, max = 24): string {
  const chars = Array.from(name);
  return chars.length > max ? chars.slice(0, max - 1).join("") + "…" : name;
}

export function attentionLabel(row: Result): string {
  const count = row.values?.count ?? 0;
  const fr = activeLanguage() === "fr";
  if (row.name.startsWith("recent_error:")) return fr ? `erreurs du journal : ${row.fields?.unit || "autres"} · ${count} en 24 h` : `journal errors: ${row.fields?.unit || "other"} · ${count} in 24 h`;
  if (row.name === "recent_errors") return fr ? `erreurs du journal : ${count} en 24 h` : `journal errors: ${count} in 24 h`;
  if (row.name.startsWith("repository:")) {
    const name = row.fields?.repository || "Repository";
    const dirty = row.values?.dirty_count ?? 0;
    if (dirty) return fr ? `${name} : ${dirty} fichier${dirty === 1 ? "" : "s"} non validé${dirty === 1 ? "" : "s"}` : `${name}: ${dirty} uncommitted ${dirty === 1 ? "file" : "files"}`;
    if (row.values?.ahead) return `${name}: ${row.values.ahead} commits ahead`;
    if (row.values?.behind) return `${name}: ${row.values.behind} commits behind`;
    return `${name}: repository needs attention`;
  }
  if (row.name.startsWith("oom:")) return `out-of-memory: ${row.fields?.process || "process"} · ${count} in 24 h`;
  if (row.name.startsWith("tls:")) return `certificate ${row.name.slice(4)}: ${Math.ceil(row.values?.days_remaining ?? 0)} days remaining`;
  if (row.name.startsWith("systemd:")) return `${row.name.slice(8)}: service ${row.state}`;
  if (row.name.startsWith("container:")) return `${row.name.split(":").at(-1)}: container ${row.state}`;
  return `${row.fields?.display_name || row.name.split(":").at(-1)?.replaceAll("_", " ") || "Check"}: ${row.state}`;
}

export function measuredSpeed(rows: SpeedSummary[]): {model:string; rate:number; estimated:boolean}[] {
  const byModel = new Map<string,{model:string;rate:number;estimated:boolean}>();
  for (const row of rows) {
    const rate = row.live_tokens_per_second && row.live_tokens_per_second > 0 ? row.live_tokens_per_second : row.median_tokens_per_second;
    if (rate === null || rate === undefined || rate <= 0) continue;
    const prior = byModel.get(row.model);
    if (!prior || rate > prior.rate) byModel.set(row.model,{model:row.model,rate,estimated:row.method === "estimated"});
  }
  return [...byModel.values()];
}

export function formatTextTable(headers: string[], rows: TextCell[][], right: number[] = []): FormattedCell[][] {
  const plain = (cell: TextCell | undefined) => typeof cell === "string" ? cell : cell?.text ?? "";
  const widths = headers.map((head, column) => Math.max(head.length, ...rows.map(row => plain(row[column]).length)));
  return [headers.map((head, column) => ({text: head.padEnd(widths[column]), tone: "dim" as const})),
    ...rows.map(row => headers.map((_, column) => {
      const cell = row[column] ?? "";
      const text = plain(cell);
      return {text: right.includes(column) ? text.padStart(widths[column]) : text.padEnd(widths[column]),
        tone: typeof cell === "string" ? undefined : cell.tone};
    }))];
}

export function blockBar(percent: number, width: number, pace?: number): {text: string; filled: boolean; marker: boolean}[] {
  const amount = Math.max(0, Math.min(width, Math.round(percent / 100 * width)));
  const marker = pace === undefined ? -1 : Math.max(0, Math.min(width - 1, Math.round(pace / 100 * width)));
  return Array.from({length: width}, (_, index) => ({
    text: index === marker ? "│" : index < amount ? "█" : "░",
    filled: index < amount,
    marker: index === marker,
  }));
}
