// Go's `uuid.UUID` with `json:",omitempty"` does not omit an unset value — it
// serializes as the zero UUID. So "no Target" arrives as
// "00000000-0000-0000-0000-000000000000" rather than null or absent, and a plain
// truthiness check treats it as a real id. Every id read off the wire goes through
// here.
//
// Endpoints are not consistent about it: `GET /unit` returns `TargetID` absent for an
// unbound Unit, while `POST /function/invoke` returns the zero UUID for the same Unit.
// Normalizing at the boundary means callers never have to know which is which.

const ZERO_UUID = '00000000-0000-0000-0000-000000000000';

/** Returns the id, or undefined when it is absent, empty, or the zero UUID. */
export function realId(id: string | undefined | null): string | undefined {
  if (!id || id === ZERO_UUID) return undefined;
  return id;
}

/** True when an id names an actual entity. */
export function hasId(id: string | undefined | null): boolean {
  return realId(id) !== undefined;
}
