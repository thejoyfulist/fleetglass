import { reconnectDelay } from "./data";

export interface LiveSource {
  onopen: (() => void) | null;
  onerror: (() => void) | null;
  addEventListener(type: string, listener: (event: MessageEvent) => void): void;
  close(): void;
}

export function browserLiveSource(path: string): LiveSource {
  const source = new EventSource(path);
  const adapter: LiveSource = {
    onopen: null,
    onerror: null,
    addEventListener(type, listener) {
      source.addEventListener(type, listener as EventListener);
    },
    close() {
      source.close();
    },
  };
  source.onopen = () => adapter.onopen?.();
  source.onerror = () => adapter.onerror?.();
  return adapter;
}

export function connectLive(
  path: string,
  create: (path: string) => LiveSource,
  onOverview: (data: unknown) => void,
  onState: (state: "connecting" | "live" | "stale") => void,
  schedule: (callback: () => void, delay: number) => number = (
    callback,
    delay,
  ) => window.setTimeout(callback, delay),
  clear: (timer: number) => void = (timer) => window.clearTimeout(timer),
): () => void {
  let source: LiveSource | undefined;
  let timer: number | undefined;
  let attempt = 0;
  let closed = false;
  function connect() {
    if (closed) return;
    onState(attempt ? "stale" : "connecting");
    source = create(path);
    source.onopen = () => {
      attempt = 0;
      onState("live");
    };
    source.addEventListener("overview", (event) => {
      try {
        onOverview(JSON.parse(event.data));
      } catch {
        /* Ignore malformed events. */
      }
    });
    source.onerror = () => {
      source?.close();
      onState("stale");
      timer = schedule(connect, reconnectDelay(attempt++));
    };
  }
  connect();
  return () => {
    closed = true;
    source?.close();
    if (timer !== undefined) clear(timer);
  };
}
