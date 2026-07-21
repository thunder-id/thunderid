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

import {Box} from '@wso2/oxygen-ui';

/**
 * `components` map for react-i18next's `<Trans>`. Renders `<code>` mentions of exact
 * configuration keys (e.g. `client_credentials`) in monospace, visually separating them from
 * the surrounding explanatory prose.
 */
export const codeComponents = {
  code: (
    <Box
      component="code"
      sx={{
        fontFamily: 'monospace',
        fontSize: '0.85em',
        color: 'primary.main',
        bgcolor: 'action.selected',
        borderRadius: 0.5,
        px: 0.5,
      }}
    />
  ),
};
