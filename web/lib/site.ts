export const SITE_NAME = "biodays";
export const SITE_TAGLINE = "实验室科学史上的今天";
export const SITE_DESCRIPTION =
  "每天一篇实验室里的历史故事：移液枪的发明、质谱的诺奖意外、液氮研钵与那些改变科学进程的器物。";

export const SITE_URL =
  process.env.BIODAYS_SITE_URL ||
  (process.env.VERCEL_URL ? `https://${process.env.VERCEL_URL}` : "http://localhost:3000");

export const DOCS_URL = process.env.NEXT_PUBLIC_BIODAYS_DOCS_URL ?? "/swagger/index.html";
