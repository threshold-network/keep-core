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
    var getNamedAccounts, deployments, helpers, _a, deployer, governance, RandomBeaconGovernance, currentGovernance, linkedBeacon, RandomBeacon, owner;
    return __generator(this, function (_b) {
        switch (_b.label) {
            case 0:
                getNamedAccounts = hre.getNamedAccounts, deployments = hre.deployments, helpers = hre.helpers;
                return [4 /*yield*/, getNamedAccounts()];
            case 1:
                _a = _b.sent(), deployer = _a.deployer, governance = _a.governance;
                return [4 /*yield*/, deployments.get("RandomBeaconGovernance")
                    // Governance moves only once, so refuse to hand it to a governance contract
                    // that is not ours or is linked to another beacon.
                ];
            case 2:
                RandomBeaconGovernance = _b.sent();
                return [4 /*yield*/, deployments.read("RandomBeacon", "governance")];
            case 3:
                currentGovernance = _b.sent();
                if (!helpers.address.equal(currentGovernance, deployer) &&
                    !helpers.address.equal(currentGovernance, RandomBeaconGovernance.address)) {
                    throw new Error("RandomBeacon governance is " + currentGovernance + ", expected the deployer or " + RandomBeaconGovernance.address);
                }
                return [4 /*yield*/, deployments.read("RandomBeaconGovernance", "randomBeacon")];
            case 4:
                linkedBeacon = _b.sent();
                return [4 /*yield*/, deployments.get("RandomBeacon")];
            case 5:
                RandomBeacon = _b.sent();
                if (!helpers.address.equal(linkedBeacon, RandomBeacon.address)) {
                    throw new Error("RandomBeaconGovernance at " + RandomBeaconGovernance.address + " points to " + linkedBeacon + ", expected " + RandomBeacon.address);
                }
                return [4 /*yield*/, deployments.read("RandomBeaconGovernance", "owner")];
            case 6:
                owner = _b.sent();
                if (!helpers.address.equal(owner, deployer) &&
                    !helpers.address.equal(owner, governance)) {
                    throw new Error("RandomBeaconGovernance is owned by " + owner + ", expected the deployer or " + governance);
                }
                if (!helpers.address.equal(owner, deployer)) return [3 /*break*/, 8];
                return [4 /*yield*/, helpers.ownable.transferOwnership("RandomBeaconGovernance", governance, deployer)];
            case 7:
                _b.sent();
                _b.label = 8;
            case 8:
                if (!helpers.address.equal(currentGovernance, deployer)) {
                    deployments.log("RandomBeacon governance is already " + currentGovernance + "; skipping transfer");
                    return [2 /*return*/];
                }
                return [4 /*yield*/, deployments.execute("RandomBeacon", { from: deployer, log: true, waitConfirmations: 1 }, "transferGovernance", RandomBeaconGovernance.address)];
            case 9:
                _b.sent();
                return [2 /*return*/];
        }
    });
}); };
exports.default = func;
func.tags = ["RandomBeaconTransferGovernance"];
func.dependencies = ["RandomBeaconGovernance"];
