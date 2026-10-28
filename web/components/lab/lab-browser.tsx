"use client";

import { Button } from "@react95/core/Button";
import { FileText } from "@react95/icons/FileText";
import { Folder } from "@react95/icons/Folder";
import { HelpBook } from "@react95/icons/HelpBook";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";

import { LabSectionIcon } from "@/components/lab/section-icon";
import { Window } from "@/components/win/window";
import { getLabArticles, LAB_SECTIONS, type LabSection, pickRandomLabArticle } from "@/lib/lab";

export function LabBrowser({ section }: { section?: LabSection }) {
  const router = useRouter();
  const title = section?.title ?? "实验室手册";
  const allSectionArticles = section ? getLabArticles(section.id) : [];

  function handleRandom() {
    const randomArticle = pickRandomLabArticle(section?.id);
    if (randomArticle) {
      router.push(`/lab/${randomArticle.section}/${randomArticle.id}`);
    }
  }

  return (
    <main>
      <Window
        title={`${title} - biodays`}
        icon={<HelpBook variant="16x16_4" width={16} height={16} aria-hidden />}
        status={[
          section
            ? `${section.title} · ${allSectionArticles.length > 0 ? `${allSectionArticles.length} 篇文章` : "暂无文章"}`
            : "3 个栏目",
          "实验室手册",
        ]}
      >
        <nav aria-label="位置" className="lab-breadcrumb">
          <Link href="/" className="text-link">
            biodays
          </Link>
          <span aria-hidden>›</span>
          {section ? (
            <>
              <Link href="/lab" className="text-link">
                实验室手册
              </Link>
              <span aria-hidden>›</span>
              <span aria-current="page">{section.title}</span>
            </>
          ) : (
            <span aria-current="page">实验室手册</span>
          )}
        </nav>

        <div className="lab-browser">
          <aside className="lab-sidebar">
            <p className="lab-sidebar-heading">目录</p>
            <nav aria-label="实验室手册栏目">
              <Link
                href="/lab"
                className="lab-folder-link"
                aria-current={!section ? "page" : undefined}
              >
                <Folder variant="16x16_4" width={16} height={16} aria-hidden />
                <span>全部栏目</span>
              </Link>
              {LAB_SECTIONS.map((item) => (
                <Link
                  key={item.id}
                  href={`/lab/${item.id}`}
                  className="lab-folder-link"
                  aria-current={section?.id === item.id ? "page" : undefined}
                >
                  <LabSectionIcon section={item.id} size={16} />
                  <span>{item.title}</span>
                </Link>
              ))}
            </nav>
            <div className="mt-4 px-2">
              <Button onClick={handleRandom} className="w-full text-xs py-1">
                随机抽一篇(R)
              </Button>
            </div>
            <p className="lab-sidebar-note">
              从原理到实践，
              <br />
              多一点实验室里的灵感。
            </p>
          </aside>

          <div className="lab-main">
            <header className="lab-heading">
              <span className="lab-heading-icon">
                {section ? (
                  <LabSectionIcon section={section.id} />
                ) : (
                  <HelpBook variant="32x32_4" width={32} height={32} aria-hidden />
                )}
              </span>
              <div>
                <p className="lab-eyebrow">{section?.english ?? "Biodays · Lab notebook"}</p>
                <h1 className="mt-1 text-lg font-bold">{title}</h1>
              </div>
            </header>
            <p className="lab-intro">
              {section?.description ??
                "一本放在实验台旁的手册。了解技术，查阅流程，发现让实验更顺手的小技巧。"}
            </p>

            {section ? <SectionContents section={section} /> : <SectionFolders />}

            <div className="lab-footer">
              <span>今天也保留一点好奇心。</span>
              <Link href="/" className="text-link">
                读一篇今日故事 →
              </Link>
            </div>
          </div>
        </div>
      </Window>
    </main>
  );
}

function SectionFolders() {
  return (
    <div className="lab-section-grid">
      {LAB_SECTIONS.map((section, index) => {
        const count = getLabArticles(section.id).length;
        return (
          <Link key={section.id} href={`/lab/${section.id}`} className="lab-section-card">
            <div className="flex items-start justify-between gap-2">
              <LabSectionIcon section={section.id} />
              <span className="lab-section-number">0{index + 1}</span>
            </div>
            <h2 className="mt-4 font-bold">{section.title}</h2>
            <p className="lab-section-summary">{section.summary}</p>
            <span className="lab-card-action">
              {count > 0 ? `打开栏目 (${count} 篇) →` : "打开栏目 →"}
            </span>
          </Link>
        );
      })}
    </div>
  );
}

function SectionContents({ section }: { section: LabSection }) {
  const articles = getLabArticles(section.id);
  const [selectedTopic, setSelectedTopic] = useState<string | null>(null);
  const [search, setSearch] = useState("");

  const filteredArticles = useMemo(() => {
    return articles.filter((a) => {
      const matchTopic = !selectedTopic || a.topic === selectedTopic;
      const q = search.trim().toLowerCase();
      const matchSearch =
        !q ||
        a.title.toLowerCase().includes(q) ||
        a.summary.toLowerCase().includes(q) ||
        a.lab_item.toLowerCase().includes(q) ||
        a.tags.some((tag) => tag.toLowerCase().includes(q));
      return matchTopic && matchSearch;
    });
  }, [articles, selectedTopic, search]);

  return (
    <section aria-label={`${section.title}文章`} className="lab-contents">
      <div className="lab-contents-header">
        <h2 className="font-bold">栏目内容</h2>
        <span>
          {filteredArticles.length} / {articles.length} 篇文章
        </span>
      </div>

      <div className="p-3 border-b border-[#c3c7cb] bg-[#f8f8f4] flex flex-wrap items-center justify-between gap-2">
        <ul className="flex flex-wrap items-center gap-1.5" aria-label="方向筛选">
          <li>
            <button
              type="button"
              onClick={() => setSelectedTopic(null)}
              className={`px-2 py-[2px] text-xs border cursor-pointer ${
                selectedTopic === null
                  ? "bg-[#000080] text-white border-[#000080]"
                  : "bg-white text-[#333] border-[#c3c7cb] hover:bg-[#e8e8e8]"
              }`}
            >
              全部 ({articles.length})
            </button>
          </li>
          {section.topics.map((topic) => {
            const count = articles.filter((a) => a.topic === topic).length;
            const isSelected = selectedTopic === topic;
            return (
              <li key={topic}>
                <button
                  type="button"
                  onClick={() => setSelectedTopic(isSelected ? null : topic)}
                  className={`px-2 py-[2px] text-xs border cursor-pointer ${
                    isSelected
                      ? "bg-[#000080] text-white border-[#000080]"
                      : "bg-white text-[#333] border-[#c3c7cb] hover:bg-[#e8e8e8]"
                  }`}
                >
                  {topic} ({count})
                </button>
              </li>
            );
          })}
        </ul>

        <div className="flex items-center gap-1">
          <input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="过滤关键词..."
            className="border border-[#868a8e] bg-white px-2 py-[2px] text-xs focus:outline-1 focus:outline-black w-[130px] sm:w-[160px]"
          />
          {search && (
            <button
              type="button"
              onClick={() => setSearch("")}
              className="text-xs text-[#555] px-1 hover:text-black cursor-pointer"
            >
              ✕
            </button>
          )}
        </div>
      </div>

      {filteredArticles.length === 0 ? (
        <div className="lab-empty">
          <Folder variant="32x32_4" width={32} height={32} aria-hidden />
          <h3 className="mt-4 font-bold">未找到匹配文章</h3>
          <p className="mt-2 max-w-[300px] leading-6 text-[#555]">
            没有找到与 “{search || selectedTopic}” 相关的内容，试试重置筛选？
          </p>
          <Button
            onClick={() => {
              setSelectedTopic(null);
              setSearch("");
            }}
            className="mt-5 px-4"
          >
            显示全部文章
          </Button>
        </div>
      ) : (
        <div className="lab-article-list">
          {filteredArticles.map((article) => (
            <article key={article.id} className="lab-article-item">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <Link
                  href={`/lab/${section.id}/${article.id}`}
                  className="lab-article-title text-link font-bold"
                >
                  <FileText variant="16x16_4" aria-hidden className="mt-[2px] shrink-0" />
                  <span>{article.title}</span>
                </Link>
                <span className="border border-[#c3c7cb] px-1 text-xs text-[#555]">
                  {article.topic}
                </span>
              </div>
              <p className="mt-2 text-xs leading-relaxed text-[#555]">{article.summary}</p>
              <div className="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs text-[#666]">
                <span className="truncate max-w-[320px]">器物：{article.lab_item}</span>
                <div className="flex flex-wrap gap-1">
                  {article.tags.map((tag) => (
                    <span key={tag} className="bg-[#f0f0f0] px-1 text-[#444]">
                      #{tag}
                    </span>
                  ))}
                </div>
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
