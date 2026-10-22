export function toParagraphs(content: string): string[] {
  const explicit = content
    .split(/\n{1,}/)
    .map((paragraph) => paragraph.trim())
    .filter(Boolean);
  if (explicit.length > 1) return explicit;

  const sentences = content.match(/[^。！？]+[。！？]?/g) ?? [content];
  const paragraphs: string[] = [];
  let buffer = "";
  for (const sentence of sentences) {
    buffer += sentence;
    if (buffer.length >= 110) {
      paragraphs.push(buffer.trim());
      buffer = "";
    }
  }
  if (buffer.trim()) paragraphs.push(buffer.trim());
  return paragraphs.length > 0 ? paragraphs : [content];
}

function countUnescaped(text: string, token: string): number {
  let count = 0;
  for (let index = 0; index < text.length; index++) {
    if (text[index] === "\\") {
      index++;
      continue;
    }
    if (text.startsWith(token, index)) {
      count++;
      index += token.length - 1;
    }
  }
  return count;
}

function hasBalancedInlineMarkdown(text: string): boolean {
  if (countUnescaped(text, "$") % 2 !== 0) return false;
  if (countUnescaped(text, "`") % 2 !== 0) return false;

  const prose = text.replace(/\$[^$]*\$/g, "").replace(/`[^`]*`/g, "");
  let strong = 0;
  let emphasis = 0;
  let strongUnderscore = 0;
  let underscore = 0;
  for (let index = 0; index < prose.length; index++) {
    if (prose[index] === "\\") {
      index++;
      continue;
    }
    if (prose.startsWith("**", index)) {
      strong++;
      index++;
      continue;
    }
    if (prose.startsWith("__", index)) {
      strongUnderscore++;
      index++;
      continue;
    }
    if (prose[index] === "*") {
      emphasis++;
    } else if (prose[index] === "_") {
      underscore++;
    }
  }
  return (
    strong % 2 === 0 && emphasis % 2 === 0 && strongUnderscore % 2 === 0 && underscore % 2 === 0
  );
}

export function toStoryMarkdown(content: string): string {
  const trimmed = content.trim();
  if (!trimmed) return "";
  if (trimmed.includes("\n")) return trimmed;

  const paragraphs: string[] = [];
  let buffer = "";
  for (const chunk of toParagraphs(trimmed)) {
    buffer = buffer ? `${buffer} ${chunk}` : chunk;
    if (hasBalancedInlineMarkdown(buffer)) {
      paragraphs.push(buffer);
      buffer = "";
    }
  }
  if (buffer) paragraphs.push(buffer);
  return paragraphs.join("\n\n");
}
