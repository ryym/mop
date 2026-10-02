// URLs of files next to the document.
//
// The page lives at /doc/<id>, so a relative URL such as "img.png" would
// resolve to the non-existent /doc/img.png instead of the file next to the
// document. Such URLs are pointed at the daemon's file endpoint instead.

// toFileUrl returns the file endpoint URL for a relative URL, or null to leave
// the URL as it is.
export function toFileUrl(url: string, fileEndpoint: string): string | null {
  if (!isRelative(url)) return null;
  // Split before decoding, so that an encoded "#" or "?" in a file name stays
  // part of the path. The fragment is kept for the browser; the query means
  // nothing to a local file and is dropped.
  const hashAt = url.indexOf("#");
  const fragment = hashAt < 0 ? "" : url.slice(hashAt);
  const path = (hashAt < 0 ? url : url.slice(0, hashAt)).split("?")[0]!;
  if (path === "") return null;
  // The URL is percent-encoded already, by markdown-it or by whoever wrote it
  // in HTML, so it is decoded first to avoid encoding it twice. A malformed
  // escape is left for the browser.
  let decoded: string;
  try {
    decoded = decodeURIComponent(path);
  } catch {
    return null;
  }
  // The path goes in the query: browsers collapse ".." in a URL path, even
  // percent-encoded, which would lose "../img.png".
  return `${fileEndpoint}?path=${encodeURIComponent(decoded)}${fragment}`;
}

// isRelative reports whether url is a path relative to the document, as
// opposed to an absolute path, an in-page anchor or an external URL.
function isRelative(url: string): boolean {
  if (url === "" || url.startsWith("/") || url.startsWith("#")) return false;
  // Exclude anything with a scheme (http:, data:, file:) or protocol relative.
  return !/^[a-z][a-z0-9+.-]*:/i.test(url) && !url.startsWith("//");
}
