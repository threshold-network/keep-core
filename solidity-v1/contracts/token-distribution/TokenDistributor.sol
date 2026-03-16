/**
▓▓▌ ▓▓ ▐▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▄
▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
  ▓▓▓▓▓▓    ▓▓▓▓▓▓▓▀    ▐▓▓▓▓▓▓    ▐▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
  ▓▓▓▓▓▓▄▄▓▓▓▓▓▓▓▀      ▐▓▓▓▓▓▓▄▄▄▄         ▓▓▓▓▓▓▄▄▄▄         ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
  ▓▓▓▓▓▓▓▓▓▓▓▓▓▀        ▐▓▓▓▓▓▓▓▓▓▓         ▓▓▓▓▓▓▓▓▓▓▌        ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
  ▓▓▓▓▓▓▀▀▓▓▓▓▓▓▄       ▐▓▓▓▓▓▓▀▀▀▀         ▓▓▓▓▓▓▀▀▀▀         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▀
  ▓▓▓▓▓▓   ▀▓▓▓▓▓▓▄     ▐▓▓▓▓▓▓     ▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌
▓▓▓▓▓▓▓▓▓▓ █▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓

                           Trust math, not hardware.
*/

pragma solidity ^0.8.0;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/cryptography/MerkleProof.sol";

/// @title Token Distributor
/// @notice This contract can be used to distribute ERC20 tokens with a merkle
/// tree distribution mechanism in a cross-chain environment.
/// An owner of the contract has to calculate a merkle tree for tokens assignments
/// based on a mapping of a recipient addresses with the assigned tokens amount.
/// Next the owner allocates the total amount of tokens in the contract by calling
/// allocate function with the calculated merkle tree root. The owner has also
/// a possibility to define a period after which unclaimed tokens will be allowed
/// to recover by the owner. Once tokens allocation is made recipients can call
/// claim function to withdraw tokens to any destination address. The destination
/// address should be signed by the recipient prior to calling the function and
/// the signature should be provided on claim. For details of the signature see
/// claim function documentation.
/// Signing of destination addresses is implemented to allow cross-chain tokens
/// distribution. It covers a situation when recipient receives a tokens assignment
/// based on their address on some chain (A), and operates under a different address
/// on another chain (B) where tokens distribution takes place (e.g. a recipient
/// is as a multi-sig wallet on chain A and receives tokens assignment on chain B,
/// they have to sign an address on chain B they control with a signature calculated
/// with a private key controlling the address on chain A).
/// @dev This contract is based on the Uniswap's Merkle Distributor
/// https://github.com/Uniswap/merkle-distributor with some modifications:
/// - added 'allocate' function that will be called to allocate tokens for a
///   merkle root,
/// - added a possibility for a recipient to redirect tokens withdrawal to another
///   address by providing a signature over that address,
/// - added a possibility for the owner to recover unclaimed tokens after the
///   unclaimed unlock duration that is configurable on tokens allocation.
contract TokenDistributor is Ownable {
    using SafeERC20 for IERC20;

    // Distributed token.
    IERC20 public token;

    // Merkle tree root.
    bytes32 public merkleRoot;

    // Timestamp after which allocated and unclaimed tokens can be recovered from
    // the contract by the owner. If the value is zero the recovery is not possible.
    uint256 public unclaimedUnlockTimestamp;

    mapping(uint256 => uint256) private claimedBitMap;

    event TokensAllocated(
        bytes32 merkleRoot,
        uint256 amount,
        uint256 unclaimedUnlockTimestamp
    );

    event TokensClaimed(
        uint256 indexed index,
        address indexed recipient,
        address indexed destination,
        uint256 amount
    );

    event TokensRecovered(address destination, uint256 amount);

    constructor(IERC20 _token) {
        token = _token;
    }

    /// @notice Claim assigned tokens. The function can be used to withdraw tokens
    /// assigned to the recipient in the merkle tree distribution. The caller has
    /// to provide merkle data: recipient address, index, amount, and merkle proof.
    /// Anyone can call this function. The function requires a destination address
    /// for a transfer to be provided. The destination address should be signed
    /// by the recipient. To construct the message to signing recipient has to
    /// hash this contract address and destination address with keccak256 and
    /// prefix the obtained digest with Ethereum specific `\x19Ethereum Signed Message:\n32`.
    /// When using web3's signing function the prefixing is done automatically,
    /// so the signing operation can look like:
    /// `sign(soliditySha3(tokenDistributor, destination), privateKey)`.
    /// The signature should be provided with the call to this function. This is
    /// to confirm that the recipient approved the destination address for a transfer.
    /// The solution allows cross-chain allocations, where a recipient from a
    /// different chain does not operate under the same address on the chain
    /// where the tokens were allocated, (e.g. recipient is a wallet)
    /// @dev Due to the malleability concern described in EIP-2, the function expects
    /// s values in the lower half of the secp256k1 curve's order and v value of
    /// 27 or 28.
    /// @param _recipient Address that received tokens assignment.
    /// @param _destination Address to send tokens to.
    /// @param _v Destination address signature's v parameter.
    /// @param _r Destination address signature's r parameter.
    /// @param _s Destination address signature's s parameter.
    /// @param _index Merkle index.
    /// @param _amount Assigned tokens amount.
    /// @param _merkleProof Merkle proof.
    function claim(
        address _recipient,
        address _destination,
        uint8 _v,
        bytes32 _r,
        bytes32 _s,
        uint256 _index,
        uint256 _amount,
        bytes32[] calldata _merkleProof
    ) external {
        require(_recipient != address(0), "recipient address cannot be zero");
        require(
            _destination != address(0),
            "destination address cannot be zero"
        );
        require(merkleRoot != "", "tokens were not allocated yet");
        require(!isClaimed(_index), "tokens already claimed");

        // Verify the signature over destination address.
        require(
            _recipient == recoverSignerAddress(_destination, _v, _r, _s),
            "invalid signature of destination address"
        );

        // Verify the merkle proof.
        bytes32 node = keccak256(abi.encodePacked(_index, _recipient, _amount));
        require(
            MerkleProof.verify(_merkleProof, merkleRoot, node),
            "invalid proof"
        );

        // Mark it claimed and send the token.
        setClaimed(_index);

        token.safeTransfer(_destination, _amount);

        emit TokensClaimed(_index, _recipient, _destination, _amount);
    }

    /// Allocates amount of tokens for the merkle root.
    /// @param _merkleRoot The merkle root.
    /// @param _amount The amount of tokens allocated for the merkle root.
    /// @param _unclaimedUnlockDurationSec Duration of a period (in seconds)
    /// after which  unclaimed tokens can be recovered from the contract. If the
    /// value is zero the recovery won't be allowed.
    function allocate(
        bytes32 _merkleRoot,
        uint256 _amount,
        uint256 _unclaimedUnlockDurationSec
    ) public onlyOwner {
        require(merkleRoot == "", "tokens were already allocated");
        require(_merkleRoot != "", "merkle root cannot be empty");
        require(_amount > 0, "amount has to be greater than zero");

        token.safeTransferFrom(msg.sender, address(this), _amount);

        merkleRoot = _merkleRoot;

        // If unclaimed unlock duration was provided calculate timestamp after
        // which unclaimed tokens will be recoverable. If the duration is set to
        // zero the tokens won't be recoverable.
        if (_unclaimedUnlockDurationSec > 0) {
            unclaimedUnlockTimestamp =
                /* solium-disable-next-line security/no-block-members */
                block.timestamp +
                _unclaimedUnlockDurationSec;
        }

        emit TokensAllocated(_merkleRoot, _amount, unclaimedUnlockTimestamp);
    }

    /// @notice Withdraws unclaimed tokens to the destination address. The function
    /// can be called only by the contract owner. Tokens are recoverable after
    /// unlock duration defined on tokens allocation.
    /// @param _destination Address to send tokens to.
    function recoverUnclaimed(address _destination) public onlyOwner {
        require(
            _destination != address(0),
            "destination address cannot be zero"
        );
        require(unclaimedUnlockTimestamp > 0, "token recovery is not allowed");
        require(
            /* solium-disable-next-line security/no-block-members */
            unclaimedUnlockTimestamp <= block.timestamp,
            "token recovery is not possible yet"
        );

        uint256 amount = token.balanceOf(address(this));
        token.safeTransfer(_destination, amount);

        emit TokensRecovered(_destination, amount);
    }

    /// @notice Checks if tokens were claimed for the given merkle index.
    /// @param _index Merkle index.
    /// @return True is tokens were claimed, false otherwise.
    function isClaimed(uint256 _index) public view returns (bool) {
        uint256 claimedWordIndex = _index / 256;
        uint256 claimedBitIndex = _index % 256;
        uint256 claimedWord = claimedBitMap[claimedWordIndex];
        uint256 mask = (1 << claimedBitIndex);
        return claimedWord & mask == mask;
    }

    /// @notice Recovers signer's address from a signature and destination address.
    /// @dev Destination address is a part of a message that should be signed.
    /// To construct the message one has to hash this contract address along with
    /// destination address with keccak256 and prefix the obtained digest with
    /// Ethereum specific `\x19Ethereum Signed Message:\n32`.
    /// Due to the malleability concern described in EIP-2, the function expects
    /// s values in the lower half of the secp256k1 curve's order and v value of
    /// 27 or 28.
    function recoverSignerAddress(
        address _destination,
        uint8 _v,
        bytes32 _r,
        bytes32 _s
    ) internal view returns (address) {
        // Validate `s` and `v` values for a malleability concern described in EIP-2.
        // Only signatures with `s` value in the lower half of the secp256k1
        // curve's order and `v` value of 27 or 28 are considered valid.
        require(
            uint256(_s) <=
                0x7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0,
            "Invalid signature 's' value"
        );
        require(_v == 27 || _v == 28, "Invalid signature 'v' value");

        bytes32 digest =
            keccak256(abi.encodePacked(address(this), _destination));
        bytes32 prefixedDigest =
            keccak256(
                abi.encodePacked("\x19Ethereum Signed Message:\n32", digest)
            );

        return ecrecover(prefixedDigest, _v, _r, _s);
    }

    /// @notice Marks the given merkle index as claimed.
    /// @param _index Merkle index.
    function setClaimed(uint256 _index) private {
        uint256 claimedWordIndex = _index / 256;
        uint256 claimedBitIndex = _index % 256;
        claimedBitMap[claimedWordIndex] =
            claimedBitMap[claimedWordIndex] |
            (1 << claimedBitIndex);
    }
}
