import { Button } from "@react95/core/Button";
import { Fieldset } from "@react95/core/Fieldset";
import { FileText } from "@react95/icons/FileText";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { NextStoryButton } from "@/components/next-story-button";
import { QuoteBlock } from "@/components/quote-block";
import { StoryContent } from "@/components/story-content";
import { StoryHistoryTracker } from "@/components/story-history-tracker";
import { Tag } from "@/components/win/tag";
import { Window } from "@/components/win/window";
import { ApiError, getStory, type Story } from "@/lib/api";
import { formatStoryDate } from "@/lib/date";
import { SITE_NAME } from "@/lib/site";
import { safeExternalUrl } from "@/lib/url";

export const revalidate = 300;

// Generate a page on first visit, so image builds do not require a running API.
export async function generateStaticParams() {
  return [];
}

type PageProps = { params: Promise<{ id: string }> };

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { id } = await params;
  try {
    const story = await getStory(id);
    return {
      title: story.title,
      description: story.summary,
      alternates: { canonical: `/story/${story.id}` },
      openGraph: {
        type: "article",
        title: story.title,
        description: story.summary,
        url: `/story/${story.id}`,
        siteName: SITE_NAME,
      },
      twitter: { card: "summary", title: story.title, description: story.summary },
    };
  } catch (error) {
    // Fail before the response starts streaming: generateMetadata runs ahead
    // of the page body, so notFound() here still yields a real HTTP 404
    // instead of a streamed 200 with a 404 UI (this segment has a loading.tsx).
    if (error instanceof ApiError && error.status === 404) notFound();
    return { title: "故事未找到" };
  }
}

export default async function StoryPage({ params }: PageProps) {
  const { id } = await params;
  let story: Story;
  try {
    story = await getStory(id);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }

  const tags = Array.isArray(story.tags) ? story.tags.filter(Boolean) : [];
  const sourceUrl = safeExternalUrl(story.source?.url ?? "");

  return (
    <main>
      <Window
        title={`${story.title} - biodays`}
        icon={<FileText variant="16x16_4" aria-hidden />}
        status={[
          `${story.year} 年 ${formatStoryDate(story.month, story.day)}`,
          (story.source?.type ?? "").toUpperCase(),
          story.id,
        ]}
      >
        <article>
          <StoryHistoryTracker storyId={story.id} />
          <header>
            <h1 className="text-base font-bold leading-snug text-balance">{story.title}</h1>
            <p className="mt-2 text-[#555]">
              {story.year} 年 {formatStoryDate(story.month, story.day)}
              {story.lab_item ? ` · ${story.lab_item}` : ""}
            </p>
            {tags.length > 0 && (
              <ul className="mt-2 flex flex-wrap gap-2">
                {tags.map((tag) => (
                  <Tag key={tag}>{tag}</Tag>
                ))}
              </ul>
            )}
          </header>

          <Fieldset legend="摘要" className="mt-5">
            <p className="reading">{story.summary}</p>
          </Fieldset>

          <QuoteBlock quote={story.geek_quote} className="mt-5" />

          <div className="reading mt-6">
            <StoryContent content={story.content} />
          </div>

          <Fieldset legend="来源" className="mt-6">
            <p>
              <span className="uppercase">{story.source?.type}</span>
              {story.source?.citation ? ` · ${story.source.citation}` : ""}
            </p>
            {sourceUrl && (
              <p className="mt-2">
                <a href={sourceUrl} target="_blank" rel="noreferrer" className="text-link">
                  查看原始来源
                </a>
              </p>
            )}
          </Fieldset>

          <Fieldset legend="纠错 / 补充线索" className="mt-6">
            <p>
              发现年份、引文或事实有误？
              <Link
                className="text-link"
                href={`/feedback?kind=issue&story_id=${encodeURIComponent(story.id)}`}
              >
                提交反馈
              </Link>
              ，无需注册。
            </p>
          </Fieldset>

          <div className="mt-6 flex flex-wrap items-center gap-2">
            <Button as={Link} href="/archive" className="px-4">
              返回归档(B)
            </Button>
            <NextStoryButton currentId={story.id} />
          </div>
        </article>
      </Window>
    </main>
  );
}
