import { createHash } from "node:crypto";
import { readdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import matter from "gray-matter";

const root = fileURLToPath(new URL("../", import.meta.url));
export async function loadContent() {
  const directory = path.join(root, "content/stories");
  const files = (await readdir(directory)).filter((name) => name.endsWith(".md")).sort();
  const stories = [];
  const ids = new Set();
  for (const name of files) {
    const parsed = matter(await readFile(path.join(directory, name), "utf8"));
    const s = { ...parsed.data, content: parsed.content.replace(/\n$/, "") };
    if (
      typeof s.id !== "string" ||
      !/^[A-Za-z0-9_-]{1,16}$/.test(s.id) ||
      name !== `${s.id}.md` ||
      ids.has(s.id)
    )
      throw new Error(`Invalid/duplicate ID: ${name}`);
    for (const key of ["title", "summary", "content", "lab_item", "geek_quote"])
      if (typeof s[key] !== "string" || !s[key].trim()) throw new Error(`Missing ${key}: ${name}`);
    if (
      !Number.isInteger(s.year) ||
      !Number.isInteger(s.month) ||
      !Number.isInteger(s.day) ||
      s.month < 1 ||
      s.month > 12 ||
      s.day < 1 ||
      s.day > new Date(Date.UTC(2000, s.month, 0)).getUTCDate()
    )
      throw new Error(`Invalid date: ${name}`);
    if (
      !Array.isArray(s.tags) ||
      s.tags.some((t) => typeof t !== "string" || !t.trim()) ||
      new Set(s.tags).size !== s.tags.length
    )
      throw new Error(`Invalid tags: ${name}`);
    if (!s.source || ["type", "citation", "url"].some((key) => typeof s.source[key] !== "string"))
      throw new Error(`Invalid source: ${name}`);
    if (!/^https?:$/.test(new URL(s.source.url).protocol))
      throw new Error(`Invalid source URL: ${name}`);
    ids.add(s.id);
    stories.push(s);
  }
  if (!stories.length) throw new Error("Empty content repository");
  stories.sort(
    (a, b) => a.month - b.month || a.day - b.day || a.year - b.year || a.id.localeCompare(b.id),
  );
  return {
    version: `sha256:${createHash("sha256").update(JSON.stringify(stories)).digest("hex")}`,
    stories,
  };
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const index = await loadContent();
  await writeFile(path.join(root, "content/index.json"), `${JSON.stringify(index)}\n`);
  console.log(`Validated ${index.stories.length} Markdown stories (${index.version})`);
}
