/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {dirname} from 'path';
import {fileURLToPath} from 'url';
import thunderIdPlugin, {createParserOptions} from '@thunderid/eslint-plugin';

const __filename = fileURLToPath(import.meta.url);

const __dirname = dirname(__filename);

export default [
  {
    ignores: ['dist/**', 'build/**', 'node_modules/**', 'coverage/**', '.docusaurus/**', 'plugins/**/*.js'],
  },
  ...thunderIdPlugin.configs.react,
  {
    files: ['**/*.ts', '**/*.tsx', '**/*.js', '**/*.jsx'],
    languageOptions: {
      parserOptions: createParserOptions({
        tsconfigRootDir: __dirname,
        project: './tsconfig.eslint.json',
      }),
    },
    rules: {
      'import-x/no-unresolved': [
        'error',
        {
          ignore: ['^@docusaurus/', '^@theme/', '^@theme-original/', '^@generated/', '^@site/'],
        },
      ],
    },
  },
  {
    files: ['**/*.mjs'],
    languageOptions: {
      parserOptions: {
        project: false,
      },
    },
  },
  {
    files: ['scripts/**/*.mjs'],
    languageOptions: {
      globals: {
        process: 'readonly',
        __dirname: 'readonly',
        __filename: 'readonly',
        URL: 'readonly',
        console: 'readonly',
        Buffer: 'readonly',
      },
    },
    rules: {
      'import/no-extraneous-dependencies': 'off',
      'import-x/extensions': 'off',
      '@thunderid/copyright-header': ['error', {allowShebang: true}],
    },
  },
  {
    files: ['plugins/shims/*.cjs'],
    languageOptions: {
      globals: {
        module: 'writable',
        require: 'readonly',
        __dirname: 'readonly',
        __filename: 'readonly',
        process: 'readonly',
        exports: 'writable',
      },
    },
  },
];
