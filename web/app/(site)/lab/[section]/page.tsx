import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { LabBrowser } from "@/components/lab/lab-browser";
import { getLabSection, LAB_SECTIONS } from "@/lib/lab";
import { SITE_NAME } from "@/lib/site";

export const dynamicParams = false;

export function generateStaticParams() {
  return LAB_SECTIONS.map(({ id }) => ({ section: id }));
}

type PageProps = { params: Promise<{ section: string }> };

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const section = getLabSection((await params).section);
  if (!section) notFound();
  const url = `/lab/${section.id}`;
  return {
    title: section.title,
    description: section.description,
    alternates: { canonical: url },
    openGraph: {
      title: `${section.title} · ${SITE_NAME}`,
      description: section.description,
      url,
      type: "website",
    },
  };
}

export default async function LabSectionPage({ params }: PageProps) {
  const section = getLabSection((await params).section);
  if (!section) notFound();
  return <LabBrowser section={section} />;
}
