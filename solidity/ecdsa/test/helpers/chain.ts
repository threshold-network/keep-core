/** Fail explicitly when a local test's block or mined receipt is unavailable. */
export default function requireResult<T>(result: T | null): T {
  if (result === null) throw new Error("Expected a non-null chain response")
  return result
}
