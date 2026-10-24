import { Addrbook } from "@react95/icons/Addrbook";
import type { Metadata } from "next";

import { LoginForm } from "@/components/admin/login-form";
import { Window } from "@/components/win/window";

export const metadata: Metadata = {
  title: "管理员登录",
};

export default function AdminLoginPage() {
  return (
    <main>
      <Window
        title="管理员登录 - biodays"
        icon={<Addrbook variant="16x16_4" aria-hidden />}
        status={["单管理员", "biodays"]}
      >
        <h1 className="text-base font-bold">管理员登录</h1>
        <div className="mt-3">
          <LoginForm />
        </div>
      </Window>
    </main>
  );
}
