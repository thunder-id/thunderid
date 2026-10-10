// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import thunderIdPlugin from '@thunderid/eslint-plugin';

export default [
  {
    ignores: ['dist/**', 'build/**', 'node_modules/**', 'coverage/**'],
  },
  ...thunderIdPlugin.configs.react,
];
