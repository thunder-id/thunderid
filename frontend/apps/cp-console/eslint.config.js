// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import thunderIdPlugin from '@thunderid/eslint-plugin';

export default [
  {
    ignores: ['dist/**', 'build/**', 'node_modules/**', 'coverage/**'],
  },
  ...thunderIdPlugin.configs.react,
  ...thunderIdPlugin.configs.vitest,
  {
    rules: {
      /* --- TEMPORARILY TURNED OFF RULES --- */
      /* TODO: Revisit these rules and enable them after refactoring the codebase. */
      'react-hooks/preserve-manual-memoization': 'off',
      'typescript-eslint/require-await': 'off',
    },
  },
];
