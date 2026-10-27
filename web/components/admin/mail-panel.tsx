"use client";

import { Button } from "@react95/core/Button";
import { Dropdown } from "@react95/core/Dropdown";
import { Frame } from "@react95/core/Frame";
import Link from "next/link";
import { useEffect, useState } from "react";

import { useAdminSession } from "@/components/admin/admin-session";
import {
  FeedbackApiError,
  type MailStatus,
  type MailTask,
  mailOutbox,
  mailStatus,
  type PageMeta,
  retryMail,
  sendTestMail,
} from "@/lib/admin-api";
import {
  formatDateTime,
  MAIL_STATUS_LABELS,
  MAIL_TYPE_LABELS,
  statusTone,
} from "@/lib/feedback-labels";

export function MailPanel() {
  const { logout } = useAdminSession();
  const [status, setStatus] = useState<MailStatus | null>(null);
  const [tasks, setTasks] = useState<MailTask[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ total: 0, page: 1, page_size: 20 });
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [refresh, setRefresh] = useState(0);

  useEffect(() => {
    void refresh; // explicit reload trigger
    let cancelled = false;
    setLoading(true);
    setError("");
    Promise.all([mailStatus(), mailOutbox({ status: filter, page, pageSize: 20 })])
      .then(([statusResponse, outboxResponse]) => {
        if (cancelled) return;
        setStatus(statusResponse.data);
        setTasks(outboxResponse.data);
        setMeta(outboxResponse.meta);
      })
      .catch((reason) => {
        if (cancelled) return;
        setError(reason instanceof FeedbackApiError ? reason.message : "无法读取邮件状态。");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [filter, page, refresh]);

  async function handleTest() {
    if (busy) return;
    setBusy(true);
    setNotice("");
    setError("");
    try {
      await sendTestMail();
      setNotice("测试邮件已加入队列，仅会发往配置的管理员接收地址。");
      setRefresh((value) => value + 1);
    } catch (reason) {
      setError(reason instanceof FeedbackApiError ? reason.message : "无法创建测试邮件。");
    } finally {
      setBusy(false);
    }
  }

  async function handleRetry(id: string) {
    if (busy) return;
    setBusy(true);
    setNotice("");
    setError("");
    try {
      await retryMail(id);
      setNotice("已重新排队。worker 会在发送前再次检查订阅与反馈状态。");
      setRefresh((value) => value + 1);
    } catch (reason) {
      setError(reason instanceof FeedbackApiError ? reason.message : "重试失败。");
    } finally {
      setBusy(false);
    }
  }

  const totalPages = Math.max(1, Math.ceil(meta.total / meta.page_size));

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <Button as={Link} href="/admin/feedback" className="px-3">
          ← 收件箱
        </Button>
        <Button as={Link} href="/admin/settings" className="px-3">
          Resend 设置
        </Button>
        <Button type="button" className="px-3" onClick={() => setRefresh((value) => value + 1)}>
          刷新
        </Button>
        <Button type="button" className="px-3" onClick={() => void logout()}>
          退出
        </Button>
      </div>

      {!status && loading && <p role="status">正在读取邮件配置…</p>}

      {status && (
        <Frame boxShadow="$in" padding="$3" className="text-[12px]">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
            <span>
              Resend：<strong>{status.resend_enabled ? "已启用" : "已关闭"}</strong>
            </span>
            <span>
              配置：<strong>{status.configured ? "齐全" : "不完整"}</strong>
            </span>
            <span>
              管理员收件：<strong>{status.notify_configured ? "已配置" : "未配置"}</strong>
            </span>
          </div>
          {!status.resend_enabled && (
            <p className="mt-2 text-[#7a4b00]">
              邮件发送暂停。反馈仍可保存，管理员通知与确认任务会保留在队列中。可在
              <Link href="/admin/settings" className="underline">
                站点设置
              </Link>
              中启用并配置 Resend。
            </p>
          )}
          {status.error && <p className="mt-2 text-[#7a0000]">发送暂停原因：{status.error}</p>}
          {status.missing.length > 0 && (
            <p className="mt-2 text-[#7a0000]">缺少配置：{status.missing.join("、")}</p>
          )}
          <div className="mt-2 flex flex-wrap gap-2">
            {Object.entries(MAIL_STATUS_LABELS).map(([key, label]) => (
              <span key={key} className={`px-2 py-[2px] ${statusTone(key)}`}>
                {label} {status.counts[key] ?? 0}
              </span>
            ))}
          </div>
          <div className="mt-3">
            <Button
              type="button"
              className="px-4"
              onClick={handleTest}
              disabled={!status.resend_enabled || busy}
            >
              发送测试邮件
            </Button>
            <span className="ml-2 text-[11px] text-[#555]">
              测试邮件只发往配置的管理员地址，不能指定任意收件人。
            </span>
          </div>
        </Frame>
      )}

      {notice && (
        <p role="status" className="border border-[#2d6a2d] bg-[#e6ffe6] px-3 py-2 text-[12px]">
          {notice}
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

      <div className="flex flex-wrap items-end gap-2">
        <label htmlFor="mail-status-filter" className="flex flex-col gap-1 text-[12px]">
          任务状态
          <Dropdown
            id="mail-status-filter"
            options={["全部", ...Object.values(MAIL_STATUS_LABELS)]}
            value={filter ? (MAIL_STATUS_LABELS[filter] ?? "全部") : "全部"}
            onChange={(event) => {
              const match = Object.entries(MAIL_STATUS_LABELS).find(
                ([, label]) => label === event.target.value,
              );
              setFilter(match ? match[0] : "");
              setPage(1);
            }}
          />
        </label>
        <span className="text-[12px]">共 {meta.total} 个任务</span>
      </div>

      <Frame boxShadow="$in" className="overflow-x-auto bg-white">
        <table className="w-full min-w-[640px] border-collapse text-left text-[12px]">
          <thead>
            <tr className="bg-[#c0c0c0]">
              <th className="px-2 py-1">任务</th>
              <th className="px-2 py-1">类型</th>
              <th className="px-2 py-1">状态</th>
              <th className="px-2 py-1">尝试</th>
              <th className="px-2 py-1 whitespace-nowrap">下次尝试</th>
              <th className="px-2 py-1">最近错误</th>
              <th className="px-2 py-1" />
            </tr>
          </thead>
          <tbody>
            {tasks.map((task) => (
              <tr key={task.id} className="align-top odd:bg-[#f7f7f7]">
                <td className="px-2 py-1 font-mono text-[10px] whitespace-nowrap">
                  {task.id.slice(0, 10)}…
                  <span className="block font-sans text-[10px] text-[#555]">
                    {formatDateTime(task.created_at)}
                  </span>
                </td>
                <td className="px-2 py-1 whitespace-nowrap">
                  {MAIL_TYPE_LABELS[task.type] ?? task.type}
                </td>
                <td className="px-2 py-1 whitespace-nowrap">
                  <span className={`px-1 ${statusTone(task.status)}`}>
                    {MAIL_STATUS_LABELS[task.status] ?? task.status}
                  </span>
                  {task.delivery_status && (
                    <span className="block text-[10px]">{task.delivery_status}</span>
                  )}
                </td>
                <td className="px-2 py-1">{task.attempts}</td>
                <td className="px-2 py-1 whitespace-nowrap">
                  {formatDateTime(task.next_attempt_at)}
                </td>
                <td className="max-w-[18rem] px-2 py-1 break-words">{task.last_error || "—"}</td>
                <td className="px-2 py-1">
                  {task.status === "failed" && (
                    <Button
                      type="button"
                      className="px-2"
                      disabled={busy}
                      onClick={() => void handleRetry(task.id)}
                    >
                      重试
                    </Button>
                  )}
                </td>
              </tr>
            ))}
            {!loading && tasks.length === 0 && (
              <tr>
                <td colSpan={7} className="px-2 py-6 text-center">
                  暂无邮件任务。
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </Frame>

      <div className="flex items-center gap-2">
        <Button
          type="button"
          className="px-3"
          disabled={page <= 1}
          onClick={() => setPage((value) => value - 1)}
        >
          上一页
        </Button>
        <span className="text-[12px]">
          第 {meta.page} / {totalPages} 页
        </span>
        <Button
          type="button"
          className="px-3"
          disabled={page >= totalPages}
          onClick={() => setPage((value) => value + 1)}
        >
          下一页
        </Button>
      </div>

      <Frame boxShadow="$in" padding="$3" className="text-[11px] text-[#555]">
        已接受发送表示 Resend
        已接受，不表示邮件已进入收件箱。状态不确定表示发送阶段断链，可能已被接受；请人工核对后再重试，避免重复邮件。
      </Frame>
    </div>
  );
}
