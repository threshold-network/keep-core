module.exports = {
  semi: false,
  trailingComma: "all",
  plugins: ["prettier-plugin-solidity", "prettier-plugin-sh"],
  overrides: [
    {
      files: "*.sol",
      options: {
        tabWidth: 4,
      },
    },
  ],
}
