"use client";

import { Button } from "@react95/core/Button";
import { Checkbox } from "@react95/core/Checkbox";
import { Dropdown } from "@react95/core/Dropdown";
import { Fieldset } from "@react95/core/Fieldset";
import { Frame } from "@react95/core/Frame";
import { TextArea } from "@react95/core/TextArea";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { useAdminSession } from "@/components/admin/admin-session";
import {
  deleteFeedback,
  FeedbackApiError,
  type FeedbackDetail,
  type FeedbackStatus,
  getFeedback,
  updateFeedback,
} from "@/lib/admin-api";
import {
  formatDateTime,
  KIND_LABELS,
  MAIL_STATUS_LABELS,
  MAIL_TYPE_LABELS,
  SPAM_SIGNAL_LABELS,
  SPAM_SOURCE_LABELS,
  SPAM_VERDICT_LABELS,
  STATUS_LABELS,
  STATUS_ORDER,
  SUBSCRIPTION_LABELS,
  statusTone,
  VERIFICATION_METHOD_LABELS,
} from "@/lib/feedback-labels";
import { safeExternalUrl } from "@/lib/url";

export function FeedbackDetailView({ id }: { id: string }) {
  const router = useRouter();
  const { logout } = useAdminSession();
  const [detail, setDetail] = useState<FeedbackDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [status, setStatus] = useState<FeedbackStatus>("pending");
  const [internalNote, setInternalNote] = useState("");
  const [publicReply, setPublicReply] = useState("");
  const [notify, setNotify] = useState(false);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [restoring, setRestoring] = useState(false);

  const applyDetail = useCallback((value: FeedbackDetail) => {
    setDetail(value);
    setStatus(value.status);
    setInternalNote(value.internal_note);
    setPublicReply(value.public_reply);
  }, []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    getFeedback(id)
      .then((response) => {
        if (!cancelled) applyDetail(response.data);
      })
      .catch((reason) => {
        if (!cancelled) {
          setLoadError(reason instanceof FeedbackApiError ? reason.message : "无法读取反馈详情。");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id, applyDetail]);

  async function handleSave() {
    if (!detail || saving) return;
    setSaving(true);
    setError("");
    setMessage("");
    try {
      const response = await updateFeedback(detail.id, {
        status,
        internal_note: internalNote,
        public_reply: publicReply,
        version: detail.version,
        notify_subscriber: notify,
      });
      applyDetail(response.data);
      setMessage(
        response.meta.notified
          ? "已保存，并把本次处理结果加入订阅者通知队列。"
          : `已保存，未发送用户邮件。${response.meta.notify_skipped_note ? ` ${response.meta.notify_skipped_note}` : ""}`,
      );
      setNotify(false);
    } catch (reason) {
      if (reason instanceof FeedbackApiError && reason.status === 409) {
        setError("这条反馈已被其他请求修改，正在重新加载最新版本。");
        const refreshed = await getFeedback(detail.id).catch(() => null);
        if (refreshed) applyDetail(refreshed.data);
      } else if (reason instanceof FeedbackApiError) {
        setError(reason.message);
      } else {
        setError("保存失败，请检查网络后重试。");
      }
    } finally {
      setSaving(false);
    }
  }

  async function handleRestore() {
    if (!detail || restoring) return;
    setRestoring(true);
    setError("");
    setMessage("");
    try {
      const response = await updateFeedback(detail.id, {
        status: "pending",
        internal_note: detail.internal_note,
        public_reply: detail.public_reply,
        version: detail.version,
        notify_subscriber: false,
      });
      applyDetail(response.data);
      setMessage("已恢复为待处理；不会发送用户邮件。");
    } catch (reason) {
      if (reason instanceof FeedbackApiError && reason.status === 409) {
        setError("这条反馈已被其他请求修改，正在重新加载最新版本。");
        const refreshed = await getFeedback(detail.id).catch(() => null);
        if (refreshed) applyDetail(refreshed.data);
      } else if (reason instanceof FeedbackApiError) {
        setError(reason.message);
      } else {
        setError("恢复失败，请检查网络后重试。");
      }
    } finally {
      setRestoring(false);
    }
  }

  async function handleDelete() {
    if (!detail || deleting) return;
    if (!confirmDelete) {
      setConfirmDelete(true);
      return;
    }
    setDeleting(true);
    try {
      await deleteFeedback(detail.id);
      router.push("/admin/feedback");
    } catch (reason) {
      setError(reason instanceof FeedbackApiError ? reason.message : "删除失败，请稍后重试。");
      setDeleting(false);
    }
  }

  if (loading) {
    return (
      <p role="status" className="reading">
        正在读取反馈详情…
      </p>
    );
  }

  if (loadError || !detail) {
    return (
      <div className="reading">
        <p role="alert">{loadError || "反馈不存在。"}</p>
        <p className="mt-3">
          <Link className="text-link" href="/admin/feedback">
            返回收件箱
          </Link>
        </p>
      </div>
    );
  }

  const sourceURL = safeExternalUrl(detail.source_url);
  const confirmed = detail.subscription_status === "confirmed";
  const statusChanged = status !== detail.status;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <Button as={Link} href="/admin/feedback" className="px-3">
            ← 收件箱
          </Button>
          <Button as={Link} href="/admin/mail" className="px-3">
            邮件状态
          </Button>
          <Button type="button" className="px-3" onClick={() => void logout()}>
            退出
          </Button>
        </div>
        <span className={`px-2 py-1 text-[12px] ${statusTone(detail.status)}`}>
          {STATUS_LABELS[detail.status] ?? detail.status} · v{detail.version}
        </span>
      </div>

      {message && (
        <p role="status" className="border border-[#2d6a2d] bg-[#e6ffe6] px-3 py-2 text-[12px]">
          {message}
        </p>
      )}
      {error && (
        <p
          role="alert"
          className="border border-[#a00000] bg-[#ffe8e8] px-3 py-2 text-[12px] text-[#7a0000]"
        >
          {error}
        </p>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        <Fieldset legend="原始投稿">
          <div className="flex flex-col gap-2 text-[13px]">
            <h2 className="text-[15px] font-bold break-words">{detail.title}</h2>
            <p className="text-[12px] text-[#555]">
              {KIND_LABELS[detail.kind] ?? detail.kind} · 编号{" "}
              <span className="font-mono">{detail.id}</span> · {formatDateTime(detail.created_at)}
            </p>
            <p className="whitespace-pre-wrap break-words">{detail.content}</p>
            {sourceURL && (
              <p className="text-[12px]">
                资源链接：
                <a
                  className="text-link break-all"
                  href={sourceURL}
                  target="_blank"
                  rel="noreferrer noopener"
                >
                  {sourceURL}
                </a>
              </p>
            )}
            {detail.story_id && (
              <p className="text-[12px]">
                关联故事：
                <Link
                  className="text-link font-mono"
                  href={`/story/${encodeURIComponent(detail.story_id)}`}
                >
                  {detail.story_id}
                </Link>
              </p>
            )}
          </div>
        </Fieldset>

        <Fieldset legend="联系方式与订阅">
          <dl className="grid grid-cols-[6rem_1fr] gap-y-2 text-[12px]">
            <dt>称呼</dt>
            <dd>{detail.nickname || "匿名访客"}</dd>
            <dt>邮箱</dt>
            <dd className="break-all">{detail.email || "未留邮箱"}</dd>
            <dt>邮箱状态</dt>
            <dd>
              {detail.email
                ? detail.email_verified
                  ? "已验证（订阅已确认）"
                  : "未验证；不会自动发送用户邮件"
                : "无"}
            </dd>
            <dt>订阅</dt>
            <dd>{SUBSCRIPTION_LABELS[detail.subscription_status] ?? detail.subscription_status}</dd>
            <dt>确认时间</dt>
            <dd>{formatDateTime(detail.confirmed_at)}</dd>
            <dt>退订时间</dt>
            <dd>{formatDateTime(detail.unsubscribed_at)}</dd>
          </dl>
        </Fieldset>
      </div>

      <Fieldset legend="反垃圾判断">
        <div className="flex flex-col gap-2 text-[12px]">
          <dl className="grid grid-cols-[6rem_1fr] gap-y-2">
            <dt>评分</dt>
            <dd>{detail.spam_score} / 100</dd>
            <dt>判定</dt>
            <dd>{(SPAM_VERDICT_LABELS[detail.spam_verdict] ?? detail.spam_verdict) || "—"}</dd>
            <dt>来源</dt>
            <dd>{(SPAM_SOURCE_LABELS[detail.spam_source] ?? detail.spam_source) || "—"}</dd>
            <dt>验证方式</dt>
            <dd>
              {VERIFICATION_METHOD_LABELS[detail.verification_method] ??
                detail.verification_method ??
                "—"}
            </dd>
            <dt>AI 模型</dt>
            <dd className="break-all">{detail.spam_model || "—"}</dd>
            <dt>垃圾概率</dt>
            <dd>
              {typeof detail.spam_probability === "number"
                ? `${Math.round(detail.spam_probability * 100)}%`
                : "—"}
            </dd>
            {detail.spam_probability == null && detail.spam_confidence > 0 && (
              <>
                <dt>历史置信度</dt>
                <dd>{Math.round(detail.spam_confidence * 100)}%</dd>
              </>
            )}
            <dt>判断时间</dt>
            <dd>{formatDateTime(detail.verified_at)}</dd>
          </dl>
          {detail.spam_signals.length > 0 && (
            <p className="text-[11px] text-[#555]">
              信号：
              {detail.spam_signals.map((signal) => SPAM_SIGNAL_LABELS[signal] ?? signal).join("、")}
            </p>
          )}
        </div>
      </Fieldset>

      <Fieldset legend="处理">
        <div className="flex flex-col gap-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <label htmlFor="feedback-status" className="flex flex-col gap-1 text-[12px]">
              状态
              <Dropdown
                id="feedback-status"
                options={STATUS_ORDER.map((value) => STATUS_LABELS[value])}
                value={STATUS_LABELS[status]}
                onChange={(event) => {
                  const match = STATUS_ORDER.find(
                    (value) => STATUS_LABELS[value] === event.target.value,
                  );
                  if (match) setStatus(match);
                }}
              />
            </label>
            <div className="flex items-end pb-1">
              <Checkbox
                id="notify-subscriber"
                checked={notify}
                disabled={!confirmed}
                onChange={(event) => setNotify(event.target.checked)}
                label="保存后通知已确认的订阅者"
              />
            </div>
          </div>
          {!confirmed && (
            <p className="text-[11px] text-[#555]">
              该反馈没有已确认的订阅者，保存不会发送用户邮件。未验证邮箱仅用于管理员联系。
            </p>
          )}
          {confirmed && !statusChanged && (
            <p className="text-[11px] text-[#555]">
              状态未变化时不会自动发信；需要再次通知请改变状态并勾选通知。
            </p>
          )}

          <div>
            <label htmlFor="internal-note" className="mb-1 block font-bold">
              内部备注{" "}
              <span className="font-normal text-[#7a0000]">
                （仅后台可见，不会出现在任何用户邮件中）
              </span>
            </label>
            <TextArea
              id="internal-note"
              rows={4}
              value={internalNote}
              onChange={(event) => setInternalNote(event.target.value)}
              className="w-full"
            />
          </div>

          <div>
            <label htmlFor="public-reply" className="mb-1 block font-bold">
              给用户的处理说明 <span className="font-normal text-[#555]">（可随订阅通知发送）</span>
            </label>
            <TextArea
              id="public-reply"
              rows={4}
              value={publicReply}
              onChange={(event) => setPublicReply(event.target.value)}
              className="w-full"
            />
            <p className="mt-1 text-[11px] text-[#555]">
              标记为垃圾信息时不发送通知；处理中与内部备注变化不会自动发信。
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              className="px-4"
              onClick={handleSave}
              disabled={saving || deleting || restoring}
            >
              {saving ? "保存中…" : notify ? "保存并通知订阅者" : "保存"}
            </Button>
            {detail.spam_source && detail.spam_verdict === "spam" && (
              <Button
                type="button"
                className="px-4"
                onClick={handleRestore}
                disabled={saving || deleting || restoring}
              >
                {restoring ? "恢复中…" : "恢复为待处理"}
              </Button>
            )}
            <Button
              type="button"
              className="px-4"
              onClick={handleDelete}
              disabled={deleting}
              style={confirmDelete ? { background: "#ffd0d0" } : undefined}
            >
              {deleting ? "删除中…" : confirmDelete ? "再次点击确认删除" : "删除记录"}
            </Button>
            {confirmDelete && (
              <Button
                type="button"
                className="px-3"
                onClick={() => setConfirmDelete(false)}
                disabled={deleting}
              >
                取消
              </Button>
            )}
          </div>
          {confirmDelete && (
            <p role="alert" className="text-[12px] text-[#7a0000]">
              删除会清理联系方式、订阅令牌、事件与未发送邮件，仅保留不含原文的最小删除审计信息。此操作不可撤销。
            </p>
          )}
        </div>
      </Fieldset>

      <div className="grid gap-4 lg:grid-cols-2">
        <Fieldset legend={`处理记录（${detail.events.length}）`}>
          {detail.events.length === 0 ? (
            <p className="text-[12px]">暂无状态变更。</p>
          ) : (
            <ol className="flex flex-col gap-3 text-[12px]">
              {detail.events.map((event) => (
                <li key={event.seq} className="border-l-2 border-[#808080] pl-3">
                  <p className="font-bold">
                    {event.from_status
                      ? `${STATUS_LABELS[event.from_status] ?? event.from_status} → `
                      : "提交 → "}
                    {STATUS_LABELS[event.to_status] ?? event.to_status}
                  </p>
                  <p className="text-[11px] text-[#555]">
                    {formatDateTime(event.created_at)} · {event.actor}
                  </p>
                  {event.public_reply && (
                    <p className="mt-1 whitespace-pre-wrap">用户说明：{event.public_reply}</p>
                  )}
                  {event.internal_note && (
                    <p className="mt-1 whitespace-pre-wrap text-[#555]">
                      内部备注：{event.internal_note}
                    </p>
                  )}
                </li>
              ))}
            </ol>
          )}
        </Fieldset>

        <Fieldset legend="邮件任务">
          {detail.mail.length === 0 ? (
            <p className="text-[12px]">暂无邮件任务。</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[420px] border-collapse text-left text-[11px]">
                <thead>
                  <tr className="bg-[#c0c0c0]">
                    <th className="px-2 py-1">类型</th>
                    <th className="px-2 py-1">状态</th>
                    <th className="px-2 py-1">尝试</th>
                    <th className="px-2 py-1">原因</th>
                  </tr>
                </thead>
                <tbody>
                  {detail.mail.map((task) => (
                    <tr key={task.id} className="align-top odd:bg-[#f7f7f7]">
                      <td className="px-2 py-1 whitespace-nowrap">
                        {MAIL_TYPE_LABELS[task.type] ?? task.type}
                      </td>
                      <td className="px-2 py-1 whitespace-nowrap">
                        <span className={`px-1 ${statusTone(task.status)}`}>
                          {MAIL_STATUS_LABELS[task.status] ?? task.status}
                        </span>
                      </td>
                      <td className="px-2 py-1">{task.attempts}</td>
                      <td className="max-w-[16rem] px-2 py-1 break-words">
                        {task.last_error || "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Fieldset>
      </div>

      <Frame boxShadow="$in" padding="$3" className="text-[11px] text-[#555]">
        邮件仅引用“给用户的处理说明”和面向用户的状态；内部备注不会进入用户输出。已接受发送表示
        Resend 已接受，不代表已进入收件箱。
      </Frame>
    </div>
  );
}
