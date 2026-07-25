// A dashboard title is prose ("Fleet Overview: what's live?"); ConfigHub's DisplayName
// is constrained to `^[A-Za-z0-9]([\-_ .|A-Za-z0-9]*[A-Za-z0-9.!?])?$`. Copying the
// title straight into DisplayName means a perfectly legal title fails the save with a
// regex in the error body.
//
// Annotations have no such restriction — any character, up to 1024 bytes — so the exact
// title goes there and nothing is lost. DisplayName gets a reduced form, which is the
// right trade for a field whose job is to read well in `cub unit list`.

/** Longest DisplayName worth sending; the server's own limit is larger. */
const MAX_LENGTH = 120;

/**
 * Best-effort DisplayName for a dashboard title. Returns undefined when nothing legal
 * survives, in which case the caller omits the field rather than failing.
 */
export function toDisplayName(title: string): string | undefined {
  const reduced = title
    // Keep the characters the pattern allows; everything else becomes a space.
    .replace(/[^A-Za-z0-9\-_ .|!?]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, MAX_LENGTH);

  // Must start with alphanumeric and end with alphanumeric or . ! ?
  const trimmed = reduced.replace(/^[^A-Za-z0-9]+/, '').replace(/[^A-Za-z0-9.!?]+$/, '');

  if (trimmed.length === 0) return undefined;
  // A single character must be alphanumeric — the optional tail group cannot apply.
  if (trimmed.length === 1 && !/^[A-Za-z0-9]$/.test(trimmed)) return undefined;
  return trimmed;
}
