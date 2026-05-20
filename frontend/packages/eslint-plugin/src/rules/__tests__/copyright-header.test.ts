/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {Linter, RuleTester} from 'eslint';
import copyrightHeaderRule from '../copyright-header.js';

const ruleTester = new RuleTester({
  languageOptions: {
    ecmaVersion: 2020,
    sourceType: 'module',
  },
} as Linter.Config);

const getCurrentYear = (): number => new Date().getFullYear();

const VALID_COPYRIGHT = `/**
 * Copyright (c) ${getCurrentYear()}, WSO2 LLC. (https://www.wso2.com).
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
 */`;

ruleTester.run('copyright-header', copyrightHeaderRule, {
  valid: [
    {
      code: `${VALID_COPYRIGHT}

export const foo = 'bar';`,
    },
    {
      code: `${VALID_COPYRIGHT}

function hello() {
  return 'world';
}`,
    },
  ],
  invalid: [
    {
      code: `export const foo = 'bar';`,
      output: `${VALID_COPYRIGHT}

export const foo = 'bar';`,
      errors: [
        {
          messageId: 'missingHeader',
        },
      ],
    },
    {
      code: `/*
 * Some other comment
 */

export const foo = 'bar';`,
      output: `${VALID_COPYRIGHT}

export const foo = 'bar';`,
      errors: [
        {
          messageId: 'incorrectHeader',
        },
      ],
    },
    {
      code: `// Single line comment

export const foo = 'bar';`,
      output: `// Single line comment

${VALID_COPYRIGHT}

export const foo = 'bar';`,
      errors: [
        {
          messageId: 'missingHeader',
        },
      ],
    },
  ],
});
