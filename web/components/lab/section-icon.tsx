import { Bulb } from "@react95/icons/Bulb";
import { FileText } from "@react95/icons/FileText";
import { HelpBook } from "@react95/icons/HelpBook";

import type { LabSectionID } from "@/lib/lab";

export function LabSectionIcon({ section, size = 32 }: { section: LabSectionID; size?: 16 | 32 }) {
  const Icon = section === "methods" ? HelpBook : section === "protocols" ? FileText : Bulb;
  return <Icon variant="32x32_4" width={size} height={size} aria-hidden />;
}
