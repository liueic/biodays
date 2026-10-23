import { Frame } from "@react95/core/Frame";
import { Bulb } from "@react95/icons/Bulb";

import { cn } from "@/lib/utils";

/** "Tip of the Day" style panel for the geek quote. */
export function QuoteBlock({ quote, className }: { quote: string; className?: string }) {
  return (
    <Frame
      boxShadow="$in"
      bgColor="$inputBackground"
      padding="$12"
      className={cn("flex min-w-0 gap-2 border-2 border-[#808080] sm:gap-4", className)}
    >
      <Bulb
        variant="32x32_4"
        width={32}
        height={32}
        aria-hidden
        className="size-6 shrink-0 sm:size-8"
      />
      <div className="min-w-0 flex-1">
        <p className="font-bold">你知道吗？</p>
        <blockquote className="mt-2 text-[18px] leading-[1.7] text-pretty sm:text-[24px] sm:leading-[1.6]">
          {quote}
        </blockquote>
      </div>
    </Frame>
  );
}
