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
 * Styles for the Card primitive component.
 *
 * BEM block: `.thunderid-card`
 *
 * Modifiers:
 *   --elevated  – medium drop shadow
 *   --outlined  – 1px border, no shadow
 *   --flat      – neither shadow nor border (default)
 */
const CARD_CSS = `
/* ============================================================
   Card
   ============================================================ */

.thunderid-card {
  background-color: var(--thunder-color-background-surface);
  border-radius: var(--thunder-card-borderRadius);
  padding: var(--thunder-card-padding);
  box-sizing: border-box;
  transition: box-shadow var(--thunder-transition-normal);
}

.thunderid-card--elevated {
  box-shadow: var(--thunder-card-shadow);
}

.thunderid-card--outlined {
  border: 1px solid var(--thunder-card-borderColor);
}

/* .thunderid-card--flat: no shadow or border */
`;

export default CARD_CSS;
