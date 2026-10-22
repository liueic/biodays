import { Button, Frame, TitleBar } from "@react95/core";
import { User2 } from "@react95/icons";
import Link from "next/link";

type ErrorStateProps = {
  title: string;
  description: string;
  code?: string;
  action?: { href: string; label: string };
  children?: React.ReactNode;
};

/** Classic Win98 message box. */
export function ErrorState({ title, description, code, action, children }: ErrorStateProps) {
  return (
    <div className="flex justify-center py-8">
      <Frame
        role="alert"
        bgColor="$material"
        boxShadow="$out"
        padding="$2"
        className="w-full max-w-[420px]"
      >
        <TitleBar
          active
          title={code ? `biodays - 错误 ${code}` : "biodays"}
          className="win98-title"
        />
        <div className="flex gap-4 p-4">
          <User2 variant="32x32_4" width={32} height={32} aria-hidden className="shrink-0" />
          <div className="min-w-0">
            <p className="font-bold">{title}</p>
            <p className="mt-2">{description}</p>
          </div>
        </div>
        <div className="flex justify-center gap-2 pb-3">
          {children}
          {action && (
            <Button as={Link} href={action.href} className="px-6">
              {action.label}
            </Button>
          )}
        </div>
      </Frame>
    </div>
  );
}
