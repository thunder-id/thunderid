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

import type {SxProps, Theme} from '@wso2/oxygen-ui';

/**
 * Shared style for the persistent dashed "add" buttons rendered inside views
 * and form blocks on the flow builder canvas.
 */
const dashedAddButtonSx: SxProps<Theme> = {
  py: 1,
  borderRadius: 1.5,
  border: '1.5px dashed',
  borderColor: 'divider',
  color: 'text.secondary',
  fontWeight: 500,
  textTransform: 'none',
  backgroundColor: 'action.hover',
  transition: 'all 0.18s ease',
  '&:hover': {
    border: '1.5px dashed',
    borderColor: 'primary.main',
    color: 'primary.main',
    backgroundColor: 'primary.50',
  },
};

export default dashedAddButtonSx;
