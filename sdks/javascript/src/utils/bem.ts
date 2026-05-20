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

/**
 * Creates a BEM-style class name by combining a base class with element and/or modifier
 *
 * @param baseClass - The base CSS class string (usually from emotion's css function)
 * @param element - The BEM element name (optional)
 * @param modifier - The BEM modifier name (optional)
 * @returns The combined class name string
 *
 * @example
 * ```tsx
 * const baseClass = css`
 *   display: flex;
 *   &__element {
 *     color: red;
 *   }
 *   &--modifier {
 *     background: blue;
 *   }
 * `;
 *
 * import bem from './utils/bem';
 *
 * const elementClass = bem(baseClass, 'element');
 * const modifierClass = bem(baseClass, null, 'modifier');
 * const elementWithModifierClass = bem(baseClass, 'element', 'modifier');
 * ```
 */
const bem = (baseClass: string, element?: string | null, modifier?: string | null): string => {
  let className: string = baseClass;

  if (element) {
    className += `__${element}`;
  }

  if (modifier) {
    className += `--${modifier}`;
  }

  return className;
};

export default bem;
