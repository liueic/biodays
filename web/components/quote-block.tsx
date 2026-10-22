import { Frame } from "@react95/core";
import { Bulb } from "@react95/icons";

import { cn } from "@/lib/utils";

/** "Tip of the Day" style panel for the geek quote. */
export function QuoteBlock({ quote, className }: { quote: string; className?: string }) {
  return (
    <Frame
      boxShadow="$in"
      bgColor="$inputBackground"
      padding="$12"
      className={cn("flex gap-4 border-2 border-[#808080]", className)}
    >
      <Bulb variant="32x32_4" width={32} height={32} aria-hidden className="shrink-0" />
      <div className="min-w-0">
        <p className="font-bold">你知道吗？</p>
        <blockquote className="mt-2 text-[24px] leading-[1.6] text-pretty">{quote}</blockquote>
      </div>
    </Frame>
  );
}
