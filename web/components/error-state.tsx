import { Button } from "@react95/core/Button";
import { Frame } from "@react95/core/Frame";
import { TitleBar } from "@react95/core/TitleBar";
import Link from "next/link";

import { MacTitleBar } from "@/components/mac/window";
import { DialogIcon, type DialogVariant } from "@/components/win/dialog-icons";

type ErrorStateProps = {
  title: string;
  description: string;
  code?: string;
  /** Message-box flavour; picks the classic 32px dialog icon. */
  variant?: DialogVariant;
  action?: { href: string; label: string };
  children?: React.ReactNode;
};

/** Classic Win98 message box: gradient title bar, 32px dialog icon, centred buttons. */
export function ErrorState({
  title,
  description,
  code,
  variant = "critical",
  action,
  children,
}: ErrorStateProps) {
  return (
    <div className="flex justify-center py-8">
      <Frame
        role="alert"
        bgColor="$material"
        boxShadow="$out"
        padding="$2"
        className="mac-window w-full max-w-[440px]"
        data-active
      >
        <TitleBar
          active
          title={code ? `biodays - ${code}` : "biodays"}
          className="win98-title"
          data-os-only="win"
        />
        <MacTitleBar title={code ? `biodays - ${code}` : "biodays"} />
        <div className="flex min-h-[96px] items-center gap-5 px-6 py-5">
          <DialogIcon variant={variant} />
          <div className="min-w-0">
            <p className="font-bold">{title}</p>
            <p className="mt-2">{description}</p>
            {code && <p className="mt-2 text-[12px] opacity-70">代码：{code}</p>}
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
