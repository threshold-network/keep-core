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
    var getNamedAccounts, deployments, helpers, _a, deployer, chaosnetOwner, execute, POOL_WEIGHT_DIVISOR, T, args, previous, sameArgs, BeaconSortitionPool, currentChaosnetOwner;
    var _b;
    return __generator(this, function (_c) {
        switch (_c.label) {
            case 0:
                getNamedAccounts = hre.getNamedAccounts, deployments = hre.deployments, helpers = hre.helpers;
                return [4 /*yield*/, getNamedAccounts()];
            case 1:
                _a = _c.sent(), deployer = _a.deployer, chaosnetOwner = _a.chaosnetOwner;
                execute = deployments.execute;
                POOL_WEIGHT_DIVISOR = "1000000000000000000";
                return [4 /*yield*/, deployments.get("T")
                    // Reuse a saved record only if it was deployed for this token, so a
                    // redeployed T never ends up with a pool bound to the old one.
                ];
            case 2:
                T = _c.sent();
                args = [T.address, POOL_WEIGHT_DIVISOR];
                return [4 /*yield*/, deployments.getOrNull("BeaconSortitionPool")];
            case 3:
                previous = _c.sent();
                sameArgs = ((_b = previous === null || previous === void 0 ? void 0 : previous.args) === null || _b === void 0 ? void 0 : _b.length) === args.length &&
                    previous.args.every(function (arg, index) {
                        return String(arg).toLowerCase() === String(args[index]).toLowerCase();
                    });
                return [4 /*yield*/, deployments.deploy("BeaconSortitionPool", {
                        contract: "SortitionPool",
                        skipIfAlreadyDeployed: sameArgs,
                        from: deployer,
                        args: args,
                        log: true,
                        waitConfirmations: hre.network.tags.etherscan ? 2 : 1,
                    })];
            case 4:
                BeaconSortitionPool = _c.sent();
                return [4 /*yield*/, deployments.read("BeaconSortitionPool", "chaosnetOwner")];
            case 5:
                currentChaosnetOwner = _c.sent();
                if (!!helpers.address.equal(currentChaosnetOwner, chaosnetOwner)) return [3 /*break*/, 7];
                return [4 /*yield*/, execute("BeaconSortitionPool", { from: deployer, log: true, waitConfirmations: 1 }, "transferChaosnetOwnerRole", chaosnetOwner)];
            case 6:
                _c.sent();
                _c.label = 7;
            case 7:
                if (!hre.network.tags.etherscan) return [3 /*break*/, 9];
                return [4 /*yield*/, helpers.etherscan.verify(BeaconSortitionPool)];
            case 8:
                _c.sent();
                _c.label = 9;
            case 9:
                if (!hre.network.tags.tenderly) return [3 /*break*/, 11];
                return [4 /*yield*/, hre.tenderly.verify({
                        name: "BeaconSortitionPool",
                        address: BeaconSortitionPool.address,
                    })];
            case 10:
                _c.sent();
                _c.label = 11;
            case 11: return [2 /*return*/];
        }
    });
}); };
exports.default = func;
func.tags = ["BeaconSortitionPool"];
// TokenStaking and T deployments are expected to be resolved from
// @threshold-network/solidity-contracts
func.dependencies = ["TokenStaking", "T"];
