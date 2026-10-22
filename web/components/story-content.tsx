import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkCjkFriendly from "remark-cjk-friendly";
import remarkMath from "remark-math";

import "katex/dist/katex.min.css";

import { toStoryMarkdown } from "@/lib/text";

type StoryContentProps = { content: string };

type MarkdownNode = {
  type: string;
  value?: string;
  children?: MarkdownNode[];
};

function stripUnparsedBold(node: MarkdownNode) {
  if (node.type === "text" && node.value) {
    node.value = node.value.replace(/\*\*/g, "");
  }
  node.children?.forEach(stripUnparsedBold);
}

function remarkStripUnparsedBold() {
  return (tree: MarkdownNode) => stripUnparsedBold(tree);
}

/**
 * Story body renderer. The seed data mixes light Markdown (**bold**, *italic*,
 * `code`) with LaTeX inline math ($...$), so render both instead of raw text.
 * Raw HTML is not enabled: anything that looks like a tag stays literal text.
 * remark-cjk-friendly keeps **bold** working when Chinese punctuation touches
 * the delimiters; the final pass removes markers from malformed source data.
 */
export function StoryContent({ content }: StoryContentProps) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkMath, remarkCjkFriendly, remarkStripUnparsedBold]}
      rehypePlugins={[[rehypeKatex, { throwOnError: false }]]}
      components={{
        strong: ({ children }) => <strong className="font-bold">{children}</strong>,
        code: ({ children }) => <code className="inline-code">{children}</code>,
      }}
    >
      {toStoryMarkdown(content)}
    </ReactMarkdown>
  );
}
