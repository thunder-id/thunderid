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

import {FC} from 'react';

export interface ChevronDownProps {
  /**
   * Color of the icon.
   */
  color?: string;
  /**
   * Height of the icon.
   */
  height?: number | string;
  /**
   * Width of the icon.
   */
  width?: number | string;
}

/**
 * ChevronDown Icon component.
 *
 * @param props - Props injected to the component.
 * @returns ChevronDown Icon component.
 */
const ChevronDown: FC<ChevronDownProps> = ({color = 'currentColor', height = 24, width = 24}: ChevronDownProps) => (
  <svg width={width} height={height} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="m6 9 6 6 6-6" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

ChevronDown.displayName = 'ChevronDown';

export default ChevronDown;
