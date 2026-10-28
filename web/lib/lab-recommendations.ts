import type { TodayStamp } from "@/lib/date";
import { getLabArticle } from "@/lib/lab";

export type LabRecommendation = {
  id: string;
  section: "protocols" | "methods";
  title: string;
  teaser: string;
};

export type DailyLabRecommendations = {
  dayIndex: number;
  protocols: LabRecommendation[];
  methods: LabRecommendation[];
};

// Short editorial introductions; the complete protocol and its sources stay in the handbook.
const introductions = [
  {
    id: "PROT-01",
    title: "SLIC：不用连接酶的无缝组装",
    teaser: "一段重叠序列，怎样让 DNA 片段找到彼此？看看这种组装方法的巧思。",
  },
  {
    id: "PROT-02",
    title: "Western Blot：一张膜如何再次检测？",
    teaser: "重复杂交前，先分清抗体洗脱与靶蛋白保留这两件事。",
  },
  {
    id: "PROT-03",
    title: "从平面培养到三维细胞微球",
    teaser: "培养表面如何影响细胞聚集？了解液体覆盖法的实验思路。",
  },
  {
    id: "PROT-04",
    title: "银染之后，还能送质谱吗？",
    teaser: "显色灵敏度与后续蛋白鉴定，为什么有时会互相牵制？",
  },
  {
    id: "PROT-05",
    title: "小肽总跑丢？认识 Tris-Tricine 电泳",
    teaser: "换一种尾随离子，如何改变小分子多肽在胶中的分离表现？",
  },
  {
    id: "PROT-06",
    title: "细胞冻存：速度为什么这么重要？",
    teaser: "从冰晶与细胞损伤的关系，梳理冻存、复苏和活率质控的关注点。",
  },
  {
    id: "PROT-07",
    title: "流式补偿：让微球替细胞当对照",
    teaser: "单染对照为什么重要？认识补偿微球的用途与适用边界。",
  },
  {
    id: "METH-03",
    title: "Cas9 也会认错路：高保真编辑的巧思",
    teaser: "从双切口酶到高保真变体，看看降低脱靶的不同设计思路。",
  },
  {
    id: "METH-02",
    title: "抗原修复：找回被藏起来的表位",
    teaser: "固定后的样本为什么有时染不出来？比较两类常见修复体系的思路。",
  },
  {
    id: "METH-01",
    title: "培养上清为什么能用来筛查支原体？",
    teaser: "从模板制备、扩增与对照，理解一种细胞培养质量检查方法。",
  },
];

/** Build small, valid recommendation pools on the server without shipping article bodies. */
export function getDailyLabRecommendations(stamp: TodayStamp): DailyLabRecommendations {
  const recommendations = introductions.flatMap((introduction): LabRecommendation[] => {
    const article = getLabArticle(introduction.id);
    if (!article || (article.section !== "protocols" && article.section !== "methods")) return [];
    return [{ ...introduction, section: article.section }];
  });

  return {
    // The server supplies the Shanghai calendar date, so SSR and hydration select the same pair.
    dayIndex: Math.floor(Date.UTC(stamp.year, stamp.month - 1, stamp.day) / 86_400_000),
    protocols: recommendations.filter((article) => article.section === "protocols"),
    methods: recommendations.filter((article) => article.section === "methods"),
  };
}
