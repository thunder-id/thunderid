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

import {Box, Button} from '@wso2/oxygen-ui';
import {QRCodeSVG} from 'qrcode.react';
import type {JSX} from 'react';
import type {FlowComponent} from '../../../models/flow';

interface QrCodeAdapterProps {
  additionalData?: Record<string, unknown>;
  component: FlowComponent;
}

export default function QrCodeAdapter({component, additionalData = {}}: QrCodeAdapterProps): JSX.Element | null {
  const sourceKey = (component as FlowComponent & {source?: string}).source;
  const rawValue = sourceKey && additionalData ? additionalData[sourceKey] : undefined;
  const uri = typeof rawValue === 'string' ? rawValue : '';

  if (!uri) {
    return null;
  }

  return (
    <Box sx={{alignItems: 'center', display: 'flex', flexDirection: 'column', gap: 2, width: '100%'}}>
      <QRCodeSVG value={uri} size={220} />
      <Button fullWidth variant="outlined" href={uri}>
        Open wallet on this device
      </Button>
    </Box>
  );
}
