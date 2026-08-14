import { NextRequest, NextResponse } from "next/server";
import { isDocxFileName, redactedFilename } from "../../lib/files";
import { resolveServiceUrl } from "../../lib/url";

export const runtime = "nodejs";

function serviceEndpoint() {
  return new URL("/redact/docx", resolveServiceUrl()).toString();
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

  if (!isDocxFileName(file.name)) {
    return NextResponse.json(
      { error: "Only .docx files are supported." },
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
