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

import type {Theme} from '@thunderid/design';
import {ElementTypes} from '../models/elements';
import {SimulationOptionKinds} from '../utils/getSimulationOptions';

/**
 * Device presets available in the simulation preview panel.
 */
export type PreviewDevice = 'mobile' | 'tablet' | 'desktop';

export const PREVIEW_DEVICES: PreviewDevice[] = ['mobile', 'tablet', 'desktop'];

/**
 * i18n fallback label per device preset (the translation keys are built dynamically).
 */
export const DEVICE_LABEL_FALLBACKS: Record<PreviewDevice, string> = {
  mobile: 'Mobile',
  tablet: 'Tablet',
  desktop: 'Desktop',
};

/**
 * Aspect ratio per device preset. The themed preview scales the login page to
 * fill a box of this shape within the panel, so the proportions convey the device.
 */
export const DEVICE_ASPECT_RATIOS: Record<PreviewDevice, string> = {
  mobile: '360 / 640',
  tablet: '540 / 680',
  desktop: '780 / 560',
};

/**
 * When an application has no design configured, the flow meta API hands the gate
 * an empty theme object (see backend flowmeta service), which the design provider
 * merges over its standard defaults. The preview mirrors that exact fallback.
 */
export const GATE_DEFAULT_THEME = {} as Theme;

/**
 * Width of the gate's sign-in card. The sketch renders at this width with the
 * exact typography variants the end user sees, then zooms down as a whole to
 * fit the panel — scaling the UI instead of substituting smaller variants.
 */
export const GATE_CARD_WIDTH = 450;

export const PANEL_CONTENT_WIDTH = 380;

export const SKETCH_ZOOM = PANEL_CONTENT_WIDTH / GATE_CARD_WIDTH;

/**
 * Element types the sketch renders as a plain text field. Narrower than the
 * flow graph's input set — OTP, checkbox, and dropdown get their own sketches.
 */
export const SKETCH_TEXT_INPUT_TYPES = new Set<string>([
  ElementTypes.TextInput,
  ElementTypes.PasswordInput,
  ElementTypes.EmailInput,
  ElementTypes.PhoneInput,
  ElementTypes.NumberInput,
  ElementTypes.DateInput,
]);

/**
 * i18n label key per transition kind, used when an option has no action label.
 */
export const KIND_LABEL_KEYS: Record<SimulationOptionKinds, string> = {
  [SimulationOptionKinds.Action]: 'flows:core.simulation.kinds.action',
  [SimulationOptionKinds.Success]: 'flows:core.simulation.kinds.success',
  [SimulationOptionKinds.Incomplete]: 'flows:core.simulation.kinds.incomplete',
  [SimulationOptionKinds.Failure]: 'flows:core.simulation.kinds.failure',
};

/**
 * i18n fallback label per transition kind.
 */
export const KIND_LABEL_FALLBACKS: Record<SimulationOptionKinds, string> = {
  [SimulationOptionKinds.Action]: 'Continue',
  [SimulationOptionKinds.Success]: 'On success',
  [SimulationOptionKinds.Incomplete]: 'On incomplete',
  [SimulationOptionKinds.Failure]: 'On failure',
};

/**
 * Palette color per transition kind — matches the edge stroke color coding on
 * the canvas.
 */
export const KIND_COLORS: Record<SimulationOptionKinds, 'primary' | 'success' | 'warning' | 'error'> = {
  [SimulationOptionKinds.Action]: 'primary',
  [SimulationOptionKinds.Success]: 'success',
  [SimulationOptionKinds.Incomplete]: 'warning',
  [SimulationOptionKinds.Failure]: 'error',
};
