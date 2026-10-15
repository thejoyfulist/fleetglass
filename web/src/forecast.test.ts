import {describe,expect,it} from "vitest";
import {timeline} from "./forecast";
import type {Quota} from "./data";

describe("quota timeline",()=>{
 it("shows a runout marker, locked cells and reset",()=>{
  const q:Quota={kind:"codex",window:"7d",window_minutes:10080,used_percent:80,resets_at:"2026-10-23T12:00:00Z",observed_at:"2026-10-20T12:00:00Z",method:"reported",forecast:{pace_pph:2,even_ratio:3.36,percent_at_reset:100,runs_out_at:"2026-10-22T00:00:00Z",locked_hours:36,daily:[]}};
  const result=timeline(q);expect(result.cells).toHaveLength(28);expect(result.marker).toBeGreaterThan(0);expect(result.cells).toContain("▒");
 });
});
