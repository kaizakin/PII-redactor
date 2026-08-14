export function isDocxFileName(name: string): boolean {
  return name.toLowerCase().endsWith(".docx");
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}

export function redactedFilename(originalName: string): string {
  const safeName = originalName.replace(/[^\w.-]+/g, "_");
  return isDocxFileName(safeName)
    ? safeName.replace(/\.docx$/i, "-redacted.docx")
    : "redacted.docx";
}

export function getEstimatedProcessingTime(_fileSizeBytes?: number): string {
  return "2–3 mins";
}
