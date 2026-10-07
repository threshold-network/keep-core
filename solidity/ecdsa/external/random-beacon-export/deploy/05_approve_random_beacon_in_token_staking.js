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
// TokenStaking.ApplicationStatus: NOT_APPROVED=0, APPROVED=1, PAUSED=2, DISABLED=3.
// Only a NOT_APPROVED application can be approved; approving any other status
// reverts, so a replay must skip it.
var APPLICATION_STATUS_NOT_APPROVED = "0";
var func = function (hre) { return __awaiter(void 0, void 0, void 0, function () {
    var getNamedAccounts, deployments, ethers, deployer, execute, get, read, log, application, TokenStaking, tokenStaking, hasFunction, info, status_1, error_1;
    var _a;
    return __generator(this, function (_b) {
        switch (_b.label) {
            case 0:
                getNamedAccounts = hre.getNamedAccounts, deployments = hre.deployments, ethers = hre.ethers;
                return [4 /*yield*/, getNamedAccounts()];
            case 1:
                deployer = (_b.sent()).deployer;
                execute = deployments.execute, get = deployments.get, read = deployments.read, log = deployments.log;
                return [4 /*yield*/, get("RandomBeacon")];
            case 2:
                application = _b.sent();
                return [4 /*yield*/, get("TokenStaking")
                    // Normalize both JSON and human-readable ABIs using the consumer's ethers.
                ];
            case 3:
                TokenStaking = _b.sent();
                return [4 /*yield*/, ethers.getContractAt(TokenStaking.abi, TokenStaking.address)];
            case 4:
                tokenStaking = _b.sent();
                hasFunction = function (name) {
                    return tokenStaking.interface.fragments.some(function (fragment) { return fragment.type === "function" && fragment.name === name; });
                };
                if (!hasFunction("approveApplication")) {
                    log("TokenStaking does not have approveApplication; skipping RandomBeacon approval");
                    return [2 /*return*/];
                }
                if (!hasFunction("applicationInfo")) return [3 /*break*/, 8];
                _b.label = 5;
            case 5:
                _b.trys.push([5, 7, , 8]);
                return [4 /*yield*/, read("TokenStaking", "applicationInfo", application.address)
                    // Support named and positional results, including BigNumber and bigint.
                ];
            case 6:
                info = _b.sent();
                status_1 = (_a = info.status) !== null && _a !== void 0 ? _a : info[0];
                if (status_1.toString() !== APPLICATION_STATUS_NOT_APPROVED) {
                    log("RandomBeacon already has TokenStaking status " + status_1 + "; skipping approval");
                    return [2 /*return*/];
                }
                return [3 /*break*/, 8];
            case 7:
                error_1 = _b.sent();
                log("Could not read TokenStaking application status (continuing): " + error_1);
                return [3 /*break*/, 8];
            case 8: return [4 /*yield*/, execute("TokenStaking", { from: deployer, log: true, waitConfirmations: 1 }, "approveApplication", application.address)];
            case 9:
                _b.sent();
                return [2 /*return*/];
        }
    });
}); };
exports.default = func;
func.tags = ["RandomBeaconApprove"];
func.dependencies = ["TokenStaking", "RandomBeacon"];
// Mainnet applications are approved outside this deploy.
func.skip = function (hre) { return __awaiter(void 0, void 0, void 0, function () { return __generator(this, function (_a) {
    return [2 /*return*/, hre.network.name === "mainnet"];
}); }); };
