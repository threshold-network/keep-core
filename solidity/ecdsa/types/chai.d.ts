/**
 * Hardhat custom matchers used in tests (revertedWithCustomError).
 * Lives under types/ so Mocha does not try to execute this file as a test.
 */
declare namespace Chai {
  interface Assertion {
    revertedWithCustomError(
      contract: unknown,
      errorName: string
    ): RevertedWithCustomErrorAssertion
  }

  // Awaitable on its own, and chainable with `.withArgs(...)` to assert the
  // custom error's arguments (mirrors @nomicfoundation/hardhat-chai-matchers).
  interface RevertedWithCustomErrorAssertion extends Promise<void> {
    withArgs(...args: unknown[]): Promise<void>
  }
}
