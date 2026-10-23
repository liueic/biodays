"use client";

import { Button } from "@react95/core/Button";
import { Dropdown } from "@react95/core/Dropdown";
import { Frame } from "@react95/core/Frame";
import { Input } from "@react95/core/Input";
import { ProgressBar } from "@react95/core/ProgressBar";
import { Folder } from "@react95/icons/Folder";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";

import { StoryRow } from "@/components/archive/story-list-item";
import { ErrorState } from "@/components/error-state";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { getStories, type StoryList, type StorySummary, type TagCount } from "@/lib/api";

const PAGE_SIZE = 12;
const TAG_PAGE_SIZE = 24;
const ALL_MONTHS = "全部月份";

type ArchiveExplorerProps = {
  initial: StoryList;
  tags: TagCount[];
  monthCounts: Record<number, number>;
};

// Props come from a server component, so guard the arrays at the client boundary:
// anything that is not an array (null from an odd API reply, an older payload shape)
// would blow up on .map() during render.
function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

export function ArchiveExplorer({ initial, tags: rawTags, monthCounts }: ArchiveExplorerProps) {
  const tags = asArray(rawTags);
  const initialTotal = initial.meta?.total ?? 0;

  const [tagSearch, setTagSearch] = useState("");
  const [tagPage, setTagPage] = useState(1);
  const filteredTags = useMemo(
    () =>
      tags.filter((item) =>
        item.name.toLocaleLowerCase().includes(tagSearch.trim().toLocaleLowerCase()),
      ),
    [tags, tagSearch],
  );
  const tagPages = Math.max(1, Math.ceil(filteredTags.length / TAG_PAGE_SIZE));
  const visibleTags = filteredTags.slice((tagPage - 1) * TAG_PAGE_SIZE, tagPage * TAG_PAGE_SIZE);
  const [month, setMonth] = useState<number | null>(null);
  const [tag, setTag] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const debouncedSearch = useDebouncedValue(search.trim(), 300);
  const topRef = useRef<HTMLDivElement>(null);

  // Pick up ?tag= from tag chips on other pages. Read after mount (not via useSearchParams)
  // so the ISR-cached page stays shared across query strings and hydration stays clean.
  useEffect(() => {
    const urlTag = new URLSearchParams(window.location.search).get("tag");
    if (urlTag) {
      setTag(urlTag);
      setPage(1);
    }
  }, []);

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset page whenever the debounced query changes
  useEffect(() => {
    setPage(1);
  }, [debouncedSearch]);

  const isDefaultView = month === null && tag === null && debouncedSearch === "" && page === 1;
  const query = useQuery({
    queryKey: ["stories", { month, tag, q: debouncedSearch, page }],
    queryFn: () =>
      getStories({
        month: month ?? undefined,
        tag: tag ?? undefined,
        q: debouncedSearch || undefined,
        page,
        pageSize: PAGE_SIZE,
      }),
    initialData: isDefaultView ? initial : undefined,
    placeholderData: keepPreviousData,
  });

  const stories = asArray(query.data?.data).filter(
    (story): story is StorySummary => story != null && typeof story === "object",
  );
  const total = query.data?.meta.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const hasFilters = month !== null || tag !== null || debouncedSearch !== "";

  const monthOptions = [
    ALL_MONTHS,
    ...Array.from({ length: 12 }, (_, i) => `${i + 1}月 (${monthCounts[i + 1] ?? 0})`),
  ];

  function resetFilters() {
    setMonth(null);
    setTag(null);
    setSearch("");
    setPage(1);
  }

  function goToPage(next: number) {
    setPage(next);
    topRef.current?.scrollIntoView({ block: "start" });
  }

  function selectTag(next: string | null) {
    setTag(next);
    setPage(1);
  }

  return (
    <div>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <div className="flex flex-1 items-center gap-2">
          <label htmlFor="archive-search" className="shrink-0">
            查找(S)
          </label>
          <Input
            id="archive-search"
            type="search"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
              setPage(1);
            }}
            placeholder="标题、摘要或器物"
            aria-label="搜索故事"
            className="min-w-0 flex-1"
          />
        </div>
        <Dropdown
          aria-label="按月份筛选"
          options={monthOptions}
          value={month === null ? ALL_MONTHS : monthOptions[month]}
          onChange={(event: React.ChangeEvent<HTMLSelectElement>) => {
            const index = monthOptions.indexOf(event.target.value);
            setMonth(index > 0 ? index : null);
            setPage(1);
          }}
        />
        <Button onClick={resetFilters} disabled={!hasFilters}>
          重置
        </Button>
      </div>

      <div className="mt-3 grid gap-3 md:grid-cols-[200px_1fr]">
        <Frame
          boxShadow="$in"
          bgColor="$inputBackground"
          className="max-h-[420px] overflow-auto p-1"
        >
          <label htmlFor="tag-search">筛选标签</label>
          <Input
            id="tag-search"
            type="search"
            value={tagSearch}
            className="my-1 w-full"
            placeholder="搜索全部标签"
            onChange={(event) => {
              setTagSearch(event.target.value);
              setTagPage(1);
            }}
          />
          <button
            type="button"
            className="menu-item block w-full text-left"
            aria-pressed={tag === null}
            onClick={() => selectTag(null)}
          >
            全部 ({initialTotal})
          </button>
          <ul aria-label="按标签筛选">
            {visibleTags.map((item) => (
              <li key={item.name}>
                <button
                  type="button"
                  className="menu-item flex w-full items-start gap-1 text-left"
                  aria-pressed={tag === item.name}
                  onClick={() => selectTag(item.name)}
                >
                  <Folder variant="16x16_4" className="shrink-0" aria-hidden />
                  <span>
                    {item.name} ({item.count})
                  </span>
                </button>
              </li>
            ))}
          </ul>
          {filteredTags.length === 0 && <p className="p-2">没有匹配的标签</p>}
          <nav aria-label="标签分页" className="mt-2 flex items-center justify-between gap-1">
            <Button
              aria-label="上一页标签"
              disabled={tagPage <= 1}
              onClick={() => setTagPage(tagPage - 1)}
            >
              ‹
            </Button>
            <span aria-live="polite">
              {tagPage} / {tagPages}
            </span>
            <Button
              aria-label="下一页标签"
              disabled={tagPage >= tagPages}
              onClick={() => setTagPage(tagPage + 1)}
            >
              ›
            </Button>
          </nav>
        </Frame>

        <div ref={topRef} className="min-w-0 scroll-mt-2">
          <Frame boxShadow="$in" bgColor="$inputBackground" className="max-h-[420px] overflow-auto">
            {query.isError ? (
              <ErrorState
                title="加载失败"
                description="暂时无法获取列表，请稍后重试。"
                action={{ href: "/", label: "回到今日" }}
              >
                <Button onClick={() => query.refetch()}>重试</Button>
              </ErrorState>
            ) : query.isPending ? (
              <div className="p-6" aria-busy="true">
                <p className="mb-2">正在读取…</p>
                <ProgressBar percent={60} width="100%" />
              </div>
            ) : stories.length === 0 ? (
              <ErrorState title="没有匹配的故事" description="换个关键词，或重置筛选条件再试。">
                <Button onClick={resetFilters}>重置</Button>
              </ErrorState>
            ) : (
              <table
                className={`w-full border-collapse text-left ${query.isPlaceholderData ? "opacity-60" : ""}`}
              >
                <thead className="sticky top-0 bg-[#c3c7cb]">
                  <tr>
                    <th
                      scope="col"
                      className="w-16 border border-[#868a8e] px-2 py-[2px] font-normal"
                    >
                      日期
                    </th>
                    <th
                      scope="col"
                      className="w-14 border border-[#868a8e] px-2 py-[2px] font-normal"
                    >
                      年份
                    </th>
                    <th scope="col" className="border border-[#868a8e] px-2 py-[2px] font-normal">
                      标题
                    </th>
                    <th
                      scope="col"
                      className="hidden w-32 border border-[#868a8e] px-2 py-[2px] font-normal sm:table-cell"
                    >
                      标签
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {stories.map((story) => (
                    <StoryRow key={story.id} story={story} />
                  ))}
                </tbody>
              </table>
            )}
          </Frame>

          {totalPages > 1 && (
            <nav aria-label="分页" className="mt-2 flex items-center justify-between">
              <Button disabled={page <= 1} onClick={() => goToPage(page - 1)}>
                上一页
              </Button>
              <span>
                {page} / {totalPages}
              </span>
              <Button disabled={page >= totalPages} onClick={() => goToPage(page + 1)}>
                下一页
              </Button>
            </nav>
          )}
        </div>
      </div>

      <p className="mt-3" aria-live="polite">
        共 {total} 个对象{hasFilters ? "（已筛选）" : ""}
        {tag ? ` · 标签：${tag}` : ""}
      </p>
    </div>
  );
}
