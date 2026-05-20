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
 * Styles for the Spinner primitive component.
 *
 * BEM block: `.thunderid-spinner`
 *
 * Modifiers:
 *   Size: --small | --medium | --large
 *
 * Elements:
 *   __svg | __circle
 *
 * Note: The `thunder-spin` and `thunder-spinner-dash` keyframe animations
 * are defined in `styles/animations.css.ts` and shared with the Button component.
 */
const SPINNER_CSS = `
/* ============================================================
   Spinner
   ============================================================ */

.thunderid-spinner {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--thunder-color-primary-main);
}

.thunderid-spinner--small {
  width: calc(var(--thunder-spacing-unit) * 2);
  height: calc(var(--thunder-spacing-unit) * 2);
}

.thunderid-spinner--medium {
  width: calc(var(--thunder-spacing-unit) * 2.5);
  height: calc(var(--thunder-spacing-unit) * 2.5);
}

.thunderid-spinner--large {
  width: calc(var(--thunder-spacing-unit) * 3.5);
  height: calc(var(--thunder-spacing-unit) * 3.5);
}

.thunderid-spinner__svg {
  width: 100%;
  height: 100%;
  animation: thunder-spin 1.4s linear infinite;
}

.thunderid-spinner__circle {
  stroke-dasharray: 80, 200;
  stroke-dashoffset: 0;
  animation: thunder-spinner-dash 1.4s ease-in-out infinite;
}
`;

export default SPINNER_CSS;
