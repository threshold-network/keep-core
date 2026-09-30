"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.parseValue = void 0;
// eslint-disable-next-line import/prefer-default-export
function parseValue(value, hre) {
    var parsed = String(value).trim().split(" ");
    if (parsed.length === 0 || parsed.length > 2) {
        throw new Error("invalid value: " + value);
    }
    return hre.ethers.utils.parseUnits(parsed[0], parsed[1] || "wei");
}
exports.parseValue = parseValue;
