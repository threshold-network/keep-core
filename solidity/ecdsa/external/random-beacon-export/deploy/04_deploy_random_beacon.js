"use strict";
var __assign = (this && this.__assign) || function () {
    __assign = Object.assign || function(t) {
        for (var s, i = 1, n = arguments.length; i < n; i++) {
            s = arguments[i];
            for (var p in s) if (Object.prototype.hasOwnProperty.call(s, p))
                t[p] = s[p];
        }
        return t;
    };
    return __assign.apply(this, arguments);
};
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __generator = (this && this.__generator) || function (thisArg, body) {
    var _ = { label: 0, sent: function() { if (t[0] & 1) throw t[1]; return t[1]; }, trys: [], ops: [] }, f, y, t, g;
    return g = { next: verb(0), "throw": verb(1), "return": verb(2) }, typeof Symbol === "function" && (g[Symbol.iterator] = function() { return this; }), g;
    function verb(n) { return function (v) { return step([n, v]); }; }
    function step(op) {
        if (f) throw new TypeError("Generator is already executing.");
        while (_) try {
            if (f = 1, y && (t = op[0] & 2 ? y["return"] : op[0] ? y["throw"] || ((t = y["return"]) && t.call(y), 0) : y.next) && !(t = t.call(y, op[1])).done) return t;
            if (y = 0, t) op = [op[0] & 2, t.value];
            switch (op[0]) {
                case 0: case 1: t = op; break;
                case 4: _.label++; return { value: op[1], done: false };
                case 5: _.label++; y = op[1]; op = [0]; continue;
                case 7: op = _.ops.pop(); _.trys.pop(); continue;
                default:
                    if (!(t = _.trys, t = t.length > 0 && t[t.length - 1]) && (op[0] === 6 || op[0] === 2)) { _ = 0; continue; }
                    if (op[0] === 3 && (!t || (op[1] > t[0] && op[1] < t[3]))) { _.label = op[1]; break; }
                    if (op[0] === 6 && _.label < t[1]) { _.label = t[1]; t = op; break; }
                    if (t && _.label < t[2]) { _.label = t[2]; _.ops.push(op); break; }
                    if (t[2]) _.ops.pop();
                    _.trys.pop(); continue;
            }
            op = body.call(thisArg, _);
        } catch (e) { op = [6, e]; y = 0; } finally { f = t = 0; }
        if (op[0] & 5) throw op[1]; return { value: op[0] ? op[1] : void 0, done: true };
    }
};
Object.defineProperty(exports, "__esModule", { value: true });
var func = function (hre) { return __awaiter(void 0, void 0, void 0, function () {
    var getNamedAccounts, deployments, helpers, deployer, T, TokenStaking, ReimbursementPool, BeaconSortitionPool, BeaconDkgValidator, deployOptions, BLS, BeaconAuthorization, BeaconDkg, BeaconInactivity, RandomBeacon;
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0:
                getNamedAccounts = hre.getNamedAccounts, deployments = hre.deployments, helpers = hre.helpers;
                return [4 /*yield*/, getNamedAccounts()];
            case 1:
                deployer = (_a.sent()).deployer;
                return [4 /*yield*/, deployments.get("T")];
            case 2:
                T = _a.sent();
                return [4 /*yield*/, deployments.get("TokenStaking")];
            case 3:
                TokenStaking = _a.sent();
                return [4 /*yield*/, deployments.get("ReimbursementPool")];
            case 4:
                ReimbursementPool = _a.sent();
                return [4 /*yield*/, deployments.get("BeaconSortitionPool")];
            case 5:
                BeaconSortitionPool = _a.sent();
                return [4 /*yield*/, deployments.get("BeaconDkgValidator")];
            case 6:
                BeaconDkgValidator = _a.sent();
                deployOptions = {
                    from: deployer,
                    log: true,
                    waitConfirmations: 1,
                };
                return [4 /*yield*/, deployments.deploy("BLS", deployOptions)];
            case 7:
                BLS = _a.sent();
                return [4 /*yield*/, deployments.deploy("BeaconAuthorization", deployOptions)];
            case 8:
                BeaconAuthorization = _a.sent();
                return [4 /*yield*/, deployments.deploy("BeaconDkg", deployOptions)];
            case 9:
                BeaconDkg = _a.sent();
                return [4 /*yield*/, deployments.deploy("BeaconInactivity", deployOptions)];
            case 10:
                BeaconInactivity = _a.sent();
                return [4 /*yield*/, deployments.deploy("RandomBeacon", __assign({ contract: process.env.TEST_USE_STUBS_BEACON === "true"
                            ? "RandomBeaconStub"
                            : undefined, args: [
                            BeaconSortitionPool.address,
                            T.address,
                            TokenStaking.address,
                            BeaconDkgValidator.address,
                            ReimbursementPool.address,
                        ], libraries: {
                            BLS: BLS.address,
                            BeaconAuthorization: BeaconAuthorization.address,
                            BeaconDkg: BeaconDkg.address,
                            BeaconInactivity: BeaconInactivity.address,
                        } }, deployOptions))];
            case 11:
                RandomBeacon = _a.sent();
                return [4 /*yield*/, helpers.ownable.transferOwnership("BeaconSortitionPool", RandomBeacon.address, deployer)];
            case 12:
                _a.sent();
                if (!hre.network.tags.etherscan) return [3 /*break*/, 19];
                return [4 /*yield*/, hre.ethers.provider.waitForTransaction(RandomBeacon.transactionHash, 2, 300000)];
            case 13:
                _a.sent();
                return [4 /*yield*/, helpers.etherscan.verify(BLS)];
            case 14:
                _a.sent();
                return [4 /*yield*/, helpers.etherscan.verify(BeaconAuthorization)];
            case 15:
                _a.sent();
                return [4 /*yield*/, helpers.etherscan.verify(BeaconDkg)];
            case 16:
                _a.sent();
                return [4 /*yield*/, helpers.etherscan.verify(BeaconInactivity)];
            case 17:
                _a.sent();
                return [4 /*yield*/, helpers.etherscan.verify(RandomBeacon)];
            case 18:
                _a.sent();
                _a.label = 19;
            case 19:
                if (!hre.network.tags.tenderly) return [3 /*break*/, 21];
                return [4 /*yield*/, hre.tenderly.verify({
                        name: "RandomBeacon",
                        address: RandomBeacon.address,
                    })];
            case 20:
                _a.sent();
                _a.label = 21;
            case 21: return [2 /*return*/];
        }
    });
}); };
exports.default = func;
func.tags = ["RandomBeacon"];
func.dependencies = [
    "T",
    "TokenStaking",
    "ReimbursementPool",
    "BeaconSortitionPool",
    "BeaconDkgValidator",
];
