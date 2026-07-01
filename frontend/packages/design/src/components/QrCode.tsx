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

import {QRCodeSVG} from 'qrcode.react';
import type {JSX} from 'react';

export interface QrCodeProps {
  /** The value encoded into the QR code. */
  value: string;
  /** Side length in pixels. Defaults to 220. */
  size?: number;
}

/** Renders a QR code as an inline SVG. */
export default function QrCode({value, size = 220}: QrCodeProps): JSX.Element {
  return <QRCodeSVG value={value} size={size} />;
}
