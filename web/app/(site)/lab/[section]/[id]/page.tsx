import { Button } from "@react95/core/Button";
import { Fieldset } from "@react95/core/Fieldset";
import { FileText } from "@react95/icons/FileText";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { QuoteBlock } from "@/components/quote-block";
import { StoryContent } from "@/components/story-content";
import { Tag } from "@/components/win/tag";
import { Window } from "@/components/win/window";
import { getLabArticle, getLabSection, labArticles } from "@/lib/lab";
import { SITE_NAME } from "@/lib/site";
import { safeExternalUrl } from "@/lib/url";

export const dynamicParams = false;

export async function generateStaticParams() {
  return labArticles.map((article) => ({
    section: article.section,
    id: article.id,
  }));
}

type PageProps = {
  params: Promise<{ section: string; id: string }>;
};

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { id } = await params;
  const article = getLabArticle(id);
  if (!article) return { title: "文章未找到" };

  return {
    title: `${article.title} · 实验室手册`,
    description: article.summary,
    alternates: { canonical: `/lab/${article.section}/${article.id}` },
    openGraph: {
      type: "article",
      title: `${article.title} · ${SITE_NAME}`,
      description: article.summary,
      url: `/lab/${article.section}/${article.id}`,
    },
  };
}

export default async function LabArticlePage({ params }: PageProps) {
  const { section: sectionId, id } = await params;
  const section = getLabSection(sectionId);
  const article = getLabArticle(id);

  if (!section || !article || article.section !== section.id) {
    notFound();
  }

  const tags = Array.isArray(article.tags) ? article.tags.filter(Boolean) : [];
  const sourceUrl = safeExternalUrl(article.source?.url ?? "");

  return (
    <main>
      <Window
        title={`${article.title} - 实验室手册`}
        icon={<FileText variant="16x16_4" aria-hidden />}
        status={[section.title, article.topic, article.id]}
      >
        <article>
          <nav aria-label="位置" className="lab-breadcrumb">
            <Link href="/" className="text-link">
              biodays
            </Link>
            <span aria-hidden>›</span>
            <Link href="/lab" className="text-link">
              实验室手册
            </Link>
            <span aria-hidden>›</span>
            <Link href={`/lab/${section.id}`} className="text-link">
              {section.title}
            </Link>
            <span aria-hidden>›</span>
            <span aria-current="page">{article.id}</span>
          </nav>

          <header>
            <div className="flex flex-wrap items-center gap-2">
              <span className="border border-[#c3c7cb] px-2 py-[1px] text-xs font-bold text-[#555]">
                {article.topic}
              </span>
              <span className="text-xs text-[#666]">{section.english}</span>
            </div>
            <h1 className="mt-2 text-base font-bold leading-snug text-balance">{article.title}</h1>
            <p className="mt-2 text-[#555]">涉及器物/材料：{article.lab_item}</p>
            {tags.length > 0 && (
              <ul className="mt-2 flex flex-wrap gap-2">
                {tags.map((tag) => (
                  <Tag key={tag}>{tag}</Tag>
                ))}
              </ul>
            )}
          </header>

          <Fieldset legend="摘要" className="mt-5">
            <p className="reading">{article.summary}</p>
          </Fieldset>

          <QuoteBlock quote={article.geek_quote} className="mt-5" />

          <div className="reading mt-6">
            <StoryContent content={article.content} />
          </div>

          <Fieldset legend="学术引用与来源" className="mt-6">
            <p>
              <span className="uppercase font-bold text-xs">[{article.source?.type}]</span>
              {article.source?.citation ? ` ${article.source.citation}` : ""}
            </p>
            {sourceUrl && (
              <p className="mt-2">
                <a href={sourceUrl} target="_blank" rel="noreferrer" className="text-link">
                  查看原始论文 / 官方条目 (DOI) →
                </a>
              </p>
            )}
          </Fieldset>

          <Fieldset legend="纠错 / 补充线索" className="mt-6">
            <p>
              发现原理、步骤或引文有误？
              <Link
                className="text-link"
                href={`/feedback?kind=issue&story_id=${encodeURIComponent(article.id)}`}
              >
                提交反馈
              </Link>
              ，无需注册。
            </p>
          </Fieldset>

          <div className="mt-6 flex flex-wrap items-center gap-2">
            <Button as={Link} href={`/lab/${section.id}`} className="px-4">
              返回 {section.title} 栏目
            </Button>
            <Button as={Link} href="/lab" className="px-4">
              实验室手册首页
            </Button>
          </div>
        </article>
      </Window>
    </main>
  );
}
