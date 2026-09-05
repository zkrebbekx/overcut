// Web Worker that runs the Go engine as WebAssembly. The main thread sends
// {id, method, args}; the worker replies {id, result} or {id, error}.
// Simulations and the optimizer run here so the UI never blocks.

/// <reference lib="webworker" />

declare const self: DedicatedWorkerGlobalScope & {
  Go: new () => { importObject: WebAssembly.Imports; run(instance: WebAssembly.Instance): Promise<void> };
  overcut: Record<string, (...args: unknown[]) => string>;
};

interface Request {
  id: number;
  method: string;
  args: unknown[];
}

const base = (self.location.pathname.match(/^(.*\/)assets\//) ?? [null, "/"])[1] ?? "/";

let ready: Promise<void> | null = null;

function boot(): Promise<void> {
  if (ready) return ready;
  ready = (async () => {
    self.importScripts(`${base}wasm_exec.js`);
    const go = new self.Go();
    const wasm = fetch(`${base}overcut.wasm`);
    let instance: WebAssembly.Instance;
    try {
      instance = (await WebAssembly.instantiateStreaming(wasm, go.importObject)).instance;
    } catch {
      // A server without the application/wasm MIME type fails the
      // streaming path; compile from bytes instead.
      const bytes = await (await fetch(`${base}overcut.wasm`)).arrayBuffer();
      instance = (await WebAssembly.instantiate(bytes, go.importObject)).instance;
    }
    void go.run(instance);
    // go.run resolves only when the program exits; wait for the API to
    // appear instead.
    while (!self.overcut) await new Promise((r) => setTimeout(r, 5));
    const season = await fetch(`${base}data/season.json`).then((r) => {
      if (!r.ok) throw new Error(`season data: ${r.status}`);
      return r.text();
    });
    const err = self.overcut.load(season);
    if (err) throw new Error(err);
  })();
  return ready;
}

self.onmessage = async (ev: MessageEvent<Request>) => {
  const { id, method, args } = ev.data;
  try {
    await boot();
    const fn = self.overcut[method];
    if (!fn) throw new Error(`unknown method ${method}`);
    const out = JSON.parse(fn(...args));
    if (out && typeof out === "object" && "error" in out && Object.keys(out).length === 1) {
      self.postMessage({ id, error: out.error });
    } else {
      self.postMessage({ id, result: out });
    }
  } catch (e) {
    self.postMessage({ id, error: String((e as Error).message ?? e) });
  }
};
