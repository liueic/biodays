import type { Metadata } from "next";

import { LabBrowser } from "@/components/lab/lab-browser";
import { SITE_NAME } from "@/lib/site";

export const dynamic = "force-static";

export const metadata: Metadata = {
  title: "实验室手册",
  description: "在 biodays 实验室手册中探索技术解读、实验 Protocol 与实验室技巧。",
  alternates: { canonical: "/lab" },
  openGraph: {
    title: `实验室手册 · ${SITE_NAME}`,
    description: "了解技术，查阅流程，发现让实验更顺手的小技巧。",
    url: "/lab",
    type: "website",
  },
};

export default function LabPage() {
  return <LabBrowser />;
}
