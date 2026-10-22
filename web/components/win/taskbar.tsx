"use client";

import { List, TaskBar } from "@react95/core";
import { FileFind, FileText, Folder, HelpBook } from "@react95/icons";
import { useRouter } from "next/navigation";

import { DOCS_URL } from "@/lib/site";

export function DesktopTaskbar() {
  const router = useRouter();

  return (
    <TaskBar
      list={
        <List>
          <List.Item icon={<FileText variant="32x32_4" />} onClick={() => router.push("/")}>
            今日故事
          </List.Item>
          <List.Item icon={<Folder variant="32x32_4" />} onClick={() => router.push("/archive")}>
            归档
          </List.Item>
          <List.Item icon={<FileFind variant="32x32_4" />} onClick={() => router.push("/about")}>
            关于
          </List.Item>
          <List.Divider />
          <List.Item
            icon={<HelpBook variant="32x32_4" />}
            onClick={() => window.open(DOCS_URL, "_blank", "noopener")}
          >
            API 文档
          </List.Item>
        </List>
      }
    />
  );
}
