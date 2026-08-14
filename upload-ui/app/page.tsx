"use client";

import {
  AlertTriangle,
  ArrowRight,
  Check,
  CheckCircle2,
  ChevronRight,
  Clock,
  Code2,
  Copy,
  Cpu,
  Download,
  Eye,
  FileCheck,
  FileCode,
  FileText,
  Github,
  Layers,
  Loader2,
  Lock,
  RefreshCw,
  RotateCcw,
  Server,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Sliders,
  Sparkles,
  Terminal,
  Upload,
  UploadCloud,
  X,
  Zap,
} from "lucide-react";
import { ChangeEvent, DragEvent, useEffect, useState } from "react";
import { formatBytes, getEstimatedProcessingTime, isDocxFileName, redactedFilename } from "./lib/files";
import { EXAMPLE_DOCUMENT_DIFF, PII_CATEGORIES } from "./lib/categories";

type UploadState = "idle" | "ready" | "uploading" | "done" | "error";
type ActiveTab = "detectors" | "diff" | "architecture" | "api";

const STAGES = [
  {
    id: "unpack",
    title: "Unpacking OpenXML Archive",
    desc: "Extracting XML text runs, document parts, and embedded media",
    durationMs: 15000,
  },
  {
    id: "structured",
    title: "Structured Algorithmic Validation",
    desc: "Scanning SSNs (SSA rules), Cards (Luhn), Phones (libphonenumber), IPs & DOBs",
    durationMs: 25000,
  },
  {
    id: "ner",
    title: "Presidio & spaCy NER Model",
    desc: "Contextual multi-token detection for Person Names, Orgs & Addresses",
    durationMs: 35000,
  },
  {
    id: "ocr",
    title: "OCR Visual Masking Pipeline",
    desc: "Extracting image text & applying pixel-level blackout bounding boxes",
    durationMs: 35000,
  },
  {
    id: "faker",
    title: "Deterministic Pseudonymization",
    desc: "Applying thread-safe consistent synthetic substitutions across pages",
    durationMs: 25000,
  },
  {
    id: "repack",
    title: "Document Reassembly & Byte Splicing",
    desc: "Rebuilding DOCX with 100% formatting, shading, and table integrity",
    durationMs: 15000,
  },
];

export default function Home() {
  const [file, setFile] = useState<File | null>(null);
  const [state, setState] = useState<UploadState>("idle");
  const [error, setError] = useState<string>("");
  const [downloadUrl, setDownloadUrl] = useState<string>("");
  const [downloadName, setDownloadName] = useState<string>("redacted.docx");
  const [isDragging, setIsDragging] = useState(false);
  const [activeTab, setActiveTab] = useState<ActiveTab>("detectors");
  const [diffViewMode, setDiffViewMode] = useState<"redacted" | "original">("redacted");

  // Options toggles
  const [useDeterministicFaker, setUseDeterministicFaker] = useState(true);
  const [enableOCRImages, setEnableOCRImages] = useState(true);
  const [enableNER, setEnableNER] = useState(true);

  // Dynamic Gateway URL from env / config
  const [gatewayUrl, setGatewayUrl] = useState<string>(
    process.env.NEXT_PUBLIC_GO_SERVICE_URL || "http://localhost:8080"
  );

  useEffect(() => {
    fetch("/api/config")
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (data?.serviceUrl) {
          setGatewayUrl(data.serviceUrl);
        }
      })
      .catch(() => {});
  }, []);

  // Live stage tracking during upload
  const [currentStageIndex, setCurrentStageIndex] = useState(0);
  const [secondsElapsed, setSecondsElapsed] = useState(0);

  // Copy feedback states
  const [copiedFilename, setCopiedFilename] = useState(false);
  const [copiedCurl, setCopiedCurl] = useState(false);

  // Timer for elapsed seconds
  useEffect(() => {
    let interval: NodeJS.Timeout;
    if (state === "uploading") {
      interval = setInterval(() => {
        setSecondsElapsed((prev) => prev + 1);
      }, 1000);
    }
    return () => clearInterval(interval);
  }, [state]);

  // Stage advancement simulation during upload
  useEffect(() => {
    if (state !== "uploading") {
      setCurrentStageIndex(0);
      setSecondsElapsed(0);
      return;
    }

    let accumulatedTime = 0;
    const timeouts: NodeJS.Timeout[] = [];

    STAGES.forEach((stage, index) => {
      accumulatedTime += stage.durationMs;
      const timeout = setTimeout(() => {
        setCurrentStageIndex((prev) => Math.max(prev, index));
      }, accumulatedTime);
      timeouts.push(timeout);
    });

    return () => {
      timeouts.forEach((t) => clearTimeout(t));
    };
  }, [state]);

  function chooseFile(nextFile: File | null) {
    if (downloadUrl) {
      URL.revokeObjectURL(downloadUrl);
    }
    setDownloadUrl("");
    setError("");

    if (!nextFile) {
      setFile(null);
      setState("idle");
      return;
    }

    if (!isDocxFileName(nextFile.name)) {
      setFile(null);
      setState("error");
      setError("Please select a valid Microsoft Word (.docx) document.");
      return;
    }

    setFile(nextFile);
    setState("ready");
  }

  function onDrop(e: DragEvent<HTMLElement>) {
    e.preventDefault();
    setIsDragging(false);
    chooseFile(e.dataTransfer.files?.[0] ?? null);
  }

  function onFileChange(e: ChangeEvent<HTMLInputElement>) {
    chooseFile(e.target.files?.[0] ?? null);
  }

  async function handleStartRedaction() {
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
        throw new Error(
          payload?.error ||
            "The Go redaction engine is unreachable or encountered an error."
        );
      }

      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const filename = redactedFilename(file.name);
      setDownloadUrl(url);
      setDownloadName(filename);
      setState("done");
    } catch (err) {
      setState("error");
      setError(
        err instanceof Error
          ? err.message
          : "An unexpected error occurred during redaction."
      );
    }
  }

  const formatTimer = (totalSecs: number) => {
    const m = Math.floor(totalSecs / 60);
    const s = totalSecs % 60;
    return `${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
  };

  const progressPercent = Math.min(
    96,
    Math.max(
      4,
      Math.floor((secondsElapsed / 150) * 100)
    )
  );

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100 selection:bg-blue-500/30">
      {/* Top Ambient Glow */}
      <div className="pointer-events-none fixed inset-0 z-0 overflow-hidden">
        <div className="absolute -top-40 left-1/2 h-[500px] w-[800px] -translate-x-1/2 rounded-full bg-gradient-to-b from-blue-600/10 via-indigo-600/5 to-transparent blur-3xl" />
        <div className="absolute top-[400px] -left-40 h-[400px] w-[400px] rounded-full bg-emerald-600/5 blur-3xl" />
      </div>

      {/* Header */}
      <header className="sticky top-0 z-50 border-b border-zinc-800/80 bg-zinc-950/80 backdrop-blur-xl">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-3.5 sm:px-6">
          <a href="#" className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-blue-500 to-indigo-600 text-white shadow-lg shadow-blue-500/20">
              <ShieldCheck className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="text-base font-bold tracking-tight text-white">
                  PII Redactor
                </span>
                <span className="rounded-md border border-blue-500/30 bg-blue-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-blue-400">
                  DOCX Engine
                </span>
              </div>
            </div>
          </a>

          {/* Desktop Nav */}
          <nav className="hidden items-center gap-1 md:flex">
            <button
              onClick={() => setActiveTab("detectors")}
              className={`rounded-lg px-3 py-1.5 text-xs font-medium transition ${
                activeTab === "detectors"
                  ? "bg-zinc-800 text-white"
                  : "text-zinc-400 hover:text-zinc-200"
              }`}
            >
              10 Detectors
            </button>
            <button
              onClick={() => setActiveTab("diff")}
              className={`rounded-lg px-3 py-1.5 text-xs font-medium transition ${
                activeTab === "diff"
                  ? "bg-zinc-800 text-white"
                  : "text-zinc-400 hover:text-zinc-200"
              }`}
            >
              Before &amp; After
            </button>
            <button
              onClick={() => setActiveTab("architecture")}
              className={`rounded-lg px-3 py-1.5 text-xs font-medium transition ${
                activeTab === "architecture"
                  ? "bg-zinc-800 text-white"
                  : "text-zinc-400 hover:text-zinc-200"
              }`}
            >
              Architecture
            </button>
            <button
              onClick={() => setActiveTab("api")}
              className={`rounded-lg px-3 py-1.5 text-xs font-medium transition ${
                activeTab === "api"
                  ? "bg-zinc-800 text-white"
                  : "text-zinc-400 hover:text-zinc-200"
              }`}
            >
              API &amp; CLI
            </button>
          </nav>

          {/* Right Status Tag & Link */}
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2 rounded-full border border-zinc-800 bg-zinc-900/90 px-3 py-1 text-xs font-medium text-zinc-300">
              <span className="h-2 w-2 rounded-full bg-emerald-400 shadow-[0_0_8px_#34d399]" />
              <span>Engine Gateway</span>
            </div>

            <a
              href="https://github.com/kaizakin/PII-redactor"
              target="_blank"
              rel="noreferrer"
              className="flex h-8 w-8 items-center justify-center rounded-lg border border-zinc-800 bg-zinc-900 text-zinc-400 transition hover:border-zinc-700 hover:text-white"
              aria-label="GitHub Repository"
            >
              <Github className="h-4 w-4" />
            </a>
          </div>
        </div>
      </header>

      {/* Main Container */}
      <main className="relative z-10 mx-auto flex max-w-5xl flex-col gap-12 px-4 py-10 sm:px-6 sm:py-16">
        {/* Hero Section */}
        <section className="flex flex-col items-center gap-4 text-center">
          <div className="inline-flex items-center gap-2 rounded-full border border-blue-500/20 bg-blue-500/10 px-3.5 py-1 text-xs font-semibold text-blue-400">
            <Shield className="h-3.5 w-3.5" />
            <span>Deterministic Word Document Sanitization</span>
          </div>

          <h1 className="max-w-3xl text-3xl font-extrabold tracking-tight text-white sm:text-5xl sm:leading-[1.15]">
            Zero-Knowledge PII Redaction for Microsoft Word
          </h1>

          <p className="max-w-2xl text-sm leading-relaxed text-zinc-400 sm:text-base">
            Sanitize sensitive data across structured text, contextual NLP entities, and
            embedded image scans without losing Word formatting, tables, shading, or custom fonts.
          </p>

          {/* Metric Badges */}
          <div className="mt-2 flex flex-wrap justify-center gap-2.5 sm:gap-3">
            <div className="flex items-center gap-2 rounded-lg border border-zinc-800/80 bg-zinc-900/60 px-3 py-1.5 text-xs font-medium text-zinc-300">
              <Check className="h-3.5 w-3.5 text-emerald-400" />
              <span>10 PII Categories</span>
            </div>
            <div className="flex items-center gap-2 rounded-lg border border-zinc-800/80 bg-zinc-900/60 px-3 py-1.5 text-xs font-medium text-zinc-300">
              <FileCode className="h-3.5 w-3.5 text-blue-400" />
              <span>Direct XML Byte Splicing</span>
            </div>
            <div className="flex items-center gap-2 rounded-lg border border-zinc-800/80 bg-zinc-900/60 px-3 py-1.5 text-xs font-medium text-zinc-300">
              <Lock className="h-3.5 w-3.5 text-purple-400" />
              <span>0-Byte Ephemeral RAM Stream</span>
            </div>
            <div className="flex items-center gap-2 rounded-lg border border-zinc-800/80 bg-zinc-900/60 px-3 py-1.5 text-xs font-medium text-zinc-300">
              <Zap className="h-3.5 w-3.5 text-amber-400" />
              <span>Luhn &amp; SSA Validated</span>
            </div>
          </div>
        </section>

        {/* Upload & Redactor Interactive Workspace */}
        <section className="relative mx-auto w-full max-w-3xl">
          <div className="relative overflow-hidden rounded-2xl border border-zinc-800 bg-zinc-900/70 p-5 shadow-2xl backdrop-blur-xl sm:p-7">
            {/* Top Workspace Header */}
            <div className="mb-5 flex items-center justify-between border-b border-zinc-800/80 pb-4">
              <div className="flex items-center gap-2.5">
                <FileCheck className="h-5 w-5 text-blue-400" />
                <h2 className="text-sm font-bold text-white sm:text-base">
                  Document Redaction Workspace
                </h2>
              </div>
              <span className="rounded-md border border-zinc-800 bg-zinc-950 px-2.5 py-1 font-mono text-[11px] font-medium text-zinc-400">
                OPENXML .DOCX
              </span>
            </div>

            {/* State: Uploading */}
            {state === "uploading" && file ? (
              <div className="flex flex-col gap-5 py-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-blue-500/10 text-blue-400">
                      <Loader2 className="h-5 w-5 animate-spin" />
                    </div>
                    <div>
                      <div className="text-sm font-bold text-white">
                        Redacting {file.name}
                      </div>
                      <div className="text-xs text-zinc-400">
                        In-memory stream processing &bull; Ephemeral buffers
                      </div>
                    </div>
                  </div>

                  <div className="rounded-lg border border-zinc-800 bg-zinc-950 px-3 py-1 font-mono text-xs font-semibold text-blue-400">
                    {formatTimer(secondsElapsed)} ELAPSED
                  </div>
                </div>

                {/* 2-3 Minutes Notice Banner */}
                <div className="flex items-center gap-2.5 rounded-xl border border-blue-500/30 bg-blue-500/10 px-3.5 py-2.5 text-xs text-blue-200">
                  <Clock className="h-4 w-4 shrink-0 text-blue-400" />
                  <span>
                    <strong>Estimated time: 2–3 minutes.</strong> Please keep this tab open while the engine executes deep spaCy NER and OCR image filters.
                  </span>
                </div>

                {/* Progress Bar */}
                <div className="h-2 w-full overflow-hidden rounded-full bg-zinc-950">
                  <div
                    className="h-full rounded-full bg-gradient-to-r from-blue-500 via-indigo-500 to-emerald-400 transition-all duration-300"
                    style={{ width: `${progressPercent}%` }}
                  />
                </div>

                {/* Stage Progression Checklist */}
                <div className="grid gap-2 pt-1">
                  {STAGES.map((stage, idx) => {
                    const isDone = idx < currentStageIndex;
                    const isActive = idx === currentStageIndex;

                    return (
                      <div
                        key={stage.id}
                        className={`flex items-center justify-between rounded-xl border px-3.5 py-2 text-xs transition-all ${
                          isDone
                            ? "border-emerald-500/20 bg-emerald-500/5 text-zinc-300"
                            : isActive
                            ? "border-blue-500/30 bg-blue-500/10 text-white shadow-sm"
                            : "border-zinc-800/60 bg-zinc-950/40 text-zinc-500"
                        }`}
                      >
                        <div className="flex items-center gap-2.5">
                          <div
                            className={`flex h-5 w-5 items-center justify-center rounded-full text-[10px] font-bold ${
                              isDone
                                ? "bg-emerald-500 text-zinc-950"
                                : isActive
                                ? "bg-blue-500 text-white"
                                : "border border-zinc-800 bg-zinc-900 text-zinc-500"
                            }`}
                          >
                            {isDone ? <Check className="h-3 w-3 stroke-[3]" /> : idx + 1}
                          </div>
                          <div>
                            <span className="font-semibold">{stage.title}</span>
                            <span className="hidden text-[11px] text-zinc-400 sm:inline sm:pl-2">
                              &bull; {stage.desc}
                            </span>
                          </div>
                        </div>

                        <span
                          className={`font-mono text-[10px] font-bold ${
                            isDone
                              ? "text-emerald-400"
                              : isActive
                              ? "text-blue-400"
                              : "text-zinc-600"
                          }`}
                        >
                          {isDone ? "DONE" : isActive ? "SCANNING" : "QUEUED"}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>
            ) : state === "done" && downloadUrl && file ? (
              /* State: Done */
              <div className="flex flex-col gap-5 rounded-xl border border-emerald-500/30 bg-emerald-500/5 p-5">
                <div className="flex items-start gap-3.5">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-emerald-500 text-zinc-950 shadow-lg shadow-emerald-500/30">
                    <CheckCircle2 className="h-6 w-6" />
                  </div>
                  <div>
                    <h3 className="text-base font-bold text-white">
                      Document Redacted &amp; Verified
                    </h3>
                    <p className="text-xs text-zinc-300">
                      All PII has been deterministically sanitized. Word formatting, tables, and styles are 100% preserved.
                    </p>
                  </div>
                </div>

                {/* Metadata Table */}
                <div className="grid grid-cols-1 gap-2.5 rounded-xl border border-zinc-800 bg-zinc-950/80 p-3 sm:grid-cols-3">
                  <div>
                    <div className="text-[10px] font-semibold tracking-wider text-zinc-500 uppercase">
                      SOURCE FILE
                    </div>
                    <div className="truncate text-xs font-semibold text-zinc-200" title={file.name}>
                      {file.name}
                    </div>
                  </div>
                  <div>
                    <div className="text-[10px] font-semibold tracking-wider text-zinc-500 uppercase">
                      STRATEGY
                    </div>
                    <div className="text-xs font-semibold text-emerald-400">
                      Deterministic Faker
                    </div>
                  </div>
                  <div>
                    <div className="text-[10px] font-semibold tracking-wider text-zinc-500 uppercase">
                      MEMORY BUFFER
                    </div>
                    <div className="text-xs font-semibold text-cyan-400">
                      Purged (0 B Left)
                    </div>
                  </div>
                </div>

                {/* Download Button */}
                <a
                  href={downloadUrl}
                  download={downloadName}
                  className="flex h-12 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-emerald-500 to-teal-600 text-sm font-bold text-zinc-950 shadow-lg shadow-emerald-500/25 transition hover:brightness-110"
                  id="download-redacted-docx"
                >
                  <Download className="h-4 w-4" />
                  <span>Download Redacted DOCX ({downloadName})</span>
                </a>

                {/* Secondary Actions */}
                <div className="flex items-center justify-between pt-1">
                  <button
                    type="button"
                    onClick={() => {
                      navigator.clipboard.writeText(downloadName);
                      setCopiedFilename(true);
                      setTimeout(() => setCopiedFilename(false), 2000);
                    }}
                    className="flex items-center gap-1.5 text-xs text-zinc-400 transition hover:text-white"
                  >
                    {copiedFilename ? (
                      <>
                        <Check className="h-3.5 w-3.5 text-emerald-400" />
                        <span className="text-emerald-400">Copied filename!</span>
                      </>
                    ) : (
                      <>
                        <Copy className="h-3.5 w-3.5" />
                        <span>Copy filename</span>
                      </>
                    )}
                  </button>

                  <button
                    type="button"
                    onClick={() => chooseFile(null)}
                    className="flex items-center gap-1.5 text-xs text-zinc-400 transition hover:text-white"
                  >
                    <RotateCcw className="h-3.5 w-3.5" />
                    <span>Redact another document</span>
                  </button>
                </div>
              </div>
            ) : (
              /* State: Idle / Ready */
              <div className="flex flex-col gap-4">
                {!file ? (
                  <label
                    htmlFor="docx-file-input-main"
                    className={`flex flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed p-10 text-center transition-all cursor-pointer select-none ${
                      isDragging
                        ? "border-blue-500 bg-blue-500/10"
                        : "border-zinc-800 bg-zinc-950/60 hover:border-zinc-700 hover:bg-zinc-950/90"
                    }`}
                    onDragOver={(e) => {
                      e.preventDefault();
                      setIsDragging(true);
                    }}
                    onDragLeave={() => setIsDragging(false)}
                    onDrop={onDrop}
                    tabIndex={0}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        document.getElementById("docx-file-input-main")?.click();
                      }
                    }}
                    aria-label="Upload Microsoft Word DOCX document"
                  >
                    <input
                      type="file"
                      accept=".docx"
                      className="hidden"
                      id="docx-file-input-main"
                      onChange={onFileChange}
                    />

                    <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-blue-500/10 text-blue-400">
                      <UploadCloud className="h-6 w-6" />
                    </div>

                    <div>
                      <span className="text-sm font-bold text-white hover:underline">
                        Choose a Word document
                      </span>
                      <span className="text-sm text-zinc-400"> or drag and drop</span>
                      <p className="mt-1 text-xs text-zinc-500">
                        Supports Microsoft Word (.docx) packages up to 50MB
                      </p>
                    </div>

                    <div className="mt-2 flex flex-wrap justify-center gap-2">
                      <span className="rounded-md border border-zinc-800 bg-zinc-900 px-2.5 py-1 text-[11px] text-zinc-400">
                        XML Text Runs
                      </span>
                      <span className="rounded-md border border-zinc-800 bg-zinc-900 px-2.5 py-1 text-[11px] text-zinc-400">
                        OCR Embedded Images
                      </span>
                      <span className="rounded-md border border-zinc-800 bg-zinc-900 px-2.5 py-1 text-[11px] text-zinc-400">
                        Formatting Intact
                      </span>
                    </div>
                  </label>
                ) : (
                  <>
                    {/* Selected File Card */}
                    <div className="flex flex-col gap-4 rounded-xl border border-zinc-800 bg-zinc-950/80 p-4">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3 min-w-0">
                          <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-blue-500/10 text-blue-400">
                            <FileText className="h-6 w-6" />
                          </div>
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <span className="truncate text-sm font-bold text-white" title={file.name}>
                                {file.name}
                              </span>
                              <span className="shrink-0 rounded-md border border-emerald-500/30 bg-emerald-500/10 px-1.5 py-0.5 text-[10px] font-bold text-emerald-400">
                                DOCX VALID
                              </span>
                            </div>
                            <div className="flex items-center gap-2 text-xs text-zinc-400">
                              <span>{formatBytes(file.size)}</span>
                              <span>&bull;</span>
                              <span>Est. scan: {getEstimatedProcessingTime(file.size)}</span>
                            </div>
                          </div>
                        </div>

                        <button
                          type="button"
                          onClick={() => chooseFile(null)}
                          className="flex h-8 w-8 items-center justify-center rounded-lg border border-zinc-800 bg-zinc-900 text-zinc-400 transition hover:border-red-500/30 hover:bg-red-500/10 hover:text-red-400"
                          title="Remove file"
                        >
                          <X className="h-4 w-4" />
                        </button>
                      </div>

                      {/* Engine Parameters */}
                      <div className="rounded-lg border border-zinc-800/80 bg-zinc-900/60 p-3">
                        <div className="mb-2 flex items-center justify-between text-[11px] font-semibold tracking-wider text-zinc-400 uppercase">
                          <span>Engine Parameters</span>
                          <Sliders className="h-3.5 w-3.5" />
                        </div>
                        <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                          <label className="flex items-center gap-2 text-xs text-zinc-300 cursor-pointer">
                            <input
                              type="checkbox"
                              checked={useDeterministicFaker}
                              onChange={(e) => setUseDeterministicFaker(e.target.checked)}
                              className="rounded border-zinc-700 bg-zinc-950 text-blue-500 focus:ring-0"
                            />
                            <span>Deterministic Fakes</span>
                          </label>
                          <label className="flex items-center gap-2 text-xs text-zinc-300 cursor-pointer">
                            <input
                              type="checkbox"
                              checked={enableOCRImages}
                              onChange={(e) => setEnableOCRImages(e.target.checked)}
                              className="rounded border-zinc-700 bg-zinc-950 text-blue-500 focus:ring-0"
                            />
                            <span>OCR Image Masking</span>
                          </label>
                          <label className="flex items-center gap-2 text-xs text-zinc-300 cursor-pointer">
                            <input
                              type="checkbox"
                              checked={enableNER}
                              onChange={(e) => setEnableNER(e.target.checked)}
                              className="rounded border-zinc-700 bg-zinc-950 text-blue-500 focus:ring-0"
                            />
                            <span>spaCy + Presidio NER</span>
                          </label>
                        </div>
                      </div>
                    </div>

                    {/* Start Action Button */}
                    <button
                      type="button"
                      onClick={handleStartRedaction}
                      className="flex h-12 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-blue-600 to-indigo-600 text-sm font-bold text-white shadow-lg shadow-blue-500/25 transition hover:brightness-110"
                      id="start-redaction-btn"
                    >
                      <ShieldCheck className="h-5 w-5" />
                      <span>Start Redaction Process</span>
                    </button>
                  </>
                )}

                {/* Error Banner */}
                {state === "error" && error ? (
                  <div className="flex flex-col gap-2 rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-xs text-red-200">
                    <div className="flex items-center gap-2 font-bold text-red-400">
                      <AlertTriangle className="h-4 w-4 shrink-0" />
                      <span>{error}</span>
                    </div>
                    <p className="text-zinc-300">
                      Please ensure the Go backend service is running on <code>{gatewayUrl}</code> (<code>go run cmd/main.go</code>).
                    </p>
                  </div>
                ) : null}
              </div>
            )}
          </div>
        </section>

        {/* Tabbed Feature Explorer */}
        <section className="flex flex-col gap-6 pt-4">
          {/* Tab Controls */}
          <div className="flex flex-wrap items-center justify-center gap-2 border-b border-zinc-800/80 pb-4">
            <button
              type="button"
              onClick={() => setActiveTab("detectors")}
              className={`flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-bold transition sm:text-sm ${
                activeTab === "detectors"
                  ? "border border-blue-500/40 bg-blue-500/10 text-blue-400 shadow-sm"
                  : "text-zinc-400 hover:text-white hover:bg-zinc-900"
              }`}
            >
              <Cpu className="h-4 w-4" />
              <span>10 Supported PII Detectors</span>
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("diff")}
              className={`flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-bold transition sm:text-sm ${
                activeTab === "diff"
                  ? "border border-blue-500/40 bg-blue-500/10 text-blue-400 shadow-sm"
                  : "text-zinc-400 hover:text-white hover:bg-zinc-900"
              }`}
            >
              <Eye className="h-4 w-4" />
              <span>Before &amp; After Diff</span>
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("architecture")}
              className={`flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-bold transition sm:text-sm ${
                activeTab === "architecture"
                  ? "border border-blue-500/40 bg-blue-500/10 text-blue-400 shadow-sm"
                  : "text-zinc-400 hover:text-white hover:bg-zinc-900"
              }`}
            >
              <Layers className="h-4 w-4" />
              <span>Architecture &amp; Privacy</span>
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("api")}
              className={`flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-bold transition sm:text-sm ${
                activeTab === "api"
                  ? "border border-blue-500/40 bg-blue-500/10 text-blue-400 shadow-sm"
                  : "text-zinc-400 hover:text-white hover:bg-zinc-900"
              }`}
            >
              <Terminal className="h-4 w-4" />
              <span>API Gateway</span>
            </button>
          </div>

          {/* TAB 1: 10 Detectors Matrix */}
          {activeTab === "detectors" && (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {PII_CATEGORIES.map((cat) => (
                <div
                  key={cat.id}
                  className="flex flex-col justify-between rounded-2xl border border-zinc-800/80 bg-zinc-900/60 p-5 transition hover:border-zinc-700 hover:bg-zinc-900/90"
                >
                  <div className="flex flex-col gap-2.5">
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-bold text-white">{cat.name}</span>
                      <span
                        className={`rounded-md px-2 py-0.5 text-[10px] font-bold ${
                          cat.pipeline === "Structured (Go)"
                            ? "border border-blue-500/30 bg-blue-500/10 text-blue-400"
                            : cat.pipeline === "NLP (Python)"
                            ? "border border-purple-500/30 bg-purple-500/10 text-purple-400"
                            : "border border-emerald-500/30 bg-emerald-500/10 text-emerald-400"
                        }`}
                      >
                        {cat.pipeline}
                      </span>
                    </div>

                    <p className="text-xs leading-relaxed text-zinc-400">
                      {cat.description}
                    </p>

                    {/* Example Box */}
                    <div className="mt-1 flex flex-col gap-1 rounded-lg border border-zinc-800 bg-zinc-950 p-2.5 font-mono text-[11px]">
                      <div className="flex items-center justify-between">
                        <span className="text-zinc-500">RAW:</span>
                        <span className="text-red-400">{cat.exampleRaw}</span>
                      </div>
                      <div className="flex items-center justify-between">
                        <span className="text-zinc-500">FAKE:</span>
                        <span className="text-emerald-400">{cat.exampleFake}</span>
                      </div>
                    </div>
                  </div>

                  <div className="mt-3 flex items-center justify-between border-t border-zinc-800/80 pt-3 text-[11px] text-zinc-400">
                    <div className="flex items-center gap-1 text-zinc-300">
                      <ShieldCheck className="h-3.5 w-3.5 text-blue-400" />
                      <span>{cat.verification}</span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* TAB 2: Before & After Diff Sandbox */}
          {activeTab === "diff" && (
            <div className="flex flex-col gap-4 rounded-2xl border border-zinc-800 bg-zinc-900/60 p-5 sm:p-6">
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-zinc-800/80 pb-4">
                <div>
                  <h3 className="text-sm font-bold text-white sm:text-base">
                    Document Diff Preview
                  </h3>
                  <p className="text-xs text-zinc-400">
                    Compare original confidential text vs deterministically sanitized output.
                  </p>
                </div>

                <div className="flex items-center rounded-xl border border-zinc-800 bg-zinc-950 p-1">
                  <button
                    type="button"
                    onClick={() => setDiffViewMode("redacted")}
                    className={`flex items-center gap-1.5 rounded-lg px-3 py-1 text-xs font-bold transition ${
                      diffViewMode === "redacted"
                        ? "bg-emerald-500 text-zinc-950 shadow-sm"
                        : "text-zinc-400 hover:text-white"
                    }`}
                  >
                    <ShieldCheck className="h-3.5 w-3.5" />
                    <span>Redacted (Sanitized)</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => setDiffViewMode("original")}
                    className={`flex items-center gap-1.5 rounded-lg px-3 py-1 text-xs font-bold transition ${
                      diffViewMode === "original"
                        ? "bg-red-500 text-white shadow-sm"
                        : "text-zinc-400 hover:text-white"
                    }`}
                  >
                    <Eye className="h-3.5 w-3.5" />
                    <span>Original (Raw PII)</span>
                  </button>
                </div>
              </div>

              {/* Document Sheet Viewer */}
              <div className="flex flex-col gap-2 rounded-xl border border-zinc-800 bg-zinc-950 p-4 font-mono text-xs sm:p-5">
                <div className="mb-2 flex items-center justify-between border-b border-zinc-800 pb-3">
                  <div>
                    <div className="font-bold text-white">{EXAMPLE_DOCUMENT_DIFF.title}</div>
                    <div className="text-[11px] text-zinc-500">{EXAMPLE_DOCUMENT_DIFF.meta}</div>
                  </div>
                  <span
                    className={`rounded-md px-2 py-0.5 text-[10px] font-bold ${
                      diffViewMode === "original"
                        ? "border border-red-500/30 bg-red-500/10 text-red-400"
                        : "border border-emerald-500/30 bg-emerald-500/10 text-emerald-400"
                    }`}
                  >
                    {diffViewMode === "original" ? "CONFIDENTIAL RAW" : "PII SANITIZED"}
                  </span>
                </div>

                {diffViewMode === "original"
                  ? EXAMPLE_DOCUMENT_DIFF.originalLines.map((line, idx) => (
                      <div
                        key={idx}
                        className={`flex items-center justify-between rounded-lg p-2 transition ${
                          line.entity
                            ? "border-l-2 border-red-500 bg-red-500/5 text-zinc-200"
                            : "text-zinc-400"
                        }`}
                      >
                        <div>
                          {line.entity ? (
                            <>
                              <span>{line.text.replace(line.pii || "", "")}</span>
                              <span className="rounded bg-red-500/20 px-1.5 py-0.5 font-bold text-red-300 border border-red-500/30">
                                {line.pii}
                              </span>
                            </>
                          ) : (
                            <span>{line.text}</span>
                          )}
                        </div>
                        {line.entity && (
                          <span className="rounded border border-red-500/30 bg-red-500/10 px-1.5 py-0.5 text-[10px] font-bold text-red-400">
                            {line.entity}
                          </span>
                        )}
                      </div>
                    ))
                  : EXAMPLE_DOCUMENT_DIFF.redactedLines.map((line, idx) => (
                      <div
                        key={idx}
                        className={`flex items-center justify-between rounded-lg p-2 transition ${
                          line.wasRedacted
                            ? "border-l-2 border-emerald-500 bg-emerald-500/5 text-zinc-200"
                            : "text-zinc-400"
                        }`}
                      >
                        <div>
                          {line.wasRedacted ? (
                            <>
                              <span>{line.text.replace(line.pii || "", "")}</span>
                              <span className="rounded bg-emerald-500/20 px-1.5 py-0.5 font-bold text-emerald-300 border border-emerald-500/30">
                                {line.pii}
                              </span>
                            </>
                          ) : (
                            <span>{line.text}</span>
                          )}
                        </div>
                        {line.entity && (
                          <span className="rounded border border-emerald-500/30 bg-emerald-500/10 px-1.5 py-0.5 text-[10px] font-bold text-emerald-400">
                            {line.entity} (SANITIZED)
                          </span>
                        )}
                      </div>
                    ))}
              </div>
            </div>
          )}

          {/* TAB 3: Architecture & Privacy */}
          {activeTab === "architecture" && (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-3 rounded-2xl border border-zinc-800/80 bg-zinc-900/60 p-5">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-blue-500/10 text-blue-400">
                    <FileCode className="h-5 w-5" />
                  </div>
                  <div>
                    <h4 className="text-sm font-bold text-white">Direct OpenXML Byte Splicing</h4>
                    <span className="text-[11px] font-mono text-zinc-500">internal/docxio/xmltext.go</span>
                  </div>
                </div>
                <p className="text-xs leading-relaxed text-zinc-400">
                  Finds exact byte offsets of <code>&lt;w:t&gt;</code> elements via XML tokenization and splices
                  replacement text directly into the original document bytes. Shading (<code>&lt;w:shd&gt;</code>),
                  tables, custom fonts, colors, bold, and headers survive 100% byte-for-byte.
                </p>
              </div>

              <div className="flex flex-col gap-3 rounded-2xl border border-zinc-800/80 bg-zinc-900/60 p-5">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-purple-500/10 text-purple-400">
                    <Lock className="h-5 w-5" />
                  </div>
                  <div>
                    <h4 className="text-sm font-bold text-white">Deterministic Thread-Safe Faker</h4>
                    <span className="text-[11px] font-mono text-zinc-500">internal/faker</span>
                  </div>
                </div>
                <p className="text-xs leading-relaxed text-zinc-400">
                  Generates realistic synthetic replacements from a thread-safe cache. The same person name, SSN,
                  or email encountered across 50 pages consistently receives the exact same fake entity throughout.
                </p>
              </div>

              <div className="flex flex-col gap-3 rounded-2xl border border-zinc-800/80 bg-zinc-900/60 p-5">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-500/10 text-emerald-400">
                    <Zap className="h-5 w-5" />
                  </div>
                  <div>
                    <h4 className="text-sm font-bold text-white">Zero-Persistence RAM Stream</h4>
                    <span className="text-[11px] font-mono text-zinc-500">internal/api</span>
                  </div>
                </div>
                <p className="text-xs leading-relaxed text-zinc-400">
                  All DOCX files are processed in ephemeral memory buffers. No unredacted fragments or output
                  documents are ever written to permanent database tables or disk storage.
                </p>
              </div>

              <div className="flex flex-col gap-3 rounded-2xl border border-zinc-800/80 bg-zinc-900/60 p-5">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-amber-500/10 text-amber-400">
                    <Layers className="h-5 w-5" />
                  </div>
                  <div>
                    <h4 className="text-sm font-bold text-white">Vision OCR &amp; Blackout Masking</h4>
                    <span className="text-[11px] font-mono text-zinc-500">python-worker/image_redactor.py</span>
                  </div>
                </div>
                <p className="text-xs leading-relaxed text-zinc-400">
                  Extracts embedded PNG/JPEG media from the ZIP container, runs Tesseract character recognition,
                  burns black bounding box masks into sensitive regions, and re-splices into the Word package.
                </p>
              </div>
            </div>
          )}

          {/* TAB 4: API Gateway */}
          {activeTab === "api" && (
            <div className="flex flex-col gap-4 rounded-2xl border border-zinc-800 bg-zinc-900/60 p-5 sm:p-6">
              <div>
                <h3 className="text-sm font-bold text-white sm:text-base">
                  Programmatic API Gateway Access
                </h3>
                <p className="text-xs text-zinc-400">
                  Submit Word documents programmatically to the Go API gateway.
                </p>
              </div>

              {/* cURL Example */}
              <div className="relative rounded-xl border border-zinc-800 bg-zinc-950 p-4 font-mono text-xs text-blue-300">
                <button
                  type="button"
                  onClick={() => {
                    navigator.clipboard.writeText(
                      `curl -X POST ${gatewayUrl}/redact/docx -F 'file=@patient_record.docx' -o redacted.docx`
                    );
                    setCopiedCurl(true);
                    setTimeout(() => setCopiedCurl(false), 2000);
                  }}
                  className="absolute right-3 top-3 flex items-center gap-1 rounded-md border border-zinc-800 bg-zinc-900 px-2 py-1 text-[11px] text-zinc-400 transition hover:text-white"
                >
                  {copiedCurl ? <Check className="h-3 w-3 text-emerald-400" /> : <Copy className="h-3 w-3" />}
                  <span>{copiedCurl ? "Copied!" : "Copy"}</span>
                </button>
                <code>
                  curl -X POST {gatewayUrl}/redact/docx \<br />
                  &nbsp;&nbsp;-F &apos;file=@patient_record.docx&apos; \<br />
                  &nbsp;&nbsp;-o redacted.docx
                </code>
              </div>
            </div>
          )}
        </section>
      </main>

      {/* Footer */}
      <footer className="border-t border-zinc-800/80 bg-zinc-950 py-8 text-center text-xs text-zinc-500">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-4 px-4 sm:px-6">
          <div className="flex items-center gap-2 text-zinc-400">
            <ShieldCheck className="h-4 w-4 text-blue-500" />
            <span className="font-semibold text-zinc-300">PII Redactor</span>
            <span>&bull;</span>
            <span>Zero-Knowledge DOCX Sanitizer</span>
          </div>

          <div className="flex items-center gap-4 text-zinc-400">
            <span>Go Engine</span>
            <span>&bull;</span>
            <span>spaCy NER</span>
            <span>&bull;</span>
            <span>Tesseract OCR</span>
            <span>&bull;</span>
            <a
              href="https://github.com/kaizakin/PII-redactor"
              target="_blank"
              rel="noreferrer"
              className="text-zinc-300 hover:text-white transition"
            >
              GitHub Repository
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
