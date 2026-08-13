import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "PII Redactor",
  description: "Upload a DOCX file and download a redacted copy.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
