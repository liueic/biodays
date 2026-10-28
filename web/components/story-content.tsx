import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkCjkFriendly from "remark-cjk-friendly";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

import "katex/dist/katex.min.css";

import { toStoryMarkdown } from "@/lib/text";

type StoryContentProps = { content: string };

/**
 * Shared story and lab renderer: CommonMark, GFM tables/lists and LaTeX math.
 * CJK emphasis stays compatible with Chinese punctuation; raw HTML is disabled.
 */
export function StoryContent({ content }: StoryContentProps) {
  return (
    <div className="markdown-content">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath, remarkCjkFriendly]}
        rehypePlugins={[[rehypeKatex, { throwOnError: false }]]}
        components={{
          table: ({ node: _node, ...props }) => (
            // biome-ignore lint/a11y/noNoninteractiveTabindex: Keyboard users must be able to scroll wide tables.
            <section className="markdown-table-scroll" aria-label="表格（可横向滚动）" tabIndex={0}>
              <table {...props} />
            </section>
          ),
        }}
      >
        {toStoryMarkdown(content)}
      </ReactMarkdown>
    </div>
  );
}
