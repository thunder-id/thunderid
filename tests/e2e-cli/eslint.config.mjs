import typescriptEslint from "@typescript-eslint/eslint-plugin";
import playwright from "eslint-plugin-playwright";
import tsParser from "@typescript-eslint/parser";

export default [
  {
    ignores: ["node_modules/**", "playwright-report/**", "test-results/**", "blob-report/**", "scripts/**"],
  },
  {
    files: ["**/*.ts"],
    plugins: {
      "@typescript-eslint": typescriptEslint,
      playwright: playwright,
    },
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 2020,
        sourceType: "module",
        project: "./tsconfig.json",
      },
    },
    rules: {
      "@typescript-eslint/no-explicit-any": "warn",
      "@typescript-eslint/explicit-function-return-type": "off",
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          argsIgnorePattern: "^_",
        },
      ],
      quotes: [
        "error",
        "double",
        {
          avoidEscape: true,
          allowTemplateLiterals: true,
        },
      ],
      "playwright/no-conditional-in-test": "error",
      "playwright/no-element-handle": "error",
      "playwright/no-eval": "error",
      "playwright/no-focused-test": "error",
      "playwright/no-skipped-test": "warn",
      "playwright/valid-expect": "error",
      "playwright/no-useless-await": "error",
      "playwright/require-top-level-describe": "error",
      /**
       * This suite drives a terminal, not a page. There is no locator to auto-retry against, so
       * the web-first assertion rule has nothing to apply to, and settling the TUI between
       * keystrokes is a real requirement rather than a smell.
       */
      "playwright/prefer-web-first-assertions": "off",
      "playwright/no-wait-for-timeout": "off",
    },
  },
];
