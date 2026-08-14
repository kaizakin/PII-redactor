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

export function getEstimatedProcessingTime(fileSizeBytes: number): string {
  if (fileSizeBytes < 500 * 1024) return "10–25s";
  if (fileSizeBytes < 2 * 1024 * 1024) return "25–45s";
  return "45–90s";
}
