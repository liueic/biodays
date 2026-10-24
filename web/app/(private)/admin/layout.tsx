import { AdminSessionProvider } from "@/components/admin/admin-session";

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return <AdminSessionProvider>{children}</AdminSessionProvider>;
}
