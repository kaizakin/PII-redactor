import { NextResponse } from "next/server";
import { resolveServiceUrl } from "../../lib/url";

export const runtime = "nodejs";

export async function GET() {
  return NextResponse.json({
    serviceUrl: resolveServiceUrl(),
  });
}
