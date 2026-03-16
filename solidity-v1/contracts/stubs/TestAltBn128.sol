pragma solidity ^0.8.0;

// it is just a stub, not a live deployment;
// we are fine with experimental feature
/* solium-disable-next-line */
pragma experimental ABIEncoderV2;

import "../cryptography/AltBn128.sol";

contract TestAltBn128 {
    function publicG1Unmarshal(bytes memory m)
        public
        pure
        returns (AltBn128.G1Point memory)
    {
        return AltBn128.g1Unmarshal(m);
    }

    function publicG2Unmarshal(bytes memory m)
        public
        pure
        returns (AltBn128.G2Point memory)
    {
        return AltBn128.g2Unmarshal(m);
    }

    function publicG2Decompress(bytes memory m)
        public
        pure
        returns (AltBn128.G2Point memory)
    {
        return AltBn128.g2Decompress(m);
    }

    AltBn128.G1Point g1 = AltBn128.g1();
    AltBn128.G2Point g2 = AltBn128.g2();

    function runHashingTest() public {
        string memory hello = "hello!";
        string memory goodbye = "goodbye.";
        AltBn128.G1Point memory p_1;
        AltBn128.G1Point memory p_2;
        p_1 = AltBn128.g1HashToPoint(bytes(hello));
        p_2 = AltBn128.g1HashToPoint(bytes(goodbye));

        require(p_1.x != 0, "X should not equal 0 in a hashed point.");
        require(p_1.y != 0, "Y should not equal 0 in a hashed point.");
        require(p_2.x != 0, "X should not equal 0 in a hashed point.");
        require(p_2.y != 0, "Y should not equal 0 in a hashed point.");

        require(
            AltBn128.isG1PointOnCurve(p_1),
            "Hashed points should be on the curve."
        );
        require(
            AltBn128.isG1PointOnCurve(p_2),
            "Hashed points should be on the curve."
        );
    }

    function runHashAndAddTest() public {
        string memory hello = "hello!";
        string memory goodbye = "goodbye.";
        AltBn128.G1Point memory p_1;
        AltBn128.G1Point memory p_2;
        p_1 = AltBn128.g1HashToPoint(bytes(hello));
        p_2 = AltBn128.g1HashToPoint(bytes(goodbye));

        AltBn128.G1Point memory p_3;
        AltBn128.G1Point memory p_4;

        p_3 = AltBn128.g1Add(p_1, p_2);
        p_4 = AltBn128.g1Add(p_2, p_1);

        require(p_3.x == p_4.x, "Point addition should be commutative.");
        require(p_3.y == p_4.y, "Point addition should be commutative.");

        require(
            AltBn128.isG1PointOnCurve(p_3),
            "Added points should be on the curve."
        );
    }

    function runHashAndScalarMultiplyTest() public {
        string memory hello = "hello!";
        AltBn128.G1Point memory p_1;
        AltBn128.G1Point memory p_2;
        p_1 = AltBn128.g1HashToPoint(bytes(hello));

        p_2 = AltBn128.scalarMultiply(p_1, 12);

        require(
            AltBn128.isG1PointOnCurve(p_2),
            "Multiplied point should be on the curve."
        );
    }

    uint256[2][] randomG1 = [
        [
            19985462441994274044747034318046506954527006367483173410210086890020894468080,
            18435086518936643964830423002803816020906755142322386776411266459735121477493
        ],
        [
            5020462286181323390508118928832214575747271080433231325350949221928552771006,
            6185819871141660402526014503512646294685393996180865467240675728617780703293
        ],
        [
            6443569433573553122968863711873646857259386088199162681359502284812487407640,
            5378163071719228060986147516945215302807920666474786022068644421654848367565
        ],
        [
            19923401560169709235429596406611561407855841155398968552083379042854674266499,
            10525710049852251332517421942831871137061760270860191079861735894813959253055
        ],
        [
            19708536568727021605314080150939514846681180496259214578335284564769723419938,
            10692356507990003585226828000662725800587874779874382732784071410185542028439
        ],
        [
            9372321588728408099991690236147178727235677209811335191981120085012199642559,
            11142558497436993571688400857990083465974854590891619188837196531526982135288
        ],
        [
            5965886725029153696599727822391947370059044516209856603831046549655428439060,
            10175397348860086021525298362240324520688370458967443904613437789517586359962
        ],
        [
            20748498912264019189558145442056089284703240490771913074152837182874426945993,
            18057592905480302483449076150943157907511999106688668826058046434471622799474
        ],
        [
            7477907739342510339540973467783537984932469471333402963930842749621045686487,
            1179596217276931579251786249459263438406283313229247981371951224605996910316
        ],
        [
            3386341017431964271492464889305868556498227248869025090652509478713128447791,
            1836930069368635496176332910536803390892441983393373783218213609800061729358
        ],
        [
            15369136978943154357167241223632015935727499997608268363280373457134516735375,
            13963714284290182895189799343803541866405397472877283227980653081636863075815
        ],
        [
            252324504554101299613500369843808394682741893676105206770010905523927747936,
            5301348107423388196155421770728871408649649793716391651642915616092861338462
        ]
    ];

    uint256[4][] randomG2 = [
        [
            11559732032986387107991004021392285783925812861821192530917403151452391805634,
            10857046999023057135944570762232829481370756359578518086990519993285655852781,
            4082367875863433681332203403145435568316851327593401208105741076214120093531,
            8495653923123431417604973247489272438418190587263600148770280649306958101930
        ],
        [
            3558222795862351239338057832504031412042231518727744074889572712970741892158,
            1306678064139060928090556321451178074402697032692562310283497263099767804676,
            2316442485869095896235201578689810877812891214989209176315292141295656899653,
            2999256016806770587400278223266487828070696882906920737522774393744811789778
        ],
        [
            14622493903084144595613313812136815995549249456289461446052351022658739726486,
            14815420576980748908539135128242740015127336122409448605930237255046504879157,
            13400921316097996971584638040633436051131826349725459963804926452735715285087,
            11851371827558083239355447328198017836652007495098247662236445322029872280124
        ],
        [
            6217401439122098088765827257305726706731572245002926407946450711747381925871,
            14805062536146767263542014365237987548032285721054252746437955688297149797718,
            2682992062255943794448341271274355111144659536522130372456554423016095772641,
            8381914770822556071474775460600158217731085727931186436939477443088764950881
        ]
    ];

    function runGfP2AddTest() public {
        uint256 i;
        uint8 j;

        AltBn128.gfP2 memory p_1;
        AltBn128.gfP2 memory p_2;
        AltBn128.gfP2 memory p_3;
        AltBn128.gfP2 memory p_4;

        for (i = 0; i < randomG2.length; i++) {
            for (j = 0; j < randomG2.length; j++) {
                p_1 = AltBn128.gfP2Add(
                    AltBn128.gfP2(randomG2[i][0], randomG2[i][1]),
                    AltBn128.gfP2(randomG2[j][0], randomG2[j][1])
                );
                p_2 = AltBn128.gfP2Add(
                    AltBn128.gfP2(randomG2[i][2], randomG2[i][3]),
                    AltBn128.gfP2(randomG2[j][2], randomG2[j][3])
                );
                p_3 = AltBn128.gfP2Add(
                    AltBn128.gfP2(randomG2[j][0], randomG2[j][1]),
                    AltBn128.gfP2(randomG2[i][0], randomG2[i][1])
                );
                p_4 = AltBn128.gfP2Add(
                    AltBn128.gfP2(randomG2[j][2], randomG2[j][3]),
                    AltBn128.gfP2(randomG2[i][2], randomG2[i][3])
                );

                require(
                    p_1.x == p_3.x,
                    "Point addition should be commutative."
                );
                require(
                    p_1.y == p_3.y,
                    "Point addition should be commutative."
                );
                require(
                    p_2.x == p_4.x,
                    "Point addition should be commutative."
                );
                require(
                    p_2.y == p_4.y,
                    "Point addition should be commutative."
                );
            }
        }
    }

    function runAddTest() public {
        uint256 i;
        uint8 j;

        AltBn128.G1Point memory p_1;
        AltBn128.G1Point memory p_2;

        for (i = 0; i < randomG1.length; i++) {
            for (j = 0; j < randomG1.length; j++) {
                p_1 = AltBn128.g1Add(
                    AltBn128.G1Point(randomG1[i][0], randomG1[i][1]),
                    AltBn128.G1Point(randomG1[j][0], randomG1[j][1])
                );
                p_2 = AltBn128.g1Add(
                    AltBn128.G1Point(randomG1[j][0], randomG1[j][1]),
                    AltBn128.G1Point(randomG1[i][0], randomG1[i][1])
                );

                require(
                    p_1.x == p_2.x,
                    "Point addition should be commutative."
                );
                require(
                    p_1.y == p_2.y,
                    "Point addition should be commutative."
                );

                require(
                    AltBn128.isG1PointOnCurve(p_1),
                    "Added points should be on the curve."
                );
            }
        }
    }

    function runScalarMultiplyTest() public {
        uint256 i;
        uint256 j;

        AltBn128.G1Point memory p_1;
        AltBn128.G1Point memory p_2;

        for (i = 1; i < randomG1.length; i++) {
            p_1 = AltBn128.scalarMultiply(
                AltBn128.G1Point(randomG1[i][0], randomG1[i][1]),
                i
            );

            require(
                AltBn128.isG1PointOnCurve(p_1),
                "Multiplied point should be on the curve."
            );

            p_2 = AltBn128.G1Point(randomG1[i][0], randomG1[i][1]);
            for (j = 1; j < i; j++) {
                p_2 = AltBn128.g1Add(
                    p_2,
                    AltBn128.G1Point(randomG1[i][0], randomG1[i][1])
                );
            }

            require(
                p_1.x == p_2.x,
                "Scalar multiplication should match repeat addition."
            );
            require(
                p_1.y == p_2.y,
                "Scalar multiplication should match repeat addition."
            );
        }
    }

    function runBasicPairingTest() public {
        bool result =
            AltBn128.pairing(
                g1,
                g2,
                AltBn128.G1Point(g1.x, AltBn128.getP() - g1.y),
                g2
            );
        require(result, "Basic pairing check should succeed.");
    }

    function runG1PointMarshalingTest() public {
        AltBn128.G1Point memory point;
        point
            .x = 656647519899395589093611455851658769732922739162315270379466002146796568126;
        point
            .y = 5296675831567268847773497112983742440203412208935796410329912816023128374551;

        bytes memory marshaledPoint = AltBn128.g1Marshal(point);
        AltBn128.G1Point memory actual = AltBn128.g1Unmarshal(marshaledPoint);

        require(
            point.x == actual.x,
            "Unmarshaling a marshaled point should give the same x coordinate."
        );
        require(
            point.y == actual.y,
            "Unmarshaling a marshaled point should give the same y coordinate."
        );
    }

    // Verifying sample data generated with bn256.go - Ethereum's bn256/cloudflare curve.
    function runVerifySignatureTest() public {
        // "hello!" message hashed to G1 point using G1HashToPoint from keep-core/pkg/bls/altbn128.go
        AltBn128.G1Point memory message;
        message
            .x = 5634139805531803244211629196316241342481813136353842610045004964364565232495;
        message
            .y = 12935759374343796368049060881302766596646163398265176009268480404372697203641;

        // G1 point hashed message above signed with private key = 123 using ScalarMult
        // from go-ethereum/crypto/bn256/cloudflare library
        AltBn128.G1Point memory signature;
        signature
            .x = 656647519899395589093611455851658769732922739162315270379466002146796568126;
        signature
            .y = 5296675831567268847773497112983742440203412208935796410329912816023128374551;

        // G2 point representing public key for private key = 123
        AltBn128.G2Point memory publicKey;
        publicKey
            .x
            .x = 14066454060412929535985836631817650877381034334390275410072431082437297539867;
        publicKey
            .x
            .y = 19276105129625393659655050515259006463014579919681138299520812914148935621072;
        publicKey
            .y
            .x = 10109651107942685361120988628892759706059655669161016107907096760613704453218;
        publicKey
            .y
            .y = 12642665914920339463975152321804664028480770144655934937445922690262428344269;

        bool result =
            AltBn128.pairing(
                signature,
                g2,
                AltBn128.G1Point(message.x, AltBn128.getP() - message.y),
                publicKey
            );
        require(
            result,
            "Verify signature using precompiled pairing contract should succeed."
        );
    }

    function runCompressG1InvertibilityTest() public {
        AltBn128.G1Point memory p_1;
        AltBn128.G1Point memory p_2;

        for (uint256 i = 0; i < randomG1.length; i++) {
            p_1.x = randomG1[i][0];
            p_1.y = randomG1[i][1];
            bytes32 compressed = AltBn128.g1Compress(p_1);
            p_2 = AltBn128.g1Decompress(compressed);
            require(
                p_1.x == p_2.x,
                "Decompressing a compressed point should give the same x coordinate."
            );
            require(
                p_1.y == p_2.y,
                "Decompressing a compressed point should give the same y coordinate."
            );
        }
    }

    function runCompressG2InvertibilityTest() public {
        AltBn128.G2Point memory p_1;
        AltBn128.G2Point memory p_2;

        for (uint256 i = 0; i < randomG2.length; i++) {
            p_1.x.x = randomG2[i][0];
            p_1.x.y = randomG2[i][1];
            p_1.y.x = randomG2[i][2];
            p_1.y.y = randomG2[i][3];

            p_2 = AltBn128.g2Decompress(AltBn128.g2Compress(p_1));
            require(
                p_1.x.x == p_2.x.x,
                "Decompressing a compressed point should give the same x coordinate."
            );
            require(
                p_1.x.y == p_2.x.y,
                "Decompressing a compressed point should give the same x coordinate."
            );
            require(
                p_1.y.x == p_2.y.x,
                "Decompressing a compressed point should give the same x coordinate."
            );
            require(
                p_1.y.y == p_2.y.y,
                "Decompressing a compressed point should give the same x coordinate."
            );
        }
    }

    function runG2PointOnCurveTest() public {
        AltBn128.G2Point memory point;

        for (uint256 i = 0; i < randomG2.length; i++) {
            point.x.x = randomG2[i][0];
            point.x.y = randomG2[i][1];
            point.y.x = randomG2[i][2];
            point.y.y = randomG2[i][3];

            require(
                AltBn128.isG2PointOnCurve(point),
                "Valid points should be on the curve."
            );
        }

        for (uint256 i = 0; i < randomG2.length; i++) {
            point.x.x = randomG2[i][2];
            point.x.y = randomG2[i][3];
            point.y.x = randomG2[i][0];
            point.y.y = randomG2[i][1];

            require(
                !AltBn128.isG2PointOnCurve(point),
                "Invalid points should not be on the curve."
            );
        }
    }
}
