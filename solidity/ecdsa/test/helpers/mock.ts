/*
 * The `__mock__*` names below are the administrative entry points of
 * `MockContract.sol`, deliberately namespaced there so they cannot be confused
 * with — or collide with — a function of the interface being mocked. They are
 * dictated by the contract, so the dangling-underscore rule does not apply.
 */
/* eslint-disable no-underscore-dangle */
import { ethers, artifacts } from "hardhat"
import { expect } from "chai"
import { BigNumber } from "ethers"

import type { BigNumberish, Contract, Signer } from "ethers"
import type { FunctionFragment, Interface, ParamType } from "ethers/lib/utils"

/**
 * Programmable contract mock, replacing `@defi-wonderland/smock`.
 *
 * smock configured its fakes by mutating in-process JavaScript state inside
 * Hardhat's EVM, which is why its API was synchronous — and also why it broke
 * on Hardhat >= 2.20 and is archived upstream with no forward path. This
 * replacement drives an ordinary deployed contract (`MockContract.sol`)
 * over the public provider API, so it depends on nothing Hardhat can change
 * underneath it.
 *
 * The consequence is that configuration is a transaction, so every setup and
 * inspection call here returns a promise and must be awaited. That is the one
 * behavioural difference from smock and the reason the migration touches call
 * sites at all.
 *
 * A transaction also mines a block, and Hardhat advances the clock a second
 * per block, where smock advanced it not at all. Suites that assert on a
 * boundary cannot absorb that: WalletProposalValidator stubs a 7200 second
 * delay, advances time by exactly 7200 and requires
 * `block.timestamp > requestedAt + minAge` to be false — any drift inverts it.
 * So every write below pins the next block's timestamp to the current one,
 * which needs `allowBlocksWithSameTimestamp` in hardhat.config.ts and is why
 * this package is on hardhat >= 2.19.
 *
 * Semantics follow Foundry's `vm.mockCall` deliberately: an exact-calldata
 * entry wins over a selector-wide default, and an unconfigured function
 * returns the zero value of its return type, matching smock.
 *
 * That last part is not free. Solidity compares returndatasize against the
 * size its ABI expects and reverts on a short answer, so a mock cannot answer
 * an unconfigured function with empty data — it has to return a correctly
 * encoded zero. Only this helper knows the mocked ABI, so it computes those
 * encodings up front and installs them in the mock as a base layer that
 * `reset` does not disturb.
 */

/**
 * Runs a configuration transaction without advancing the chain clock.
 *
 * `allowBlocksWithSameTimestamp` only *permits* a block to reuse the previous
 * timestamp; it does not make it happen. Determinism comes from setting the
 * next timestamp explicitly, so that is done here rather than left to how fast
 * the machine happens to be.
 */
async function withoutAdvancingTime<T>(write: () => Promise<T>): Promise<T> {
  const { timestamp } = await ethers.provider.getBlock("latest")
  await ethers.provider.send("evm_setNextBlockTimestamp", [timestamp])
  return write()
}

/** A recorded call, decoded against the mocked interface. */
export interface MockCall {
  /** Decoded arguments, in declaration order. */
  args: unknown[]
  /** `msg.value` the call carried, as smock's `getCall(n).value` did. */
  value: BigNumber
}

/** Configuration and inspection handle for one function of a mock. */
export interface MockedFunction {
  /**
   * Answers every call to this function with `value`, unless a
   * `whenCalledWith` entry matches first. Call with no argument for a function
   * that returns nothing.
   */
  returns(value?: unknown): Promise<void>
  /** Makes every call to this function revert. */
  reverts(reason?: string): Promise<void>
  /** Narrows the next `returns`/`reverts` to one exact argument list. */
  whenCalledWith(...args: unknown[]): {
    returns(value?: unknown): Promise<void>
    reverts(reason?: string): Promise<void>
  }
  /** Drops every response configured for this function and every call recorded for it. */
  reset(): Promise<void>
  /** Number of calls recorded for this function. */
  callCount(): Promise<number>
  /** The i-th recorded call, decoded. */
  getCall(index: number): Promise<MockCall>
  /** Every recorded call, decoded. */
  getCalls(): Promise<MockCall[]>
}

export type Mock<T> = {
  [K in keyof T]: T[K] extends (...args: never[]) => unknown
    ? T[K] & MockedFunction
    : T[K]
} & {
  address: string
  /** Signer that sends from the mock's own address, as smock's `fake.wallet` did. */
  wallet: Signer
  /** Underlying deployed `MockContract`, for anything this helper does not wrap. */
  mockContract: Contract
  /**
   * The mocked interface bound to `signer`, as smock's `FakeContract.connect`
   * was. smock's fake extended `ethers.Contract` and inherited this; the proxy
   * here resolves only the mocked ABI and the keys above, so without it
   * `mock.connect(someone)` is `undefined`.
   */
  connect(signer: Signer): Contract
  /** Drops all configured responses and all recorded calls. */
  reset(): Promise<void>
  /**
   * Turns call recording on or off.
   *
   * Recording costs real gas — smock zeroed gas for faked calls, this mock
   * SSTOREs the calldata — so a test asserting on the gas of the contract
   * under test should switch it off first, or it measures the mock too.
   */
  setRecording(enabled: boolean): Promise<void>
}

/** Selectors of `MockContract`'s own administrative entry points. */
function adminSelectors(mockInterface: Interface): Set<string> {
  return new Set(
    Object.keys(mockInterface.functions)
      .filter((signature) => signature.startsWith("__mock__"))
      .map((signature) => mockInterface.getSighash(signature))
  )
}

/**
 * `MockContract` answers the mocked interface through its fallback, so its own
 * `__mock__*` entry points share the selector space with whatever is being
 * mocked. A four-byte collision would silently shadow a real function, which
 * would be a very confusing test failure. It cannot happen by accident, but it
 * costs nothing to prove rather than assume.
 */
function assertNoSelectorCollision(
  target: Interface,
  mockInterface: Interface,
  targetName: string
): void {
  const reserved = adminSelectors(mockInterface)

  Object.keys(target.functions).forEach((signature) => {
    const selector = target.getSighash(signature)
    if (reserved.has(selector)) {
      throw new Error(
        `${targetName}.${signature} has selector ${selector}, which collides ` +
          "with a MockContract administrative function. This mock cannot " +
          "represent that interface."
      )
    }
  })
}

function fragmentsByName(target: Interface): Map<string, FunctionFragment[]> {
  const byName = new Map<string, FunctionFragment[]>()

  Object.values(target.functions).forEach((fragment) => {
    const existing = byName.get(fragment.name)
    if (existing) {
      existing.push(fragment)
    } else {
      byName.set(fragment.name, [fragment])
    }
  })

  return byName
}

/**
 * Picks the fragment to use for a name. Overloads are ambiguous by name alone;
 * rather than guess, fail loudly and let the caller address the mock's
 * `mockContract` directly.
 */
function resolveFragment(
  fragments: FunctionFragment[],
  name: string,
  targetName: string
): FunctionFragment {
  if (fragments.length > 1) {
    throw new Error(
      `${targetName}.${name} is overloaded (${fragments.length} signatures). ` +
        "Address it through the mock's mockContract handle instead."
    )
  }

  return fragments[0]
}

/**
 * The zero value of an ABI type, shaped the way `defaultAbiCoder` wants it.
 *
 * Dynamic types cannot be zero-filled word-wise, and tuples have to be built
 * component by component, so this walks the type rather than assuming a flat
 * layout.
 */
function zeroValueFor(type: ParamType): unknown {
  if (type.baseType === "array") {
    if (type.arrayLength === -1) {
      return []
    }
    return Array.from({ length: type.arrayLength }, () =>
      zeroValueFor(type.arrayChildren)
    )
  }

  if (type.baseType === "tuple") {
    return type.components.map((component) => zeroValueFor(component))
  }

  if (type.baseType === "address") {
    return ethers.constants.AddressZero
  }

  if (type.baseType === "bool") {
    return false
  }

  if (type.baseType === "string") {
    return ""
  }

  if (type.baseType === "bytes") {
    return "0x"
  }

  const fixedBytes = /^bytes(\d+)$/.exec(type.baseType)
  if (fixedBytes) {
    return `0x${"00".repeat(Number(fixedBytes[1]))}`
  }

  // Everything left is uint*/int*.
  return 0
}

/**
 * Shapes a configured return value the way `defaultAbiCoder` wants it.
 *
 * smock accepted a named object for a multi-output function — Bridge's
 * `depositParameters` returns four values and is configured as
 * `{ depositDustThreshold, depositTreasuryFeeDivisor, ... }`. The coder wants
 * those positionally, so they are mapped back by output name. A single-output
 * function is different: an object there is a struct, and the coder handles it.
 */
function toPositional(outputs: ParamType[], value: unknown): unknown[] {
  if (outputs.length === 1) {
    return [value]
  }

  if (Array.isArray(value)) {
    return value
  }

  if (value !== null && typeof value === "object") {
    const named = value as Record<string, unknown>
    return outputs.map((output, index) =>
      output.name && output.name in named ? named[output.name] : named[index]
    )
  }

  return [value]
}

function encodeReturn(fragment: FunctionFragment, value: unknown): string {
  if (fragment.outputs === null || fragment.outputs.length === 0) {
    return "0x"
  }

  return ethers.utils.defaultAbiCoder.encode(
    fragment.outputs,
    toPositional(fragment.outputs, value)
  )
}

function encodeRevert(reason?: string): string {
  if (reason === undefined) {
    return "0x"
  }

  return (
    ethers.utils.id("Error(string)").slice(0, 10) +
    ethers.utils.defaultAbiCoder.encode(["string"], [reason]).slice(2)
  )
}

/**
 * Deploys a programmable mock answering `target`'s interface.
 *
 * @param target Name of the contract or interface to mock, as passed to
 *        `artifacts.readArtifact` — e.g. `"IBridge"`.
 * @param options.address Deploy the mock at this exact address, replacing
 *        whatever is there. Mirrors smock's `{ address }` option.
 * @returns A handle exposing each of `target`'s functions with `returns`,
 *          `whenCalledWith`, `reverts`, `reset`, `callCount` and `getCall`.
 */
export async function createMock<T>(
  target: string,
  options: { address?: string } = {}
): Promise<Mock<T>> {
  const targetArtifact = await artifacts.readArtifact(target)
  const targetInterface = new ethers.utils.Interface(targetArtifact.abi)

  const mockFactory = await ethers.getContractFactory("MockContract")
  // Deploying is a transaction too, and a mock is routinely created inside a
  // `before` hook after the test has already captured a baseline timestamp.
  let mockContract = await withoutAdvancingTime(() => mockFactory.deploy())
  await mockContract.deployed()

  assertNoSelectorCollision(targetInterface, mockContract.interface, target)

  if (options.address !== undefined) {
    // Move the deployed bytecode to the requested address, before anything is
    // configured. `hardhat_setCode` copies code and not storage, and every
    // configuration below — the base returns, the non-recording flags, and
    // later every `returns`/`whenCalledWith` — is storage. Configuring first
    // and relocating afterwards left a pinned mock with none of it.
    const code = await ethers.provider.getCode(mockContract.address)
    await ethers.provider.send("hardhat_setCode", [options.address, code])
    mockContract = mockContract.attach(options.address)
  }

  // Install the response of last resort for every function, so an unstubbed
  // one answers with a correctly sized zero instead of reverting the caller.
  const baseFragments = Object.values(targetInterface.functions)
  const baseSelectors = baseFragments.map((fragment) =>
    targetInterface.getSighash(fragment)
  )
  const baseReturns = baseFragments.map((fragment) =>
    fragment.outputs === null || fragment.outputs.length === 0
      ? "0x"
      : ethers.utils.defaultAbiCoder.encode(
          fragment.outputs,
          fragment.outputs.map((output) => zeroValueFor(output))
        )
  )
  await withoutAdvancingTime(() =>
    mockContract.__mock__setBaseReturns(baseSelectors, baseReturns)
  )

  // Flag the read-only functions. Solidity reaches them by STATICCALL, where
  // the storage write recording needs is impossible, so the mock must not even
  // attempt it. Doing this from the ABI rather than discovering it at runtime
  // also lets `callCount`/`getCall` refuse loudly below.
  const nonRecordingSelectors = baseFragments
    .filter(
      (fragment) =>
        fragment.stateMutability === "view" ||
        fragment.stateMutability === "pure"
    )
    .map((fragment) => targetInterface.getSighash(fragment))
  if (nonRecordingSelectors.length > 0) {
    await withoutAdvancingTime(() =>
      mockContract.__mock__setNonRecordingSelectors(nonRecordingSelectors)
    )
  }

  await ethers.provider.send("hardhat_impersonateAccount", [
    mockContract.address,
  ])
  await ethers.provider.send("hardhat_setBalance", [
    mockContract.address,
    "0x21e19e0c9bab2400000", // 10_000 ETH, so the mock can pay for its own sends
  ])
  const wallet = await ethers.getSigner(mockContract.address)

  const byName = fragmentsByName(targetInterface)

  function buildFunction(name: string): MockedFunction {
    const fragment = resolveFragment(byName.get(name) ?? [], name, target)
    const selector = targetInterface.getSighash(fragment)
    const readOnly =
      fragment.stateMutability === "view" || fragment.stateMutability === "pure"

    // Calls to a read-only function cannot be recorded, so answering "0 calls"
    // would be a lie that reads as a passing assertion. Refuse instead.
    const refuseIfReadOnly = () => {
      if (readOnly) {
        throw new Error(
          `${target}.${name} is ${fragment.stateMutability}, so Solidity ` +
            "reaches it by STATICCALL and the mock cannot record the call. " +
            "Assert on the state-changing function that consumed the value " +
            "instead."
        )
      }
    }

    const setForCalldata = async (
      args: unknown[],
      behaviour: "return" | "revert",
      payload: unknown
    ): Promise<void> => {
      let callData: string
      try {
        callData = targetInterface.encodeFunctionData(fragment, args as never[])
      } catch (error) {
        // smock stored `whenCalledWith` arguments as JavaScript values and
        // compared them after decoding, so arguments that are not valid for
        // the signature simply never matched and the call fell through to the
        // selector-wide default. Encoding them up front turns the same
        // situation into a thrown error, which would fail tests that pass
        // today for reasons unrelated to this migration — a value converted to
        // a Wormhole address twice, for instance, is 44 bytes where the
        // signature wants 32.
        //
        // Keep smock's outcome — an entry that can never match — but say so,
        // because it does mean the narrowing in that test is dead.
        // eslint-disable-next-line no-console
        console.warn(
          `mock: ${target}.${name} whenCalledWith(...) arguments cannot be ` +
            "encoded for this signature, so the entry can never match and " +
            `is being skipped: ${(error as Error).message.split("(")[0].trim()}`
        )
        return
      }

      await withoutAdvancingTime(() =>
        behaviour === "return"
          ? mockContract.__mock__setReturnForCalldata(
              callData,
              encodeReturn(fragment, payload)
            )
          : mockContract.__mock__setRevertForCalldata(
              callData,
              encodeRevert(payload as string | undefined)
            )
      )
    }

    const decodeCall = (callData: string, value: BigNumber): MockCall => ({
      args: Array.from(
        targetInterface.decodeFunctionData(fragment, callData)
      ) as unknown[],
      value,
    })

    return {
      async returns(value?: unknown): Promise<void> {
        await withoutAdvancingTime(() =>
          mockContract.__mock__setReturnForSelector(
            selector,
            encodeReturn(fragment, value)
          )
        )
      },

      async reverts(reason?: string): Promise<void> {
        await withoutAdvancingTime(() =>
          mockContract.__mock__setRevertForSelector(
            selector,
            encodeRevert(reason)
          )
        )
      },

      whenCalledWith(...args: unknown[]) {
        return {
          returns: (value?: unknown) => setForCalldata(args, "return", value),
          reverts: (reason?: string) => setForCalldata(args, "revert", reason),
        }
      },

      async reset(): Promise<void> {
        await withoutAdvancingTime(() =>
          mockContract.__mock__resetSelector(selector)
        )
      },

      async callCount(): Promise<number> {
        refuseIfReadOnly()

        const count: BigNumberish =
          await mockContract.__mock__callCountForSelector(selector)
        return Number(count)
      },

      async getCall(index: number): Promise<MockCall> {
        refuseIfReadOnly()

        const [callData, value] = await Promise.all([
          mockContract.__mock__callForSelectorAt(selector, index),
          mockContract.__mock__callValueForSelectorAt(selector, index),
        ])
        return decodeCall(callData as string, value as BigNumber)
      },

      async getCalls(): Promise<MockCall[]> {
        refuseIfReadOnly()

        const count: BigNumberish =
          await mockContract.__mock__callCountForSelector(selector)
        const calls: MockCall[] = []

        for (let i = 0; i < Number(count); i++) {
          // Sequential on purpose: ordering is the point of this accessor.
          // eslint-disable-next-line no-await-in-loop
          const [callData, value] = await Promise.all([
            mockContract.__mock__callForSelectorAt(selector, i),
            mockContract.__mock__callValueForSelectorAt(selector, i),
          ])
          calls.push(decodeCall(callData as string, value as BigNumber))
        }

        return calls
      },
    }
  }

  const functions = new Map<string, MockedFunction>()
  const readContract = new ethers.Contract(
    mockContract.address,
    targetArtifact.abi,
    ethers.provider
  )

  const handle = {
    address: mockContract.address,
    wallet,
    mockContract,
    connect(signer: Signer): Contract {
      return new ethers.Contract(
        mockContract.address,
        targetArtifact.abi,
        signer
      )
    },
    async reset(): Promise<void> {
      await withoutAdvancingTime(() => mockContract.__mock__reset())
    },
    async setRecording(enabled: boolean): Promise<void> {
      await withoutAdvancingTime(() =>
        mockContract.__mock__setRecording(enabled)
      )
    },
  }

  return new Proxy(handle, {
    get(base, property: string | symbol, receiver) {
      if (typeof property !== "string" || property in base) {
        return Reflect.get(base, property, receiver)
      }

      if (!byName.has(property)) {
        return Reflect.get(base, property, receiver)
      }

      if (!functions.has(property)) {
        // Callable like the contract itself, so a test can read through the
        // mock, with the configuration surface hung off the same object —
        // which is the shape smock's fakes had.
        const callable = (...args: unknown[]) => readContract[property](...args)
        functions.set(
          property,
          Object.assign(callable, buildFunction(property)) as MockedFunction
        )
      }

      return functions.get(property)
    },
  }) as unknown as Mock<T>
}

/**
 * Assertion helpers replacing smock's chai matchers.
 *
 * smock's `expect(fake.fn).to.have.been.calledOnce` worked because the call log
 * was in-process JavaScript, so a chai *property* could read it synchronously.
 * Here the log is on chain, so reading it is asynchronous, and a property
 * cannot be awaited — `await expect(x).to.have.been.calledOnce` would await the
 * assertion object, not the count, and pass unconditionally. These are
 * functions so that the `await` is real.
 */

async function counted(fn: MockedFunction): Promise<number> {
  return fn.callCount()
}

/** `expect(fake.fn).to.have.been.called` */
export async function expectCalled(fn: MockedFunction): Promise<void> {
  const count = await counted(fn)
  expect(count, "expected the function to have been called").to.be.greaterThan(
    0
  )
}

/** `expect(fake.fn).to.have.been.calledOnce` */
export async function expectCalledOnce(fn: MockedFunction): Promise<void> {
  expect(await counted(fn), "expected exactly one call").to.equal(1)
}

/** `expect(fake.fn).to.not.have.been.called` */
export async function expectNotCalled(fn: MockedFunction): Promise<void> {
  expect(await counted(fn), "expected no calls").to.equal(0)
}

/** `expect(fake.fn).to.have.been.calledThrice` */
export async function expectCalledThrice(fn: MockedFunction): Promise<void> {
  expect(await counted(fn), "expected exactly three calls").to.equal(3)
}

/** `expect(fake.fn).to.have.been.calledTwice` */
export async function expectCalledTwice(fn: MockedFunction): Promise<void> {
  expect(await counted(fn), "expected exactly two calls").to.equal(2)
}

/** `expect(fake.fn).to.have.been.calledOnceWith(...args)` */
/**
 * Puts one recorded or expected argument into a comparable form.
 *
 * The two sides never arrive in the same representation. ethers decodes an ABI
 * integer to a `BigNumber` above 48 bits and to a plain `number` at or below
 * it, so a `uint256` argument reaches this as a `BigNumber` while the
 * `uint32` getter the test compared it against yields a `number`; and a struct
 * or dynamic array puts both one level down, where the previous top-level-only
 * check never looked. smock compared `BigNumberish` values numerically at any
 * depth, so both shapes used to pass.
 *
 * Numerics are wrapped rather than rendered bare, so that a genuine string
 * argument of `"100"` still fails against a numeric `100`. Everything else —
 * addresses, bytes, booleans — is left exactly as it came, because for those
 * the two sides already agree and loosening the comparison would only hide a
 * real mismatch.
 */
function normalizeForComparison(value: unknown): unknown {
  if (BigNumber.isBigNumber(value)) {
    return { numeric: value.toString() }
  }
  if (typeof value === "number" || typeof value === "bigint") {
    return { numeric: value.toString() }
  }
  if (Array.isArray(value)) {
    return value.map(normalizeForComparison)
  }
  return value
}

export async function expectCalledOnceWith(
  fn: MockedFunction,
  args: unknown[]
): Promise<void> {
  expect(await counted(fn), "expected exactly one call").to.equal(1)

  const call = await fn.getCall(0)

  expect(call.args.length, "argument count").to.equal(args.length)
  args.forEach((expected, index) => {
    const actual = call.args[index]
    expect(normalizeForComparison(actual), `argument ${index}`).to.deep.equal(
      normalizeForComparison(expected)
    )
  })
}

/**
 * `expect(fake.fn).to.have.been.calledWith(...)` — some recorded call matched.
 *
 * Deliberately weaker than `expectCalledOnceWith`: smock's `calledWith` says
 * nothing about how many times the function ran, so translating those sites to
 * the `Once` variant would quietly add an assertion the test never made.
 */
export async function expectCalledWith(
  fn: MockedFunction,
  args: unknown[]
): Promise<void> {
  const calls = await fn.getCalls()

  expect(calls.length, "expected at least one call").to.be.greaterThan(0)

  const wanted = args.map(normalizeForComparison)
  const seen = calls.map((call) => call.args.map(normalizeForComparison))

  expect(
    seen,
    `expected a call with ${calls.length} recorded, none matching`
  ).to.deep.include(wanted)
}

export default createMock
