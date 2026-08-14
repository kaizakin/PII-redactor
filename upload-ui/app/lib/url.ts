/**
 * Resolves the configured Go service base URL, tolerating values that are
 * missing a scheme (e.g. "myhost:8080" instead of "http://myhost:8080").
 * Without this, a bare host:port is parsed by URL()/fetch() as an unknown
 * scheme and fails before any network request is made.
 */
export function resolveServiceUrl(): string {
  const raw =
    process.env.GO_SERVICE_URL ||
    process.env.NEXT_PUBLIC_GO_SERVICE_URL ||
    "http://localhost:8080";

  const withScheme = /^[a-zA-Z][a-zA-Z\d+\-.]*:\/\//.test(raw)
    ? raw
    : `http://${raw}`;

  return withScheme.replace(/\/+$/, "");
}
