"use client";

import { Button } from "@react95/core/Button";
import { Dropdown } from "@react95/core/Dropdown";
import { Input } from "@react95/core/Input";
import { Bookmark } from "@react95/icons/Bookmark";
import { Folder } from "@react95/icons/Folder";
import { FolderOpen } from "@react95/icons/FolderOpen";
import { Mail } from "@react95/icons/Mail";
import { Mailnews2 } from "@react95/icons/Mailnews2";
import { Refresh } from "@react95/icons/Refresh";
import { Sendmail2001 } from "@react95/icons/Sendmail2001";
import Link from "next/link";
import { useEffect, useState } from "react";

import { useAdminSession } from "@/components/admin/admin-session";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import {
  FeedbackApiError,
  type FeedbackDetail,
  type FeedbackListItem,
  getFeedback,
  listFeedback,
  type PageMeta,
} from "@/lib/admin-api";
import {
  formatDateTime,
  KIND_LABELS,
  SPAM_VERDICT_LABELS,
  STATUS_LABELS,
  STATUS_ORDER,
  SUBSCRIPTION_LABELS,
} from "@/lib/feedback-labels";

const PAGE_SIZE = 20;
const KIND_OPTIONS = ["全部类型", ...Object.values(KIND_LABELS)];
const FOLDERS = [
  { status: "", label: "收件箱" },
  ...STATUS_ORDER.map((status) => ({ status, label: STATUS_LABELS[status] })),
];

export function FeedbackInbox() {
  const { session, logout } = useAdminSession();
  const [kind, setKind] = useState("");
  const [status, setStatus] = useState("");
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [items, setItems] = useState<FeedbackListItem[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ total: 0, page: 1, page_size: PAGE_SIZE });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<FeedbackDetail | null>(null);
  const [previewID, setPreviewID] = useState("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState("");
  const [refreshing, setRefreshing] = useState(0);
  const debouncedQuery = useDebouncedValue(query, 300);

  useEffect(() => {
    void refreshing;
    let cancelled = false;
    setLoading(true);
    setError("");
    listFeedback({ kind, status, q: debouncedQuery.trim(), page, pageSize: PAGE_SIZE })
      .then((response) => {
        if (cancelled) return;
        setItems(response.data);
        setMeta(response.meta);
        setPreviewID((current) =>
          response.data.some((item) => item.id === current)
            ? current
            : (response.data[0]?.id ?? ""),
        );
      })
      .catch((reason) => {
        if (cancelled) return;
        setError(reason instanceof FeedbackApiError ? reason.message : "无法加载反馈列表。");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [kind, status, debouncedQuery, page, refreshing]);

  useEffect(() => {
    if (!previewID) {
      setPreview(null);
      setPreviewError("");
      setPreviewLoading(false);
      return;
    }
    let cancelled = false;
    setPreview(null);
    setPreviewError("");
    setPreviewLoading(true);
    getFeedback(previewID)
      .then((response) => {
        if (!cancelled) setPreview(response.data);
      })
      .catch(() => {
        if (!cancelled) {
          setPreview(null);
          setPreviewError("无法读取这封邮件，请刷新后重试。");
        }
      })
      .finally(() => {
        if (!cancelled) setPreviewLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [previewID]);

  const totalPages = Math.max(1, Math.ceil(meta.total / meta.page_size));
  const currentFolder = FOLDERS.find((folder) => folder.status === status) ?? {
    status: "",
    label: "收件箱",
  };
  const hasFilters = Boolean(kind || status || query);

  function selectFolder(value: string) {
    setStatus(value);
    setPage(1);
    setPreviewID("");
  }

  function clearFilters() {
    setKind("");
    setStatus("");
    setQuery("");
    setPage(1);
    setPreviewID("");
  }

  return (
    <div className="mail-inbox">
      <div className="mail-toolbar" role="toolbar" aria-label="收件箱操作">
        <div className="mail-toolbar-group">
          <Button as={Link} href="/feedback" className="mail-tool-button">
            <Sendmail2001 variant="32x32_4" aria-hidden />
            <span>写邮件</span>
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
            <span>{loading ? "接收中…" : "发送/接收"}</span>
          </Button>
          <Button
            as={Link}
            href="/admin/mail"
            className="mail-tool-button mail-tool-button--inline"
          >
            <Mail variant="16x16_4" aria-hidden />
            <span>邮件状态</span>
          </Button>
          <Button
            as={Link}
            href="/admin/stories"
            className="mail-tool-button mail-tool-button--inline"
          >
            <Bookmark variant="16x16_4" aria-hidden />
            <span>故事管理</span>
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
        <label htmlFor="filter-kind" className="mail-filter-field">
          <span>类型</span>
          <Dropdown
            id="filter-kind"
            options={KIND_OPTIONS}
            value={kind ? (KIND_LABELS[kind] ?? "全部类型") : "全部类型"}
            onChange={(event) => {
              const match = Object.entries(KIND_LABELS).find(
                ([, label]) => label === event.target.value,
              );
              setKind(match ? match[0] : "");
              setPage(1);
            }}
            aria-label="按类型筛选"
          />
        </label>
        <label htmlFor="filter-query" className="mail-filter-field mail-filter-field--search">
          <span>搜索</span>
          <Input
            id="filter-query"
            type="search"
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setPage(1);
            }}
            placeholder="主题、正文或称呼"
            aria-label="搜索反馈"
          />
        </label>
        {hasFilters && (
          <Button
            type="button"
            className="mail-tool-button mail-tool-button--inline"
            onClick={clearFilters}
          >
            清除筛选
          </Button>
        )}
      </div>

      {error && (
        <p role="alert" className="mail-alert">
          {error}
        </p>
      )}

      <div className="mail-folder-select">
        <label htmlFor="folder-select">文件夹</label>
        <Dropdown
          id="folder-select"
          options={FOLDERS.map((folder) => folder.label)}
          value={currentFolder.label}
          onChange={(event) => {
            const folder = FOLDERS.find((item) => item.label === event.target.value);
            if (folder) selectFolder(folder.status);
          }}
        />
      </div>

      <div className="mail-inbox-body">
        <aside className="mail-folder-pane" aria-label="邮件文件夹">
          <div className="mail-folder-root">
            <Mailnews2 variant="16x16_4" aria-hidden />
            <span>Outlook Express</span>
          </div>
          <ul className="mail-folder-list">
            {FOLDERS.map((folder) => {
              const selected = status === folder.status;
              return (
                <li key={folder.status || "inbox"}>
                  <button
                    type="button"
                    className="mail-folder-button"
                    aria-current={selected ? "true" : undefined}
                    onClick={() => selectFolder(folder.status)}
                  >
                    {selected ? (
                      <FolderOpen variant="16x16_4" width={16} height={16} aria-hidden />
                    ) : (
                      <Folder variant="16x16_4" width={16} height={16} aria-hidden />
                    )}
                    <span>{folder.label}</span>
                  </button>
                </li>
              );
            })}
            <li className="mail-folder-separator" aria-hidden="true" />
            <li>
              <Link className="mail-folder-button" href="/admin/mail">
                <Mail variant="16x16_4" width={16} height={16} aria-hidden />
                <span>邮件发件箱</span>
              </Link>
            </li>
          </ul>
        </aside>

        <section className="mail-list-pane" aria-label="邮件列表">
          <div className="mail-list-caption">
            <span>
              {currentFolder.label}
              {kind ? ` · ${KIND_LABELS[kind]}` : ""}
            </span>
            <span aria-live="polite">{loading ? "接收中…" : `${meta.total} 封邮件`}</span>
          </div>

          <div className="mail-list-scroll">
            <table className="mail-message-table">
              <thead>
                <tr>
                  <th className="mail-col-status">状态</th>
                  <th className="mail-col-type">类型</th>
                  <th>主题</th>
                  <th className="mail-col-contact">联系</th>
                  <th className="mail-col-date">接收时间</th>
                  <th className="mail-col-open">
                    <span className="sr-only">操作</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => {
                  const selected = previewID === item.id;
                  return (
                    <tr key={item.id} className="mail-message-row" data-selected={selected}>
                      <td className="mail-col-status">
                        <span className="mail-status-text" data-status={item.status}>
                          {STATUS_LABELS[item.status] ?? item.status}
                        </span>
                      </td>
                      <td className="mail-col-type">{KIND_LABELS[item.kind] ?? item.kind}</td>
                      <td>
                        <Link
                          href={`/admin/feedback/${item.id}`}
                          className="mail-message-subject mail-message-open-subject"
                          title={item.title}
                        >
                          <span className="mail-message-title-text">
                            {item.title}
                            {item.spam_verdict && item.spam_verdict !== "allow" && (
                              <span
                                className={`ml-1 inline-block px-1 text-[10px] ${
                                  item.spam_verdict === "spam"
                                    ? "bg-[#ffdede] text-[#7a0000]"
                                    : "bg-[#fff4c2] text-[#6b5200]"
                                }`}
                              >
                                {SPAM_VERDICT_LABELS[item.spam_verdict] ?? item.spam_verdict}{" "}
                                {item.spam_score}
                              </span>
                            )}
                          </span>
                          <span className="mail-message-snippet">{item.summary}</span>
                        </Link>
                        <button
                          type="button"
                          className="mail-message-subject mail-message-select-subject"
                          aria-current={selected ? "true" : undefined}
                          title={`预览：${item.title}`}
                          onClick={() => setPreviewID(item.id)}
                        >
                          <span className="mail-message-title-text">{item.title}</span>
                          <span className="mail-message-snippet">{item.summary}</span>
                        </button>
                      </td>
                      <td className="mail-col-contact">
                        {item.has_email ? "可联系" : item.has_nickname ? "有称呼" : "匿名"}
                      </td>
                      <td className="mail-col-date">
                        <time dateTime={item.created_at} title={formatDateTime(item.created_at)}>
                          {formatMailDate(item.created_at)}
                        </time>
                      </td>
                      <td className="mail-col-open">
                        <Link className="mail-message-link" href={`/admin/feedback/${item.id}`}>
                          打开
                        </Link>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            {!loading && items.length === 0 && (
              <div className="mail-list-empty">这个文件夹里没有符合条件的邮件。</div>
            )}
            {loading && items.length === 0 && (
              <div className="mail-list-empty" role="status">
                正在接收邮件…
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

        <aside className="mail-preview-pane" aria-label="阅读邮件" aria-busy={previewLoading}>
          {previewLoading ? (
            <div className="mail-preview-empty" role="status">
              <Mailnews2 variant="32x32_4" aria-hidden />
              <p>正在打开邮件…</p>
            </div>
          ) : previewError ? (
            <div className="mail-preview-empty" role="alert">
              <Mailnews2 variant="32x32_4" aria-hidden />
              <p>{previewError}</p>
            </div>
          ) : preview ? (
            <article>
              <header className="mail-preview-header">
                <h2>{preview.title}</h2>
                <dl className="mail-preview-meta">
                  <dt>发件人</dt>
                  <dd>{preview.nickname || "匿名访客"}</dd>
                  <dt>时间</dt>
                  <dd>{formatDateTime(preview.created_at)}</dd>
                  <dt>类型</dt>
                  <dd>{KIND_LABELS[preview.kind] ?? preview.kind}</dd>
                  <dt>状态</dt>
                  <dd>
                    <span className="mail-status-text" data-status={preview.status}>
                      {STATUS_LABELS[preview.status] ?? preview.status}
                    </span>
                  </dd>
                </dl>
              </header>
              <div className="mail-preview-content">{preview.content}</div>
              <dl className="mail-preview-details">
                <dt>邮箱</dt>
                <dd>{preview.email || "未留邮箱"}</dd>
                <dt>订阅</dt>
                <dd>
                  {SUBSCRIPTION_LABELS[preview.subscription_status] ?? preview.subscription_status}
                </dd>
                <dt>邮件编号</dt>
                <dd className="font-mono">{preview.id}</dd>
              </dl>
              <div className="mail-preview-actions">
                <Button
                  as={Link}
                  href={`/admin/feedback/${preview.id}`}
                  className="mail-tool-button mail-tool-button--inline"
                >
                  打开完整邮件与处理
                </Button>
              </div>
            </article>
          ) : (
            <div className="mail-preview-empty">
              <Mailnews2 variant="32x32_4" aria-hidden />
              <p>选择一封邮件即可在这里阅读。</p>
            </div>
          )}
        </aside>
      </div>

      <div className="mail-inbox-status">
        <span className="mail-status-cell">
          {query ? `搜索“${query}” · ${meta.total} 封结果` : `共 ${meta.total} 封邮件`}
        </span>
        <span className="mail-status-cell mail-status-cell--shrink">
          {preview ? "已选择 1 封" : "未选择邮件"}
        </span>
      </div>
    </div>
  );
}

function formatMailDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}
