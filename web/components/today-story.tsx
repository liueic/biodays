"use client";

import { Button } from "@react95/core/Button";
import { Bulb } from "@react95/icons/Bulb";
import Link from "next/link";
import { useState } from "react";

import { QuoteBlock } from "@/components/quote-block";
import { RandomButton } from "@/components/random-button";
import { Tag } from "@/components/win/tag";
import { Window } from "@/components/win/window";
import type { Story } from "@/lib/api";
import { formatStoryDate, type TodayStamp } from "@/lib/date";

type TodayStoryProps = {
  initialStory: Story;
  isFallback: boolean;
  stamp: TodayStamp;
};

export function TodayStory({ initialStory, isFallback, stamp }: TodayStoryProps) {
  const [story, setStory] = useState(initialStory);
  const [shuffled, setShuffled] = useState(false);

  const tags = Array.isArray(story.tags) ? story.tags.filter(Boolean) : [];
  const notice = shuffled
    ? "随机浏览中，非今日故事"
    : isFallback
      ? "今日暂无当日故事，已随机推荐"
      : "今日故事";

  return (
    <Window
      title="每日提示 - biodays"
      icon={<Bulb variant="32x32_4" width={16} height={16} aria-hidden />}
      status={[notice, `${stamp.year}年${stamp.month}月${stamp.day}日 ${stamp.weekday}`]}
    >
      <QuoteBlock quote={story.geek_quote} />

      <article className="mt-6">
        <p className="text-[#555]">
          {story.year} 年 {formatStoryDate(story.month, story.day)}
          {story.lab_item ? ` · ${story.lab_item}` : ""}
        </p>
        <h1 className="mt-1 text-base font-bold leading-snug">
          <Link href={`/story/${story.id}`} className="text-link" title="阅读全文">
            {story.title}
          </Link>
        </h1>
        <p className="reading mt-2">{story.summary}</p>

        {tags.length > 0 && (
          <ul className="mt-2 flex flex-wrap gap-2">
            {tags.map((tag) => (
              <Tag key={tag}>{tag}</Tag>
            ))}
          </ul>
        )}

        <div className="mt-6 flex flex-wrap items-center gap-2">
          <Button as={Link} href={`/story/${story.id}`} className="px-4">
            阅读全文(R)
          </Button>
          <RandomButton
            exclude={[story.id]}
            onStory={(next) => {
              setStory(next);
              setShuffled(true);
            }}
          />
        </div>
      </article>
    </Window>
  );
}
