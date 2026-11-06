"""Run the deployed WalletRegistry callback bytecode under Osaka/Amsterdam."""

import json
from pathlib import Path

from ethereum.crypto.hash import Hash32, keccak256
from ethereum.state import Account, Address
from ethereum.state_mpt import State, set_account, set_storage, store_code
from ethereum_types.bytes import Bytes, Bytes32
from ethereum_types.numeric import U64, U256, Uint


ORIGIN = Address(bytes.fromhex("5499f54b4a1cb4816eefcf78962040461be3d80b"))
TARGET = Address(bytes.fromhex("46d52e41c2f300bc82217ce22b920c34995204eb"))
IMPLEMENTATION = Address(bytes.fromhex("420c99a4b0e4cb65c4cabf2b4d2526372aa1ac6e"))
ADMIN = Address(bytes.fromhex("44" * 20))
SORTITION_POOL = Address(bytes.fromhex("33" * 20))
ZERO32 = Bytes32(bytes(32))
TRUE_RETURNER = Bytes.fromhex("600160005260206000f3")
PROXY_CODE = Bytes.fromhex("60806040526004361061004e5760003560e01c80633659cfe6146100655780634f1ef286146100855780635c60da1b146100985780638f283970146100c9578063f851a440146100e95761005d565b3661005d5761005b6100fe565b005b61005b6100fe565b34801561007157600080fd5b5061005b6100803660046106ed565b610118565b61005b610093366004610707565b610164565b3480156100a457600080fd5b506100ad6101da565b6040516001600160a01b03909116815260200160405180910390f35b3480156100d557600080fd5b5061005b6100e43660046106ed565b610217565b3480156100f557600080fd5b506100ad610241565b6101066102a2565b610116610111610346565b610355565b565b610120610379565b6001600160a01b0316336001600160a01b0316141561015957610154816040518060200160405280600081525060006103ac565b610161565b6101616100fe565b50565b61016c610379565b6001600160a01b0316336001600160a01b031614156101cd576101c88383838080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250600192506103ac915050565b6101d5565b6101d56100fe565b505050565b60006101e4610379565b6001600160a01b0316336001600160a01b0316141561020c57610205610346565b9050610214565b6102146100fe565b90565b61021f610379565b6001600160a01b0316336001600160a01b03161415610159576101548161040b565b600061024b610379565b6001600160a01b0316336001600160a01b0316141561020c57610205610379565b606061029183836040518060600160405280602781526020016108016027913961045f565b9392505050565b803b15155b919050565b6102aa610379565b6001600160a01b0316336001600160a01b031614156103415760405162461bcd60e51b815260206004820152604260248201527f5472616e73706172656e745570677261646561626c6550726f78793a2061646d60448201527f696e2063616e6e6f742066616c6c6261636b20746f2070726f78792074617267606482015261195d60f21b608482015260a4015b60405180910390fd5b610116565b600061035061053a565b905090565b3660008037600080366000845af43d6000803e808015610374573d6000f35b3d6000fd5b60007fb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d61035b546001600160a01b0316905090565b6103b583610562565b6040516001600160a01b038416907fbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b90600090a26000825111806103f65750805b156101d557610405838361026c565b50505050565b7f7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f610434610379565b604080516001600160a01b03928316815291841660208301520160405180910390a161016181610611565b606061046a84610298565b6104c55760405162461bcd60e51b815260206004820152602660248201527f416464726573733a2064656c65676174652063616c6c20746f206e6f6e2d636f6044820152651b9d1c9858dd60d21b6064820152608401610338565b600080856001600160a01b0316856040516104e09190610785565b600060405180830381855af49150503d806000811461051b576040519150601f19603f3d011682016040523d82523d6000602084013e610520565b606091505b509150915061053082828661069d565b9695505050505050565b60007f360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc61039d565b61056b81610298565b6105cd5760405162461bcd60e51b815260206004820152602d60248201527f455243313936373a206e657720696d706c656d656e746174696f6e206973206e60448201526c1bdd08184818dbdb9d1c9858dd609a1b6064820152608401610338565b807f360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc5b80546001600160a01b0319166001600160a01b039290921691909117905550565b6001600160a01b0381166106765760405162461bcd60e51b815260206004820152602660248201527f455243313936373a206e65772061646d696e20697320746865207a65726f206160448201526564647265737360d01b6064820152608401610338565b807fb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d61036105f0565b606083156106ac575081610291565b8251156106bc5782518084602001fd5b8160405162461bcd60e51b815260040161033891906107a1565b80356001600160a01b038116811461029d57600080fd5b6000602082840312156106fe578081fd5b610291826106d6565b60008060006040848603121561071b578182fd5b610724846106d6565b9250602084013567ffffffffffffffff80821115610740578384fd5b818601915086601f830112610753578384fd5b813581811115610761578485fd5b876020828501011115610772578485fd5b6020830194508093505050509250925092565b600082516107978184602087016107d4565b9190910192915050565b60006020825282518060208401526107c08160408501602087016107d4565b601f01601f19169190910160400192915050565b60005b838110156107ef5781810151838201526020016107d7565b83811115610405575050600091015256fe416464726573733a206c6f772d6c6576656c2064656c65676174652063616c6c206661696c6564a264697066735822122093f028255035b61df476b13b9dba3c4f06f60e51b9b4caee31680b389aef327f64736f6c63430008020033")
CALLDATA = Bytes(
    keccak256(b"__beaconCallback(uint256,uint256)")[:4]
    + (1234).to_bytes(32, "big")
    + (0).to_bytes(32, "big")
)
IMPLEMENTATION_SLOT = Bytes32(
    bytes.fromhex("360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc")
)
ADMIN_SLOT = Bytes32(
    bytes.fromhex("b53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103")
)


def slot(value: int) -> Bytes32:
    return Bytes32(value.to_bytes(32, "big"))


def wallet_registry_code() -> Bytes:
    source = Path(__file__).with_name("wallet-registry-sourcify.json")
    data = json.loads(source.read_text())
    return Bytes.fromhex(data["runtimeBytecode"]["onchainBytecode"][2:])


def make_pre_state(use_proxy: bool = False) -> State:
    state = State()
    implementation_code_hash = store_code(state, wallet_registry_code())
    pool_code_hash = store_code(state, TRUE_RETURNER)
    set_account(
        state,
        ORIGIN,
        Account(nonce=Uint(0), balance=U256(10**18), code_hash=Hash32(ZERO32)),
    )
    if use_proxy:
        proxy_code_hash = store_code(state, PROXY_CODE)
        set_account(
            state,
            TARGET,
            Account(nonce=Uint(1), balance=U256(0), code_hash=proxy_code_hash),
        )
        set_account(
            state,
            IMPLEMENTATION,
            Account(
                nonce=Uint(1), balance=U256(0), code_hash=implementation_code_hash
            ),
        )
        set_storage(
            state,
            TARGET,
            IMPLEMENTATION_SLOT,
            U256.from_be_bytes(IMPLEMENTATION),
        )
        set_storage(state, TARGET, ADMIN_SLOT, U256.from_be_bytes(ADMIN))
    else:
        set_account(
            state,
            TARGET,
            Account(
                nonce=Uint(1), balance=U256(0), code_hash=implementation_code_hash
            ),
        )
    set_account(
        state,
        SORTITION_POOL,
        Account(nonce=Uint(1), balance=U256(0), code_hash=pool_code_hash),
    )
    # WalletRegistry layout: dkg begins at slot 151; its sortitionPool is +0,
    # startBlock is +8 and seed is +9. randomBeacon is slot 262.
    set_storage(state, TARGET, slot(151), U256.from_be_bytes(SORTITION_POOL))
    set_storage(state, TARGET, slot(262), U256.from_be_bytes(ORIGIN))
    return state


def result(tx_state, output, get_storage, fork: str) -> dict[str, object]:
    return {
        "fork": fork,
        "success": output.error is None,
        "start_block": int(get_storage(tx_state, TARGET, slot(159))),
        "seed": int(get_storage(tx_state, TARGET, slot(160))),
        "error": type(output.error).__name__ if output.error else None,
    }


def run_osaka(gas: int, use_proxy: bool = False) -> dict[str, object]:
    from ethereum.forks.osaka.state_tracker import (
        BlockState,
        TransactionState,
        get_storage,
    )
    from ethereum.forks.osaka.transactions import LegacyTransaction
    from ethereum.forks.osaka.utils.message import prepare_message
    from ethereum.forks.osaka.vm import BlockEnvironment, TransactionEnvironment
    from ethereum.forks.osaka.vm.interpreter import process_message_call

    tx_state = TransactionState(BlockState(make_pre_state(use_proxy)))
    block_env = BlockEnvironment(
        chain_id=U64(1),
        state=tx_state.parent,
        block_gas_limit=Uint(200_000_000),
        block_hashes=[],
        coinbase=ORIGIN,
        number=Uint(20_000_000),
        base_fee_per_gas=Uint(0),
        time=U256(0),
        prev_randao=ZERO32,
        excess_blob_gas=U64(0),
        parent_beacon_block_root=ZERO32,
    )
    tx_env = TransactionEnvironment(
        origin=ORIGIN,
        gas_price=Uint(0),
        gas=Uint(gas),
        access_list_addresses=set(),
        access_list_storage_keys=set(),
        state=tx_state,
        blob_versioned_hashes=(),
        authorizations=(),
        index_in_block=Uint(0),
        tx_hash=None,
    )
    tx = LegacyTransaction(
        nonce=U256(0),
        gas_price=Uint(0),
        gas=Uint(gas),
        to=TARGET,
        value=U256(0),
        data=CALLDATA,
        v=U256(0),
        r=U256(0),
        s=U256(0),
    )
    output = process_message_call(prepare_message(block_env, tx_env, tx))
    data = result(tx_state, output, get_storage, "osaka")
    data["path"] = "proxy" if use_proxy else "implementation"
    data["gas_left"] = int(output.gas_left)
    return data


def run_amsterdam(
    execution_gas: int, state_reservoir: int, use_proxy: bool = False
) -> dict[str, object]:
    from ethereum.forks.amsterdam.block_access_lists import BlockAccessListBuilder
    from ethereum.forks.amsterdam.fork_types import ExecutionGas, StateGas
    from ethereum.forks.amsterdam.state_tracker import (
        BlockState,
        TransactionState,
        get_storage,
    )
    from ethereum.forks.amsterdam.vm import BlockEnvironment, TransactionEnvironment
    from ethereum.forks.amsterdam.vm.interpreter import process_top_level

    tx_state = TransactionState(BlockState(make_pre_state(use_proxy)))
    block_env = BlockEnvironment(
        chain_id=U64(1),
        state=tx_state.parent,
        block_gas_limit=Uint(200_000_000),
        block_hashes=[],
        coinbase=ORIGIN,
        number=Uint(20_000_000),
        base_fee_per_gas=Uint(0),
        time=U256(0),
        prev_randao=ZERO32,
        excess_blob_gas=U64(0),
        parent_beacon_block_root=ZERO32,
        block_access_list_builder=BlockAccessListBuilder(),
        slot_number=U64(0),
    )
    tx_env = TransactionEnvironment(
        origin=ORIGIN,
        recipient=TARGET,
        is_create=False,
        data=CALLDATA,
        value=U256(0),
        gas_limit=Uint(execution_gas + state_reservoir),
        effective_gas_price=Uint(0),
        execution_gas_grant=ExecutionGas(Uint(execution_gas)),
        state_gas_reservoir=StateGas(Uint(state_reservoir)),
        calldata_floor=Uint(0),
        access_list_addresses=set(),
        access_list_storage_keys=set(),
        accounts_with_paid_writes=set(),
        state=tx_state,
        blob_versioned_hashes=(),
        authorizations=(),
        index_in_block=Uint(0),
        tx_hash=None,
    )
    output = process_top_level(block_env, tx_env)
    data = result(tx_state, output, get_storage, "amsterdam")
    data["path"] = "proxy" if use_proxy else "implementation"
    data["execution_gas_limit"] = execution_gas
    data["state_reservoir_limit"] = state_reservoir
    data["execution_gas_left"] = int(output.gas_left)
    data["state_gas_left"] = int(output.state_gas_left)
    return data


if __name__ == "__main__":
    assert run_osaka(64_000)["success"]
    assert not run_amsterdam(64_000, 0, True)["success"]
    assert run_amsterdam(300_000, 0, True)["success"]
    assert run_amsterdam(64_000, 2 * 97_920, True)["success"]
    print(run_osaka(64_000))
    print(run_amsterdam(64_000, 0))
    print(run_amsterdam(64_000, 2 * 97_920))
    for gas in range(220_000, 311_000, 10_000):
        print(run_amsterdam(gas, 0))
    for gas in (220_000, 230_000, 240_000, 250_000, 300_000):
        print(run_amsterdam(gas, 0, use_proxy=True))

    lower = 240_000
    upper = 250_000
    while lower < upper:
        candidate = (lower + upper) // 2
        if run_amsterdam(candidate, 0, use_proxy=True)["success"]:
            upper = candidate
        else:
            lower = candidate + 1
    print({"proxy_first_success": lower, "result": run_amsterdam(lower, 0, True)})
    assert lower == 240_768
