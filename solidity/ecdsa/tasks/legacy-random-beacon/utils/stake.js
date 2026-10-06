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
exports.calculateTokensNeededForStake = exports.stake = void 0;
function stake(hre, owner, provider, amount, beneficiary, authorizer) {
    return __awaiter(this, void 0, void 0, function () {
        var ethers, helpers, _a, to1e18, from1e18, ownerAddress, providerAddress, stakeAmount, beneficiaryAddress, authorizerAddress, staking, currentStake, _b, _c, topUpAmount, _d, _e;
        return __generator(this, function (_f) {
            switch (_f.label) {
                case 0:
                    ethers = hre.ethers, helpers = hre.helpers;
                    _a = helpers.number, to1e18 = _a.to1e18, from1e18 = _a.from1e18;
                    ownerAddress = ethers.utils.getAddress(owner);
                    providerAddress = ethers.utils.getAddress(provider);
                    stakeAmount = to1e18(amount);
                    beneficiaryAddress = beneficiary
                        ? ethers.utils.getAddress(beneficiary)
                        : ownerAddress;
                    authorizerAddress = authorizer
                        ? ethers.utils.getAddress(authorizer)
                        : ownerAddress;
                    return [4 /*yield*/, helpers.contracts.getContract("TokenStaking")];
                case 1:
                    staking = _f.sent();
                    return [4 /*yield*/, staking.callStatic.stakes(providerAddress)];
                case 2:
                    currentStake = (_f.sent()).tStake;
                    console.log("Current stake for " + providerAddress + " is " + from1e18(currentStake) + " T");
                    if (!currentStake.eq(0)) return [3 /*break*/, 6];
                    console.log("Staking " + from1e18(stakeAmount) + " T to the staking provider " + providerAddress + "...");
                    _c = (_b = staking)
                        .connect;
                    return [4 /*yield*/, ethers.getSigner(ownerAddress)];
                case 3: return [4 /*yield*/, _c.apply(_b, [_f.sent()])
                        .stake(providerAddress, beneficiaryAddress, authorizerAddress, stakeAmount)];
                case 4: return [4 /*yield*/, (_f.sent()).wait()];
                case 5:
                    _f.sent();
                    return [3 /*break*/, 10];
                case 6:
                    if (!currentStake.lt(stakeAmount)) return [3 /*break*/, 10];
                    topUpAmount = stakeAmount.sub(currentStake);
                    console.log("Topping up " + from1e18(topUpAmount) + " T to the staking provider " + providerAddress + "...");
                    _e = (_d = staking)
                        .connect;
                    return [4 /*yield*/, ethers.getSigner(ownerAddress)];
                case 7: return [4 /*yield*/, _e.apply(_d, [_f.sent()])
                        .topUp(providerAddress, topUpAmount)];
                case 8: return [4 /*yield*/, (_f.sent()).wait()];
                case 9:
                    _f.sent();
                    _f.label = 10;
                case 10: return [2 /*return*/];
            }
        });
    });
}
exports.stake = stake;
function calculateTokensNeededForStake(hre, provider, amount) {
    return __awaiter(this, void 0, void 0, function () {
        var ethers, helpers, _a, to1e18, from1e18, stakeAmount, staking, currentStake;
        return __generator(this, function (_b) {
            switch (_b.label) {
                case 0:
                    ethers = hre.ethers, helpers = hre.helpers;
                    _a = helpers.number, to1e18 = _a.to1e18, from1e18 = _a.from1e18;
                    stakeAmount = to1e18(amount);
                    return [4 /*yield*/, helpers.contracts.getContract("TokenStaking")];
                case 1:
                    staking = _b.sent();
                    return [4 /*yield*/, staking.callStatic.stakes(provider)];
                case 2:
                    currentStake = (_b.sent()).tStake;
                    if (currentStake.lt(stakeAmount)) {
                        return [2 /*return*/, ethers.BigNumber.from(from1e18(stakeAmount.sub(currentStake)))];
                    }
                    return [2 /*return*/, ethers.constants.Zero];
            }
        });
    });
}
exports.calculateTokensNeededForStake = calculateTokensNeededForStake;
