import { Addrbook } from "@react95/icons/Addrbook";
import type { Metadata } from "next";

import { LoginForm } from "@/components/admin/login-form";
import { Window } from "@/components/win/window";

export const metadata: Metadata = {
  title: "管理员登录",
};

export default async function AdminLoginPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const { error } = await searchParams;
  const initialError =
    error === "not_admin"
      ? "此账号没有管理员权限，请使用已获授权的账号。"
      : error === "auth_unavailable"
        ? "管理员认证暂不可用，请稍后重试。"
        : error === "github_failed"
          ? "GitHub 登录未完成，请重试。若持续失败，请检查 GitHub 登录与回调地址配置。"
          : "";
  return (
    <main>
      <Window
        title="管理员登录 - biodays"
        icon={<Addrbook variant="16x16_4" aria-hidden />}
        status={["单管理员", "biodays"]}
      >
        <h1 className="text-base font-bold">管理员登录</h1>
        <div className="mt-3">
          <LoginForm initialError={initialError} />
        </div>
      </Window>
    </main>
  );
}
