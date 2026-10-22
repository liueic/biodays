import { Frame } from "@react95/core";
import Link from "next/link";

/** Tag chip linking to the archive filtered by this tag. */
export function Tag({ children }: { children: React.ReactNode }) {
  const name = String(children);
  return (
    <Frame as="li" boxShadow="$out" bgColor="$material">
      <Link
        href={`/archive?tag=${encodeURIComponent(name)}`}
        className="tag-link"
        title={`查看标签「${name}」下的全部故事`}
      >
        {children}
      </Link>
    </Frame>
  );
}
