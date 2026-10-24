"use client";

import { Mail } from "@react95/icons/Mail";
import { usePathname, useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { Window } from "@/components/win/window";
import {
  type AdminSession,
  login as apiLogin,
  logout as apiLogout,
  FeedbackApiError,
  me,
} from "@/lib/admin-api";

type AdminSessionContextValue = {
  session: AdminSession | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AdminSessionContext = createContext<AdminSessionContextValue | null>(null);

export function AdminSessionProvider({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const isLoginPage = pathname === "/admin/login";
  const [session, setSession] = useState<AdminSession | null>(null);
  const [loading, setLoading] = useState(!isLoginPage);
  const [error, setError] = useState("");

  useEffect(() => {
    if (isLoginPage) {
      setLoading(false);
      setSession(null);
      setError("");
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError("");
    me()
      .then((value) => {
        if (!cancelled) setSession(value);
      })
      .catch((reason) => {
        if (cancelled) return;
        if (reason instanceof FeedbackApiError && reason.status === 401) {
          router.replace("/admin/login");
          return;
        }
        if (reason instanceof FeedbackApiError && reason.status === 503) {
          setError("反馈功能尚未启用。");
          return;
        }
        setError("无法验证管理员会话，请检查服务状态后重试。");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [isLoginPage, router]);

  const login = useCallback(
    async (username: string, password: string) => {
      const value = await apiLogin(username, password);
      setSession(value);
      router.replace("/admin/feedback");
    },
    [router],
  );

  const logout = useCallback(async () => {
    await apiLogout();
    setSession(null);
    router.replace("/admin/login");
  }, [router]);

  const value = useMemo(() => ({ session, login, logout }), [session, login, logout]);

  if (!isLoginPage && loading) {
    return (
      <main>
        <Window
          title="管理员登录 - biodays"
          icon={<Mail variant="16x16_4" aria-hidden />}
          status={["验证会话"]}
        >
          <p role="status" className="reading">
            正在验证管理员会话…
          </p>
        </Window>
      </main>
    );
  }

  if (!isLoginPage && error) {
    return (
      <main>
        <Window
          title="管理员登录 - biodays"
          icon={<Mail variant="16x16_4" aria-hidden />}
          status={["无法验证"]}
        >
          <p role="alert" className="reading">
            {error}
          </p>
          <p className="reading mt-3">
            返回{" "}
            <a className="text-link" href="/">
              今日故事
            </a>
            。
          </p>
        </Window>
      </main>
    );
  }

  if (!isLoginPage && !session) {
    return (
      <main>
        <Window
          title="管理员登录 - biodays"
          icon={<Mail variant="16x16_4" aria-hidden />}
          status={["需要登录"]}
        >
          <p role="status" className="reading">
            正在跳转到登录页…
          </p>
        </Window>
      </main>
    );
  }

  return <AdminSessionContext.Provider value={value}>{children}</AdminSessionContext.Provider>;
}

export function useAdminSession(): AdminSessionContextValue {
  const value = useContext(AdminSessionContext);
  if (!value) {
    throw new Error("useAdminSession must be used inside AdminSessionProvider");
  }
  return value;
}
