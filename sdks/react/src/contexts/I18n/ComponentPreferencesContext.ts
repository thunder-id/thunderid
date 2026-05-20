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

import {Preferences} from '@thunderid/browser';
import {Context, createContext} from 'react';

/**
 * Context for component-level preferences overrides.
 * Presentational components can provide this context to override the global i18n
 * and theme settings for their entire subtree, including all nested components.
 */
const ComponentPreferencesContext: Context<Preferences | undefined> = createContext<Preferences | undefined>(undefined);

export default ComponentPreferencesContext;
