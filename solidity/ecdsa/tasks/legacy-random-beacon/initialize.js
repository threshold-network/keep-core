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
exports.TASK_ADD_BETA_OPERATOR = exports.TASK_REGISTER = exports.TASK_AUTHORIZE = exports.TASK_STAKE = exports.TASK_MINT = exports.TASK_INITIALIZE_STAKING = exports.TASK_INITIALIZE = void 0;
var config_1 = require("hardhat/config");
var utils_1 = require("./utils");
// Main task executing all child tasks.
exports.TASK_INITIALIZE = "initialize";
// Subtask for staking.
exports.TASK_INITIALIZE_STAKING = exports.TASK_INITIALIZE + ":staking";
// Staking tasks.
exports.TASK_MINT = "mint";
exports.TASK_STAKE = "stake";
// Name prefix that should be used in tasks implementation for specific application.
exports.TASK_AUTHORIZE = "authorize";
exports.TASK_REGISTER = "register";
exports.TASK_ADD_BETA_OPERATOR = "add_beta_operator";
// Subtask for the Random Beacon application.
var TASK_INITIALIZE_BEACON = exports.TASK_INITIALIZE + ":beacon";
var TASK_AUTHORIZE_BEACON = exports.TASK_AUTHORIZE + ":beacon";
var TASK_REGISTER_BEACON = exports.TASK_REGISTER + ":beacon";
var TASK_ADD_BETA_OPERATOR_BEACON = exports.TASK_ADD_BETA_OPERATOR + ":beacon";
(0, config_1.task)(exports.TASK_INITIALIZE, "Initializes staking and the Random Beacon application for a staking provider and an operator")
    .addParam("owner", "Stake Owner address", undefined, config_1.types.string)
    .addParam("provider", "Staking Provider", undefined, config_1.types.string)
    .addParam("operator", "Staking Operator", undefined, config_1.types.string)
    .addOptionalParam("beneficiary", "Stake Beneficiary", undefined, config_1.types.string)
    .addOptionalParam("authorizer", "Stake Authorizer", undefined, config_1.types.string)
    .addOptionalParam("amount", "Stake amount", 1000000, config_1.types.int)
    .addOptionalParam("authorization", "Authorization amount (default: minimumAuthorization)", undefined, config_1.types.int)
    .setAction(function (args, hre) { return __awaiter(void 0, void 0, void 0, function () {
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0: 
            // Initialize staking
            return [4 /*yield*/, hre.run(exports.TASK_INITIALIZE_STAKING, args)
                // Initialize Beacon
            ];
            case 1:
                // Initialize staking
                _a.sent();
                // Initialize Beacon
                return [4 /*yield*/, hre.run(TASK_INITIALIZE_BEACON, args)
                    // Set the operator as a beta operator
                ];
            case 2:
                // Initialize Beacon
                _a.sent();
                // Set the operator as a beta operator
                return [4 /*yield*/, hre.run(TASK_ADD_BETA_OPERATOR_BEACON, args)];
            case 3:
                // Set the operator as a beta operator
                _a.sent();
                return [2 /*return*/];
        }
    });
}); });
(0, config_1.task)(exports.TASK_INITIALIZE_STAKING, "Initializes staking for a service provider")
    .addParam("owner", "Stake Owner address", undefined, config_1.types.string)
    .addParam("provider", "Staking Provider", undefined, config_1.types.string)
    .addOptionalParam("beneficiary", "Stake Beneficiary", undefined, config_1.types.string)
    .addOptionalParam("authorizer", "Stake Authorizer", undefined, config_1.types.string)
    .addOptionalParam("amount", "Stake amount", 1000000, config_1.types.int)
    .setAction(function (args, hre) { return __awaiter(void 0, void 0, void 0, function () {
    var tokensToMint;
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0: return [4 /*yield*/, (0, utils_1.calculateTokensNeededForStake)(hre, args.provider, args.amount)];
            case 1:
                tokensToMint = _a.sent();
                if (!!tokensToMint.isZero()) return [3 /*break*/, 3];
                return [4 /*yield*/, hre.run(exports.TASK_MINT, __assign(__assign({}, args), { amount: tokensToMint.toNumber() }))];
            case 2:
                _a.sent();
                _a.label = 3;
            case 3: return [4 /*yield*/, hre.run(exports.TASK_STAKE, args)];
            case 4:
                _a.sent();
                return [2 /*return*/];
        }
    });
}); });
(0, config_1.task)(exports.TASK_MINT, "Mints T tokens")
    .addParam("owner", "Stake Owner address", undefined, config_1.types.string)
    .addOptionalParam("amount", "Stake amount", 1000000, config_1.types.int)
    .setAction(function (args, hre) { return __awaiter(void 0, void 0, void 0, function () {
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0: return [4 /*yield*/, (0, utils_1.mint)(hre, args.owner, args.amount)];
            case 1:
                _a.sent();
                return [2 /*return*/];
        }
    });
}); });
(0, config_1.task)(exports.TASK_STAKE, "Stakes T tokens")
    .addParam("owner", "Stake Owner address", undefined, config_1.types.string)
    .addParam("provider", "Staking Provider", undefined, config_1.types.string)
    .addOptionalParam("beneficiary", "Stake Beneficiary", undefined, config_1.types.string)
    .addOptionalParam("authorizer", "Stake Authorizer", undefined, config_1.types.string)
    .addOptionalParam("amount", "Stake amount", 1000000, config_1.types.int)
    .setAction(function (args, hre) { return __awaiter(void 0, void 0, void 0, function () {
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0: return [4 /*yield*/, (0, utils_1.stake)(hre, args.owner, args.provider, args.amount, args.beneficiary, args.authorizer)];
            case 1:
                _a.sent();
                return [2 /*return*/];
        }
    });
}); });
(0, config_1.task)(TASK_INITIALIZE_BEACON, "Initializes operator for Beacon").setAction(function (args, hre) { return __awaiter(void 0, void 0, void 0, function () {
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0: return [4 /*yield*/, hre.run(TASK_AUTHORIZE_BEACON, args)];
            case 1:
                _a.sent();
                return [4 /*yield*/, hre.run(TASK_REGISTER_BEACON, args)];
            case 2:
                _a.sent();
                return [2 /*return*/];
        }
    });
}); });
(0, config_1.task)(TASK_AUTHORIZE_BEACON, "Sets authorization for Beacon")
    .addParam("owner", "Stake Owner address", undefined, config_1.types.string)
    .addParam("provider", "Staking Provider", undefined, config_1.types.string)
    .addOptionalParam("authorizer", "Stake Authorizer", undefined, config_1.types.string)
    .addOptionalParam("authorization", "Authorization amount (default: minimumAuthorization)", undefined, config_1.types.int)
    .setAction(function (args, hre) { return __awaiter(void 0, void 0, void 0, function () {
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0: return [4 /*yield*/, (0, utils_1.authorize)(hre, "RandomBeacon", args.owner, args.provider, args.authorizer, args.authorization)];
            case 1:
                _a.sent();
                return [2 /*return*/];
        }
    });
}); });
(0, config_1.task)(TASK_REGISTER_BEACON, "Registers an operator for a staking provider in Beacon")
    .addParam("provider", "Staking Provider", undefined, config_1.types.string)
    .addParam("operator", "Operator Address", undefined, config_1.types.string)
    .setAction(function (args, hre) { return __awaiter(void 0, void 0, void 0, function () {
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0: return [4 /*yield*/, (0, utils_1.register)(hre, "RandomBeacon", args.provider, args.operator)];
            case 1:
                _a.sent();
                return [2 /*return*/];
        }
    });
}); });
(0, config_1.task)(TASK_ADD_BETA_OPERATOR_BEACON, "Adds an operator to the set of beta operators in Beacon")
    .addParam("operator", "Operator Address", undefined, config_1.types.string)
    .setAction(function (args, hre) { return __awaiter(void 0, void 0, void 0, function () {
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0: return [4 /*yield*/, (0, utils_1.addBetaOperator)(hre, "BeaconSortitionPool", args.operator)];
            case 1:
                _a.sent();
                return [2 /*return*/];
        }
    });
}); });
