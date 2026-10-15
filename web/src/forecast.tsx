import {quotaWindow, type Quota} from "./data";
import {activeLanguage} from "./i18n";

const fr = () => activeLanguage()==="fr";
const date = (s:string) => new Date(s).toLocaleString(fr()?"fr-FR":"en-GB",{weekday:"short",hour:"2-digit",minute:"2-digit",hour12:false});
export function forecastText(q:Quota):string {
  const f=q.forecast;
  if(!f)return fr()?"rythme inconnu":"pace unknown";
  if(f.runs_out_at)return `${fr()?"épuisé":"runs out"} ${date(f.runs_out_at)} · ${fr()?"bloqué":"locked"} ${Math.round(f.locked_hours)} h`;
  return `~${Math.round(f.percent_at_reset)}% ${fr()?"à la remise à zéro":"at reset"}`;
}
export function forecastHeadline(quotas:Quota[]):string {
 const risky=quotas.filter(q=>q.forecast?.runs_out_at).sort((a,b)=>new Date(a.forecast!.runs_out_at!).getTime()-new Date(b.forecast!.runs_out_at!).getTime());
 if(risky.length){const q=risky[0];return `${q.kind} ${quotaWindow(q.window,q.window_minutes)} ${forecastText(q)}`}
 return quotas.some(q=>q.forecast)?(fr()?"Aucun quota prévu épuisé avant sa remise à zéro":"No plan runs out before reset"):(fr()?"Prévision inconnue : moins de deux observations récentes":"Forecast unknown: fewer than two recent observations");
}
export function timeline(q:Quota):{cells:string[];marker:number} {
 const f=q.forecast;if(!f)return {cells:[],marker:-1};
 const count=q.window_minutes===10080?28:20;
 const reset=new Date(q.resets_at).getTime(),start=reset-(q.window_minutes??10080)*60000;
 const run=f.runs_out_at?new Date(f.runs_out_at).getTime():Infinity;
 const usedEnd=start+(Math.min(100,f.percent_at_reset)/100)*(reset-start);
 const cells=Array.from({length:count},(_,i)=>{const at=start+(i+.5)*(reset-start)/count;return at>=run?"▒":at<=usedEnd?"█":"░"});
 const marker=Number.isFinite(run)?Math.max(0,Math.min(count-1,Math.floor((run-start)/(reset-start)*count))):-1;
 return {cells,marker};
}
export function ForecastView({quotas}:{quotas:Quota[]}) {
 return <div className="forecast-view"><h3>{fr()?"Prévisions":"Forecast"}</h3>
  <p>{forecastHeadline(quotas)}</p>
  {quotas.map(q=>{const f=q.forecast, line=timeline(q);return <div className="forecast-row" key={`${q.kind}:${q.window}`}>
   <div><strong>{q.kind} {quotaWindow(q.window,q.window_minutes)}</strong> · {forecastText(q)}{f?` · ${f.even_ratio.toFixed(1)}× ${fr()?"rythme régulier":"even pace"}`:""}</div>
   {f?<><div className="forecast-timeline" aria-label={forecastText(q)}>{line.cells.map((cell,i)=><span key={i} className={cell==="▒"?"terminal-warn":undefined}>{i===line.marker?"▴":cell}</span>)}<span>│</span></div>
   <div className="terminal-dim">{fr()?"Remise à zéro":"Reset"} {date(q.resets_at)}</div>
   {f.daily.map(d=><div className={`forecast-day ${d.used_pp>d.allowed_pp?"terminal-warn":""}`} key={d.day}>{d.day} · {d.used_pp.toFixed(1)} pp / {d.allowed_pp.toFixed(1)} pp {fr()?"autorisés":"allowed"} <span>{"█".repeat(Math.min(20,Math.ceil(d.used_pp/d.allowed_pp*10)))}</span></div>)}</>:<div className="terminal-dim">{fr()?"Moins de deux observations récentes":"Fewer than two recent observations"}</div>}
  </div>})}
 </div>;
}
