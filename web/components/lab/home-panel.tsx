"use client";

import { Button } from "@react95/core/Button";
import { Frame } from "@react95/core/Frame";
import { HelpBook } from "@react95/icons/HelpBook";
import Link from "next/link";
import { useState } from "react";

import { LabSectionIcon } from "@/components/lab/section-icon";
import { getLabSection, LAB_SECTIONS, type LabSectionID, pickLabSection } from "@/lib/lab";

export function LabHomePanel({ initialSectionId }: { initialSectionId: LabSectionID }) {
  const [selection, setSelection] = useState({
    id: initialSectionId,
    seen: [initialSectionId],
  });
  const section = getLabSection(selection.id) ?? LAB_SECTIONS[0];

  function drawNext() {
    const newRound = selection.seen.length === LAB_SECTIONS.length;
    const next = pickLabSection(newRound ? [section.id] : selection.seen);
    setSelection({
      id: next.id,
      seen: newRound ? [next.id] : [...selection.seen, next.id],
    });
  }

  return (
    <Frame
      as="section"
      aria-labelledby="home-lab-title"
      bgColor="$material"
      boxShadow="$out"
      className="relative mx-auto mt-5 w-full max-w-[640px] p-3"
    >
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 id="home-lab-title" className="flex items-center gap-2 font-bold">
          <HelpBook variant="16x16_4" width={16} height={16} aria-hidden />
          实验室手册
        </h2>
        <Link href="/lab" className="text-link">
          打开手册 →
        </Link>
      </div>
      <Frame boxShadow="$in" bgColor="$inputBackground" className="min-h-[188px] p-4 sm:p-5">
        <div aria-live="polite" aria-atomic="true">
          <div className="flex items-center gap-3">
            <LabSectionIcon section={section.id} />
            <div className="min-w-0">
              <p className="text-[#555]">栏目导览 · {section.english}</p>
              <h3 className="mt-1 text-base font-bold">{section.title}</h3>
            </div>
          </div>
          <p className="mt-4 min-h-12 font-bold leading-6 sm:min-h-0">{section.summary}</p>
          <p className="mt-2 min-h-12 leading-6 text-[#555] sm:min-h-0">{section.description}</p>
          <ul aria-label="当前栏目方向" className="mt-3 flex flex-wrap gap-2 text-[#555]">
            {section.topics.map((topic) => (
              <li key={topic} className="border border-[#c3c7cb] px-2 py-[1px]">
                {topic}
              </li>
            ))}
          </ul>
        </div>
      </Frame>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap gap-2">
          <Button as={Link} href={`/lab/${section.id}`} className="px-4">
            进入栏目(R)
          </Button>
          <Button onClick={drawNext} className="px-4">
            下一条(N)
          </Button>
        </div>
        <p role="status" className="text-[#555]">
          已浏览 {selection.seen.length} / {LAB_SECTIONS.length} 个栏目
        </p>
      </div>
    </Frame>
  );
}
