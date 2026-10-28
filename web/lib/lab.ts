import labIndex from "@/content/lab-index.json";

export const LAB_SECTIONS = [
  {
    id: "methods",
    title: "技术解读",
    english: "Methods",
    summary: "有意思的技术，讲明白它为什么有用。",
    description: "从技术原理到应用场景，认识一种方法，也了解它的优势与边界。",
    topics: ["技术原理", "方法比较", "应用场景"],
    emptyDescription: "技术原理、方法比较与应用解读，将陆续收录在这里。",
  },
  {
    id: "protocols",
    title: "实验 Protocol",
    english: "Protocols",
    summary: "把实验的准备、步骤和关键点放在一起。",
    description: "查阅实验流程，关注材料准备、关键步骤、质量控制与常见问题。",
    topics: ["实验准备", "操作流程", "质控排错"],
    emptyDescription: "实验流程、关键步骤与配套说明，将陆续收录在这里。",
  },
  {
    id: "tips",
    title: "实验室技巧",
    english: "Lab tips",
    summary: "小小的改进，让实验做得更顺手。",
    description: "收集实验中的实用改进与方法小技巧，关注它解决的问题和适用条件。",
    topics: ["省时提效", "方法改进", "实用经验"],
    emptyDescription: "实验中的实用技巧与方法改进，将陆续收录在这里。",
  },
] as const;

export type LabSection = (typeof LAB_SECTIONS)[number];
export type LabSectionID = LabSection["id"];

export type LabArticleSource = {
  type: string;
  citation: string;
  url: string;
};

export type LabArticle = {
  id: string;
  section: LabSectionID;
  topic: string;
  title: string;
  lab_item: string;
  tags: string[];
  summary: string;
  geek_quote: string;
  source: LabArticleSource;
  content: string;
};

export const labArticles: LabArticle[] = labIndex.articles as LabArticle[];

export function getLabSection(id: string) {
  return LAB_SECTIONS.find((section) => section.id === id);
}

export function pickLabSection(exclude: readonly LabSectionID[] = []): LabSection {
  const available = LAB_SECTIONS.filter((section) => !exclude.includes(section.id));
  const candidates = available.length ? available : LAB_SECTIONS;
  return candidates[Math.floor(Math.random() * candidates.length)];
}

export function getLabArticles(sectionId?: string): LabArticle[] {
  if (!sectionId) return labArticles;
  return labArticles.filter((article) => article.section === sectionId);
}

export function getLabArticle(id: string): LabArticle | undefined {
  return labArticles.find((article) => article.id === id);
}

export function pickRandomLabArticle(sectionId?: string): LabArticle | undefined {
  const pool = getLabArticles(sectionId);
  if (!pool.length) return undefined;
  return pool[Math.floor(Math.random() * pool.length)];
}
