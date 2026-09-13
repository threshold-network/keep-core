// Preserve the active non-React rules from the former Thesis/Airbnb config.
// Formatting is checked by Prettier; generated code is excluded below.
import tseslint from "typescript-eslint"
import importPlugin from "eslint-plugin-import-x"
import noOnlyTests from "eslint-plugin-no-only-tests"
import stylistic from "@stylistic/eslint-plugin"
import globals from "globals"

export default [
  {
    ignores: [
      "**/.hardhat/**",
      "**/artifacts/**",
      "**/build/**",
      "**/cache/**",
      "**/deployments/**",
      "**/export/**",
      "**/external/**",
      "**/hardhat-dependency-compiler/**",
      "**/typechain/**",
      "**/export.json",
    ],
  },
  {
    files: ["**/*.{js,cjs,mjs,ts}"],
    languageOptions: { ecmaVersion: "latest", globals: globals.node },
    plugins: {
      import: importPlugin,
      "no-only-tests": noOnlyTests,
      "@stylistic": stylistic,
    },
    settings: {
      "import-x/resolver": {
        typescript: { project: "./tsconfig.json" },
        node: true,
      },
      "import-x/parsers": { "@typescript-eslint/parser": [".ts"] },
    },
    rules: {
      "import/no-extraneous-dependencies": [
        "error",
        {
          devDependencies: [
            "test/**/*.ts",
            "tasks/**/*.ts",
            "hardhat.config.ts",
            "*.config.mjs",
            ".prettierrc.js",
          ],
        },
      ],
      "no-plusplus": [
        "error",
        {
          allowForLoopAfterthoughts: true,
        },
      ],
      "import/order": [
        "warn",
        {
          groups: ["builtin", "external", "parent", "sibling", "index", "type"],
          "newlines-between": "always",
          warnOnUnassignedImports: false,
        },
      ],
      "no-param-reassign": [
        "error",
        {
          props: true,
          ignorePropertyModificationsFor: [
            "acc",
            "accumulator",
            "e",
            "ctx",
            "context",
            "req",
            "request",
            "res",
            "response",
            "$scope",
            "staticContext",
          ],
          ignorePropertyModificationsForRegex: ["^immer"],
        },
      ],
      "no-only-tests/no-only-tests": ["error"],
      "no-var": ["error"],
      "prefer-const": [
        "error",
        {
          destructuring: "any",
          ignoreReadBeforeAssign: true,
        },
      ],
      "prefer-rest-params": ["error"],
      "prefer-spread": ["error"],
      "import/no-unresolved": [
        "error",
        {
          commonjs: true,
          caseSensitive: true,
        },
      ],
      "import/namespace": ["error"],
      "import/default": ["error"],
      "import/export": ["error"],
      "import/no-named-as-default": ["warn"],
      "import/no-named-as-default-member": ["warn"],
      "import/no-duplicates": ["warn"],
      "import/extensions": [
        "error",
        "ignorePackages",
        {
          js: "never",
          mjs: "never",
          jsx: "never",
          ts: "never",
          tsx: "never",
        },
      ],
      "no-underscore-dangle": [
        "error",
        {
          allow: ["__REDUX_DEVTOOLS_EXTENSION_COMPOSE__"],
          allowAfterThis: false,
          allowAfterSuper: false,
          enforceInMethodNames: true,
          allowAfterThisConstructor: false,
          allowFunctionParams: true,
        },
      ],
      "class-methods-use-this": ["error"],
      strict: ["error", "never"],
      "import/no-mutable-exports": ["error"],
      "import/no-amd": ["error"],
      "import/first": ["error"],
      "import/newline-after-import": ["error"],
      "import/prefer-default-export": ["error"],
      "import/no-absolute-path": ["error"],
      "import/no-dynamic-require": ["error"],
      "import/no-webpack-loader-syntax": ["error"],
      "import/no-named-default": ["error"],
      "import/no-self-import": ["error"],
      "import/no-cycle": [
        "error",
        {
          ignoreExternal: false,
        },
      ],
      "import/no-useless-path-segments": [
        "error",
        {
          commonjs: true,
        },
      ],
      "arrow-body-style": [
        "error",
        "as-needed",
        {
          requireReturnForObjectLiteral: false,
        },
      ],
      "no-class-assign": ["error"],
      "no-useless-computed-key": ["error"],
      "no-useless-rename": [
        "error",
        {
          ignoreDestructuring: false,
          ignoreImport: false,
          ignoreExport: false,
        },
      ],
      "object-shorthand": [
        "error",
        "always",
        {
          ignoreConstructors: false,
          avoidQuotes: true,
        },
      ],
      "prefer-arrow-callback": [
        "error",
        {
          allowNamedFunctions: false,
          allowUnboundThis: true,
        },
      ],
      "prefer-destructuring": [
        "error",
        {
          VariableDeclarator: {
            array: false,
            object: true,
          },
          AssignmentExpression: {
            array: true,
            object: false,
          },
        },
        {
          enforceForRenamedProperties: false,
        },
      ],
      "prefer-numeric-literals": ["error"],
      "prefer-template": ["error"],
      "require-yield": ["error"],
      "symbol-description": ["error"],
      "no-delete-var": ["error"],
      "no-label-var": ["error"],
      "no-restricted-globals": ["error", "isFinite", "isNaN"],
      "no-shadow-restricted-names": ["error"],
      "no-undef-init": ["error"],
      "func-names": ["warn"],
      "new-cap": [
        "error",
        {
          newIsCap: true,
          newIsCapExceptions: [],
          capIsNew: false,
          capIsNewExceptions: [
            "Immutable.Map",
            "Immutable.Set",
            "Immutable.List",
          ],
          properties: true,
        },
      ],
      "no-bitwise": ["error"],
      "no-continue": ["error"],
      "no-lonely-if": ["error"],
      "no-multi-assign": ["error"],
      "no-nested-ternary": ["error"],
      "no-restricted-syntax": [
        "error",
        {
          selector: "ForInStatement",
        },
        {
          selector: "ForOfStatement",
        },
        {
          selector: "LabeledStatement",
        },
        {
          selector: "WithStatement",
        },
      ],
      "no-unneeded-ternary": [
        "error",
        {
          defaultAssignment: false,
        },
      ],
      "one-var": ["error", "never"],
      "operator-assignment": ["error", "always"],
      "prefer-object-spread": ["error"],
      "global-require": ["error"],
      "no-buffer-constructor": ["error"],
      "no-new-require": ["error"],
      "no-path-concat": ["error"],
      "for-direction": ["error"],
      "no-async-promise-executor": ["error"],
      "no-compare-neg-zero": ["error"],
      "no-cond-assign": ["error", "always"],
      "no-console": ["warn"],
      "no-constant-condition": ["warn"],
      "no-control-regex": ["error"],
      "no-debugger": ["error"],
      "no-duplicate-case": ["error"],
      "no-empty": ["error"],
      "no-empty-character-class": ["error"],
      "no-ex-assign": ["error"],
      "no-extra-boolean-cast": ["error"],
      "no-inner-declarations": ["error"],
      "no-invalid-regexp": ["error"],
      "no-irregular-whitespace": ["error"],
      "no-misleading-character-class": ["error"],
      "no-prototype-builtins": ["error"],
      "no-regex-spaces": ["error"],
      "no-sparse-arrays": ["error"],
      "no-template-curly-in-string": ["error"],
      "no-unsafe-finally": ["error"],
      "use-isnan": ["error"],
      "array-callback-return": [
        "error",
        {
          allowImplicit: true,
          checkForEach: false,
        },
      ],
      "block-scoped-var": ["error"],
      "consistent-return": ["error"],
      "default-case": [
        "error",
        {
          commentPattern: "^no default$",
        },
      ],
      eqeqeq: [
        "error",
        "always",
        {
          null: "ignore",
        },
      ],
      "guard-for-in": ["error"],
      "max-classes-per-file": ["error", 1],
      "no-alert": ["warn"],
      "no-caller": ["error"],
      "no-case-declarations": ["error"],
      "no-else-return": [
        "error",
        {
          allowElseIf: false,
        },
      ],
      "no-empty-pattern": ["error"],
      "no-eval": ["error"],
      "no-extend-native": ["error"],
      "no-extra-bind": ["error"],
      "no-extra-label": ["error"],
      "no-fallthrough": ["error"],
      "no-global-assign": [
        "error",
        {
          exceptions: [],
        },
      ],
      "no-iterator": ["error"],
      "no-labels": [
        "error",
        {
          allowLoop: false,
          allowSwitch: false,
        },
      ],
      "no-lone-blocks": ["error"],
      "no-multi-str": ["error"],
      "no-new": ["error"],
      "no-new-wrappers": ["error"],
      "no-octal": ["error"],
      "no-octal-escape": ["error"],
      "no-proto": ["error"],
      "no-restricted-properties": [
        "error",
        {
          object: "arguments",
          property: "callee",
        },
        {
          object: "global",
          property: "isFinite",
        },
        {
          object: "self",
          property: "isFinite",
        },
        {
          object: "window",
          property: "isFinite",
        },
        {
          object: "global",
          property: "isNaN",
        },
        {
          object: "self",
          property: "isNaN",
        },
        {
          object: "window",
          property: "isNaN",
        },
        {
          property: "__defineGetter__",
        },
        {
          property: "__defineSetter__",
        },
        {
          object: "Math",
          property: "pow",
        },
      ],
      "no-return-assign": ["error", "always"],
      "no-script-url": ["error"],
      "no-self-assign": [
        "error",
        {
          props: true,
        },
      ],
      "no-self-compare": ["error"],
      "no-sequences": ["error"],
      "no-unused-labels": ["error"],
      "no-useless-catch": ["error"],
      "no-useless-concat": ["error"],
      "no-useless-escape": ["error"],
      "no-useless-return": ["error"],
      "no-void": ["error"],
      "no-with": ["error"],
      "prefer-promise-reject-errors": [
        "error",
        {
          allowEmptyReject: true,
        },
      ],
      radix: ["error"],
      "vars-on-top": ["error"],
      yoda: ["error"],
      "no-object-constructor": ["error"],
      "@stylistic/spaced-comment": [
        "error",
        "always",
        {
          line: {
            exceptions: ["-", "+"],
            markers: ["=", "!", "/"],
          },
          block: {
            exceptions: ["-", "+"],
            markers: ["=", "!", ":", "::"],
            balanced: true,
          },
        },
      ],
      "@stylistic/lines-between-class-members": [
        "error",
        "always",
        {
          exceptAfterSingleLine: false,
        },
      ],
    },
  },
  {
    files: ["**/*.ts"],
    languageOptions: {
      parser: tseslint.parser,
      sourceType: "module",
      parserOptions: {
        project: "./.tsconfig-eslint.json",
        tsconfigRootDir: import.meta.dirname,
      },
    },
    plugins: { "@typescript-eslint": tseslint.plugin },
    rules: {
      "@typescript-eslint/no-explicit-any": ["warn"],
      "@typescript-eslint/consistent-type-imports": ["warn"],
      "@typescript-eslint/adjacent-overload-signatures": ["error"],
      "@typescript-eslint/ban-ts-comment": ["error"],
      "@typescript-eslint/explicit-module-boundary-types": ["warn"],
      "@typescript-eslint/no-array-constructor": ["error"],
      "@typescript-eslint/no-empty-function": [
        "error",
        {
          allow: ["arrowFunctions", "functions", "methods"],
        },
      ],
      "@typescript-eslint/no-extra-non-null-assertion": ["error"],
      "@typescript-eslint/no-inferrable-types": ["error"],
      "@typescript-eslint/no-misused-new": ["error"],
      "@typescript-eslint/no-namespace": ["error"],
      "@typescript-eslint/no-non-null-asserted-optional-chain": ["error"],
      "@typescript-eslint/no-non-null-assertion": ["warn"],
      "@typescript-eslint/no-this-alias": ["error"],
      "@typescript-eslint/no-unused-vars": [
        "warn",
        {
          vars: "all",
          args: "after-used",
          ignoreRestSiblings: true,
        },
      ],
      "@typescript-eslint/prefer-as-const": ["error"],
      "@typescript-eslint/prefer-namespace-keyword": ["error"],
      "@typescript-eslint/triple-slash-reference": ["error"],
      "@typescript-eslint/naming-convention": [
        "error",
        {
          selector: "variable",
          format: ["camelCase", "PascalCase", "UPPER_CASE"],
        },
        {
          selector: "function",
          format: ["camelCase", "PascalCase"],
        },
        {
          selector: "typeLike",
          format: ["PascalCase"],
        },
      ],
      "@typescript-eslint/dot-notation": [
        "error",
        {
          allowKeywords: true,
          allowPattern: "",
          allowPrivateClassPropertyAccess: false,
          allowProtectedClassPropertyAccess: false,
          allowIndexSignaturePropertyAccess: false,
        },
      ],
      "@typescript-eslint/no-dupe-class-members": ["error"],
      "@typescript-eslint/no-implied-eval": ["error"],
      "@typescript-eslint/no-loop-func": ["error"],
      "@typescript-eslint/no-redeclare": ["error"],
      "@typescript-eslint/no-shadow": ["error"],
      "@typescript-eslint/no-unused-expressions": [
        "error",
        {
          allowShortCircuit: false,
          allowTernary: false,
          allowTaggedTemplates: false,
          enforceForJSX: false,
        },
      ],
      "@typescript-eslint/no-useless-constructor": ["error"],
      "@typescript-eslint/return-await": ["error"],
      "@typescript-eslint/no-empty-object-type": ["error"],
      "@typescript-eslint/only-throw-error": ["error"],
      "@typescript-eslint/no-require-imports": [
        "error",
        { allowAsImport: true },
      ],
      "@typescript-eslint/no-unsafe-function-type": ["error"],
      "@typescript-eslint/no-wrapper-object-types": ["error"],
    },
  },
  { files: ["test/**/*.ts"], languageOptions: { globals: globals.mocha } },
  { files: ["**/*.{js,cjs}"], languageOptions: { sourceType: "commonjs" } },
]
