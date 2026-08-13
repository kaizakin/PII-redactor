import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";

const MAX_UPLOAD_BYTES = 64 * 1024 * 1024;

function serviceEndpoint() {
  const baseUrl = process.env.GO_SERVICE_URL;
  if (!baseUrl) {
    throw new Error("GO_SERVICE_URL is not configured");
  }

  return new URL("/redact/docx", baseUrl).toString();
}

function redactedFilename(originalName: string) {
  const safeName = originalName.replace(/[^\w.-]+/g, "_");
  return safeName.toLowerCase().endsWith(".docx")
    ? safeName.replace(/\.docx$/i, "-redacted.docx")
    : "redacted.docx";
}

export async function POST(request: NextRequest) {
  let formData: FormData;

  try {
    formData = await request.formData();
  } catch {
    return NextResponse.json(
      { error: "Upload could not be read. Please try again." },
      { status: 400 },
    );
  }

  const file = formData.get("file");
  if (!(file instanceof File)) {
    return NextResponse.json(
      { error: "Choose a .docx file before uploading." },
      { status: 400 },
    );
  }

  if (!file.name.toLowerCase().endsWith(".docx")) {
    return NextResponse.json(
      { error: "Only .docx files are supported." },
      { status: 400 },
    );
  }

  if (file.size > MAX_UPLOAD_BYTES) {
    return NextResponse.json(
      { error: "File is too large. Please upload a file under 64 MB." },
      { status: 400 },
    );
  }

  const upstreamForm = new FormData();
  upstreamForm.set("file", file, file.name);

  let upstreamResponse: Response;
  try {
    upstreamResponse = await fetch(serviceEndpoint(), {
      method: "POST",
      body: upstreamForm,
    });
  } catch {
    return NextResponse.json(
      { error: "Could not reach the redaction service." },
      { status: 502 },
    );
  }

  if (!upstreamResponse.ok || !upstreamResponse.body) {
    const message = await upstreamResponse.text().catch(() => "");
    return NextResponse.json(
      {
        error:
          message.trim() ||
          "The redaction service could not process this document.",
      },
      { status: upstreamResponse.status || 502 },
    );
  }

  return new NextResponse(upstreamResponse.body, {
    headers: {
      "Content-Type":
        upstreamResponse.headers.get("Content-Type") ||
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "Content-Disposition": `attachment; filename="${redactedFilename(file.name)}"`,
    },
  });
}
