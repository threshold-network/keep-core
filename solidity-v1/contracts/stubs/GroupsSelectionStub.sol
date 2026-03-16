pragma solidity ^0.8.0;
import "../libraries/operator/GroupSelection.sol";

contract GroupSelectionStub {
    using GroupSelection for GroupSelection.Storage;
    GroupSelection.Storage groupSelection;

    constructor(uint256 groupSize) {
        groupSelection.groupSize = groupSize;
    }

    function addTicket(uint64 newTicketValue) public {
        groupSelection.addTicket(newTicketValue);
    }

    /**
     * @dev Gets submitted group candidate tickets so far.
     */
    function getTickets() public view returns (uint64[] memory) {
        return groupSelection.tickets;
    }

    /**
     * @dev Gets an index of the highest ticket value (tail).
     */
    function getTail() public view returns (uint256) {
        return groupSelection.tail;
    }

    /**
     * @dev Gets an index of a ticket that a higherTicketValueIndex points to.
     * Ex. tickets[23, 5, 65]
     * getPreviousTicketIndex(2) = 0
     */
    function getPreviousTicketIndex(uint256 higherTicketValueIndex)
        public
        view
        returns (uint256)
    {
        return
            uint256(
                uint8(
                    groupSelection.previousTicketIndices[higherTicketValueIndex]
                )
            );
    }
}
