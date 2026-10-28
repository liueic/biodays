"use client";

import { Button } from "@react95/core/Button";
import { HelpBook } from "@react95/icons/HelpBook";
import Link from "next/link";
import { useState } from "react";

import { LabSectionIcon } from "@/components/lab/section-icon";
import type { DailyLabRecommendations } from "@/lib/lab-recommendations";

export function DailyRecommendations({
  recommendations,
}: {
  recommendations: DailyLabRecommendations;
}) {
  const [rotation, setRotation] = useState(0);
  const { dayIndex, protocols, methods } = recommendations;
  const selection = [protocols, methods].flatMap((pool) =>
    pool.length ? [pool[(dayIndex + rotation) % pool.length]] : [],
  );
  if (!selection.length) return null;

  return (
    <section aria-labelledby="daily-lab-title" className="daily-lab">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id="daily-lab-title" className="flex items-center gap-2 font-bold">
          <HelpBook variant="16x16_4" width={16} height={16} aria-hidden />
          实验室灵感
        </h2>
        {(protocols.length > 1 || methods.length > 1) && (
          <Button
            onClick={() => setRotation((value) => value + 1)}
            className="mac-button shrink-0 px-2"
          >
            换一组
          </Button>
        )}
      </div>

      <div className="daily-lab-grid" aria-live="polite" aria-atomic="true">
        {selection.map((article) => (
          <Link
            key={article.id}
            href={`/lab/${article.section}/${article.id}`}
            className="daily-lab-card"
          >
            <span className="daily-lab-category">
              <LabSectionIcon section={article.section} size={16} />
              {article.section === "protocols" ? "Protocol 推荐" : "有趣的技术"}
              <span className="ml-auto" aria-hidden>
                ↗
              </span>
            </span>
            <h3 className="daily-lab-card-title">{article.title}</h3>
            <p className="daily-lab-teaser">{article.teaser}</p>
          </Link>
        ))}
      </div>

      <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-[#555]">
        <p>每天一点新思路，点击卡片查看原理与流程。</p>
        <Link href="/lab" className="text-link">
          打开实验室手册 →
        </Link>
      </div>
    </section>
  );
}
