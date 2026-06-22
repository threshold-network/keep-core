"use strict";
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
    var getNamedAccounts, deployments, helpers, _a, deployer, chaosnetOwner, execute, to1e18, POOL_WEIGHT_DIVISOR, T, BeaconSortitionPool;
    return __generator(this, function (_b) {
        switch (_b.label) {
            case 0:
                getNamedAccounts = hre.getNamedAccounts, deployments = hre.deployments, helpers = hre.helpers;
                return [4 /*yield*/, getNamedAccounts()];
            case 1:
                _a = _b.sent(), deployer = _a.deployer, chaosnetOwner = _a.chaosnetOwner;
                execute = deployments.execute;
                to1e18 = helpers.number.to1e18;
                POOL_WEIGHT_DIVISOR = to1e18(1);
                return [4 /*yield*/, deployments.get("T")];
            case 2:
                T = _b.sent();
                return [4 /*yield*/, deployments.deploy("BeaconSortitionPool", {
                        contract: "SortitionPool",
                        from: deployer,
                        args: [T.address, POOL_WEIGHT_DIVISOR],
                        log: true,
                        waitConfirmations: 1,
                    })];
            case 3:
                BeaconSortitionPool = _b.sent();
                return [4 /*yield*/, execute("BeaconSortitionPool", { from: deployer, log: true, waitConfirmations: 1 }, "transferChaosnetOwnerRole", chaosnetOwner)];
            case 4:
                _b.sent();
                if (!hre.network.tags.etherscan) return [3 /*break*/, 7];
                return [4 /*yield*/, hre.ethers.provider.waitForTransaction(BeaconSortitionPool.transactionHash, 2, 300000)];
            case 5:
                _b.sent();
                return [4 /*yield*/, helpers.etherscan.verify(BeaconSortitionPool)];
            case 6:
                _b.sent();
                _b.label = 7;
            case 7:
                if (!hre.network.tags.tenderly) return [3 /*break*/, 9];
                return [4 /*yield*/, hre.tenderly.verify({
                        name: "BeaconSortitionPool",
                        address: BeaconSortitionPool.address,
                    })];
            case 8:
                _b.sent();
                _b.label = 9;
            case 9: return [2 /*return*/];
        }
    });
}); };
exports.default = func;
func.tags = ["BeaconSortitionPool"];
// TokenStaking and T deployments are expected to be resolved from
// @threshold-network/solidity-contracts
func.dependencies = ["TokenStaking", "T"];
