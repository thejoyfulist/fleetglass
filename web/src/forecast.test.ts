import {describe,expect,it,vi} from "vitest";
import {timeline,timelineDayLabels,timelineNowCell} from "./forecast";
import type {Quota} from "./data";

vi.stubEnv("TZ","Europe/Paris");
const quota=(resets_at:string,window_minutes=10080):Quota=>({kind:"claude",window:window_minutes===300?"5h":"7d",window_minutes,used_percent:70,resets_at,observed_at:resets_at,method:"reported",forecast:{pace_pph:1,even_ratio:1.68,percent_at_reset:100,locked_hours:0,daily:[]}});

describe("quota timeline",()=>{
 it("shows a runout marker, locked cells and reset",()=>{
  const q:Quota={kind:"codex",window:"7d",window_minutes:10080,used_percent:80,resets_at:"2026-10-23T12:00:00Z",observed_at:"2026-10-20T12:00:00Z",method:"reported",forecast:{pace_pph:2,even_ratio:3.36,percent_at_reset:100,runs_out_at:"2026-10-22T00:00:00Z",locked_hours:36,daily:[]}};
  const result=timeline(q);expect(result.cells).toHaveLength(28);expect(result.marker).toBeGreaterThan(0);expect(result.cells).toContain("▒");
 });
 it("labels a local day at cell zero when reset is just after midnight",()=>{
  const labels=timelineDayLabels(quota("2026-10-19T22:35:00Z"));
  expect(labels.map(d=>d.label)).toEqual(["Tue","Wed","Thu","Fri","Sat","Sun","Mon","Tue"]);
  expect(labels.map(d=>d.start)).toEqual([0,3,7,11,15,19,23,27]);
 });
 it("skips a cramped partial first day and marks actual mid-day window boundaries",()=>{
  const labels=timelineDayLabels(quota("2026-10-22T12:00:00Z"));
  expect(labels.map(d=>d.label)).toEqual(["Fri","Sat","Sun","Mon","Tue","Wed","Thu"]);
  expect(labels.map(d=>d.start)).toEqual([1,5,9,13,17,21,25]);
 });
 it("uses 25-hour local day spacing across the Paris autumn DST change",()=>{
  const labels=timelineDayLabels(quota("2026-10-27T11:00:00Z"));
  expect(labels.map(d=>d.label)).toEqual(["Wed","Thu","Fri","Sat","Sun","Mon","Tue"]);
  expect(labels.map(d=>d.start)).toEqual([1,5,9,13,17,22,26]);
 });
 it("uses local clock labels for five-hour windows and locates now",()=>{
  const q=quota("2026-10-20T03:00:00Z",300);
  expect(timelineDayLabels(q)).toEqual([{label:"00:00",start:0},{label:"03:00",start:12}]);
  expect(timelineNowCell(q,new Date("2026-10-19T22:45:00Z").getTime())).toBe(3);
  expect(timelineNowCell(q,new Date("2026-10-20T03:00:00Z").getTime())).toBe(-1);
 });
});
