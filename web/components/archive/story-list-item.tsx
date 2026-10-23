import { FileText } from "@react95/icons/FileText";
import Link from "next/link";

import type { StorySummary } from "@/lib/api";
import { formatMonthDay } from "@/lib/date";

/** One row of the explorer "details" view. */
export function StoryRow({ story }: { story: StorySummary }) {
  const tags = Array.isArray(story.tags) ? story.tags : [];
  return (
    <tr className="group align-top hover:bg-[#000080] hover:text-white">
      <td className="whitespace-nowrap px-2 py-1">{formatMonthDay(story.month, story.day)}</td>
      <td className="whitespace-nowrap px-2 py-1">{story.year}</td>
      <td className="px-2 py-1">
        <Link
          href={`/story/${story.id}`}
          className="inline-flex max-w-full min-w-0 items-start gap-1 !text-inherit no-underline group-hover:underline"
        >
          <FileText variant="16x16_4" aria-hidden className="mt-[1px] shrink-0" />
          <span className="min-w-0">{story.title}</span>
        </Link>
      </td>
      <td className="hidden px-2 py-1 sm:table-cell">{tags.slice(0, 2).join("、")}</td>
    </tr>
  );
}
