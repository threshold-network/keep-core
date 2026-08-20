#!/usr/bin/env node
//
// Usage:
//   node scripts/formal/check_roast_attempt_context_vectors.mjs
//
// Verifies that every entry in
//   pkg/tbtc/signer/test/vectors/roast-attempt-context-v1.json
// reproduces the canonical ROAST attempt-context hash derived from the
// FROST-ROAST-INCLUDED-FPR-v1 / FROST-ROAST-ATTEMPT-ID-v1 domain constants.
// Exits 0 on full conformance, 1 on any mismatch (which would invalidate
// the canonical signer state-machine contract).
//
import crypto from "crypto"
import fs from "fs"
import path from "path"
import { fileURLToPath } from "url"

const ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN =
  "FROST-ROAST-INCLUDED-FPR-v1"
const ROAST_ATTEMPT_ID_DOMAIN = "FROST-ROAST-ATTEMPT-ID-v1"
const VECTOR_SCHEMA_VERSION = "roast-attempt-context-v1"

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(scriptDir, "../..")
// Path normalization (allowlisted-divergence per source manifest):
// canonical signer layout places the ROAST attempt context vector at
// `<rootDir>/test/vectors/roast-attempt-context-v1.json` where rootDir
// is `pkg/tbtc/signer/`. Monorepo source path was
// `docs/frost-migration/test-vectors/roast-attempt-context-v1.json`
// relative to monorepo root.
const vectorsPath = path.join(
  rootDir,
  "test/vectors/roast-attempt-context-v1.json"
)

const fail = (message) => {
  console.error(`[vector-conformance] ${message}`)
  process.exit(1)
}

const pushFramedComponent = (components, component) => {
  const componentBuffer = Buffer.isBuffer(component)
    ? component
    : Buffer.from(component)
  if (componentBuffer.length > 0xffffffff) {
    fail("component exceeds u32 framing limit")
  }

  const lengthBuffer = Buffer.allocUnsafe(4)
  lengthBuffer.writeUInt32BE(componentBuffer.length, 0)
  components.push(lengthBuffer, componentBuffer)
}

const hashHex = (payload) =>
  crypto.createHash("sha256").update(payload).digest("hex")

const roastHashHexWithComponents = (domain, components) => {
  const payloadComponents = []
  pushFramedComponent(payloadComponents, Buffer.from(domain, "utf8"))
  for (const component of components) {
    pushFramedComponent(payloadComponents, component)
  }
  return hashHex(Buffer.concat(payloadComponents))
}

const canonicalizeParticipants = (participants, vectorId) => {
  if (!Array.isArray(participants) || participants.length === 0) {
    fail(`vector ${vectorId}: included_participants must be non-empty`)
  }

  const canonical = [...participants].sort((left, right) => left - right)
  const seen = new Set()
  for (const participantIdentifier of canonical) {
    if (
      !Number.isInteger(participantIdentifier) ||
      participantIdentifier <= 0 ||
      participantIdentifier > 0xffff
    ) {
      fail(`vector ${vectorId}: invalid participant identifier`)
    }
    if (seen.has(participantIdentifier)) {
      fail(
        `vector ${vectorId}: duplicate participant identifier ${participantIdentifier}`
      )
    }
    seen.add(participantIdentifier)
  }

  return canonical
}

const roastIncludedParticipantsFingerprintHex = (participants) => {
  const participantPayloadComponents = []
  for (const participantIdentifier of participants) {
    const participantBytes = Buffer.allocUnsafe(2)
    participantBytes.writeUInt16BE(participantIdentifier, 0)
    pushFramedComponent(participantPayloadComponents, participantBytes)
  }

  return roastHashHexWithComponents(
    ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN,
    [Buffer.concat(participantPayloadComponents)]
  )
}

const roastAttemptIdHex = (
  sessionId,
  messageDigestHex,
  attemptNumber,
  coordinatorIdentifier,
  includedParticipantsFingerprintHex
) => {
  if (!Number.isInteger(attemptNumber) || attemptNumber <= 0) {
    fail("attempt_number must be a positive integer")
  }
  if (!Number.isInteger(coordinatorIdentifier) || coordinatorIdentifier <= 0) {
    fail("coordinator_identifier must be a positive integer")
  }

  const attemptNumberBytes = Buffer.allocUnsafe(4)
  attemptNumberBytes.writeUInt32BE(attemptNumber, 0)
  const coordinatorBytes = Buffer.allocUnsafe(2)
  coordinatorBytes.writeUInt16BE(coordinatorIdentifier, 0)

  return roastHashHexWithComponents(ROAST_ATTEMPT_ID_DOMAIN, [
    Buffer.from(sessionId, "utf8"),
    Buffer.from(messageDigestHex, "utf8"),
    attemptNumberBytes,
    coordinatorBytes,
    Buffer.from(includedParticipantsFingerprintHex, "utf8"),
  ])
}

const vectors = JSON.parse(fs.readFileSync(vectorsPath, "utf8"))
if (vectors.schema_version !== VECTOR_SCHEMA_VERSION) {
  fail(
    `unsupported vector schema [${vectors.schema_version}] expected [${VECTOR_SCHEMA_VERSION}]`
  )
}

const configuredFingerprintDomain =
  vectors.hash_domains?.included_participants_fingerprint
const configuredAttemptIdDomain = vectors.hash_domains?.attempt_id
if (
  configuredFingerprintDomain !== ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN ||
  configuredAttemptIdDomain !== ROAST_ATTEMPT_ID_DOMAIN
) {
  fail("vector hash_domains do not match canonical domain constants")
}

let verified = 0
for (const vector of vectors.vectors ?? []) {
  const vectorId = vector.id ?? "unknown"
  const canonicalParticipants = canonicalizeParticipants(
    vector.included_participants,
    vectorId
  )
  const expectedFingerprint =
    vector.expected_included_participants_fingerprint?.toLowerCase()
  const expectedAttemptId = vector.expected_attempt_id?.toLowerCase()
  const messageDigestHex = vector.message_digest_hex?.toLowerCase()

  const actualFingerprint = roastIncludedParticipantsFingerprintHex(
    canonicalParticipants
  )
  const actualAttemptId = roastAttemptIdHex(
    vector.session_id,
    messageDigestHex,
    vector.attempt_number,
    vector.coordinator_identifier,
    actualFingerprint
  )

  if (actualFingerprint !== expectedFingerprint) {
    fail(
      `vector ${vectorId}: fingerprint mismatch expected [${expectedFingerprint}] got [${actualFingerprint}]`
    )
  }
  if (actualAttemptId !== expectedAttemptId) {
    fail(
      `vector ${vectorId}: attempt id mismatch expected [${expectedAttemptId}] got [${actualAttemptId}]`
    )
  }

  verified += 1
}

if (verified === 0) {
  fail("no vectors found")
}

console.log(`[vector-conformance] verified ${verified} shared vectors`)
