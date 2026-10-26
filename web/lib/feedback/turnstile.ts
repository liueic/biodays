export type TurnstileRenderOptions = {
  sitekey: string;
  theme?: "light" | "dark" | "auto";
  size?: "normal" | "flexible" | "compact";
  appearance?: "always" | "execute" | "interaction-only";
  action?: string;
  callback?: (token: string) => void;
  "expired-callback"?: () => void;
  "error-callback"?: (error?: string) => void;
  "timeout-callback"?: () => void;
};

export type TurnstileWidget = {
  render(container: HTMLElement | string, options: TurnstileRenderOptions): string | undefined;
  reset(widgetId?: string): void;
  remove(widgetId?: string): void;
};

declare global {
  interface Window {
    turnstile?: TurnstileWidget;
  }
}

const SCRIPT_ID = "cf-turnstile-script";
let loader: Promise<TurnstileWidget> | null = null;

export function loadTurnstile(timeoutMs = 8000): Promise<TurnstileWidget> {
  if (typeof window === "undefined") {
    return Promise.reject(new Error("turnstile_ssr"));
  }
  if (window.turnstile) {
    return Promise.resolve(window.turnstile);
  }
  if (loader) {
    return loader;
  }
  loader = new Promise<TurnstileWidget>((resolve, reject) => {
    const startedAt = Date.now();
    let done = false;

    const fail = () => {
      if (done) return;
      done = true;
      loader = null;
      reject(new Error("turnstile_unavailable"));
    };
    const succeed = () => {
      if (done) return;
      done = true;
      if (window.turnstile) {
        resolve(window.turnstile);
      } else {
        fail();
      }
    };
    const poll = () => {
      if (done) return;
      if (window.turnstile) {
        succeed();
        return;
      }
      if (Date.now() - startedAt >= timeoutMs) {
        fail();
        return;
      }
      setTimeout(poll, 50);
    };

    const existing = document.getElementById(SCRIPT_ID) as HTMLScriptElement | null;
    if (existing) {
      existing.addEventListener("load", poll, { once: true });
      existing.addEventListener("error", fail, { once: true });
      poll();
      return;
    }

    const script = document.createElement("script");
    script.id = SCRIPT_ID;
    script.src = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";
    script.async = true;
    script.defer = true;
    script.onload = poll;
    script.onerror = fail;
    document.head.appendChild(script);
    poll();
  });
  return loader;
}
