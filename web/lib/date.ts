const TIME_ZONE = "Asia/Shanghai";

export const EN_MONTHS = [
  "JAN",
  "FEB",
  "MAR",
  "APR",
  "MAY",
  "JUN",
  "JUL",
  "AUG",
  "SEP",
  "OCT",
  "NOV",
  "DEC",
] as const;

export const WEEKDAYS = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"] as const;

export type TodayStamp = {
  year: number;
  month: number;
  day: number;
  weekday: string;
  monthEn: string;
};

export function getTodayStamp(now: Date = new Date()): TodayStamp {
  const formatter = new Intl.DateTimeFormat("en-CA", {
    timeZone: TIME_ZONE,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    weekday: "short",
  });
  const parts = Object.fromEntries(
    formatter.formatToParts(now).map((part) => [part.type, part.value]),
  );
  const month = Number(parts.month);
  const weekdayMap: Record<string, string> = {
    Sun: "周日",
    Mon: "周一",
    Tue: "周二",
    Wed: "周三",
    Thu: "周四",
    Fri: "周五",
    Sat: "周六",
  };
  return {
    year: Number(parts.year),
    month,
    day: Number(parts.day),
    weekday: weekdayMap[parts.weekday ?? ""] ?? "",
    monthEn: EN_MONTHS[month - 1] ?? "",
  };
}

export function formatStoryDate(month: number, day: number): string {
  return `${month}月${day}日`;
}

export function formatMonthDay(month: number, day: number): string {
  return `${String(month).padStart(2, "0")}.${String(day).padStart(2, "0")}`;
}

export function formatIndex(index: number): string {
  return String(index).padStart(2, "0");
}
