// One-time, deterministic conversion of the original JSON snapshot. Production
// never reads it. This overwrites stories: use only to reproduce the migration.
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import matter from "gray-matter";

const root = fileURLToPath(new URL("../../", import.meta.url));
const target = path.join(root, "web/content/stories");
await mkdir(target, { recursive: true });
let count = 0;
const originals = JSON.parse(
  await readFile(path.join(root, "web/tests/fixtures/legacy-stories.json"), "utf8"),
);
for (const story of originals) {
  const { content, ...metadata } = story;
  const frontmatter = matter.engines.yaml.stringify(metadata, { lineWidth: -1, noRefs: true });
  await writeFile(path.join(target, `${story.id}.md`), `---\n${frontmatter}---\n${content}\n`);
  count++;
}
console.log(`Converted ${count} stories`);
