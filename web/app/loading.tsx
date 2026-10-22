import { ProgressBar } from "@react95/core";

export default function Loading() {
  return (
    <main className="mx-auto flex max-w-[420px] flex-col gap-2 p-6" aria-busy="true">
      <p>正在读取…</p>
      <ProgressBar percent={50} width="100%" />
    </main>
  );
}
