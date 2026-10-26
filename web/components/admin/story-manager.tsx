"use client";

import { Button } from "@react95/core/Button";
import { Input } from "@react95/core/Input";
import { TextArea } from "@react95/core/TextArea";
import { Bookmark } from "@react95/icons/Bookmark";
import { Mailnews2 } from "@react95/icons/Mailnews2";
import { Refresh } from "@react95/icons/Refresh";
import Link from "next/link";
import { useEffect, useState } from "react";

import { useAdminSession } from "@/components/admin/admin-session";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import {
  type AdminStory,
  type AdminStorySummary,
  deleteStory,
  FeedbackApiError,
  getStory,
  listStories,
  type PageMeta,
  upsertStory,
} from "@/lib/admin-api";

const PAGE_SIZE = 20;

type StoryFormState = {
  id: string;
  month: string;
  day: string;
  year: string;
  title: string;
  lab_item: string;
  tags: string;
  summary: string;
  content: string;
  geek_quote: string;
  source_type: string;
  source_citation: string;
  source_url: string;
};

const EMPTY_FORM: StoryFormState = {
  id: "",
  month: "",
  day: "",
  year: "",
  title: "",
  lab_item: "",
  tags: "",
  summary: "",
  content: "",
  geek_quote: "",
  source_type: "",
  source_citation: "",
  source_url: "",
};

export function StoryManager() {
  const { session, logout } = useAdminSession();
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [items, setItems] = useState<AdminStorySummary[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ total: 0, page: 1, page_size: PAGE_SIZE });
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState("");
  const [refreshing, setRefreshing] = useState(0);
  const debouncedQuery = useDebouncedValue(query, 300);

  const [selectedID, setSelectedID] = useState("");
  const [form, setForm] = useState<StoryFormState>(EMPTY_FORM);
  const [editorMode, setEditorMode] = useState<"empty" | "new" | "edit">("empty");
  const [editorLoading, setEditorLoading] = useState(false);
  const [formError, setFormError] = useState("");
  const [statusMessage, setStatusMessage] = useState("");
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const debouncedQueryTrimmed = debouncedQuery.trim();

  useEffect(() => {
    void refreshing;
    let cancelled = false;
    setLoading(true);
    setListError("");
    listStories({ q: debouncedQueryTrimmed, page, pageSize: PAGE_SIZE })
      .then((response) => {
        if (cancelled) return;
        setItems(response.data);
        setMeta(response.meta);
      })
      .catch((reason) => {
        if (cancelled) return;
        setListError(reason instanceof FeedbackApiError ? reason.message : "无法加载故事列表。");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [debouncedQueryTrimmed, page, refreshing]);

  function resetEditor() {
    setSelectedID("");
    setForm(EMPTY_FORM);
    setEditorMode("empty");
    setFormError("");
    setStatusMessage("");
    setConfirmDelete(false);
  }

  function startCreate() {
    setSelectedID("");
    setForm(EMPTY_FORM);
    setEditorMode("new");
    setFormError("");
    setStatusMessage("");
    setConfirmDelete(false);
  }

  function openStory(id: string) {
    setSelectedID(id);
    setEditorMode("edit");
    setFormError("");
    setStatusMessage("");
    setConfirmDelete(false);
    setEditorLoading(true);
    getStory(id)
      .then((response) => {
        setForm(storyToForm(response.data));
      })
      .catch((reason) => {
        setFormError(reason instanceof FeedbackApiError ? reason.message : "无法读取这个故事。");
      })
      .finally(() => setEditorLoading(false));
  }

  function update<K extends keyof StoryFormState>(key: K, value: string) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  function save() {
    const problems = validateForm(form);
    if (problems.length > 0) {
      setFormError(problems.join("; "));
      return;
    }
    setSaving(true);
    setFormError("");
    setStatusMessage("");
    upsertStory(formToStory(form))
      .then((response) => {
        setStatusMessage(
          response.data.created ? `已发布 ${response.data.id}。` : `已更新 ${response.data.id}。`,
        );
        setSelectedID(response.data.id);
        setEditorMode("edit");
        setRefreshing((value) => value + 1);
      })
      .catch((reason) => {
        if (reason instanceof FeedbackApiError && reason.fields.story) {
          setFormError(reason.fields.story);
        } else {
          setFormError(reason instanceof FeedbackApiError ? reason.message : "保存失败，请重试。");
        }
      })
      .finally(() => setSaving(false));
  }

  function remove() {
    if (!confirmDelete) {
      setConfirmDelete(true);
      return;
    }
    setSaving(true);
    deleteStory(selectedID)
      .then(() => {
        setStatusMessage(`已删除 ${selectedID}。公开页面缓存最多 5 分钟后同步。`);
        resetEditor();
        setRefreshing((value) => value + 1);
      })
      .catch((reason) => {
        setFormError(reason instanceof FeedbackApiError ? reason.message : "删除失败，请重试。");
      })
      .finally(() => setSaving(false));
  }

  const totalPages = Math.max(1, Math.ceil(meta.total / meta.page_size));

  return (
    <div className="mail-inbox story-admin">
      <div className="mail-toolbar" role="toolbar" aria-label="故事管理操作">
        <div className="mail-toolbar-group">
          <Button type="button" className="mail-tool-button" onClick={startCreate}>
            <Bookmark variant="32x32_4" aria-hidden />
            <span>新故事</span>
          </Button>
        </div>
        <div className="mail-toolbar-separator" aria-hidden="true" />
        <div className="mail-toolbar-group">
          <Button
            type="button"
            className="mail-tool-button mail-tool-button--inline"
            onClick={() => setRefreshing((value) => value + 1)}
          >
            <Refresh variant="16x16_4" aria-hidden />
            <span>{loading ? "刷新中…" : "刷新"}</span>
          </Button>
          <Button
            as={Link}
            href="/admin/feedback"
            className="mail-tool-button mail-tool-button--inline"
          >
            <Mailnews2 variant="16x16_4" aria-hidden />
            <span>反馈收件箱</span>
          </Button>
        </div>
        <div className="mail-toolbar-spacer" />
        <Button
          type="button"
          className="mail-tool-button mail-tool-button--inline"
          onClick={() => void logout()}
        >
          退出（{session?.username}）
        </Button>
      </div>

      <div className="mail-inbox-filters">
        <label htmlFor="story-filter-query" className="mail-filter-field mail-filter-field--search">
          <span>搜索</span>
          <Input
            id="story-filter-query"
            type="search"
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setPage(1);
            }}
            placeholder="ID、标题、摘要或条目"
            aria-label="搜索故事"
          />
        </label>
      </div>

      {listError && (
        <p role="alert" className="mail-alert">
          {listError}
        </p>
      )}

      <div className="mail-inbox-body">
        <section className="mail-list-pane" aria-label="故事列表">
          <div className="mail-list-caption">
            <span>全部故事</span>
            <span aria-live="polite">{loading ? "加载中…" : `共 ${meta.total} 篇`}</span>
          </div>
          <div className="mail-list-scroll">
            <table className="mail-message-table">
              <thead>
                <tr>
                  <th className="story-col-id">ID</th>
                  <th className="story-col-date">日期</th>
                  <th>标题</th>
                  <th className="mail-col-open">
                    <span className="sr-only">操作</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => {
                  const selected = selectedID === item.id;
                  return (
                    <tr key={item.id} className="mail-message-row" data-selected={selected}>
                      <td className="story-col-id font-mono">{item.id}</td>
                      <td className="story-col-date">
                        {String(item.month).padStart(2, "0")}-{String(item.day).padStart(2, "0")}-
                        {item.year}
                      </td>
                      <td>
                        <button
                          type="button"
                          className="mail-message-subject mail-message-select-subject story-open-button"
                          aria-current={selected ? "true" : undefined}
                          title={`编辑：${item.title}`}
                          onClick={() => openStory(item.id)}
                        >
                          <span className="mail-message-title-text">{item.title}</span>
                          <span className="mail-message-snippet">{item.tags.join(" / ")}</span>
                        </button>
                      </td>
                      <td className="mail-col-open">
                        <button
                          type="button"
                          className="mail-message-link"
                          onClick={() => openStory(item.id)}
                        >
                          编辑
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            {!loading && items.length === 0 && (
              <div className="mail-list-empty">没有符合条件的故事。</div>
            )}
            {loading && items.length === 0 && (
              <div className="mail-list-empty" role="status">
                正在加载故事…
              </div>
            )}
          </div>
          <div className="mail-list-footer">
            <span>
              第 {meta.page} / {totalPages} 页
            </span>
            <div className="mail-list-footer-group">
              <Button
                type="button"
                className="mail-tool-button mail-tool-button--inline"
                disabled={page <= 1 || loading}
                onClick={() => setPage((value) => Math.max(1, value - 1))}
              >
                上一页
              </Button>
              <Button
                type="button"
                className="mail-tool-button mail-tool-button--inline"
                disabled={page >= totalPages || loading}
                onClick={() => setPage((value) => value + 1)}
              >
                下一页
              </Button>
            </div>
          </div>
        </section>

        <aside className="mail-preview-pane story-editor" aria-label="故事编辑">
          {editorLoading ? (
            <div className="mail-preview-empty" role="status">
              <Bookmark variant="32x32_4" aria-hidden />
              <p>正在打开故事…</p>
            </div>
          ) : editorMode === "empty" ? (
            <div className="mail-preview-empty">
              <Bookmark variant="32x32_4" aria-hidden />
              <p>选择一篇故事进行编辑，或点击“新故事”发布新内容。</p>
            </div>
          ) : (
            <form
              className="story-form"
              onSubmit={(event) => {
                event.preventDefault();
                save();
              }}
            >
              <h2 className="story-form-heading">
                {editorMode === "new" ? "发布新故事" : `编辑 ${selectedID}`}
              </h2>

              <div className="story-form-row">
                <div className="story-form-field">
                  <label htmlFor="story-field-id">ID（≤16 字符）</label>
                  <Input
                    id="story-field-id"
                    value={form.id}
                    onChange={(event) => update("id", event.target.value)}
                    disabled={editorMode === "edit"}
                    required
                  />
                </div>
                <div className="story-form-field">
                  <label htmlFor="story-field-month">月</label>
                  <Input
                    id="story-field-month"
                    type="number"
                    min={1}
                    max={12}
                    value={form.month}
                    onChange={(event) => update("month", event.target.value)}
                    required
                  />
                </div>
                <div className="story-form-field">
                  <label htmlFor="story-field-day">日</label>
                  <Input
                    id="story-field-day"
                    type="number"
                    min={1}
                    max={31}
                    value={form.day}
                    onChange={(event) => update("day", event.target.value)}
                    required
                  />
                </div>
                <div className="story-form-field">
                  <label htmlFor="story-field-year">年份</label>
                  <Input
                    id="story-field-year"
                    type="number"
                    min={1}
                    value={form.year}
                    onChange={(event) => update("year", event.target.value)}
                    required
                  />
                </div>
              </div>

              <div className="story-form-field">
                <label htmlFor="story-field-title">标题</label>
                <Input
                  id="story-field-title"
                  value={form.title}
                  onChange={(event) => update("title", event.target.value)}
                  required
                />
              </div>

              <div className="story-form-row">
                <div className="story-form-field">
                  <label htmlFor="story-field-lab-item">实验条目</label>
                  <Input
                    id="story-field-lab-item"
                    value={form.lab_item}
                    onChange={(event) => update("lab_item", event.target.value)}
                  />
                </div>
                <div className="story-form-field">
                  <label htmlFor="story-field-tags">标签（逗号分隔）</label>
                  <Input
                    id="story-field-tags"
                    value={form.tags}
                    onChange={(event) => update("tags", event.target.value)}
                    placeholder="仪器极客, 工业设计"
                  />
                </div>
              </div>

              <div className="story-form-field">
                <label htmlFor="story-field-summary">摘要</label>
                <TextArea
                  id="story-field-summary"
                  value={form.summary}
                  onChange={(event) => update("summary", event.target.value)}
                  rows={2}
                  className="story-form-input"
                />
              </div>

              <div className="story-form-field">
                <label htmlFor="story-field-content">正文</label>
                <TextArea
                  id="story-field-content"
                  value={form.content}
                  onChange={(event) => update("content", event.target.value)}
                  rows={10}
                  className="story-form-input"
                  required
                />
              </div>

              <div className="story-form-field">
                <label htmlFor="story-field-quote">极客语录</label>
                <TextArea
                  id="story-field-quote"
                  value={form.geek_quote}
                  onChange={(event) => update("geek_quote", event.target.value)}
                  rows={2}
                  className="story-form-input"
                  required
                />
              </div>

              <fieldset className="story-form-fieldset">
                <legend>来源</legend>
                <div className="story-form-field">
                  <label htmlFor="story-field-source-type">类型</label>
                  <Input
                    id="story-field-source-type"
                    value={form.source_type}
                    onChange={(event) => update("source_type", event.target.value)}
                    placeholder="paper / patent / nobel"
                  />
                </div>
                <div className="story-form-field">
                  <label htmlFor="story-field-source-citation">引用</label>
                  <Input
                    id="story-field-source-citation"
                    value={form.source_citation}
                    onChange={(event) => update("source_citation", event.target.value)}
                  />
                </div>
                <div className="story-form-field">
                  <label htmlFor="story-field-source-url">链接</label>
                  <Input
                    id="story-field-source-url"
                    type="url"
                    value={form.source_url}
                    onChange={(event) => update("source_url", event.target.value)}
                    placeholder="https://"
                  />
                </div>
              </fieldset>

              {formError && (
                <p role="alert" className="mail-alert">
                  {formError}
                </p>
              )}
              {statusMessage && <p className="story-form-status">{statusMessage}</p>}

              <div className="story-form-actions">
                <Button
                  type="submit"
                  className="mail-tool-button mail-tool-button--inline"
                  disabled={saving}
                >
                  {saving ? "保存中…" : editorMode === "new" ? "发布" : "保存修改"}
                </Button>
                {editorMode === "edit" && (
                  <Button
                    type="button"
                    className="mail-tool-button mail-tool-button--inline"
                    disabled={saving}
                    onClick={remove}
                  >
                    {confirmDelete ? "确认删除？" : "删除"}
                  </Button>
                )}
                <Button
                  type="button"
                  className="mail-tool-button mail-tool-button--inline"
                  onClick={resetEditor}
                >
                  关闭
                </Button>
              </div>
            </form>
          )}
        </aside>
      </div>

      <div className="mail-inbox-status">
        <span className="mail-status-cell">{`共 ${meta.total} 篇故事`}</span>
        <span className="mail-status-cell mail-status-cell--shrink">
          {editorMode === "edit"
            ? `正在编辑 ${selectedID}`
            : editorMode === "new"
              ? "新故事"
              : "未选择"}
        </span>
      </div>
    </div>
  );
}

function storyToForm(story: AdminStory): StoryFormState {
  return {
    id: story.id,
    month: String(story.month),
    day: String(story.day),
    year: String(story.year),
    title: story.title,
    lab_item: story.lab_item,
    tags: story.tags.join(", "),
    summary: story.summary,
    content: story.content,
    geek_quote: story.geek_quote,
    source_type: story.source.type,
    source_citation: story.source.citation,
    source_url: story.source.url,
  };
}

function formToStory(form: StoryFormState): AdminStory {
  return {
    id: form.id.trim(),
    month: Number(form.month),
    day: Number(form.day),
    year: Number(form.year),
    title: form.title,
    lab_item: form.lab_item,
    tags: form.tags
      .split(/[,，]/)
      .map((tag) => tag.trim())
      .filter(Boolean),
    summary: form.summary,
    content: form.content,
    geek_quote: form.geek_quote,
    source: {
      type: form.source_type.trim(),
      citation: form.source_citation.trim(),
      url: form.source_url.trim(),
    },
  };
}

// validateForm mirrors the server's seed rules so obvious mistakes never leave
// the browser.
function validateForm(form: StoryFormState): string[] {
  const problems: string[] = [];
  const id = form.id.trim();
  if (!id) problems.push("id is empty");
  else if (id.length > 16) problems.push("id longer than 16 characters");
  const month = Number(form.month);
  const day = Number(form.day);
  const year = Number(form.year);
  if (!Number.isInteger(month) || month < 1 || month > 12) problems.push("month out of range 1-12");
  if (!Number.isInteger(day) || day < 1 || day > 31) problems.push("day out of range 1-31");
  if (!Number.isInteger(year) || year <= 0) problems.push("year must be positive");
  if (Number.isInteger(month) && Number.isInteger(day) && Number.isInteger(year) && year > 0) {
    const date = new Date(Date.UTC(year, month - 1, day));
    if (date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day) {
      problems.push("invalid calendar date");
    }
  }
  if (!form.title.trim()) problems.push("title is empty");
  if (!form.content.trim()) problems.push("content is empty");
  if (!form.geek_quote.trim()) problems.push("geek_quote is empty");
  return problems;
}
