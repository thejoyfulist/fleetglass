import type {Change, Point, ProbePoint} from "./data";
import {formatPercent, formatNumber, formatBytes, formatRate} from "./data";
import {t} from "./i18n";
import {TerminalBar} from "./terminal";

const blocks = "▁▂▃▄▅▆▇█";
function trace(values: number[], width = 48): string {
  if (!values.length) return "—";
  const sample = Array.from({length: Math.min(width, values.length)}, (_, i) => values[Math.floor(i * values.length / Math.min(width, values.length))]);
  const min = Math.min(...sample), max = Math.max(...sample), span = max - min || 1;
  return sample.map(value => blocks[Math.min(7, Math.floor((value - min) / span * 7))]).join("");
}
export function Sparkline({values, idLabel}: {values:number[]; idLabel:string}) {
  return values.length < 2 ? null : <span className="terminal-spark" role="img" aria-label={t("chart.trend", {label:idLabel})}>{trace(values, 24)}</span>;
}
export function Ring({good,total}: {good:number;total:number}) {
  return <span className="terminal-checks" aria-label={total?t("chart.checks_normal", {good,total}):t("chart.no_checks")}><TerminalBar percent={total?good/total*100:null} width={12}/> {good}/{total}</span>;
}
export function Gauge({used,paceValue,label}: {used:number;paceValue:number;label:string}) {
  return <span aria-label={label}><TerminalBar percent={used} width={12} pace={paceValue}/> {formatPercent(used)}</span>;
}
export function Meter({value,max=100}: {value:number;max?:number}) {
  return <span><TerminalBar percent={max?100*value/max:null} width={12}/> {formatPercent(value)}</span>;
}
export function AreaChart({primary,secondary,label,period,rightUnit="W",unit="percent"}: {primary:Point[];secondary?:Point[];label:string;period:string;rightUnit?:string;changes?:Change[];unit?:"percent"|"bytes"|"rate"}) {
  const format = (value:number) => unit==="bytes"?formatBytes(value):unit==="rate"?formatRate(value):rightUnit==="W"?formatPercent(value):`${formatNumber(value,1)} ${rightUnit}`;
  const row = (points:Point[],name:string) => <div className="terminal-chart-row"><span>{name}</span><span className="terminal-spark" role="img" aria-label={`${label} ${name} ${period}: ${trace(points.map(p=>p.value))}`}>{trace(points.map(p=>p.value))}</span><span>{points.length?format(points.at(-1)!.value):"—"}</span></div>;
  return <div className="terminal-chart">{row(primary,label)}{secondary?.length?row(secondary,t("terminal.label_used")):null}</div>;
}
export function ResponseChart({points,label,period}: {points:ProbePoint[];label:string;period:string}) {
  const good=points.filter(p=>p.state==="ok" && Number.isFinite(p.latency_ms));
  return <div className="terminal-chart"><div className="terminal-chart-row"><span>{label}</span><span className="terminal-spark" role="img" aria-label={t("chart.response", {label,period,failed:points.length-good.length})}>{trace(good.map(p=>p.latency_ms))}</span><span>{good.length?`${formatNumber(good.at(-1)!.latency_ms)} ms`:"—"}</span></div><div className="terminal-dim">{points.length-good.length} {t("terminal.label_errors")}</div></div>;
}
export function Bars({values}: {values:{label:string;value:number}[]}) {
  const max=Math.max(1,...values.map(v=>v.value));
  return <div className="terminal-chart">{values.map((v,i)=><div className="terminal-chart-row" key={`${v.label}-${i}`}><span>{v.label}</span><TerminalBar percent={v.value/max*100} width={24}/><span>{formatNumber(v.value)}</span></div>)}</div>;
}
