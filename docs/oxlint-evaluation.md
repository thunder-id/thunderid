---
title: Oxlint Evaluation Report
sidebar_label: Oxlint Evaluation
description: Evaluation of oxlint as a potential ESLint replacement for the frontend codebase
docType: guide
---

# ⚡ Oxlint Evaluation Report: ESLint Replacement Assessment

> **Status:** Evaluation Only (No pipeline modifications)  
> **Target Scope:** `@thunderid/frontend` (2,042 `.ts`, `.tsx`, `.js`, `.jsx` files across 25 monorepo packages & apps)

## 1. Executive Summary

This document evaluates **oxlint** (Rust-based linter from the Oxc toolchain, v1.77.0) as a potential replacement for **ESLint** in the frontend codebase. 

### Key Findings
- **Performance:** oxlint (non-type-aware) is **~144.9x faster** than ESLint (type-aware) on local development hardware (**1.23s** vs **178.52s**).
- **TypeScript & React Parsing:** Parsed 2,042 TypeScript and React files with **0 syntax or AST parser errors**.
- **Rule Coverage:** Covers the vast majority of standard JavaScript, React Hooks, JSX accessibility (`jsx-a11y`), and basic TypeScript lint rules. Caught **4 circular dependency cycles** (`import/no-cycle`) in under 500ms.
- **Recommendation:** **Hybrid Pipeline Recommended.** Full replacement is currently **blocked** by custom plugin requirements (`@thunderid/copyright-header`) and full type-aware linting (`@typescript-eslint/recommendedTypeChecked`). However, adopting oxlint as a pre-commit / fast-feedback linter alongside ESLint yields immediate productivity gains.

## 2. Benchmark Comparison

Benchmarks were conducted locally on macOS Apple Silicon (arm64, 8 CPU threads, Node.js v20.11.0) executing across the full frontend workspace (`frontend/`).

### Local Execution Results

| Run | ESLint (Flat Config + Type-Aware) | oxlint v1.77.0 (Non-Type-Aware) | Speedup |
| :--- | :--- | :--- | :--- |
| **Run 1 (Warmup)** | 178.52 s | 1.896 s | 94.1x |
| **Run 2** | 177.68 s | 1.232 s | 144.2x |
| **Run 3** | 179.10 s | 1.192 s | 150.2x |
| **Median Time** | **178.52 s (~2m 58s)** | **1.232 s (~1.2s)** | **~144.9x faster** |

```text
Time Comparison (Median):
ESLint :  ████████████████████████████████████████ 178.52s
oxlint :  █ 1.23s
```

### Estimated CI Execution Impact (GitHub Actions `ubuntu-latest` 2-Core)

| Pipeline Metric | Existing ESLint Pipeline | Potential oxlint Pipeline |
| :--- | :--- | :--- |
| **CI Lint Job Time** | ~4 - 6 minutes | ~3 - 5 seconds |
| **Dependency Footprint** | ~50+ npm packages (~180MB node_modules) | Single binary executable (~15MB) |
| **Memory Footprint** | ~2GB - 4GB (Heap Out-of-Memory risk without `--max-old-space-size`) | ~120MB peak RAM |

## 3. Supported vs. Unsupported Rules

The current frontend ESLint setup is defined in `@thunderid/eslint-plugin`. Below is a breakdown of rule parity with oxlint.

### A. Equivalent & Covered Rules in Oxlint

| ESLint Rule | Oxlint Parity | Notes |
| :--- | :--- | :--- |
| `no-console` | ✅ Supported (`no-console`) | Configured as `error`. |
| `no-new` | ✅ Supported (`no-new`) | Configured as `error`. |
| `import-x/no-cycle` | ✅ Supported (`import/no-cycle`) | **Actively firing:** Instantly caught 4 circular imports across components. |
| `import-x/extensions` | ✅ Supported (`import/extensions`) | Supported via import plugin; minor path-alias extension-suppression behavior differences apply. |
| `@typescript-eslint/no-explicit-any` | ✅ Supported (`@typescript-eslint/no-explicit-any`) | Configured as `error`. |
| `react/no-danger` | ✅ Supported (`react/no-danger`) | Configured as `error`. |
| `react/jsx-no-useless-fragment` | ✅ Supported (`react/jsx-no-useless-fragment`) | Configured as `error`. |
| `react/no-array-index-key` | ✅ Supported (`react/no-array-index-key`) | Configured as `error`. |
| `react-hooks/rules-of-hooks` | ✅ Supported (`react/rules-of-hooks`) | Enforced accurately. |
| `react-hooks/exhaustive-deps` | ✅ Supported (`react/exhaustive-deps`) | **Actively firing:** Caught 1 warning in `TranslationsList.tsx`. |
| `jsx-a11y/*` (all 25+ rules) | ✅ Supported (`jsx-a11y/*`) | Complete parity with `eslint-plugin-jsx-a11y`. |
| `vitest/expect-expect` | ✅ Supported (`vitest/expect-expect`) | Configured for test files. |
| `react-refresh/only-export-components` | ✅ Supported (`react/only-export-components`) | Supported via React plugin. |

### B. Unsupported ESLint Rules (Gaps & Blockers)

| ESLint Rule / Category | Status in Oxlint | Impact on Codebase |
| :--- | :--- | :--- |
| `@thunderid/copyright-header` | ❌ **Unsupported** | **CRITICAL BLOCKER.** Custom JS rule enforcing Apache 2.0 headers. Tested via Oxlint's `jsPlugins` option with `@thunderid/eslint-plugin`, but custom AST rule execution is incompatible. |
| `@typescript-eslint/recommendedTypeChecked` | ⚠️ **Limited / Non-Type-Aware** | **BLOCKER.** Type-aware rules (e.g. `@typescript-eslint/no-unsafe-assignment`, `@typescript-eslint/await-thenable`, `@typescript-eslint/no-floating-promises`) require TypeScript compiler type info. |
| `import-x/order` | ❌ **Unsupported** | **Relied upon.** Enforces custom import sorting (`builtin`, `external`, `index`, `sibling`, `parent`, `internal`). Oxlint does not support configured import sorting. |
| `react/require-default-props` | ❌ **Unsupported** | Not present in oxlint rule catalog. |
| `react/no-unused-prop-types` | ❌ **Unsupported** | Not present in oxlint rule catalog. |

### C. Useful Oxlint Rules Not Currently Used in ESLint Config

Oxlint bundles 840+ built-in rules without extra npm plugins. The following high-value rules could be adopted:

- `unicorn/prefer-node-protocol`: Enforces `node:` prefix for Node builtins (e.g., `import fs from 'node:fs'`).
- `unicorn/prefer-array-flat-map`: Prefers `.flatMap()` over `.map().flat()`.
- `unicorn/no-abusive-eslint-disable`: Prevents blanket `// eslint-disable-next-line` without rule names.
- `oxc/number-arg-out-of-range`: Prevents out-of-range arguments in standard JS methods.
- Extended `vitest` suite: `vitest/no-disabled-tests`, `vitest/prefer-to-be-truthy`, `vitest/require-to-throw-message`.

## 4. TypeScript & React Compatibility & Behavioral Differences

### 1. Parser Compatibility
- **TS/TSX Parsing:** 100% compatible. Oxlint successfully parsed 2,042 files including complex generic React components, JSX syntax, type aliases, and TS 5+ features (`satisfies`, const type parameters).
- **Parser Errors:** 0 parser errors encountered.

### 2. Behavioral Differences & Observations
- **Circular Import Detection:** Oxlint's `import/no-cycle` is dramatically faster than ESLint. It detected circular import cycles in under 500ms, including:
  - `McpServerSectionContent.tsx` ↔ `PermissionCatalog.tsx`
  - `StackAdapter.tsx` ↔ `FlowComponentRenderer.tsx`
- **Memory Efficiency:** ESLint requires `NODE_OPTIONS="--max-old-space-size=8192"` when processing the monorepo to avoid `JavaScript heap out of memory` crashes. Oxlint executes natively with ~120MB peak RAM.
- **Formatting Rules:** ESLint config includes `object-curly-spacing`. Oxlint deliberately defers formatting rules to Prettier, which aligns with modern best practices.

## 5. Overall Recommendation & Migration Feasibility

### Is full migration to oxlint feasible today?
**No, full replacement is blocked at this time.**

### Primary Blockers:
1. **Custom Copyright Header Rule (`@thunderid/copyright-header`):** Open-source governance requires Apache 2.0 headers on all source files. Tested via Oxlint's `jsPlugins` with `@thunderid/eslint-plugin`, but custom AST rule execution is incompatible.
2. **Type-Aware Linting (`@typescript-eslint/recommendedTypeChecked`):** The frontend relies on type-aware checking to catch type unsafety across monorepo packages.

### Recommended Strategy: Dual / Hybrid Linter Setup

```text
[Developer Edit / Save] ──▶ oxlint (Sub-second pre-commit check)
                                  │
                                  ▼
[PR / CI Check]          ──▶ oxlint (Fast failure check: ~1s)
                                  │
                                  ▼
[CI Verification]        ──▶ ESLint (Type-aware & Copyright check)
```

1. **Pre-commit & Fast CI:** Use `oxlint` for instant developer feedback on JS/TS AST errors, React Hooks rules, and circular dependency checks.
2. **Comprehensive CI Validation:** Retain ESLint in CI for type-aware safety checks and copyright header verification.

## 6. Artifacts Delivered

- `.oxlintrc.json` & `frontend/.oxlintrc.json`: Temporary oxlint configuration mirroring current ESLint intent.
- `scripts/benchmark-linter.sh`: Executable benchmark script for reproducing timing tests.
- `docs/oxlint-evaluation.md`: This comprehensive evaluation report.
