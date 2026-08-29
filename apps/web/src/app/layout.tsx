import type { Metadata } from "next";

import { QueryProvider } from "@/components/providers/query-provider";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "Опора", template: "%s · Опора" },
  description: "Защищённая система сопровождения детей и работы специалистов школы",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ru">
      <body><QueryProvider>{children}</QueryProvider></body>
    </html>
  );
}
