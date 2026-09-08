/* eslint-disable @typescript-eslint/no-unused-expressions */
import { deployments, ethers, helpers } from "hardhat"
import { expect } from "chai"

import {
  constants,
  params,
  setupAllowlist,
  walletRegistryFixture,
} from "./fixtures"

import type { ContractTransaction, Signer } from "ethers"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type {
  Allowlist,
  WalletRegistry,
  WalletRegistryGovernance,
  SortitionPool,
} from "../typechain"

const { to1e18 } = helpers.number
const { createSnapshot, restoreSnapshot } = helpers.snapshot
const ZERO_ADDRESS = ethers.constants.AddressZero
const MAX_UINT64 = ethers.BigNumber.from("18446744073709551615")

describe("WalletRegistry - Allowlist Authorization", () => {
  let walletRegistry: WalletRegistry
  let walletRegistryGovernance: WalletRegistryGovernance
  let sortitionPool: SortitionPool
  let allowlist: Allowlist
  let deployer: SignerWithAddress
  let governance: SignerWithAddress
  let stakingProvider: SignerWithAddress
  let operator: SignerWithAddress
  let thirdParty: SignerWithAddress
  let walletOwnerSigner: Signer
  const providerWeight = to1e18(1000000)
  const { minimumAuthorization } = params

  // Deploy separately so a cold run and the 100-operator setup each have their
  // own hook timeout.
  before("deploy contracts", async () => {
    await deployments.fixture()
  })

  before("load Allowlist fixture", async () => {
    const fixture = await walletRegistryFixture({ useAllowlist: true })
    ;({
      walletRegistry,
      walletRegistryGovernance,
      sortitionPool,
      deployer,
      governance,
      thirdParty,
    } = fixture)
    allowlist = fixture.allowlist
    walletOwnerSigner = fixture.walletOwner.wallet
    // Named accounts are separate from the 100 operators registered by the fixture.
    const signers = await helpers.signers.getNamedSigners()
    stakingProvider = signers.esdm
    operator = signers.chaosnetOwner
  })

  describe("registerOperator", () => {
    context("when called with zero-address operator", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(stakingProvider).registerOperator(ZERO_ADDRESS)
        ).to.be.revertedWith("Operator can not be zero address")
      })
    })
    context(
      "when operator has been already registered for the staking provider",
      () => {
        before(async () => {
          await createSnapshot()
          await walletRegistry
            .connect(stakingProvider)
            .registerOperator(operator.address)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should revert", async () => {
          await expect(
            walletRegistry
              .connect(stakingProvider)
              .registerOperator(operator.address)
          ).to.be.revertedWith("Operator already set for the staking provider")
          await expect(
            walletRegistry
              .connect(stakingProvider)
              .registerOperator(thirdParty.address)
          ).to.be.revertedWith("Operator already set for the staking provider")
        })
      }
    )
    context("when the operator is already in use", () => {
      before(async () => {
        await createSnapshot()
        await walletRegistry
          .connect(thirdParty)
          .registerOperator(operator.address)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(stakingProvider)
            .registerOperator(operator.address)
        ).to.be.revertedWith("Operator address already in use")
      })
    })
    context("when staking provider is registering new operator", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()
        tx = await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should set staking provider -> operator mapping", async () => {
        expect(
          await walletRegistry.stakingProviderToOperator(
            stakingProvider.address
          )
        ).to.equal(operator.address)
      })

      it("should set operator -> staking provider mapping", async () => {
        expect(
          await walletRegistry.operatorToStakingProvider(operator.address)
        ).to.equal(stakingProvider.address)
      })

      it("should emit OperatorRegistered event", async () => {
        await expect(tx)
          .to.emit(walletRegistry, "OperatorRegistered")
          .withArgs(stakingProvider.address, operator.address)
      })

      it("should not register operator in the pool", async () => {
        expect(await walletRegistry.isOperatorInPool(operator.address)).to.be
          .false
      })
    })
    context("when there is a pending authorization decrease request", () => {
      before(async () => {
        await createSnapshot()

        await allowlist
          .connect(deployer)
          .addStakingProvider(stakingProvider.address, providerWeight)

        const deauthorizingBy = to1e18(1)

        await allowlist
          .connect(deployer)
          .requestWeightDecrease(
            stakingProvider.address,
            (
              await allowlist.authorizedStake(
                stakingProvider.address,
                walletRegistry.address
              )
            ).sub(deauthorizingBy)
          )
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(stakingProvider)
            .registerOperator(operator.address)
        ).to.be.revertedWith(
          "There is a pending authorization decrease request"
        )
      })
    })
    context("when authorization decrease request was approved", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        await allowlist
          .connect(deployer)
          .addStakingProvider(stakingProvider.address, providerWeight)

        const deauthorizingBy = to1e18(1)

        await allowlist
          .connect(deployer)
          .requestWeightDecrease(
            stakingProvider.address,
            (
              await allowlist.authorizedStake(
                stakingProvider.address,
                walletRegistry.address
              )
            ).sub(deauthorizingBy)
          )

        await walletRegistry.approveAuthorizationDecrease(
          stakingProvider.address
        )

        tx = await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should set staking provider -> operator mapping", async () => {
        expect(
          await walletRegistry.stakingProviderToOperator(
            stakingProvider.address
          )
        ).to.equal(operator.address)
      })

      it("should set operator -> staking provider mapping", async () => {
        expect(
          await walletRegistry.operatorToStakingProvider(operator.address)
        ).to.equal(stakingProvider.address)
      })

      it("should emit OperatorRegistered event", async () => {
        await expect(tx)
          .to.emit(walletRegistry, "OperatorRegistered")
          .withArgs(stakingProvider.address, operator.address)
      })
    })
  })

  describe("authorizationIncreased", () => {
    context("when called not by the Allowlist contract", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(thirdParty)
            .authorizationIncreased(stakingProvider.address, 0, providerWeight)
        ).to.be.revertedWithCustomError(
          walletRegistry,
          "CallerNotStakingContract"
        )
      })
    })

    context("when authorization is below the minimum", () => {
      it("should revert", async () => {
        await expect(
          allowlist
            .connect(deployer)
            .addStakingProvider(
              stakingProvider.address,
              minimumAuthorization.sub(1)
            )
        ).to.be.revertedWith("Authorization below the minimum")
      })
    })
    context("when the operator is unknown", () => {
      context("when increasing to the minimum possible value", () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()
          tx = await allowlist
            .connect(deployer)
            .addStakingProvider(stakingProvider.address, minimumAuthorization)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should emit AuthorizationIncreased", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationIncreased")
            .withArgs(
              stakingProvider.address,
              ZERO_ADDRESS,
              0,
              minimumAuthorization
            )
        })
      })
      context("when increasing to a weight above the minimum", () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()
          tx = await allowlist
            .connect(deployer)
            .addStakingProvider(stakingProvider.address, providerWeight)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should emit AuthorizationIncreased", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationIncreased")
            .withArgs(stakingProvider.address, ZERO_ADDRESS, 0, providerWeight)
        })
      })
    })
    context("when the operator is registered", () => {
      before(async () => {
        await createSnapshot()

        await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)
      })

      after(async () => {
        await restoreSnapshot()
      })
      context("when increasing to the minimum possible value", () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          tx = await allowlist
            .connect(deployer)
            .addStakingProvider(stakingProvider.address, minimumAuthorization)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should emit AuthorizationIncreased", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationIncreased")
            .withArgs(
              stakingProvider.address,
              operator.address,
              0,
              minimumAuthorization
            )
        })
      })
      context("when increasing to a weight above the minimum", () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          tx = await allowlist
            .connect(deployer)
            .addStakingProvider(stakingProvider.address, providerWeight)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should emit AuthorizationIncreased", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationIncreased")
            .withArgs(
              stakingProvider.address,
              operator.address,
              0,
              providerWeight
            )
        })
      })
    })
  })

  describe("authorizationDecreaseRequested", () => {
    context("when called not by the Allowlist contract", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(thirdParty)
            .authorizationDecreaseRequested(stakingProvider.address, 100, 99)
        ).to.be.revertedWithCustomError(
          walletRegistry,
          "CallerNotStakingContract"
        )
      })
    })
    context("when the operator is unknown", () => {
      before(async () => {
        await createSnapshot()
        await allowlist
          .connect(deployer)
          .addStakingProvider(stakingProvider.address, providerWeight)
      })

      after(async () => {
        await restoreSnapshot()
      })
      context("when decreasing to a non-zero value below the minimum", () => {
        it("should revert", async () => {
          const deauthorizingTo = minimumAuthorization.sub(1)
          const deauthorizingBy = providerWeight.sub(deauthorizingTo)

          await expect(
            allowlist
              .connect(deployer)
              .requestWeightDecrease(
                stakingProvider.address,
                (
                  await allowlist.authorizedStake(
                    stakingProvider.address,
                    walletRegistry.address
                  )
                ).sub(deauthorizingBy)
              )
          ).to.be.revertedWith(
            "Authorization amount should be 0 or above the minimum"
          )
        })
      })
      context("when decreasing to zero", () => {
        let tx: ContractTransaction
        const decreasingTo = 0
        let decreasingBy

        before(async () => {
          await createSnapshot()

          decreasingBy = providerWeight.sub(decreasingTo)
          tx = await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(decreasingBy)
            )
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should require no time delay before approving", async () => {
          expect(
            await walletRegistry.remainingAuthorizationDecreaseDelay(
              stakingProvider.address
            )
          ).to.equal(0)
        })

        it("should emit AuthorizationDecreaseRequested event", async () => {
          const now = await helpers.time.lastBlockTime()
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationDecreaseRequested")
            .withArgs(
              stakingProvider.address,
              ZERO_ADDRESS,
              providerWeight,
              decreasingTo,
              now
            )
        })

        it("should capture deauthorizing amount", async () => {
          expect(
            await walletRegistry.pendingAuthorizationDecrease(
              stakingProvider.address
            )
          ).to.equal(decreasingBy)
        })
      })

      context("when decreasing to the minimum", () => {
        let tx: ContractTransaction
        let decreasingTo
        let decreasingBy

        before(async () => {
          await createSnapshot()

          decreasingTo = minimumAuthorization
          decreasingBy = providerWeight.sub(decreasingTo)
          tx = await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(decreasingBy)
            )
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should require no time delay before approving", async () => {
          expect(
            await walletRegistry.remainingAuthorizationDecreaseDelay(
              stakingProvider.address
            )
          ).to.equal(0)
        })

        it("should emit AuthorizationDecreaseRequested event", async () => {
          const now = await helpers.time.lastBlockTime()
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationDecreaseRequested")
            .withArgs(
              stakingProvider.address,
              ZERO_ADDRESS,
              providerWeight,
              decreasingTo,
              now
            )
        })

        it("should capture deauthorizing amount", async () => {
          expect(
            await walletRegistry.pendingAuthorizationDecrease(
              stakingProvider.address
            )
          ).to.equal(decreasingBy)
        })
      })

      context("when decreasing to a value above the minimum", () => {
        let tx: ContractTransaction
        let decreasingTo
        let decreasingBy

        before(async () => {
          await createSnapshot()

          decreasingTo = minimumAuthorization.add(1)
          decreasingBy = providerWeight.sub(decreasingTo)
          tx = await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(decreasingBy)
            )
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should require no time delay before approving", async () => {
          expect(
            await walletRegistry.remainingAuthorizationDecreaseDelay(
              stakingProvider.address
            )
          ).to.equal(0)
        })

        it("should emit AuthorizationDecreaseRequested event", async () => {
          const now = await helpers.time.lastBlockTime()
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationDecreaseRequested")
            .withArgs(
              stakingProvider.address,
              ZERO_ADDRESS,
              providerWeight,
              decreasingTo,
              now
            )
        })

        it("should capture deauthorizing amount", async () => {
          expect(
            await walletRegistry.pendingAuthorizationDecrease(
              stakingProvider.address
            )
          ).to.equal(decreasingBy)
        })
      })

      context("when called one more time", () => {
        const deauthorizingFirst = to1e18(10)
        const deauthorizingSecond = to1e18(20)

        before(async () => {
          await createSnapshot()

          await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(deauthorizingFirst)
            )
        })

        after(async () => {
          await restoreSnapshot()
        })

        context("when change period is equal delay", () => {
          before(async () => {
            const {
              authorizationDecreaseDelay,
              authorizationDecreaseChangePeriod,
            } = await walletRegistry.authorizationParameters()
            expect(authorizationDecreaseDelay).to.equal(
              authorizationDecreaseChangePeriod
            )
          })

          context("when delay passed", () => {
            before(async () => {
              await createSnapshot()
              await helpers.time.increaseTime(params.authorizationDecreaseDelay)

              await allowlist
                .connect(deployer)
                .requestWeightDecrease(
                  stakingProvider.address,
                  (
                    await allowlist.authorizedStake(
                      stakingProvider.address,
                      walletRegistry.address
                    )
                  ).sub(deauthorizingSecond)
                )
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should overwrite the previous request", async () => {
              expect(
                await walletRegistry.pendingAuthorizationDecrease(
                  stakingProvider.address
                )
              ).to.be.equal(deauthorizingSecond)
            })
          })

          context("when delay did not pass", () => {
            before(async () => {
              await createSnapshot()
              await helpers.time.increaseTime(
                params.authorizationDecreaseDelay - 60 // -1min
              )

              await allowlist
                .connect(deployer)
                .requestWeightDecrease(
                  stakingProvider.address,
                  (
                    await allowlist.authorizedStake(
                      stakingProvider.address,
                      walletRegistry.address
                    )
                  ).sub(deauthorizingSecond)
                )
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should overwrite the previous request", async () => {
              expect(
                await walletRegistry.pendingAuthorizationDecrease(
                  stakingProvider.address
                )
              ).to.be.equal(deauthorizingSecond)
            })
          })
        })

        context("when change period is zero", () => {
          before(async () => {
            await createSnapshot()

            await walletRegistryGovernance
              .connect(governance)
              .beginAuthorizationDecreaseChangePeriodUpdate(0)
            await helpers.time.increaseTime(constants.governanceDelay)
            await walletRegistryGovernance
              .connect(governance)
              .finalizeAuthorizationDecreaseChangePeriodUpdate()

            await allowlist
              .connect(deployer)
              .requestWeightDecrease(
                stakingProvider.address,
                (
                  await allowlist.authorizedStake(
                    stakingProvider.address,
                    walletRegistry.address
                  )
                ).sub(deauthorizingSecond)
              )
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should overwrite the previous request", async () => {
            expect(
              await walletRegistry.pendingAuthorizationDecrease(
                stakingProvider.address
              )
            ).to.be.equal(deauthorizingSecond)
          })
        })

        context("when change period is not equal delay and is non-zero", () => {
          const newChangePeriod = 3600 // 1h before delay end

          before(async () => {
            await createSnapshot()

            await walletRegistryGovernance
              .connect(governance)
              .beginAuthorizationDecreaseChangePeriodUpdate(newChangePeriod)
            await helpers.time.increaseTime(constants.governanceDelay)
            await walletRegistryGovernance
              .connect(governance)
              .finalizeAuthorizationDecreaseChangePeriodUpdate()
          })

          after(async () => {
            await restoreSnapshot()
          })

          context("when delay passed", () => {
            before(async () => {
              await createSnapshot()
              await helpers.time.increaseTime(params.authorizationDecreaseDelay)

              await allowlist
                .connect(deployer)
                .requestWeightDecrease(
                  stakingProvider.address,
                  (
                    await allowlist.authorizedStake(
                      stakingProvider.address,
                      walletRegistry.address
                    )
                  ).sub(deauthorizingSecond)
                )
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should overwrite the previous request", async () => {
              expect(
                await walletRegistry.pendingAuthorizationDecrease(
                  stakingProvider.address
                )
              ).to.be.equal(deauthorizingSecond)
            })
          })

          context("when change period activated", () => {
            before(async () => {
              await createSnapshot()
              await helpers.time.increaseTime(
                params.authorizationDecreaseDelay - newChangePeriod + 60
              ) // +1min

              await allowlist
                .connect(deployer)
                .requestWeightDecrease(
                  stakingProvider.address,
                  (
                    await allowlist.authorizedStake(
                      stakingProvider.address,
                      walletRegistry.address
                    )
                  ).sub(deauthorizingSecond)
                )
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should overwrite the previous request", async () => {
              expect(
                await walletRegistry.pendingAuthorizationDecrease(
                  stakingProvider.address
                )
              ).to.be.equal(deauthorizingSecond)
            })
          })

          context("when change period did not activate", () => {
            before(async () => {
              await createSnapshot()
              await helpers.time.increaseTime(
                params.authorizationDecreaseDelay - newChangePeriod - 60 // -1min
              )

              await allowlist
                .connect(deployer)
                .requestWeightDecrease(
                  stakingProvider.address,
                  (
                    await allowlist.authorizedStake(
                      stakingProvider.address,
                      walletRegistry.address
                    )
                  ).sub(deauthorizingSecond)
                )
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should overwrite the previous request", async () => {
              expect(
                await walletRegistry.pendingAuthorizationDecrease(
                  stakingProvider.address
                )
              ).to.be.equal(deauthorizingSecond)
            })
          })
        })
      })
    })
    context("when the operator is registered", () => {
      before(async () => {
        await createSnapshot()
        await allowlist
          .connect(deployer)
          .addStakingProvider(stakingProvider.address, providerWeight)
        await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("when decreasing to a non-zero value below the minimum", () => {
        it("should revert", async () => {
          const deauthorizingTo = minimumAuthorization.sub(1)
          const deauthorizingBy = providerWeight.sub(deauthorizingTo)

          await expect(
            allowlist
              .connect(deployer)
              .requestWeightDecrease(
                stakingProvider.address,
                (
                  await allowlist.authorizedStake(
                    stakingProvider.address,
                    walletRegistry.address
                  )
                ).sub(deauthorizingBy)
              )
          ).to.be.revertedWith(
            "Authorization amount should be 0 or above the minimum"
          )
        })
      })

      context("when decreasing to zero", () => {
        let tx: ContractTransaction
        const decreasingTo = 0
        let decreasingBy

        before(async () => {
          await createSnapshot()

          decreasingBy = providerWeight.sub(decreasingTo)
          tx = await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(decreasingBy)
            )
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should require updating the pool before approving", async () => {
          expect(
            await walletRegistry.remainingAuthorizationDecreaseDelay(
              stakingProvider.address
            )
          ).to.equal(MAX_UINT64)
        })

        it("should emit AuthorizationDecreaseRequested event", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationDecreaseRequested")
            .withArgs(
              stakingProvider.address,
              operator.address,
              providerWeight,
              decreasingTo,
              MAX_UINT64
            )
        })

        it("should capture deauthorizing amount", async () => {
          expect(
            await walletRegistry.pendingAuthorizationDecrease(
              stakingProvider.address
            )
          ).to.equal(decreasingBy)
        })
      })

      context("when decreasing to the minimum", () => {
        let tx: ContractTransaction
        let decreasingTo
        let decreasingBy

        before(async () => {
          await createSnapshot()

          decreasingTo = minimumAuthorization
          decreasingBy = providerWeight.sub(decreasingTo)
          tx = await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(decreasingBy)
            )
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should require updating the pool before approving", async () => {
          expect(
            await walletRegistry.remainingAuthorizationDecreaseDelay(
              stakingProvider.address
            )
          ).to.equal(MAX_UINT64)
        })

        it("should emit AuthorizationDecreaseRequested event", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationDecreaseRequested")
            .withArgs(
              stakingProvider.address,
              operator.address,
              providerWeight,
              decreasingTo,
              MAX_UINT64
            )
        })

        it("should capture deauthorizing amount", async () => {
          expect(
            await walletRegistry.pendingAuthorizationDecrease(
              stakingProvider.address
            )
          ).to.equal(decreasingBy)
        })
      })

      context("when decreasing to a value above the minimum", () => {
        let tx: ContractTransaction
        let decreasingTo
        let decreasingBy

        before(async () => {
          await createSnapshot()

          decreasingTo = minimumAuthorization.add(1)
          decreasingBy = providerWeight.sub(decreasingTo)
          tx = await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(decreasingBy)
            )
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should require updating the pool before approving", async () => {
          expect(
            await walletRegistry.remainingAuthorizationDecreaseDelay(
              stakingProvider.address
            )
          ).to.equal(MAX_UINT64)
        })

        it("should emit AuthorizationDecreaseRequested event", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationDecreaseRequested")
            .withArgs(
              stakingProvider.address,
              operator.address,
              providerWeight,
              decreasingTo,
              MAX_UINT64
            )
        })

        it("should capture deauthorizing amount", async () => {
          expect(
            await walletRegistry.pendingAuthorizationDecrease(
              stakingProvider.address
            )
          ).to.equal(decreasingBy)
        })
      })

      context("when called one more time", () => {
        const deauthorizingFirst = to1e18(11)
        const deauthorizingSecond = to1e18(21)

        before(async () => {
          await createSnapshot()

          await walletRegistry.connect(operator).joinSortitionPool()

          await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(deauthorizingFirst)
            )
        })

        after(async () => {
          await restoreSnapshot()
        })

        context("when change period is equal delay", () => {
          before(async () => {
            const {
              authorizationDecreaseDelay,
              authorizationDecreaseChangePeriod,
            } = await walletRegistry.authorizationParameters()
            expect(authorizationDecreaseDelay).to.equal(
              authorizationDecreaseChangePeriod
            )
          })

          context("when called before sortition pool was updated", () => {
            before(async () => {
              await createSnapshot()

              await allowlist
                .connect(deployer)
                .requestWeightDecrease(
                  stakingProvider.address,
                  (
                    await allowlist.authorizedStake(
                      stakingProvider.address,
                      walletRegistry.address
                    )
                  ).sub(deauthorizingSecond)
                )
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should overwrite the previous request", async () => {
              expect(
                await walletRegistry.pendingAuthorizationDecrease(
                  stakingProvider.address
                )
              ).to.be.equal(deauthorizingSecond)
            })

            it("should require updating the pool before approving", async () => {
              expect(
                await walletRegistry.remainingAuthorizationDecreaseDelay(
                  stakingProvider.address
                )
              ).to.equal(MAX_UINT64)
            })
          })

          context("when called after sortition pool was updated", () => {
            before(async () => {
              await createSnapshot()
              await walletRegistry.updateOperatorStatus(operator.address)
            })

            after(async () => {
              await restoreSnapshot()
            })

            context("when delay passed", () => {
              before(async () => {
                await createSnapshot()
                await helpers.time.increaseTime(
                  params.authorizationDecreaseDelay
                )
              })

              after(async () => {
                await restoreSnapshot()
              })

              before(async () => {
                await createSnapshot()

                await allowlist
                  .connect(deployer)
                  .requestWeightDecrease(
                    stakingProvider.address,
                    (
                      await allowlist.authorizedStake(
                        stakingProvider.address,
                        walletRegistry.address
                      )
                    ).sub(deauthorizingSecond)
                  )
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should overwrite the previous request", async () => {
                expect(
                  await walletRegistry.pendingAuthorizationDecrease(
                    stakingProvider.address
                  )
                ).to.be.equal(deauthorizingSecond)
              })

              it("should require updating the pool before approving", async () => {
                expect(
                  await walletRegistry.remainingAuthorizationDecreaseDelay(
                    stakingProvider.address
                  )
                ).to.equal(MAX_UINT64)
              })
            })

            context("when delay did not pass", () => {
              before(async () => {
                await createSnapshot()

                await helpers.time.increaseTime(
                  params.authorizationDecreaseDelay - 60 // -1min
                )

                await allowlist
                  .connect(deployer)
                  .requestWeightDecrease(
                    stakingProvider.address,
                    (
                      await allowlist.authorizedStake(
                        stakingProvider.address,
                        walletRegistry.address
                      )
                    ).sub(deauthorizingSecond)
                  )
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should overwrite the previous request", async () => {
                expect(
                  await walletRegistry.pendingAuthorizationDecrease(
                    stakingProvider.address
                  )
                ).to.be.equal(deauthorizingSecond)
              })

              it("should require updating the pool before approving", async () => {
                expect(
                  await walletRegistry.remainingAuthorizationDecreaseDelay(
                    stakingProvider.address
                  )
                ).to.equal(MAX_UINT64)
              })
            })
          })
        })

        context("when change period is zero", () => {
          before(async () => {
            await createSnapshot()

            await walletRegistryGovernance
              .connect(governance)
              .beginAuthorizationDecreaseChangePeriodUpdate(0)
            await helpers.time.increaseTime(constants.governanceDelay)
            await walletRegistryGovernance
              .connect(governance)
              .finalizeAuthorizationDecreaseChangePeriodUpdate()
          })

          after(async () => {
            await restoreSnapshot()
          })

          context("when called before sortition pool was updated", () => {
            before(async () => {
              await createSnapshot()

              await allowlist
                .connect(deployer)
                .requestWeightDecrease(
                  stakingProvider.address,
                  (
                    await allowlist.authorizedStake(
                      stakingProvider.address,
                      walletRegistry.address
                    )
                  ).sub(deauthorizingSecond)
                )
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should overwrite the previous request", async () => {
              expect(
                await walletRegistry.pendingAuthorizationDecrease(
                  stakingProvider.address
                )
              ).to.be.equal(deauthorizingSecond)
            })

            it("should require updating the pool before approving", async () => {
              expect(
                await walletRegistry.remainingAuthorizationDecreaseDelay(
                  stakingProvider.address
                )
              ).to.equal(MAX_UINT64)
            })
          })

          context("when called after sortition pool was updated", () => {
            before(async () => {
              await createSnapshot()

              await walletRegistry.updateOperatorStatus(operator.address)
            })

            after(async () => {
              await restoreSnapshot()
            })

            context("when called before delay passed", () => {
              it("should revert", async () => {
                await expect(
                  allowlist
                    .connect(deployer)
                    .requestWeightDecrease(
                      stakingProvider.address,
                      (
                        await allowlist.authorizedStake(
                          stakingProvider.address,
                          walletRegistry.address
                        )
                      ).sub(deauthorizingSecond)
                    )
                ).to.be.revertedWith(
                  "Not enough time passed since the original request"
                )
              })
            })

            context("when called after delay passed", () => {
              before(async () => {
                await createSnapshot()
                await helpers.time.increaseTime(
                  params.authorizationDecreaseDelay
                )

                await allowlist
                  .connect(deployer)
                  .requestWeightDecrease(
                    stakingProvider.address,
                    (
                      await allowlist.authorizedStake(
                        stakingProvider.address,
                        walletRegistry.address
                      )
                    ).sub(deauthorizingSecond)
                  )
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should overwrite the previous request", async () => {
                expect(
                  await walletRegistry.pendingAuthorizationDecrease(
                    stakingProvider.address
                  )
                ).to.be.equal(deauthorizingSecond)
              })

              it("should require updating the pool before approving", async () => {
                expect(
                  await walletRegistry.remainingAuthorizationDecreaseDelay(
                    stakingProvider.address
                  )
                ).to.equal(MAX_UINT64)
              })
            })
          })
        })

        context("when change period is not equal delay and is non-zero", () => {
          const newChangePeriod = 3600 // 1h before delay end

          before(async () => {
            await createSnapshot()

            await walletRegistryGovernance
              .connect(governance)
              .beginAuthorizationDecreaseChangePeriodUpdate(newChangePeriod)
            await helpers.time.increaseTime(constants.governanceDelay)
            await walletRegistryGovernance
              .connect(governance)
              .finalizeAuthorizationDecreaseChangePeriodUpdate()
          })

          after(async () => {
            await restoreSnapshot()
          })

          context("when called before sortition pool was updated", () => {
            before(async () => {
              await createSnapshot()

              await allowlist
                .connect(deployer)
                .requestWeightDecrease(
                  stakingProvider.address,
                  (
                    await allowlist.authorizedStake(
                      stakingProvider.address,
                      walletRegistry.address
                    )
                  ).sub(deauthorizingSecond)
                )
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should overwrite the previous request", async () => {
              expect(
                await walletRegistry.pendingAuthorizationDecrease(
                  stakingProvider.address
                )
              ).to.be.equal(deauthorizingSecond)
            })

            it("should require updating the pool before approving", async () => {
              expect(
                await walletRegistry.remainingAuthorizationDecreaseDelay(
                  stakingProvider.address
                )
              ).to.equal(MAX_UINT64)
            })
          })

          context("when called after sortition pool was updated", () => {
            before(async () => {
              await createSnapshot()

              await walletRegistry.updateOperatorStatus(operator.address)
            })

            after(async () => {
              await restoreSnapshot()
            })

            context("when change period did not activate", () => {
              before(async () => {
                await createSnapshot()
                await helpers.time.increaseTime(
                  params.authorizationDecreaseDelay - newChangePeriod - 60 // -1min
                )
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should revert", async () => {
                await expect(
                  allowlist
                    .connect(deployer)
                    .requestWeightDecrease(
                      stakingProvider.address,
                      (
                        await allowlist.authorizedStake(
                          stakingProvider.address,
                          walletRegistry.address
                        )
                      ).sub(deauthorizingSecond)
                    )
                ).to.be.revertedWith(
                  "Not enough time passed since the original request"
                )
              })
            })

            context("when change period did activate", () => {
              before(async () => {
                await createSnapshot()
                await helpers.time.increaseTime(
                  params.authorizationDecreaseDelay - newChangePeriod + 60 // +1min
                )

                await allowlist
                  .connect(deployer)
                  .requestWeightDecrease(
                    stakingProvider.address,
                    (
                      await allowlist.authorizedStake(
                        stakingProvider.address,
                        walletRegistry.address
                      )
                    ).sub(deauthorizingSecond)
                  )
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should overwrite the previous request", async () => {
                expect(
                  await walletRegistry.pendingAuthorizationDecrease(
                    stakingProvider.address
                  )
                ).to.be.equal(deauthorizingSecond)
              })

              it("should require updating the pool before approving", async () => {
                expect(
                  await walletRegistry.remainingAuthorizationDecreaseDelay(
                    stakingProvider.address
                  )
                ).to.equal(MAX_UINT64)
              })
            })

            context("when delay passed", () => {
              before(async () => {
                await createSnapshot()
                await helpers.time.increaseTime(
                  params.authorizationDecreaseDelay
                )

                await allowlist
                  .connect(deployer)
                  .requestWeightDecrease(
                    stakingProvider.address,
                    (
                      await allowlist.authorizedStake(
                        stakingProvider.address,
                        walletRegistry.address
                      )
                    ).sub(deauthorizingSecond)
                  )
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should overwrite the previous request", async () => {
                expect(
                  await walletRegistry.pendingAuthorizationDecrease(
                    stakingProvider.address
                  )
                ).to.be.equal(deauthorizingSecond)
              })

              it("should require updating the pool before approving", async () => {
                expect(
                  await walletRegistry.remainingAuthorizationDecreaseDelay(
                    stakingProvider.address
                  )
                ).to.equal(MAX_UINT64)
              })
            })
          })
        })
      })
    })
  })

  describe("approveAuthorizationDecrease", () => {
    before(async () => {
      await createSnapshot()
      await allowlist
        .connect(deployer)
        .addStakingProvider(stakingProvider.address, providerWeight)
    })

    after(async () => {
      await restoreSnapshot()
    })

    context("when decrease was not requested", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.approveAuthorizationDecrease(stakingProvider.address)
        ).to.be.revertedWith("Authorization decrease not requested")
      })
    })

    context("when the operator is unknown", () => {
      context("when the decrease was requested", () => {
        before(async () => {
          await createSnapshot()

          const deauthorizingBy = providerWeight

          await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(deauthorizingBy)
            )
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should let to approve immediately", async () => {
          const tx = await walletRegistry.approveAuthorizationDecrease(
            stakingProvider.address
          )
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationDecreaseApproved")
            .withArgs(stakingProvider.address)
        })
      })
    })

    context("when the operator is registered", () => {
      before(async () => {
        await createSnapshot()

        await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)

        const deauthorizingBy = providerWeight
        await allowlist
          .connect(deployer)
          .requestWeightDecrease(
            stakingProvider.address,
            (
              await allowlist.authorizedStake(
                stakingProvider.address,
                walletRegistry.address
              )
            ).sub(deauthorizingBy)
          )
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("when the pool was not updated", () => {
        before(async () => {
          await createSnapshot()
          await helpers.time.increaseTime(params.authorizationDecreaseDelay)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should revert", async () => {
          await expect(
            walletRegistry.approveAuthorizationDecrease(stakingProvider.address)
          ).to.be.revertedWith("Authorization decrease request not activated")
        })
      })

      context("when the pool was updated but the delay did not pass", () => {
        before(async () => {
          await createSnapshot()

          await walletRegistry.updateOperatorStatus(operator.address)
          await helpers.time.increaseTime(
            params.authorizationDecreaseDelay - 60 // -1min
          )
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should revert", async () => {
          await expect(
            walletRegistry.approveAuthorizationDecrease(stakingProvider.address)
          ).to.be.revertedWith("Authorization decrease delay not passed")
        })
      })

      context("when the pool was updated and the delay passed", () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistry.updateOperatorStatus(operator.address)
          await helpers.time.increaseTime(params.authorizationDecreaseDelay)

          tx = await walletRegistry.approveAuthorizationDecrease(
            stakingProvider.address
          )
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should reduce authorized stake amount", async () => {
          expect(await sortitionPool.getPoolWeight(operator.address)).to.equal(
            0
          )
        })

        it("should emit AuthorizationDecreaseApproved event", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "AuthorizationDecreaseApproved")
            .withArgs(stakingProvider.address)
        })

        it("should clear pending authorization decrease", async () => {
          expect(
            await walletRegistry.pendingAuthorizationDecrease(
              stakingProvider.address
            )
          ).to.equal(0)
        })
      })
    })
  })

  describe("joinSortitionPool", () => {
    context("when the operator is unknown", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(thirdParty).joinSortitionPool()
        ).to.be.revertedWith("Unknown operator")
      })
    })

    context("when the operator has no stake authorized", () => {
      before(async () => {
        await createSnapshot()

        await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should revert", async () => {
        await expect(
          walletRegistry.connect(operator).joinSortitionPool()
        ).to.be.revertedWith("Authorization below the minimum")
      })
    })

    context("when the operator has the minimum stake authorized", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)

        await allowlist
          .connect(deployer)
          .addStakingProvider(stakingProvider.address, minimumAuthorization)

        tx = await walletRegistry.connect(operator).joinSortitionPool()
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should insert operator into the pool", async () => {
        expect(await walletRegistry.isOperatorInPool(operator.address)).to.be
          .true
      })

      it("should use a correct stake weight", async () => {
        expect(await sortitionPool.getPoolWeight(operator.address)).to.equal(
          minimumAuthorization.div(constants.poolWeightDivisor)
        )
      })

      it("should emit OperatorJoinedSortitionPool", async () => {
        await expect(tx)
          .to.emit(walletRegistry, "OperatorJoinedSortitionPool")
          .withArgs(stakingProvider.address, operator.address)
      })
    })

    context(
      "when the operator has more than the minimum stake authorized",
      () => {
        let authorizedStake

        before(async () => {
          await createSnapshot()

          await walletRegistry
            .connect(stakingProvider)
            .registerOperator(operator.address)

          authorizedStake = minimumAuthorization.mul(2)

          await allowlist
            .connect(deployer)
            .addStakingProvider(stakingProvider.address, authorizedStake)

          await walletRegistry.connect(operator).joinSortitionPool()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should insert operator into the pool", async () => {
          expect(await walletRegistry.isOperatorInPool(operator.address)).to.be
            .true
        })

        it("should use a correct stake weight", async () => {
          expect(await sortitionPool.getPoolWeight(operator.address)).to.equal(
            authorizedStake.div(constants.poolWeightDivisor)
          )
        })
      }
    )

    context("when operator is in the process of deauthorizing", () => {
      let deauthorizingTo

      before(async () => {
        await createSnapshot()

        await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)

        const authorizedStake = providerWeight

        await allowlist
          .connect(deployer)
          .addStakingProvider(stakingProvider.address, authorizedStake)

        deauthorizingTo = minimumAuthorization.add(to1e18(1337))
        const deauthorizingBy = authorizedStake.sub(deauthorizingTo)

        await allowlist
          .connect(deployer)
          .requestWeightDecrease(
            stakingProvider.address,
            (
              await allowlist.authorizedStake(
                stakingProvider.address,
                walletRegistry.address
              )
            ).sub(deauthorizingBy)
          )

        await walletRegistry.connect(operator).joinSortitionPool()
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should insert operator into the pool", async () => {
        expect(await walletRegistry.isOperatorInPool(operator.address)).to.be
          .true
      })

      it("should use a correct stake weight", async () => {
        expect(await sortitionPool.getPoolWeight(operator.address)).to.equal(
          deauthorizingTo.div(constants.poolWeightDivisor)
        )
      })

      it("should activate authorization decrease delay", async () => {
        expect(
          await walletRegistry.remainingAuthorizationDecreaseDelay(
            stakingProvider.address
          )
        ).to.equal(params.authorizationDecreaseDelay)
      })
    })
  })

  describe("updateOperatorStatus", () => {
    context("when the operator is unknown", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.updateOperatorStatus(thirdParty.address)
        ).to.be.revertedWith("Unknown operator")
      })
    })

    context("when operator is not in the sortition pool", () => {
      before(async () => {
        await createSnapshot()

        await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("when the authorization increased", () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await allowlist
            .connect(deployer)
            .addStakingProvider(stakingProvider.address, minimumAuthorization)

          tx = await walletRegistry
            .connect(thirdParty)
            .updateOperatorStatus(operator.address)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should not insert operator into the pool", async () => {
          expect(await walletRegistry.isOperatorInPool(operator.address)).to.be
            .false
        })

        it("should emit OperatorStatusUpdated", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "OperatorStatusUpdated")
            .withArgs(stakingProvider.address, operator.address)
        })
      })

      context("when there was an authorization decrease request", () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await allowlist
            .connect(deployer)
            .addStakingProvider(stakingProvider.address, providerWeight)

          const deauthorizingBy = to1e18(100)
          await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(deauthorizingBy)
            )

          tx = await walletRegistry
            .connect(thirdParty)
            .updateOperatorStatus(operator.address)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should not insert operator into the pool", async () => {
          expect(await walletRegistry.isOperatorInPool(operator.address)).to.be
            .false
        })

        it("should activate authorization decrease delay", async () => {
          expect(
            await walletRegistry.remainingAuthorizationDecreaseDelay(
              stakingProvider.address
            )
          ).to.equal(params.authorizationDecreaseDelay)
        })

        it("should emit OperatorStatusUpdated", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "OperatorStatusUpdated")
            .withArgs(stakingProvider.address, operator.address)
        })
      })
    })

    context("when operator is in the sortition pool", () => {
      before(async () => {
        await createSnapshot()

        await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)

        await allowlist
          .connect(deployer)
          .addStakingProvider(
            stakingProvider.address,
            minimumAuthorization.mul(2)
          )

        await walletRegistry.connect(operator).joinSortitionPool()
      })

      after(async () => {
        await restoreSnapshot()
      })

      context(
        "when there was an authorization decrease request to non-zero",
        () => {
          let tx: ContractTransaction
          let expectedWeight

          before(async () => {
            await createSnapshot()
            const deauthorizingTo = minimumAuthorization.add(to1e18(1337))
            const deauthorizingBy = minimumAuthorization
              .mul(2)
              .sub(deauthorizingTo)
            expectedWeight = deauthorizingTo.div(constants.poolWeightDivisor)

            await allowlist
              .connect(deployer)
              .requestWeightDecrease(
                stakingProvider.address,
                (
                  await allowlist.authorizedStake(
                    stakingProvider.address,
                    walletRegistry.address
                  )
                ).sub(deauthorizingBy)
              )

            tx = await walletRegistry
              .connect(thirdParty)
              .updateOperatorStatus(operator.address)
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should update the pool", async () => {
            expect(
              await sortitionPool.getPoolWeight(operator.address)
            ).to.equal(expectedWeight)
          })

          it("should activate authorization decrease delay", async () => {
            expect(
              await walletRegistry.remainingAuthorizationDecreaseDelay(
                stakingProvider.address
              )
            ).to.equal(params.authorizationDecreaseDelay)
          })

          it("should emit OperatorStatusUpdated", async () => {
            await expect(tx)
              .to.emit(walletRegistry, "OperatorStatusUpdated")
              .withArgs(stakingProvider.address, operator.address)
          })
        }
      )

      context(
        "when there was an authorization decrease request to zero",
        () => {
          let tx: ContractTransaction

          before(async () => {
            await createSnapshot()
            const deauthorizingBy = minimumAuthorization.mul(2)

            await allowlist
              .connect(deployer)
              .requestWeightDecrease(
                stakingProvider.address,
                (
                  await allowlist.authorizedStake(
                    stakingProvider.address,
                    walletRegistry.address
                  )
                ).sub(deauthorizingBy)
              )

            tx = await walletRegistry
              .connect(thirdParty)
              .updateOperatorStatus(operator.address)
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should remove operator from the sortition pool", async () => {
            expect(await walletRegistry.isOperatorInPool(operator.address)).to
              .be.false
          })

          it("should activate authorization decrease delay", async () => {
            expect(
              await walletRegistry.remainingAuthorizationDecreaseDelay(
                stakingProvider.address
              )
            ).to.equal(params.authorizationDecreaseDelay)
          })

          it("should emit OperatorStatusUpdated", async () => {
            await expect(tx)
              .to.emit(walletRegistry, "OperatorStatusUpdated")
              .withArgs(stakingProvider.address, operator.address)
          })
        }
      )
    })
  })

  describe("eligibleStake", () => {
    context("when staking provider has no stake authorized", () => {
      it("should return zero", async () => {
        expect(
          await walletRegistry.eligibleStake(stakingProvider.address)
        ).to.equal(0)
      })
    })

    context("when staking provider has stake authorized", () => {
      let authorizedAmount

      before(async () => {
        await createSnapshot()

        authorizedAmount = minimumAuthorization
        await allowlist
          .connect(deployer)
          .addStakingProvider(stakingProvider.address, authorizedAmount)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should return authorized amount", async () => {
        expect(
          await walletRegistry.eligibleStake(stakingProvider.address)
        ).to.equal(authorizedAmount)
      })
    })

    context(
      "when staking provider has some part of the stake deauthorizing",
      () => {
        let authorizedAmount
        let deauthorizingAmount

        before(async () => {
          await createSnapshot()

          authorizedAmount = minimumAuthorization.add(to1e18(2000))

          await allowlist
            .connect(deployer)
            .addStakingProvider(stakingProvider.address, authorizedAmount)

          deauthorizingAmount = to1e18(1337)
          await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(deauthorizingAmount)
            )
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should return authorized amount minus deauthorizing amount", async () => {
          expect(
            await walletRegistry.eligibleStake(stakingProvider.address)
          ).to.equal(authorizedAmount.sub(deauthorizingAmount))
        })
      }
    )

    context("when staking provider has all of the stake deauthorizing", () => {
      before(async () => {
        await createSnapshot()

        const authorizedAmount = minimumAuthorization
        await allowlist
          .connect(deployer)
          .addStakingProvider(stakingProvider.address, authorizedAmount)

        await allowlist
          .connect(deployer)
          .requestWeightDecrease(
            stakingProvider.address,
            (
              await allowlist.authorizedStake(
                stakingProvider.address,
                walletRegistry.address
              )
            ).sub(authorizedAmount)
          )
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should return zero", async () => {
        expect(
          await walletRegistry.eligibleStake(stakingProvider.address)
        ).to.equal(0)
      })
    })

    context("when staking provider has all of the stake deauthorized", () => {
      before(async () => {
        await createSnapshot()

        const authorizedAmount = minimumAuthorization.add(1200)
        await allowlist
          .connect(deployer)
          .addStakingProvider(stakingProvider.address, authorizedAmount)

        await allowlist
          .connect(deployer)
          .requestWeightDecrease(
            stakingProvider.address,
            (
              await allowlist.authorizedStake(
                stakingProvider.address,
                walletRegistry.address
              )
            ).sub(authorizedAmount)
          )

        await walletRegistry.approveAuthorizationDecrease(
          stakingProvider.address
        )
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should return zero", async () => {
        expect(
          await walletRegistry.eligibleStake(stakingProvider.address)
        ).to.equal(0)
      })
    })
  })

  describe("remainingAuthorizationDecreaseDelay", () => {
    before(async () => {
      await createSnapshot()

      const authorizedAmount = minimumAuthorization.add(1200)
      await allowlist
        .connect(deployer)
        .addStakingProvider(stakingProvider.address, authorizedAmount)

      await walletRegistry
        .connect(stakingProvider)
        .registerOperator(operator.address)
      await walletRegistry.connect(operator).joinSortitionPool()

      await allowlist
        .connect(deployer)
        .requestWeightDecrease(
          stakingProvider.address,
          (
            await allowlist.authorizedStake(
              stakingProvider.address,
              walletRegistry.address
            )
          ).sub(authorizedAmount)
        )
    })

    after(async () => {
      await restoreSnapshot()
    })

    it("should not activate before sortition pool is updated", async () => {
      expect(
        await walletRegistry.remainingAuthorizationDecreaseDelay(
          stakingProvider.address
        )
      ).to.equal(MAX_UINT64)
    })

    it("should activate after updating sortition pool", async () => {
      await walletRegistry.updateOperatorStatus(operator.address)
      expect(
        await walletRegistry.remainingAuthorizationDecreaseDelay(
          stakingProvider.address
        )
      ).to.equal(params.authorizationDecreaseDelay)
    })

    it("should reduce over time", async () => {
      await walletRegistry.updateOperatorStatus(operator.address)
      await helpers.time.increaseTime(params.authorizationDecreaseDelay / 2)
      expect(
        await walletRegistry.remainingAuthorizationDecreaseDelay(
          stakingProvider.address
        )
      ).to.be.closeTo(
        ethers.BigNumber.from(params.authorizationDecreaseDelay / 2),
        5 // +- 5sec
      )
    })

    it("should eventually go to zero", async () => {
      await walletRegistry.updateOperatorStatus(operator.address)
      await helpers.time.increaseTime(params.authorizationDecreaseDelay)
      expect(
        await walletRegistry.remainingAuthorizationDecreaseDelay(
          stakingProvider.address
        )
      ).to.equal(0)
      await helpers.time.increaseTime(3600) // +1h
      expect(
        await walletRegistry.remainingAuthorizationDecreaseDelay(
          stakingProvider.address
        )
      ).to.equal(0)
    })
  })

  describe("isOperatorUpToDate", () => {
    context("when the operator is unknown", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.isOperatorUpToDate(thirdParty.address)
        ).to.be.revertedWith("Unknown operator")
      })
    })

    context("when the operator is not in the sortition pool", () => {
      before(async () => {
        await createSnapshot()

        await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("when the operator has no authorized stake", () => {
        it("should return true", async () => {
          expect(await walletRegistry.isOperatorUpToDate(operator.address)).to
            .be.true
        })
      })

      context("when the operator has authorized stake", () => {
        before(async () => {
          await createSnapshot()

          await allowlist
            .connect(deployer)
            .addStakingProvider(stakingProvider.address, minimumAuthorization)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should return false", async () => {
          expect(await walletRegistry.isOperatorUpToDate(operator.address)).to
            .be.false
        })
      })
    })

    context("when the operator is in the sortition pool", () => {
      before(async () => {
        await createSnapshot()

        await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)

        await allowlist
          .connect(deployer)
          .addStakingProvider(
            stakingProvider.address,
            minimumAuthorization.mul(2)
          )

        await walletRegistry.connect(operator).joinSortitionPool()
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("when the operator just joined the pool", () => {
        it("should return true", async () => {
          expect(await walletRegistry.isOperatorUpToDate(operator.address)).to
            .be.true
        })
      })

      context("when authorization decrease was requested", () => {
        before(async () => {
          await createSnapshot()

          const deauthorizingBy = to1e18(1)
          await allowlist
            .connect(deployer)
            .requestWeightDecrease(
              stakingProvider.address,
              (
                await allowlist.authorizedStake(
                  stakingProvider.address,
                  walletRegistry.address
                )
              ).sub(deauthorizingBy)
            )
        })

        after(async () => {
          await restoreSnapshot()
        })

        context("when sortition pool was not yet updated", () => {
          it("should return false", async () => {
            expect(await walletRegistry.isOperatorUpToDate(operator.address)).to
              .be.false
          })
        })

        context("when the sortition pool was updated", () => {
          it("should return true", async () => {
            await walletRegistry.updateOperatorStatus(operator.address)
            expect(await walletRegistry.isOperatorUpToDate(operator.address)).to
              .be.true
          })
        })
      })
    })
  })

  describe("Allowlist provider lifecycle", () => {
    beforeEach(async () => {
      await createSnapshot()
    })

    afterEach(async () => {
      await restoreSnapshot()
    })

    it("should restrict adding providers and decreasing weights to the Allowlist owner", async () => {
      await expect(
        allowlist
          .connect(thirdParty)
          .addStakingProvider(stakingProvider.address, providerWeight)
      ).to.be.revertedWith("Ownable: caller is not the owner")
      await allowlist
        .connect(deployer)
        .addStakingProvider(stakingProvider.address, providerWeight)
      await expect(
        allowlist
          .connect(thirdParty)
          .requestWeightDecrease(stakingProvider.address, minimumAuthorization)
      ).to.be.revertedWith("Ownable: caller is not the owner")
    })

    it("should reject adding weight to an already allowlisted provider", async () => {
      await allowlist
        .connect(deployer)
        .addStakingProvider(stakingProvider.address, providerWeight)
      await expect(
        allowlist
          .connect(deployer)
          .addStakingProvider(stakingProvider.address, providerWeight)
      ).to.be.revertedWithCustomError(allowlist, "StakingProviderAlreadyAdded")
      expect(
        await walletRegistry.eligibleStake(stakingProvider.address)
      ).to.equal(providerWeight)
    })

    const targetWeights = [params.minimumAuthorization, ethers.constants.Zero]
    targetWeights.forEach((targetWeight) => {
      it(`should finalize a decrease to ${targetWeight} after synchronizing the pool and waiting`, async () => {
        await allowlist
          .connect(deployer)
          .addStakingProvider(stakingProvider.address, providerWeight)
        await walletRegistry
          .connect(stakingProvider)
          .registerOperator(operator.address)
        await walletRegistry.connect(operator).joinSortitionPool()
        await allowlist
          .connect(deployer)
          .requestWeightDecrease(stakingProvider.address, targetWeight)
        expect(
          await allowlist.authorizedStake(
            stakingProvider.address,
            walletRegistry.address
          )
        ).to.equal(providerWeight)
        expect(
          await walletRegistry.eligibleStake(stakingProvider.address)
        ).to.equal(targetWeight)
        await walletRegistry
          .connect(thirdParty)
          .updateOperatorStatus(operator.address)
        await helpers.time.increaseTime(params.authorizationDecreaseDelay)
        const tx = await walletRegistry
          .connect(thirdParty)
          .approveAuthorizationDecrease(stakingProvider.address)
        await expect(tx)
          .to.emit(allowlist, "WeightDecreaseFinalized")
          .withArgs(stakingProvider.address, providerWeight, targetWeight)
        const info = await allowlist.stakingProviders(stakingProvider.address)
        expect(info.weight).to.equal(targetWeight)
        expect(info.pendingNewWeight).to.equal(0)
        expect(info.decreasePending).to.be.false
        expect(
          await walletRegistry.pendingAuthorizationDecrease(
            stakingProvider.address
          )
        ).to.equal(0)
        expect(await walletRegistry.isOperatorUpToDate(operator.address)).to.be
          .true
      })
    })

    it("should allow a removed provider to be added again with the same operator", async () => {
      await allowlist
        .connect(deployer)
        .addStakingProvider(stakingProvider.address, providerWeight)
      await walletRegistry
        .connect(stakingProvider)
        .registerOperator(operator.address)
      await walletRegistry.connect(operator).joinSortitionPool()
      await allowlist
        .connect(deployer)
        .requestWeightDecrease(stakingProvider.address, 0)
      await walletRegistry.updateOperatorStatus(operator.address)
      await helpers.time.increaseTime(params.authorizationDecreaseDelay)
      await walletRegistry.approveAuthorizationDecrease(stakingProvider.address)
      expect(await walletRegistry.isOperatorInPool(operator.address)).to.be
        .false
      await allowlist
        .connect(deployer)
        .addStakingProvider(stakingProvider.address, minimumAuthorization)
      await walletRegistry.connect(operator).joinSortitionPool()
      expect(
        await walletRegistry.stakingProviderToOperator(stakingProvider.address)
      ).to.equal(operator.address)
      expect(await sortitionPool.getPoolWeight(operator.address)).to.equal(
        minimumAuthorization.div(constants.poolWeightDivisor)
      )
      expect(await walletRegistry.isOperatorUpToDate(operator.address)).to.be
        .true
    })

    it("should defer pool synchronization while a wallet is being created", async () => {
      await allowlist
        .connect(deployer)
        .addStakingProvider(stakingProvider.address, providerWeight)
      await walletRegistry
        .connect(stakingProvider)
        .registerOperator(operator.address)
      await walletRegistry.connect(operator).joinSortitionPool()
      await walletRegistry.connect(walletOwnerSigner).requestNewWallet()
      await allowlist
        .connect(deployer)
        .requestWeightDecrease(stakingProvider.address, minimumAuthorization)
      await expect(
        walletRegistry.updateOperatorStatus(operator.address)
      ).to.be.revertedWith("Sortition pool locked")
      expect(
        await walletRegistry.remainingAuthorizationDecreaseDelay(
          stakingProvider.address
        )
      ).to.equal(MAX_UINT64)
      await helpers.time.mineBlocks(params.dkgSeedTimeout)
      await walletRegistry.notifySeedTimeout()
      await walletRegistry.updateOperatorStatus(operator.address)
      expect(
        await walletRegistry.remainingAuthorizationDecreaseDelay(
          stakingProvider.address
        )
      ).to.equal(params.authorizationDecreaseDelay)
      expect(await sortitionPool.getPoolWeight(operator.address)).to.equal(
        minimumAuthorization.div(constants.poolWeightDivisor)
      )
    })

    const callbacks = [
      "authorizationIncreased",
      "authorizationDecreaseRequested",
      "involuntaryAuthorizationDecrease",
    ] as const
    callbacks.forEach((method) => {
      it(`should reject ${method} from the legacy staking contract after migration`, async () => {
        const stakingAddress = await walletRegistry.staking()
        await ethers.provider.send("hardhat_impersonateAccount", [
          stakingAddress,
        ])
        await ethers.provider.send("hardhat_setBalance", [
          stakingAddress,
          "0x56BC75E2D63100000",
        ])
        try {
          const signer = await ethers.getSigner(stakingAddress)
          await expect(
            walletRegistry
              .connect(signer)
              [method](
                stakingProvider.address,
                providerWeight,
                minimumAuthorization
              )
          ).to.be.revertedWithCustomError(
            walletRegistry,
            "CallerNotStakingContract"
          )
        } finally {
          await ethers.provider.send("hardhat_stopImpersonatingAccount", [
            stakingAddress,
          ])
        }
      })
    })
  })
})

describe("WalletRegistry - Allowlist migration", () => {
  let walletRegistry: WalletRegistry
  let deployer: SignerWithAddress
  let stakingProvider: SignerWithAddress
  let operator: SignerWithAddress

  beforeEach(async () => {
    await deployments.fixture()
    walletRegistry = await helpers.contracts.getContract("WalletRegistry")
    const signers = await helpers.signers.getNamedSigners()
    deployer = signers.deployer
    stakingProvider = signers.esdm
    operator = signers.chaosnetOwner
    const sortitionPool: SortitionPool = await helpers.contracts.getContract(
      "EcdsaSortitionPool"
    )
    await sortitionPool.connect(signers.chaosnetOwner).deactivateChaosnet()
  })

  it("should preserve existing operator registrations when switching authorization to the Allowlist", async () => {
    await walletRegistry
      .connect(stakingProvider)
      .registerOperator(operator.address)
    expect(await walletRegistry.allowlist()).to.equal(ZERO_ADDRESS)
    expect(
      await walletRegistry.eligibleStake(stakingProvider.address)
    ).to.equal(0)
    const allowlist = await setupAllowlist(walletRegistry, deployer)
    await allowlist
      .connect(deployer)
      .addStakingProvider(stakingProvider.address, params.minimumAuthorization)
    expect(await walletRegistry.allowlist()).to.equal(allowlist.address)
    expect(
      await walletRegistry.stakingProviderToOperator(stakingProvider.address)
    ).to.equal(operator.address)
    expect(
      await walletRegistry.eligibleStake(stakingProvider.address)
    ).to.equal(params.minimumAuthorization)
    await walletRegistry.connect(operator).joinSortitionPool()
    expect(await walletRegistry.isOperatorUpToDate(operator.address)).to.be.true
  })

  it("should reject a zero Allowlist address", async () => {
    await expect(
      walletRegistry.initializeV2(ZERO_ADDRESS)
    ).to.be.revertedWithCustomError(walletRegistry, "AllowlistAddressZero")
  })

  it("should keep the Allowlist address after a rejected reinitialization", async () => {
    const allowlist = await setupAllowlist(walletRegistry, deployer)
    await expect(
      walletRegistry.initializeV2(operator.address)
    ).to.be.revertedWith("Initializable: contract is already initialized")
    expect(await walletRegistry.allowlist()).to.equal(allowlist.address)
  })
})
