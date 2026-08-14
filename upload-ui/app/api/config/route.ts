import { NextResponse } from "next/server";

export const runtime = "nodejs";

export async function GET() {
  const serviceUrl =
    process.env.GO_SERVICE_URL ||
    process.env.NEXT_PUBLIC_GO_SERVICE_URL ||
    "http://localhost:8080";

  const cleanUrl = serviceUrl.replace(/\/+$/, "");

  return NextResponse.json({
    serviceUrl: cleanUrl,
  });
}
