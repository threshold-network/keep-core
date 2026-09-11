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
exports.mint = void 0;
// eslint-disable-next-line import/prefer-default-export
function mint(hre, owner, amount) {
    return __awaiter(this, void 0, void 0, function () {
        var ethers, helpers, _a, to1e18, from1e18, ownerAddress, stakeAmount, t, staking, tokenContractOwner, currentBalance, mintAmount, _b, _c, currentAllowance, _d, _e;
        return __generator(this, function (_f) {
            switch (_f.label) {
                case 0:
                    ethers = hre.ethers, helpers = hre.helpers;
                    _a = helpers.number, to1e18 = _a.to1e18, from1e18 = _a.from1e18;
                    ownerAddress = ethers.utils.getAddress(owner);
                    stakeAmount = to1e18(amount);
                    return [4 /*yield*/, helpers.contracts.getContract("T")];
                case 1:
                    t = _f.sent();
                    return [4 /*yield*/, helpers.contracts.getContract("TokenStaking")];
                case 2:
                    staking = _f.sent();
                    return [4 /*yield*/, t.owner()];
                case 3:
                    tokenContractOwner = _f.sent();
                    return [4 /*yield*/, t.balanceOf(ownerAddress)];
                case 4:
                    currentBalance = _f.sent();
                    console.log("Account " + ownerAddress + " balance is " + from1e18(currentBalance) + " T");
                    if (!currentBalance.lt(stakeAmount)) return [3 /*break*/, 8];
                    mintAmount = stakeAmount.sub(currentBalance);
                    console.log("Minting " + from1e18(mintAmount) + " T for " + ownerAddress + "...");
                    _c = (_b = t)
                        .connect;
                    return [4 /*yield*/, ethers.getSigner(tokenContractOwner)];
                case 5: return [4 /*yield*/, _c.apply(_b, [_f.sent()])
                        .mint(ownerAddress, mintAmount)];
                case 6: return [4 /*yield*/, (_f.sent()).wait()];
                case 7:
                    _f.sent();
                    _f.label = 8;
                case 8: return [4 /*yield*/, t.allowance(ownerAddress, staking.address)];
                case 9:
                    currentAllowance = _f.sent();
                    console.log("Account " + ownerAddress + " allowance for " + staking.address + " is " + from1e18(currentAllowance) + " T");
                    if (!currentAllowance.lt(stakeAmount)) return [3 /*break*/, 13];
                    console.log("Approving " + from1e18(stakeAmount) + " T for " + staking.address + "...");
                    _e = (_d = t)
                        .connect;
                    return [4 /*yield*/, ethers.getSigner(ownerAddress)];
                case 10: return [4 /*yield*/, _e.apply(_d, [_f.sent()])
                        .approve(staking.address, stakeAmount)];
                case 11: return [4 /*yield*/, (_f.sent()).wait()];
                case 12:
                    _f.sent();
                    _f.label = 13;
                case 13: return [2 /*return*/];
            }
        });
    });
}
exports.mint = mint;
