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
 * Styles for the Typography primitive component.
 *
 * BEM block: `.thunderid-typography`
 *
 * Modifiers (variant):
 *   --h1 | --h2 | --h3 | --h4 | --h5 | --h6
 *   --subtitle1 | --subtitle2
 *   --body1 | --body2
 *   --caption | --overline
 */
const TYPOGRAPHY_CSS = `
/* ============================================================
   Typography
   ============================================================ */

.thunderid-typography {
  font-family: var(--thunder-typography-fontFamily);
  color: var(--thunder-color-text-primary);
  margin: 0;
  line-height: var(--thunder-typography-lineHeight-normal);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.thunderid-typography--h1 {
  font-size: var(--thunder-typography-fontSize-3xl);
  font-weight: var(--thunder-typography-fontWeight-bold);
  line-height: var(--thunder-typography-lineHeight-tight);
  letter-spacing: var(--thunder-typography-letterSpacing-tight);
}

.thunderid-typography--h2 {
  font-size: var(--thunder-typography-fontSize-2xl);
  font-weight: var(--thunder-typography-fontWeight-bold);
  line-height: var(--thunder-typography-lineHeight-tight);
  letter-spacing: var(--thunder-typography-letterSpacing-tight);
}

.thunderid-typography--h3 {
  font-size: var(--thunder-typography-fontSize-xl);
  font-weight: var(--thunder-typography-fontWeight-semibold);
  line-height: var(--thunder-typography-lineHeight-tight);
}

.thunderid-typography--h4 {
  font-size: var(--thunder-typography-fontSize-lg);
  font-weight: var(--thunder-typography-fontWeight-semibold);
}

.thunderid-typography--h5 {
  font-size: var(--thunder-typography-fontSize-md);
  font-weight: var(--thunder-typography-fontWeight-semibold);
}

.thunderid-typography--h6 {
  font-size: var(--thunder-typography-fontSize-sm);
  font-weight: var(--thunder-typography-fontWeight-semibold);
  text-transform: uppercase;
  letter-spacing: var(--thunder-typography-letterSpacing-wide);
}

.thunderid-typography--subtitle1 {
  font-size: var(--thunder-typography-fontSize-lg);
  font-weight: var(--thunder-typography-fontWeight-medium);
}

.thunderid-typography--subtitle2 {
  font-size: var(--thunder-typography-fontSize-md);
  font-weight: var(--thunder-typography-fontWeight-medium);
  color: var(--thunder-color-text-secondary);
}

.thunderid-typography--body1 {
  font-size: var(--thunder-typography-fontSize-md);
  font-weight: var(--thunder-typography-fontWeight-normal);
  line-height: var(--thunder-typography-lineHeight-relaxed);
}

.thunderid-typography--body2 {
  font-size: var(--thunder-typography-fontSize-sm);
  font-weight: var(--thunder-typography-fontWeight-normal);
  line-height: var(--thunder-typography-lineHeight-relaxed);
  color: var(--thunder-color-text-secondary);
}

.thunderid-typography--caption {
  font-size: var(--thunder-typography-fontSize-xs);
  font-weight: var(--thunder-typography-fontWeight-normal);
  color: var(--thunder-color-text-secondary);
}

.thunderid-typography--overline {
  font-size: var(--thunder-typography-fontSize-xs);
  font-weight: var(--thunder-typography-fontWeight-medium);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--thunder-color-text-secondary);
}
`;

export default TYPOGRAPHY_CSS;
