import { t } from "./i18n";
import { useEffect, useId, useRef, useState } from "react";
import type { Change, Point, ProbePoint } from "./data";
import {formatBytes, formatNumber, formatPercent, formatQuotaPercent, resetTime} from "./data";
function coords(
  values: number[],
  w: number,
  h: number,
  min: number,
  max: number,
  left = 0,
  right = 0,
  top = 4,
  bottom = 4,
) {
  const span = max - min || 1;
  return values.map(
    (v, i) =>
      [
        left + ((w - left - right) * i) / Math.max(1, values.length - 1),
        top + (h - top - bottom) * (1 - (v - min) / span),
      ] as const,
  );
}
function path(points: readonly (readonly [number, number])[]) {
  return points
    .map(([x, y], i) => `${i ? "L" : "M"}${x.toFixed(1)} ${y.toFixed(1)}`)
    .join(" ");
}
export function Sparkline({
  values,
  idLabel,
}: {
  values: number[];
  idLabel: string;
}) {
  const id = useId().replace(/:/g, "");
  if (values.length < 2) return null;
  const pts = coords(values, 200, 42, Math.min(...values), Math.max(...values));
  const line = path(pts);
  return (
    <svg
      className="spark"
      viewBox="0 0 200 42"
      preserveAspectRatio="none"
      role="img"
      aria-label={t("chart.trend", {label:idLabel})}
    >
      <defs>
        <linearGradient id={`fill${id}`} x2="0" y2="1">
          <stop stopColor="var(--b1)" stopOpacity=".22" />
          <stop offset="1" stopColor="var(--b1)" stopOpacity="0" />
        </linearGradient>
        <linearGradient id={`line${id}`}>
          <stop stopColor="var(--b1)" />
          <stop offset="1" stopColor="var(--b2)" />
        </linearGradient>
      </defs>
      <path d={`${line} L200 42 L0 42 Z`} fill={`url(#fill${id})`} />
      <path
        d={line}
        fill="none"
        stroke={`url(#line${id})`}
        strokeWidth="1.6"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
export function Ring({ good, total }: { good: number; total: number }) {
  const id = useId().replace(/:/g, "");
  const ratio = total ? good / total : 0;
  return (
    <svg
      className="ring"
      viewBox="0 0 120 120"
      role="img"
      aria-label={
        total ? t("chart.checks_normal", {good,total}) : t("chart.no_checks")
      }
    >
      <defs>
        <linearGradient id={id}>
          <stop stopColor="var(--b1)" />
          <stop offset="1" stopColor="var(--b2)" />
        </linearGradient>
      </defs>
      <circle
        cx="60"
        cy="60"
        r="50"
        fill="none"
        stroke="var(--track)"
        strokeWidth="10"
      />
      <circle
        cx="60"
        cy="60"
        r="50"
        fill="none"
        stroke={`url(#${id})`}
        strokeWidth="10"
        strokeLinecap="round"
        strokeDasharray={`${Math.max(0, ratio * 314)} 314`}
        transform="rotate(-90 60 60)"
      />
      <text
        x="60"
        y="58"
        textAnchor="middle"
        fontSize="26"
        fontWeight="600"
        fill="var(--ink)"
      >
        {total ? formatNumber(good) : "—"}
      </text>
      <text x="60" y="76" textAnchor="middle" fontSize="11" fill="var(--ink3)">
        {t("chart.of_checks", {total})}</text>
    </svg>
  );
}
export function Gauge({
  used,
  paceValue,
  label,
}: {
  used: number;
  paceValue: number;
  label: string;
}) {
  const id = useId().replace(/:/g, "");
  const displayed = Math.round(used);
  const angle = Math.PI * (1 - paceValue / 100);
  const x = 75 + 60 * Math.cos(angle),
    y = 85 - 60 * Math.sin(angle);
  return (
    <svg
      className="gauge"
      viewBox="0 0 150 100"
      role="img"
      aria-label={t("chart.gauge", {label, used:formatQuotaPercent(displayed), pace:formatQuotaPercent(paceValue)})}
    >
      <defs>
        <linearGradient id={id}>
          <stop stopColor="var(--warnbar1)" />
          <stop offset="1" stopColor="var(--warnbar2)" />
        </linearGradient>
      </defs>
      <path
        d="M15 85 A60 60 0 0 1 135 85"
        fill="none"
        stroke="var(--track)"
        strokeWidth="12"
        strokeLinecap="round"
      />
      <path
        d="M15 85 A60 60 0 0 1 135 85"
        fill="none"
        stroke={used > paceValue + 5 ? `url(#${id})` : "var(--b1)"}
        strokeWidth="12"
        strokeLinecap="round"
        strokeDasharray={`${Math.max(0, Math.min(100, displayed)) * 1.885} 188.5`}
      />
      <line
        x1={x}
        x2={x + Math.cos(angle) * 11}
        y1={y}
        y2={y - Math.sin(angle) * 11}
        stroke="var(--ink)"
        strokeWidth="2"
      />
      <text
        x="75"
        y="76"
        textAnchor="middle"
        fontSize="24"
        fontWeight="600"
        fill="var(--ink)"
      >
        {formatQuotaPercent(displayed)}
      </text>
      <text x="75" y="94" textAnchor="middle" fontSize="10" fill="var(--ink3)">
        {label}
      </text>
    </svg>
  );
}
export function Meter({ value, max = 100 }: { value: number; max?: number }) {
  return (
    <span className="meter">
      <span>{formatPercent(value)}</span>
      <i>
        <b style={{ width: `${Math.min(100, (100 * value) / max)}%` }} />
      </i>
    </span>
  );
}
export function AreaChart({
  primary,
  secondary,
  label,
  period,
  rightUnit = "W",
  changes = [],
	unit = "percent",
}: {
  primary: Point[];
  secondary?: Point[];
  label: string;
  period: string;
  rightUnit?: string;
  changes?: Change[];
	unit?: "percent"|"bytes"|"rate";
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ w: 600, h: 240 });
  const id = useId().replace(/:/g, "");
  useEffect(() => {
    if (!ref.current) return;
    const obs = new ResizeObserver((e) =>
      setSize({
        w: Math.round(e[0].contentRect.width),
        h: Math.round(e[0].contentRect.height),
      }),
    );
    obs.observe(ref.current);
    return () => obs.disconnect();
  }, []);
  const { w, h } = size,
	left = unit==="percent"?34:58,
    right = secondary?.length ? 40 : 12,
    top = 8,
    bottom = 24;
  const values = primary.map((x) => x.value),
    secondaryValues = secondary?.map((x) => x.value) || [];
  const max = Math.max(100, ...values),
    rightMax = Math.max(1, ...secondaryValues) * 1.15;
  const pts = coords(values, w, h, 0, max, left, right, top, bottom),
    pts2 = coords(secondaryValues, w, h, 0, rightMax, left, right, top, bottom);
  const endAt=Date.now(), startAt=endAt-({"1h":36e5,"12h":12*36e5,"24h":864e5,"7d":7*864e5,"30d":30*864e5}[period] || 864e5);
  const markers=changes.filter(c=>{const at=new Date(c.at).getTime();return at>=startAt&&at<=endAt}).slice(0,40).map(c=>({c,x:left+(w-left-right)*(new Date(c.at).getTime()-startAt)/(endAt-startAt)}));
  return (
    <div ref={ref} className="chart">
      <svg
        viewBox={`0 0 ${w} ${h}`}
        role="img"
        aria-label={`${t("chart.latest", {label,period,latest:values.length ? formatNumber(values.at(-1)!, 1) : t("value.unknown")})} ${secondaryValues.length ? t("chart.secondary", {latest:formatNumber(secondaryValues.at(-1)!, 1), unit:rightUnit}) : t("chart.no_secondary")}`}
      >
        <defs>
          <linearGradient id={`area${id}`} x2="0" y2="1">
            <stop stopColor="var(--b1)" stopOpacity=".25" />
            <stop offset="1" stopColor="var(--b1)" stopOpacity="0" />
          </linearGradient>
          <linearGradient id={`stroke${id}`}>
            <stop stopColor="var(--b1)" />
            <stop offset="1" stopColor="var(--b2)" />
          </linearGradient>
        </defs>
        {markers.map(({c,x})=><line key={c.id} x1={x} x2={x} y1={top} y2={h-bottom} stroke="var(--power)" strokeOpacity=".55" strokeDasharray="3 4"><title>{c.type}: {c.text}</title></line>)}
        {[0, 0.25, 0.5, 0.75, 1].map((t) => {
          const y = top + (h - top - bottom) * (1 - t);
          return (
            <g key={t}>
              <line
                x1={left}
                x2={w - right}
                y1={y}
                y2={y}
                stroke="var(--hair)"
              />
              <text className="axis" x={left - 6} y={y + 4} textAnchor="end">
				{unit==="percent"?formatNumber(Math.round(max*t)):formatBytes(max*t)}
              </text>
              {secondaryValues.length > 0 ? (
                <text className="axis" x={w - right + 7} y={y + 4}>
                  {formatNumber(rightMax * t)}
                </text>
              ) : null}
            </g>
          );
        })}
        {pts.length > 1 ? (
          <>
            <path
              d={`${path(pts)} L${w - right} ${h - bottom} L${left} ${h - bottom} Z`}
              fill={`url(#area${id})`}
            />
            <path
              d={path(pts)}
              fill="none"
              stroke={`url(#stroke${id})`}
              strokeWidth="1.8"
              vectorEffect="non-scaling-stroke"
            />
          </>
        ) : null}
        {pts2.length > 1 ? (
          <path
            d={path(pts2)}
            fill="none"
            stroke="var(--power)"
            strokeWidth="1.4"
            vectorEffect="non-scaling-stroke"
          />
        ) : null}
        {(period === "1h"
		  ? ["−60 m", "−45 m", "−30 m", "−15 m", "now"]
		  : period === "12h"
			? ["−12 h", "−9 h", "−6 h", "−3 h", "now"]
		  : period === "24h"
          ? ["−24 h", "−18 h", "−12 h", "−6 h", "now"]
          : period === "7d"
            ? ["−7 d", "−5 d", "−3 d", "−1 d", "now"]
            : ["−30 d", "−22 d", "−15 d", "−7 d", "now"]
        ).map((tick, i) => (
          <text
            key={i}
            className="axis"
            x={left + ((w - left - right) * i) / 4}
            y={h - 4}
            textAnchor={i === 0 ? "start" : i === 4 ? "end" : "middle"}
          >
            {tick === "now" ? t("ui.now.c9bc8") : tick}
          </text>
        ))}
      </svg>
    </div>
  );
}
export function ResponseChart({points, label, period}: {points: ProbePoint[]; label: string; period: string}) {
  const ref = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({w: 600, h: 190});
  const id = useId().replace(/:/g, "");
  useEffect(() => {
    if (!ref.current) return;
    const observer = new ResizeObserver(([entry]) => setSize({w: Math.round(entry.contentRect.width), h: Math.round(entry.contentRect.height)}));
    observer.observe(ref.current);
    return () => observer.disconnect();
  }, []);
  const {w, h} = size;
  const left = 42, right = 12, top = 10, bottom = 25;
  const end = Date.now();
  const start = end - ({"1h":36e5,"12h":12*36e5,"24h": 864e5, "7d": 7*864e5, "30d": 30*864e5}[period] || 864e5);
  const good = points.filter(p => p.state === "ok" && Number.isFinite(p.latency_ms) && p.latency_ms >= 0);
  const max = Math.max(1, ...good.map(p => p.latency_ms)) * 1.15;
  const x = (at: string) => left + (w-left-right) * Math.max(0, Math.min(1, (new Date(at).getTime()-start)/(end-start)));
  const y = (ms: number) => top + (h-top-bottom) * (1-ms/max);
  const segments: ProbePoint[][] = [];
  for (const [index, point] of points.entries()) {
    if (point.state !== "ok" || !Number.isFinite(point.latency_ms)) continue;
    if (!segments.length || points[index-1]?.state !== "ok") segments.push([]);
    segments.at(-1)!.push(point);
  }
  return <div ref={ref} className="chart response-chart">
    <svg viewBox={`0 0 ${w} ${h}`} role="img" aria-label={t("chart.response", {label,period,failed:points.filter(p=>p.state!=="ok").length})}>
      <defs><linearGradient id={`response-${id}`} x2="0" y2="1"><stop stopColor="var(--b1)" stopOpacity=".23"/><stop offset="1" stopColor="var(--b1)" stopOpacity="0"/></linearGradient></defs>
      {[0,.25,.5,.75,1].map(fraction => {const yy=top+(h-top-bottom)*(1-fraction); return <g key={fraction}><line x1={left} x2={w-right} y1={yy} y2={yy} stroke="var(--hair)"/><text className="axis" x={left-6} y={yy+4} textAnchor="end">{formatNumber(Math.round(max*fraction))} {" "}{t("ui.ms.26cc3")}</text></g>})}
      {segments.filter(s=>s.length>1).map((s,i) => {const line=path(s.map(p=>[x(p.at),y(p.latency_ms)] as const));return <g key={i}><path d={`${line} L${x(s.at(-1)!.at)} ${h-bottom} L${x(s[0].at)} ${h-bottom} Z`} fill={`url(#response-${id})`}/><path d={line} fill="none" stroke="var(--b1)" strokeWidth="1.8" vectorEffect="non-scaling-stroke"/></g>})}
      {good.length===1?<circle cx={x(good[0].at)} cy={y(good[0].latency_ms)} r="3" fill="var(--b1)"/>:null}
      {points.filter(p=>p.state!=="ok").map((p,i)=><g key={`${p.at}-${i}`}><line x1={x(p.at)} x2={x(p.at)} y1={top} y2={h-bottom} stroke={p.state==="crit"?"var(--crit)":p.state==="warn"?"var(--warn)":"var(--ink3)"} strokeDasharray="3 4"/><circle cx={x(p.at)} cy={h-bottom-4} r="3" fill={p.state==="crit"?"var(--crit)":p.state==="warn"?"var(--warn)":"var(--ink3)"}><title>{t(p.state==="crit"?"state.down":p.state==="warn"?"state.degraded":"state.unknown")} {" "}{t("ui.at.27e90")}{" "}{resetTime(p.at)}</title></circle></g>)}
      {(period==="1h"?["−60 m","−45 m","−30 m","−15 m","now"]:period==="12h"?["−12 h","−9 h","−6 h","−3 h","now"]:period==="24h"?["−24 h","−18 h","−12 h","−6 h","now"]:period==="7d"?["−7 d","−5 d","−3 d","−1 d","now"]:["−30 d","−22 d","−15 d","−7 d","now"]).map((tick,i)=><text key={tick} className="axis" x={left+(w-left-right)*i/4} y={h-4} textAnchor={i===0?"start":i===4?"end":"middle"}>{tick === "now" ? t("ui.now.c9bc8") : tick}</text>)}
    </svg>
  </div>;
}
export function Bars({
  values,
}: {
  values: { label: string; value: number }[];
}) {
  const max = Math.max(1, ...values.map((x) => x.value));
  return (
    <div
      className="days"
      role="img"
      aria-label={values.map((v) => `${v.label}: ${v.value} tokens`).join(", ")}
    >
      {values.map((v, i) => (
        <div key={`${v.label}${i}`}>
          <i
            className={i === values.length - 1 ? t("ui.now.c9bc8") : ""}
            style={{ height: `${Math.max(4, (70 * v.value) / max)}px` }}
            title={`${v.value} tokens`}
          />
          <span>{v.label}</span>
        </div>
      ))}
    </div>
  );
}
