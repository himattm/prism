// Prism — Pi coding agent integration.
//
// Pi has no "run a command for the status line" mechanism like Claude Code does;
// the only way to render status is from a TypeScript extension loaded into Pi's
// own runtime. This file is that thin shim: it collects Pi's live session data,
// hands it to the `prism` binary over stdin using the exact same JSON contract
// prism already speaks for Claude Code, and pushes prism's rendered output into
// Pi's footer via ctx.ui.setStatus().
//
// This file is shipped *inside* the prism binary and materialized on disk by
// `prism install-pi`, which also writes the sibling `prism-bin.txt` pointing at
// the prism binary to invoke. Users never have to write or install it by hand.

import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

// Types are erased at runtime; we only ever use the `pi`/`ctx` objects Pi passes
// in, so the extension has no runtime dependency on the pi packages.
type ExecResult = { code: number; stdout: string };

const STATUS_KEY = "prism";
const RENDER_TIMEOUT_MS = 5000;
const HOOK_TIMEOUT_MS = 3000;
const MIN_RENDER_INTERVAL_MS = 600;

// Resolve the prism binary: explicit override, then the path recorded by
// `prism install-pi` next to this file, then fall back to PATH.
function resolvePrismBin(): string {
  if (process.env.PRISM_BIN) return process.env.PRISM_BIN;
  try {
    const here = dirname(fileURLToPath(import.meta.url));
    const recorded = readFileSync(join(here, "prism-bin.txt"), "utf8").trim();
    if (recorded) return recorded;
  } catch {
    /* fall through to PATH lookup */
  }
  return "prism";
}

const PRISM_BIN = resolvePrismBin();

function sessionId(ctx: any): string {
  try {
    return ctx?.sessionManager?.getSessionFile?.() ?? "";
  } catch {
    return "";
  }
}

// Build the JSON contract prism already understands (see internal/statusline
// types.go). Pi doesn't expose per-message cost, so cost is left at zero — and
// prism's default layout doesn't include a cost section, so nothing is lost.
function buildInput(ctx: any): string {
  const usage = (() => {
    try {
      return ctx?.getContextUsage?.();
    } catch {
      return undefined;
    }
  })();

  return JSON.stringify({
    agent: "pi",
    session_id: sessionId(ctx),
    model: {
      display_name: ctx?.model?.name ?? ctx?.model?.id ?? "",
    },
    workspace: {
      project_dir: ctx?.cwd ?? "",
      current_dir: ctx?.cwd ?? "",
    },
    context_window: {
      context_window_size: usage?.contextWindow ?? 0,
      used_percentage: usage?.percent ?? 0,
      current_usage: {
        input_tokens: usage?.tokens ?? 0,
        output_tokens: 0,
        cache_creation_input_tokens: 0,
        cache_read_input_tokens: 0,
      },
    },
  });
}

function runPrism(args: string[], stdinData: string, cwd: string, timeoutMs: number): Promise<ExecResult> {
  return new Promise((resolve) => {
    let child;
    try {
      // Ignore the child's stderr: we only consume stdout + the exit code, and
      // an unread piped stderr can block the child once its buffer fills.
      child = spawn(PRISM_BIN, args, { cwd, stdio: ["pipe", "pipe", "ignore"] });
    } catch {
      resolve({ code: 1, stdout: "" });
      return;
    }

    let stdout = "";
    let settled = false;
    // Declared before `done` so the closure can't hit a temporal dead zone.
    let timer: ReturnType<typeof setTimeout> | undefined;
    const done = (result: ExecResult) => {
      if (settled) return;
      settled = true;
      if (timer) clearTimeout(timer);
      resolve(result);
    };

    timer = setTimeout(() => {
      try {
        child.kill("SIGKILL");
      } catch {
        /* ignore */
      }
      done({ code: 124, stdout });
    }, timeoutMs);

    child.stdout?.on("data", (chunk: { toString(): string }) => {
      stdout += chunk.toString();
    });
    child.on("error", () => done({ code: 1, stdout: "" }));
    child.on("close", (code: number | null) => done({ code: code ?? 0, stdout }));
    // If prism exits before reading all of stdin, the write can emit an async
    // 'error' (EPIPE). Without this listener that would crash the host process.
    child.stdin?.on("error", () => {
      /* ignored: the close/error handlers settle the promise */
    });

    try {
      child.stdin?.write(stdinData);
      child.stdin?.end();
    } catch {
      /* the close/error handler will resolve */
    }
  });
}

export default function prism(pi: any) {
  let lastRender = 0;
  let pending = false; // a render is queued behind the in-flight one
  let inFlight = false;
  let trailingTimer: ReturnType<typeof setTimeout> | null = null;
  let latestCtx: any = null; // always render against the most recent event's ctx

  function clearTrailing(): void {
    if (trailingTimer) {
      clearTimeout(trailingTimer);
      trailingTimer = null;
    }
  }

  async function render(ctx: any): Promise<void> {
    latestCtx = ctx;
    clearTrailing(); // a direct render supersedes any queued trailing render
    if (inFlight) {
      pending = true; // coalesce: run exactly once more after the current one
      return;
    }
    inFlight = true;
    lastRender = Date.now();
    try {
      const cur = latestCtx;
      const { code, stdout } = await runPrism([], buildInput(cur), cur?.cwd ?? process.cwd(), RENDER_TIMEOUT_MS);
      if (code === 0) {
        const text = stdout.replace(/\n+$/, "");
        cur?.ui?.setStatus?.(STATUS_KEY, text.length > 0 ? text : undefined);
      }
    } catch {
      /* keep the previous status on failure */
    } finally {
      inFlight = false;
      if (pending) {
        pending = false;
        void render(latestCtx); // re-render with the freshest ctx, not a stale one
      }
    }
  }

  // Throttle bursty events (e.g. message_update) to at most one render per
  // interval, but always render once more on the trailing edge so the final
  // state is never dropped.
  function scheduleRender(ctx: any): void {
    latestCtx = ctx;
    if (inFlight) {
      pending = true;
      return;
    }
    const remaining = MIN_RENDER_INTERVAL_MS - (Date.now() - lastRender);
    if (remaining <= 0) {
      void render(latestCtx);
      return;
    }
    if (!trailingTimer) {
      trailingTimer = setTimeout(() => {
        trailingTimer = null;
        void render(latestCtx);
      }, remaining);
    }
  }

  // Reuse prism's existing idle/busy marker machinery so the idle indicator and
  // any idle-gated plugins (e.g. auto-update) behave the same as in Claude Code.
  async function setBusy(ctx: any, busy: boolean): Promise<void> {
    const id = sessionId(ctx);
    if (!id) return;
    await runPrism(["hook", busy ? "busy" : "idle"], JSON.stringify({ session_id: id }), ctx?.cwd ?? process.cwd(), HOOK_TIMEOUT_MS);
  }

  pi.on("session_start", async (_event: unknown, ctx: any) => {
    await setBusy(ctx, false);
    await render(ctx);
  });

  pi.on("agent_start", async (_event: unknown, ctx: any) => {
    await setBusy(ctx, true);
    scheduleRender(ctx);
  });

  pi.on("agent_end", async (_event: unknown, ctx: any) => {
    await setBusy(ctx, false);
    lastRender = 0; // force a fresh render on completion
    await render(ctx);
  });

  // Context usage / model changes during a turn — refresh the bar.
  pi.on("context", (_event: unknown, ctx: any) => scheduleRender(ctx));
  pi.on("message_update", (_event: unknown, ctx: any) => scheduleRender(ctx));
  pi.on("model_select", (_event: unknown, ctx: any) => {
    lastRender = 0;
    void render(ctx);
  });

  // Don't leave a pending timer holding the event loop open on shutdown.
  pi.on("session_shutdown", () => clearTrailing());
}
