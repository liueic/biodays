import { ErrorState } from "@/components/error-state";

export default function NotFound() {
  return (
    <main className="mx-auto max-w-3xl px-5">
      <ErrorState
        code="404"
        title="没有找到这篇故事"
        description="链接可能有误，或该条目尚未收录。"
        action={{ href: "/archive", label: "浏览归档" }}
      />
    </main>
  );
}
