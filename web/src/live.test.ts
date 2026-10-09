import { describe, it, expect } from "vitest";
import { connectLive, type LiveSource } from "./live";

class FakeSource implements LiveSource {
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  listener: ((event: MessageEvent) => void) | null = null;
  closed = false;
  addEventListener(_type: string, listener: (event: MessageEvent) => void) {
    this.listener = listener;
  }
  close() {
    this.closed = true;
  }
}

describe("live stream", () => {
  it("updates, reconnects with backoff, and stops after cleanup", () => {
    const sources: FakeSource[] = [];
    const scheduled: { callback: () => void; delay: number }[] = [];
    const states: string[] = [];
    const values: unknown[] = [];
    const stop = connectLive(
      "/api/v1/live",
      () => {
        const s = new FakeSource();
        sources.push(s);
        return s;
      },
      (data) => values.push(data),
      (state) => states.push(state),
      (callback, delay) => {
        scheduled.push({ callback, delay });
        return scheduled.length;
      },
      () => {},
    );
    sources[0].onopen?.();
    sources[0].listener?.({ data: '{"host_id":"one"}' } as MessageEvent);
    sources[0].onerror?.();
    expect(values).toEqual([{ host_id: "one" }]);
    expect(sources[0].closed).toBe(true);
    expect(scheduled[0].delay).toBe(1000);
    scheduled[0].callback();
    sources[1].onerror?.();
    expect(scheduled[1].delay).toBe(2000);
    stop();
    scheduled[1].callback();
    expect(sources).toHaveLength(2);
    expect(states).toContain("stale");
  });
});
