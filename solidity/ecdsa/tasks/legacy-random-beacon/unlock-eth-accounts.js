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
var config_1 = require("hardhat/config");
(0, config_1.task)("unlock-accounts", "Unlock ethereum accounts").setAction(function (args, hre) { return __awaiter(void 0, void 0, void 0, function () {
    var ethers, password, provider, accounts, i, account, error_1;
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0:
                ethers = hre.ethers;
                if (!(hre.network.name === "development")) return [3 /*break*/, 8];
                password = process.env.KEEP_ETHEREUM_PASSWORD || "password";
                provider = new ethers.providers.JsonRpcProvider(hre.network.config.url);
                return [4 /*yield*/, provider.listAccounts()];
            case 1:
                accounts = _a.sent();
                console.log("Total accounts: " + accounts.length);
                console.log("---------------------------------");
                i = 0;
                _a.label = 2;
            case 2:
                if (!(i < accounts.length)) return [3 /*break*/, 8];
                account = accounts[i];
                _a.label = 3;
            case 3:
                _a.trys.push([3, 5, , 6]);
                console.log("\nUnlocking account: " + account);
                // An explicit duration of zero seconds unlocks the key until geth exits.
                return [4 /*yield*/, provider.send("personal_unlockAccount", [
                        account.toLowerCase(),
                        password,
                        0,
                    ])];
            case 4:
                // An explicit duration of zero seconds unlocks the key until geth exits.
                _a.sent();
                console.log("Account unlocked!");
                return [3 /*break*/, 6];
            case 5:
                error_1 = _a.sent();
                console.log("\nAccount: " + account + " not unlocked!");
                console.error(error_1);
                return [3 /*break*/, 6];
            case 6:
                console.log("\n---------------------------------");
                _a.label = 7;
            case 7:
                i++;
                return [3 /*break*/, 2];
            case 8: return [2 /*return*/];
        }
    });
}); });
