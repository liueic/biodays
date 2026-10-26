import { ErrorState } from "@/components/error-state";

// Root 404: rendered for URLs that match no route in either group, inside the
// minimal root layout (teal desktop + pixel font, but no taskbar — there is
// no window to task-switch to). notFound() calls inside a group render that
// group's not-found.tsx with the full desktop instead.
export default function RootNotFound() {
  return (
    <main className="mx-auto w-full max-w-[440px] px-5 pt-[18vh]">
      <ErrorState
        code="404"
        variant="warning"
        title="没有找到这个页面"
        description="链接可能有误，或该页面已被移动。"
        action={{ href: "/", label: "回到今日" }}
      />
    </main>
  );
}
