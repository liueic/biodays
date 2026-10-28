import { createHash } from "node:crypto";
import { readdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import matter from "gray-matter";

const root = fileURLToPath(new URL("../", import.meta.url));

async function getMarkdownFiles(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const res = path.resolve(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await getMarkdownFiles(res)));
    } else if (entry.isFile() && entry.name.endsWith(".md")) {
      files.push(res);
    }
  }
  return files;
}

export async function loadLabContent() {
  const labDirectory = path.join(root, "content/lab");
  const files = (await getMarkdownFiles(labDirectory)).sort();
  const articles = [];
  const ids = new Set();

  for (const filePath of files) {
    const fileName = path.basename(filePath);
    const parsed = matter(await readFile(filePath, "utf8"));
    const a = { ...parsed.data, content: parsed.content.replace(/\n$/, "") };

    if (
      typeof a.id !== "string" ||
      !/^[A-Za-z0-9_-]{1,16}$/.test(a.id) ||
      fileName !== `${a.id}.md` ||
      ids.has(a.id)
    ) {
      throw new Error(`Invalid or duplicate Lab article ID: ${fileName}`);
    }

    if (!["methods", "protocols", "tips"].includes(a.section)) {
      throw new Error(`Invalid section "${a.section}" in ${fileName}`);
    }

    for (const key of ["title", "summary", "content", "lab_item", "geek_quote", "topic"]) {
      if (typeof a[key] !== "string" || !a[key].trim()) {
        throw new Error(`Missing or empty ${key} in Lab article: ${fileName}`);
      }
    }

    if (
      !Array.isArray(a.tags) ||
      a.tags.some((t) => typeof t !== "string" || !t.trim()) ||
      new Set(a.tags).size !== a.tags.length
    ) {
      throw new Error(`Invalid tags in Lab article: ${fileName}`);
    }

    if (!a.source || ["type", "citation", "url"].some((key) => typeof a.source[key] !== "string")) {
      throw new Error(`Invalid source metadata in Lab article: ${fileName}`);
    }

    if (!/^https?:$/.test(new URL(a.source.url).protocol)) {
      throw new Error(`Invalid source URL in Lab article: ${fileName}`);
    }

    ids.add(a.id);
    articles.push(a);
  }

  articles.sort((a, b) => a.id.localeCompare(b.id));

  return {
    version: `sha256:${createHash("sha256").update(JSON.stringify(articles)).digest("hex")}`,
    articles,
  };
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const index = await loadLabContent();
  await writeFile(path.join(root, "content/lab-index.json"), `${JSON.stringify(index, null, 2)}\n`);
  console.log(`Validated ${index.articles.length} Lab articles (${index.version})`);
}
