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

import {FlowTimer, type FlowTimerRenderProps} from '@thunderid/react';
import {cn} from '@thunderid/utils';
import {Alert, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';

/**
 * Props for the TimerAdapter component.
 */
interface TimerAdapterProps {
  /** Duration in seconds until the step expires */
  expiresIn: number;
  /** Text template with {time} placeholder, resolved from the component label */
  textTemplate?: string;
}

/**
 * Oxygen-UI styled timer adapter.
 *
 * Uses the SDK's `FlowTimer` render-prop component to manage
 * the countdown, then renders oxygen-ui styled text.
 */
export default function TimerAdapter({
  expiresIn,
  textTemplate = 'Time remaining: {time}',
}: TimerAdapterProps): JSX.Element {
  return (
    <FlowTimer expiresIn={expiresIn}>
      {({isExpired, formattedTime}: FlowTimerRenderProps) =>
        isExpired ? (
          <Alert className={cn('Flow--timer', 'Alert--root')} severity="warning" sx={{mt: 1}}>
            <Typography className={cn('Text--body2')} variant="body2">
              {formattedTime}
            </Typography>
          </Alert>
        ) : (
          <Typography className={cn('Flow--timer', 'Text--body2')} variant="body2" color="warning.main" sx={{mt: 1}}>
            {textTemplate.replace('{time}', formattedTime)}
          </Typography>
        )
      }
    </FlowTimer>
  );
}
