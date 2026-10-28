import { describe, expect, test } from "bun:test";
import { runInNewContext } from "node:vm";
import { detectOsTheme, OS_THEME_INLINE_SCRIPT } from "../lib/os-theme";

const agents = [
  { name: "Mac", ua: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", touch: 0, os: "mac" },
  { name: "Windows", ua: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", touch: 0, os: "win" },
  { name: "Linux", ua: "Mozilla/5.0 (X11; Linux x86_64)", touch: 0, os: "win" },
  {
    name: "iPhone",
    ua: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)",
    touch: 5,
    os: "win",
  },
  {
    name: "iPad desktop mode",
    ua: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
    touch: 5,
    os: "win",
  },
] as const;

function bootstrap(ua: string, touch = 0, cookie = "", blocked = false) {
  const dataset: Record<string, string> = {};
  runInNewContext(OS_THEME_INLINE_SCRIPT, {
    navigator: { userAgent: ua, maxTouchPoints: touch },
    document: {
      documentElement: { dataset },
      get cookie() {
        if (blocked) throw new Error("Cookies blocked");
        return cookie;
      },
    },
  });
  return dataset.os;
}

describe("OS theme preferences", () => {
  for (const agent of agents) {
    test(`${agent.name} selects the same theme before and after hydration`, () => {
      expect(detectOsTheme(agent.ua, agent.touch)).toBe(agent.os);
      expect(bootstrap(agent.ua, agent.touch)).toBe(agent.os);
    });
  }

  test("saved preference takes priority over the operating system", () => {
    expect(bootstrap(agents[0].ua, 0, "other=value; biodays.os=win; x=y")).toBe("win");
    expect(bootstrap(agents[1].ua, 0, "biodays.os=mac")).toBe("mac");
  });

  test("invalid cookies are ignored and blocked cookies preserve OS detection", () => {
    expect(bootstrap(agents[1].ua, 0, "biodays.os=mac-invalid")).toBe("win");
    expect(bootstrap(agents[1].ua, 0, "not-biodays.os=mac")).toBe("win");
    expect(bootstrap(agents[0].ua, 0, "", true)).toBe("mac");
  });
});
