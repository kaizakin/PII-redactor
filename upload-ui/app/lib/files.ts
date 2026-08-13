export function isDocxFileName(name: string) {
  return name.toLowerCase().endsWith(".docx");
}

export function redactedFilename(originalName: string) {
  const safeName = originalName.replace(/[^\w.-]+/g, "_");
  return isDocxFileName(safeName)
    ? safeName.replace(/\.docx$/i, "-redacted.docx")
    : "redacted.docx";
}
