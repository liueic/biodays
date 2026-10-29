import { stories } from "@/lib/content";
import { getLabArticle } from "@/lib/lab";

export type FeedbackArticle = {
  id: string;
  title: string;
  summary: string;
  path: string;
};

// Resolve public article metadata on the server, never from submitted titles or URLs.
export function feedbackArticle(id: string): FeedbackArticle | undefined {
  if (!id) return undefined;
  const story = stories.find((article) => article.id === id);
  const lab = story ? undefined : getLabArticle(id);
  const article = story ?? lab;
  if (!article) return undefined;
  return {
    id: article.id,
    title: article.title,
    summary: article.summary,
    path: lab
      ? `/lab/${lab.section}/${encodeURIComponent(article.id)}`
      : `/story/${encodeURIComponent(article.id)}`,
  };
}
