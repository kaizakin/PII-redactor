"use client";

import {
  CheckCircle2,
  FileText,
  Loader2,
  ShieldCheck,
  Upload,
  X,
} from "lucide-react";
import { ChangeEvent, DragEvent, FormEvent, useMemo, useState } from "react";

type UploadState = "idle" | "ready" | "uploading" | "done" | "error";

function formatBytes(bytes: number) {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}

export default function Home() {
  const [file, setFile] = useState<File | null>(null);
  const [state, setState] = useState<UploadState>("idle");
  const [error, setError] = useState("");
  const [downloadUrl, setDownloadUrl] = useState("");
  const [downloadName, setDownloadName] = useState("redacted.docx");
  const [isDragging, setIsDragging] = useState(false);

  const canUpload = useMemo(() => file && state !== "uploading", [file, state]);

  function chooseFile(nextFile: File | null) {
    if (downloadUrl) URL.revokeObjectURL(downloadUrl);
    setDownloadUrl("");
    setError("");

    if (!nextFile) {
      setFile(null);
      setState("idle");
      return;
    }

    if (!nextFile.name.toLowerCase().endsWith(".docx")) {
      setFile(null);
      setState("error");
      setError("Please choose a .docx file.");
      return;
    }

    setFile(nextFile);
    setState("ready");
  }

  function onFileChange(event: ChangeEvent<HTMLInputElement>) {
    chooseFile(event.target.files?.[0] ?? null);
  }

  function onDrop(event: DragEvent<HTMLLabelElement>) {
    event.preventDefault();
    setIsDragging(false);
    chooseFile(event.dataTransfer.files?.[0] ?? null);
  }

  async function uploadFile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!file) return;

    setState("uploading");
    setError("");

    const formData = new FormData();
    formData.set("file", file);

    try {
      const response = await fetch("/api/redact", {
        method: "POST",
        body: formData,
      });

      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(payload?.error || "Redaction failed. Please try again.");
      }

      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const filename = file.name.replace(/\.docx$/i, "-redacted.docx");
      setDownloadUrl(url);
      setDownloadName(filename);
      setState("done");
    } catch (err) {
      setState("error");
      setError(err instanceof Error ? err.message : "Something went wrong.");
    }
  }

  return (
    <main className="page-shell">
      <section className="upload-panel" aria-label="Document redaction upload">
        <div className="heading-row">
          <div className="brand-mark" aria-hidden="true">
            <ShieldCheck size={26} />
          </div>
          <div>
            <p className="eyebrow">PII Redactor</p>
            <h1>Upload a DOCX for redaction</h1>
          </div>
        </div>

        <form onSubmit={uploadFile} className="upload-form">
          <label
            className={`drop-zone ${isDragging ? "is-dragging" : ""}`}
            onDragOver={(event) => {
              event.preventDefault();
              setIsDragging(true);
            }}
            onDragLeave={() => setIsDragging(false)}
            onDrop={onDrop}
          >
            <input type="file" accept=".docx" onChange={onFileChange} />
            <span className="drop-icon" aria-hidden="true">
              <Upload size={26} />
            </span>
            <span className="drop-title">Drop your document here</span>
            <span className="drop-copy">or browse for a Microsoft Word .docx file</span>
          </label>

          {file ? (
            <div className="file-row">
              <FileText size={22} aria-hidden="true" />
              <div className="file-meta">
                <span>{file.name}</span>
                <small>{formatBytes(file.size)}</small>
              </div>
              <button
                type="button"
                className="icon-button"
                aria-label="Remove file"
                onClick={() => chooseFile(null)}
                disabled={state === "uploading"}
              >
                <X size={18} />
              </button>
            </div>
          ) : null}

          {state === "uploading" ? (
            <div className="progress-card" role="status" aria-live="polite">
              <div className="scan-animation" aria-hidden="true">
                <span />
              </div>
              <div>
                <strong>Redacting your document</strong>
                <p>Estimated time: 2-3 minutes. Keep this tab open while we process text and images.</p>
              </div>
            </div>
          ) : null}

          {state === "error" && error ? <p className="message error">{error}</p> : null}

          {state === "done" && downloadUrl ? (
            <div className="message success">
              <CheckCircle2 size={20} aria-hidden="true" />
              <span>Redacted file is ready.</span>
              <a href={downloadUrl} download={downloadName}>
                Download
              </a>
            </div>
          ) : null}

          <button className="primary-button" type="submit" disabled={!canUpload}>
            {state === "uploading" ? (
              <>
                <Loader2 className="spin" size={20} />
                Processing
              </>
            ) : (
              <>
                <Upload size={20} />
                Upload and redact
              </>
            )}
          </button>
        </form>
      </section>
    </main>
  );
}
