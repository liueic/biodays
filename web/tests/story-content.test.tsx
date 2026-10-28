import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { StoryContent } from "@/components/story-content";
import { getLabArticle, labArticles } from "@/lib/lab";

function render(content: string) {
  return renderToStaticMarkup(<StoryContent content={content} />);
}

describe("Markdown rendering", () => {
  test("METH-03 renders its four-column comparison with emphasis and math inside cells", () => {
    const article = getLabArticle("METH-03");
    expect(article).toBeDefined();
    const html = render(article?.content ?? "");
    expect(html.match(/<th[ >]/g)).toHaveLength(4);
    expect(html.match(/<td[ >]/g)).toHaveLength(12);
    expect(html).toContain('<section class="markdown-table-scroll"');
    expect(html).toContain("<strong>SpCas9-HF1（高保真变体 1）</strong>");
    expect(html).toMatch(/<td[^>]*>[^<]*[（(]<span class="katex"/);
    expect(html).not.toContain("| :---");
    expect(html).not.toContain("katex-error");
  });

  test("every lab article with a pipe table produces a semantic table", () => {
    const articles = labArticles.filter((article) => /^\|\s*:?-{3}/m.test(article.content));
    expect(articles.length).toBeGreaterThan(0);
    for (const article of articles) {
      expect(render(article.content)).toContain("<thead>");
    }
  });

  test("supports CommonMark and GFM while retaining code languages and table alignment", () => {
    const html = render(`## 实验记录

> **中文强调**，以及 *斜体* 和 ~~过期记录~~。

3. 保留列表起始序号
   - 嵌套条目

- [x] 已完成
- [ ] 待确认

| 左对齐 | 居中 | 右对齐 |
| :--- | :---: | ---: |
| \\| 转义分隔符 | $x^2$ | \\*\\*字面星号\\*\\* |

\`\`\`python
print("**保留代码**")
\`\`\`

行内 \`value = 1\` 和 https://example.com

---

脚注引用[^note]。

[^note]: 脚注内容
`);
    expect(html).toContain("<h2>实验记录</h2>");
    expect(html).toContain("<blockquote>");
    expect(html).toContain("<strong>中文强调</strong>");
    expect(html).toContain("<em>斜体</em>");
    expect(html).toContain("<del>过期记录</del>");
    expect(html).toContain('<ol start="3">');
    expect(html).toContain('class="task-list-item"');
    expect(html.match(/type="checkbox"/g)).toHaveLength(2);
    expect(html).toContain('disabled="" checked=""');
    expect(html).toContain('style="text-align:center"');
    expect(html).toContain('style="text-align:right"');
    expect(html).toContain("| 转义分隔符");
    expect(html).toContain("**字面星号**");
    expect(html).toContain('<pre><code class="language-python">');
    expect(html).toContain("**保留代码**");
    expect(html).toContain("<code>value = 1</code>");
    expect(html).toContain('href="https://example.com"');
    expect(html).toContain("<hr/>");
    expect(html).toContain('data-footnotes="true"');
    expect(html).not.toContain("katex-error");
  });

  test("does not execute embedded HTML or unsafe Markdown links", () => {
    const html = render("<script>alert(1)</script>\n\n[链接](javascript:alert%281%29)");
    expect(html).not.toContain("<script>");
    expect(html).not.toContain('href="javascript:');
  });
});
