export const KIND_LABELS: Record<string, string> = {
  topic: "想看话题",
  resource: "趣味资源",
  feature: "功能建议",
  issue: "问题纠错",
};

export const STATUS_LABELS: Record<string, string> = {
  pending: "待处理",
  reviewing: "处理中",
  accepted: "已采纳",
  completed: "已完成",
  rejected: "未采纳",
  spam: "垃圾信息",
};

export const STATUS_ORDER = [
  "pending",
  "reviewing",
  "accepted",
  "completed",
  "rejected",
  "spam",
] as const;

export const SUBSCRIPTION_LABELS: Record<string, string> = {
  none: "未订阅",
  pending_confirmation: "待确认",
  confirmed: "已确认",
  unsubscribed: "已退订",
};

export const MAIL_TYPE_LABELS: Record<string, string> = {
  admin_new_feedback: "管理员新反馈",
  subscription_confirm: "订阅确认",
  user_status_update: "处理结果",
  smtp_test: "SMTP 测试",
};

export const MAIL_STATUS_LABELS: Record<string, string> = {
  pending: "待发送",
  sending: "发送中",
  sent: "已投递",
  failed: "失败",
  unknown: "状态不确定",
  cancelled: "已取消",
};

export function formatDateTime(value: string | undefined): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

export function statusTone(status: string): string {
  switch (status) {
    case "pending":
      return "bg-[#fff4c2] text-[#6b5200]";
    case "reviewing":
      return "bg-[#d8ecff] text-[#003a66]";
    case "accepted":
    case "completed":
      return "bg-[#d9f2d9] text-[#145214]";
    case "rejected":
      return "bg-[#e8e8e8] text-[#333]";
    case "spam":
      return "bg-[#ffdede] text-[#7a0000]";
    case "sent":
      return "bg-[#d9f2d9] text-[#145214]";
    case "failed":
      return "bg-[#ffdede] text-[#7a0000]";
    case "unknown":
      return "bg-[#ffe0b3] text-[#5a3200]";
    default:
      return "bg-[#ececec] text-[#333]";
  }
}
