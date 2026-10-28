# Fusion Pixel source font

`fusion-pixel-source.woff2` is the Simplified Chinese 12px proportional font from
[TakWolf/fusion-pixel-font release 2026.08.11](https://github.com/TakWolf/fusion-pixel-font/releases/tag/2026.08.11),
original filename `fusion-pixel-12px-proportional-zh_hans.otf.woff2`.

The source and its component fonts are distributed under the SIL Open Font License.
Their notices are preserved in `../public/licenses/fusion-pixel/` and shipped with the site.

`bun run prepare:assets` uses subset-font (HarfBuzz) to generate two disjoint subsets:
ASCII/UI characters, preloaded through `next/font/local`, and remaining characters
from the Git-tracked stories. Generated files in `app/fonts/` are ignored by Git.
Every build regenerates them, including after a story update; arbitrary external
content can fall back to system fonts. Run `bun run dev` or `bun run build` normally.

`app/react95-global.css` preserves React95 10.0.6's reset and classic scrollbars,
while using these subsets instead of embedding the unused MS Sans Serif TTFs.

The classic Mac frontend's `chicago-flf-source.ttf` is reused for Latin UI text.
Its original public-domain notice is preserved in
`../public/licenses/chicagoflf/README.ChicagoFLF.txt`. The asset build generates
`app/fonts/chicago.woff2`; Chinese falls back to Fusion Pixel and long-form
reading retains the existing serif font stack.
