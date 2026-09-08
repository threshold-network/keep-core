"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.parseValue = parseValue;
// eslint-disable-next-line import/prefer-default-export
function parseValue(value, hre) {
    const parsed = String(value).trim().split(" ");
    if (parsed.length === 0 || parsed.length > 2) {
        throw new Error(`invalid value: ${value}`);
    }
    return hre.ethers.parseUnits(parsed[0], parsed[1] || "wei");
}
