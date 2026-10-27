import { Fieldset } from "@react95/core/Fieldset";
import { HelpBook } from "@react95/icons/HelpBook";
import type { Metadata } from "next";
import { Window } from "@/components/win/window";
import { getMeta } from "@/lib/server-api";
import { DOCS_URL, SITE_NAME } from "@/lib/site";

export const metadata: Metadata = {
  title: "关于",
  description: "biodays 的每日机制、数据覆盖度、公开 REST API 与来源声明。",
  alternates: { canonical: "/about" },
  openGraph: {
    title: `关于 · ${SITE_NAME}`,
    description: "biodays 的每日机制、数据覆盖度、公开 REST API 与来源声明。",
    url: "/about",
    siteName: SITE_NAME,
  },
};

const ENDPOINTS = [
  {
    path: "GET /api/v1/today",
    description: "今日故事。支持 tz（时区偏移）与 fallback（random / none）。",
  },
  {
    path: "GET /api/v1/stories",
    description: "故事列表。month、day、tag、q、page、page_size，按时间线排序。",
  },
  {
    path: "GET /api/v1/stories/{id}",
    description: "单篇详情，包含正文与来源引用。",
  },
  {
    path: "GET /api/v1/stories/random",
    description: "随机一篇，可用 exclude 排除正在展示的 id。",
  },
  {
    path: "GET /api/v1/tags",
    description: "全部标签与故事计数。",
  },
  {
    path: "GET /api/v1/meta",
    description: "数据集概况：总数、季度与月份覆盖。",
  },
  {
    path: "GET /healthz",
    description: "健康检查，不参与限流。",
  },
];

const ERRORS = [
  ["INVALID_PARAM", "400", "参数校验失败，例如 month=13"],
  ["STORY_NOT_FOUND", "404", "id 不存在，或当日无故事且 fallback=none"],
  ["RATE_LIMITED", "429", "触发单 IP 限流"],
  ["INTERNAL_ERROR", "500", "未预期错误"],
];

export default async function AboutPage() {
  const meta = await getMeta()
    .then((response) => response.data)
    .catch(() => null);

  return (
    <main>
      <Window
        title="关于 biodays"
        icon={<HelpBook variant="16x16_4" aria-hidden />}
        status={[meta ? `${meta.total} 篇` : "—", SITE_NAME]}
      >
        <h1 className="text-base font-bold">biodays</h1>
        <p className="reading mt-2">
          “实验室科学史上的今天”：每天一则发生在该月该日的实验室故事，或者是一些好玩的生物学小知识。
        </p>

        <Fieldset legend="每天怎么选故事" className="mt-5">
          <div className="reading">
            <p>
              服务端以 <code className="font-mono">Asia/Shanghai</code>（+08:00）判断“今天”，
              查找同月同日的条目；同日有多条时优先随机推荐未读故事，不限制故事编号。
            </p>
            <p>
              数据尚未覆盖的日期会随机推荐一篇并加以标注，也可通过{" "}
              <code className="font-mono">fallback=none</code> 关闭，直接得到 404。
            </p>
          </div>
        </Fieldset>

        <Fieldset legend="数据覆盖" className="mt-5">
          {meta ? (
            <>
              <dl className="grid grid-cols-[6rem_1fr] gap-y-1">
                <dt>故事总数</dt>
                <dd>{meta.total} 篇</dd>
                <dt>覆盖范围</dt>
                <dd>{meta.coverage || "暂无故事"}</dd>
                <dt>季度</dt>
                <dd>{meta.quarters.join("、") || "暂无故事"}</dd>
              </dl>
              <p className="mt-2">
                统计包含所有编号的故事，季度与月份按故事日期计算。月份覆盖不代表每天都有故事，缺口日期会随机推荐。
              </p>
            </>
          ) : (
            <p role="status">
              暂时无法读取数据统计，请<a href="/about">刷新重试</a>。
            </p>
          )}
        </Fieldset>

        <Fieldset legend="公开 API" className="mt-5">
          <p>
            版本化 REST API，前缀 <code className="font-mono">/api/v1</code>，完整定义见{" "}
            <a href={DOCS_URL} target="_blank" rel="noreferrer">
              Swagger 文档
            </a>
            。
          </p>
          <div className="mt-2 overflow-x-auto">
            <table className="w-full border-collapse text-left">
              <tbody>
                {ENDPOINTS.map((endpoint) => (
                  <tr key={endpoint.path} className="border-b border-[#c3c7cb] align-top">
                    <td className="py-1 pr-3 font-mono text-[11px] whitespace-nowrap">
                      {endpoint.path}
                    </td>
                    <td className="py-1">{endpoint.description}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="mt-3 font-bold">错误码</p>
          <table className="w-full border-collapse text-left">
            <tbody>
              {ERRORS.map(([code, status, description]) => (
                <tr key={code} className="border-b border-[#c3c7cb] align-top">
                  <td className="py-1 pr-3 font-mono text-[11px]">{code}</td>
                  <td className="py-1 pr-3">{status}</td>
                  <td className="py-1">{description}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Fieldset>

        <Fieldset legend="来源与版权" className="mt-5">
          <div className="reading">
            <p>
              每篇故事基于专利、论文、诺奖新闻稿或机构档案整理改写，文末标注引用并尽可能给出原文链接。
              正文是面向读者的叙述，不是文献原文，历史细节以原始来源为准。
            </p>
            <p>
              如发现事实、年份或引用有误，欢迎通过{" "}
              <a href="/feedback?kind=issue" className="text-link">
                灵感信箱
              </a>{" "}
              匿名指出；也可以推荐想看的话题或有趣的实验室资源。转载请注明出处。
            </p>
          </div>
        </Fieldset>
      </Window>
    </main>
  );
}
