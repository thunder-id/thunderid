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
 * Styles for the Logo primitive component.
 *
 * BEM block: `.thunderid-logo`
 *
 * Elements:
 *   __image
 */
const LOGO_CSS = `
/* ============================================================
   Logo
   ============================================================ */

.thunderid-logo {
  display: inline-flex;
  align-items: center;
  text-decoration: none;
  transition: opacity var(--thunder-transition-fast);
}

.thunderid-logo:hover {
  opacity: 0.85;
}

.thunderid-logo__image {
  display: block;
  max-height: 100%;
  width: auto;
  height: auto;
  object-fit: contain;
}
`;

export default LOGO_CSS;
