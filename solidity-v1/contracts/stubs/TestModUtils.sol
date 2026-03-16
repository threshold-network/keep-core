pragma solidity ^0.8.0;

import "../utils/ModUtils.sol";

contract TestModUtils {
    using ModUtils for uint256;

    uint256[16] smallOddPrimes = [
        3,
        5,
        7,
        11,
        13,
        17,
        19,
        23,
        29,
        31,
        37,
        41,
        43,
        47,
        53,
        59
    ];

    function runModExponentTest() public {
        uint256 a = 21;
        // a simple test
        require(a.modExp(2, 5) == 1, "");
        // test for overflow - (2 ^ 256 - 1) ^ 2 % alt_bn128_P
        uint256 almostOverflow = (2**256 - 1);
        uint256 result =
            almostOverflow.modExp(
                2,
                21888242871839275222246405745257275088696311157297823662689037894645226208583
            );
        require(
            result ==
                12283109618583340521412061117291584720854994367414008739435419022702680857751,
            "modExp() should not overflow"
        );
    }

    int256[][] smallOddPrimesLegendre = [
        [int256(3), int256(0), int256(0)],
        [int256(3), int256(1), int256(1)],
        [int256(3), int256(2), int256(-1)],
        [int256(5), int256(0), int256(0)],
        [int256(5), int256(1), int256(1)],
        [int256(5), int256(2), int256(-1)],
        [int256(5), int256(3), int256(-1)],
        [int256(5), int256(4), int256(1)],
        [int256(7), int256(0), int256(0)],
        [int256(7), int256(1), int256(1)],
        [int256(7), int256(2), int256(1)],
        [int256(7), int256(3), int256(-1)],
        [int256(7), int256(4), int256(1)],
        [int256(7), int256(5), int256(-1)],
        [int256(7), int256(6), int256(-1)],
        [int256(11), int256(0), int256(0)],
        [int256(11), int256(1), int256(1)],
        [int256(11), int256(2), int256(-1)],
        [int256(11), int256(3), int256(1)],
        [int256(11), int256(4), int256(1)],
        [int256(11), int256(5), int256(1)],
        [int256(11), int256(6), int256(-1)],
        [int256(11), int256(7), int256(-1)],
        [int256(11), int256(8), int256(-1)],
        [int256(11), int256(9), int256(1)],
        [int256(11), int256(10), int256(-1)],
        [int256(13), int256(0), int256(0)],
        [int256(13), int256(1), int256(1)],
        [int256(13), int256(2), int256(-1)],
        [int256(13), int256(3), int256(1)],
        [int256(13), int256(4), int256(1)],
        [int256(13), int256(5), int256(-1)],
        [int256(13), int256(6), int256(-1)],
        [int256(13), int256(7), int256(-1)],
        [int256(13), int256(8), int256(-1)],
        [int256(13), int256(9), int256(1)],
        [int256(13), int256(10), int256(1)],
        [int256(13), int256(11), int256(-1)],
        [int256(13), int256(12), int256(1)],
        [int256(17), int256(0), int256(0)],
        [int256(17), int256(1), int256(1)],
        [int256(17), int256(2), int256(1)],
        [int256(17), int256(3), int256(-1)],
        [int256(17), int256(4), int256(1)],
        [int256(17), int256(5), int256(-1)],
        [int256(17), int256(6), int256(-1)],
        [int256(17), int256(7), int256(-1)],
        [int256(17), int256(8), int256(1)],
        [int256(17), int256(9), int256(1)]
    ];

    function runLegendreRangeTest() public {
        uint256 i;
        uint256 j;
        int256 leg;
        for (i = 0; i < smallOddPrimes.length; i++) {
            for (j = 0; j < 50; j++) {
                leg = ModUtils.legendre(j, smallOddPrimes[i]);
                require(
                    leg == 0 || leg == 1 || leg == -1,
                    "legendre() should only return [-1, 0, 1]"
                );
            }
        }
    }

    function runLegendreListTest() public {
        uint256 i;
        int256 leg;

        for (i = 0; i < smallOddPrimesLegendre.length; i++) {
            leg = ModUtils.legendre(
                uint256(smallOddPrimesLegendre[i][1]),
                uint256(smallOddPrimesLegendre[i][0])
            );
            require(
                leg == smallOddPrimesLegendre[i][2],
                "legendre() result differed from list"
            );
        }
    }

    function runModSqrtOf0Test() public {
        uint256 p;
        uint256 i;
        uint256 zero = 0;

        // a = 0 mod p
        for (i = 0; i < smallOddPrimes.length; i++) {
            p = smallOddPrimes[i];
            require(zero == zero.modSqrt(p), "0 mod p should always equal 0");
        }
    }

    function runModSqrtMultipleOfPTest() public {
        uint256 p;
        uint256 pMult;
        uint256 i;
        uint256 j;
        uint256 zero = 0;

        // a = 0 mod p
        for (i = 0; i < smallOddPrimes.length; i++) {
            p = smallOddPrimes[i];
            for (j = 0; j < 20; j++) {
                pMult = p * i;
                require(
                    zero == pMult.modSqrt(p),
                    "(n * p) mod p should always equal 0"
                );
            }
        }
    }

    function runModSqrtAgainstListTest() public {
        uint256 i;
        uint256 a;
        uint256 p;
        uint256 root;

        uint8[3][30] memory smallOddPrimesResults =
            [
                [3, 1, 1],
                [5, 1, 1],
                [5, 4, 3],
                [7, 1, 1],
                [7, 2, 4],
                [7, 4, 2],
                [11, 1, 1],
                [11, 3, 5],
                [11, 4, 9],
                [11, 5, 4],
                [11, 9, 3],
                [13, 1, 1],
                [13, 3, 9],
                [13, 4, 11],
                [13, 9, 3],
                [13, 10, 7],
                [13, 12, 8],
                [17, 1, 1],
                [17, 2, 6],
                [17, 4, 2],
                [17, 8, 12],
                [17, 9, 14],
                [17, 13, 8],
                [17, 15, 7],
                [17, 16, 4],
                [19, 1, 1],
                [19, 4, 17],
                [19, 5, 9],
                [19, 6, 5],
                [19, 7, 11]
            ];

        for (i = 0; i < smallOddPrimesResults.length; i++) {
            p = smallOddPrimesResults[i][0];
            a = smallOddPrimesResults[i][1];
            root = a.modSqrt(p);

            require(
                root == smallOddPrimesResults[i][2],
                "modSqrt() result differed from list"
            );
        }
    }

    function runModSqrtAgainstNonSquaresTest() public {
        uint8 i;
        uint256 a;
        uint256 p;
        uint256 root;

        uint8[3][23] memory smallOddPrimesResults =
            [
                [3, 2, 0],
                [5, 2, 0],
                [5, 3, 0],
                [7, 3, 0],
                [7, 5, 0],
                [7, 6, 0],
                [11, 2, 0],
                [11, 6, 0],
                [11, 7, 0],
                [11, 8, 0],
                [13, 2, 0],
                [13, 5, 0],
                [13, 6, 0],
                [13, 7, 0],
                [13, 8, 0],
                [13, 11, 0],
                [17, 3, 0],
                [17, 5, 0],
                [17, 6, 0],
                [17, 7, 0],
                [17, 11, 0],
                [17, 12, 0],
                [17, 14, 0]
            ];

        for (i = 0; i < smallOddPrimesResults.length; i++) {
            p = smallOddPrimesResults[i][0];
            a = smallOddPrimesResults[i][1];
            root = a.modSqrt(p);

            require(
                root == smallOddPrimesResults[i][2],
                "modSqrt() result differed from list"
            );
        }
    }

    function runModSqrtALessThanPTest() public {
        uint256 p;
        uint256 root;
        uint256 i;
        uint256 a;

        // a < p for small p
        for (i = 0; i < smallOddPrimes.length; i++) {
            p = smallOddPrimes[i];
            for (a = 1; a < p; a++) {
                root = a.modSqrt(p);
                if (root != 0) {
                    require(
                        a % p == (root * root) % p,
                        "Invalid modular square root for a < p"
                    );
                }
            }
        }
    }

    function runModSqrtAGreaterThanPTest() public {
        uint256 p;
        uint256 root;
        uint8 i;
        uint256 a;

        // a > p for small p
        for (i = 0; i < smallOddPrimes.length; i++) {
            p = smallOddPrimes[i];
            for (a = p + 1; a < p + 10; a++) {
                root = a.modSqrt(p);
                if (root != 0) {
                    require(
                        a % p == (root * root) % p,
                        "Invalid modular square root for a > p"
                    );
                }
            }
        }
    }
}
