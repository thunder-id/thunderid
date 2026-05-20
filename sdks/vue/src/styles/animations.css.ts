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
 * Shared CSS keyframe animations used by multiple primitive components.
 *
 * `thunder-spin`          - used by Spinner (__svg) and Button (__spinner)
 * `thunder-spinner-dash`  - used by Spinner (__circle)
 *
 * Defined once here rather than in each component's CSS file to avoid
 * duplicate `@keyframes` blocks in the injected stylesheet.
 */
const ANIMATIONS_CSS = `
/* ============================================================
   ThunderID Vue SDK – shared keyframe animations
   ============================================================ */

@keyframes thunder-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

@keyframes thunder-spinner-dash {
  0% {
    stroke-dasharray: 1, 200;
    stroke-dashoffset: 0;
  }
  50% {
    stroke-dasharray: 89, 200;
    stroke-dashoffset: -35px;
  }
  100% {
    stroke-dasharray: 89, 200;
    stroke-dashoffset: -124px;
  }
}
`;

export default ANIMATIONS_CSS;
