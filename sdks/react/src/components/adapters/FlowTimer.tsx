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

import {FC, ReactNode, useEffect, useState} from 'react';
import Typography from '../primitives/Typography/Typography';

/**
 * Render props exposed by FlowTimer when using the render-prop pattern.
 */
export interface FlowTimerRenderProps {
  /** Human-readable formatted time string (e.g. "2:30" or "Timed out"). */
  formattedTime: string;
  /** Whether the timer has expired. */
  isExpired: boolean;
  /** Remaining time in seconds. 0 when expired. */
  remaining: number;
}

/**
 * Props for the FlowTimer component.
 */
export interface FlowTimerProps {
  /**
   * Render-props callback. When provided, the default countdown display is replaced
   * with whatever JSX the callback returns.
   *
   * @example
   * ```tsx
   * <FlowTimer expiresIn={300}>
   *   {({ remaining, isExpired, formattedTime }) => (
   *     <span style={{ color: isExpired ? 'red' : 'inherit' }}>
   *       {isExpired ? 'Session expired' : `Time left: ${formattedTime}`}
   *     </span>
   *   )}
   * </FlowTimer>
   * ```
   */
  children?: (props: FlowTimerRenderProps) => ReactNode;
  /** Initial number of seconds for the countdown. 0 or negative means no timer. */
  expiresIn?: number;
  /** Text template for the countdown display. Use {time} as a placeholder. */
  textTemplate?: string;
}

/**
 * Flow countdown timer component.
 *
 * Displays a countdown from the given number of seconds. When the time expires,
 * shows "Timed out". Returns null if expiresIn <= 0.
 */
const FlowTimer: FC<FlowTimerProps> = ({
  expiresIn = 0,
  textTemplate = 'Time remaining: {time}',
  children,
}: FlowTimerProps) => {
  const [remaining, setRemaining] = useState<number>(expiresIn > 0 ? expiresIn : 0);

  useEffect(() => {
    if (expiresIn <= 0) {
      return undefined;
    }

    setRemaining(expiresIn);

    const interval: NodeJS.Timeout = setInterval(() => {
      setRemaining((prev: number) => {
        if (prev <= 1) {
          clearInterval(interval);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(interval);
  }, [expiresIn]);

  if (expiresIn <= 0) {
    return null;
  }

  const formatTime = (seconds: number): string => {
    if (seconds <= 0) {
      return 'Timed out';
    }
    const m: number = Math.floor(seconds / 60);
    const s: number = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  };

  const isExpired: boolean = remaining <= 0;
  const formattedTime: string = formatTime(remaining);

  if (children) {
    return <>{children({formattedTime, isExpired, remaining})}</>;
  }

  const displayText: string = isExpired ? 'Timed out' : textTemplate.replace('{time}', formattedTime);

  return <Typography variant="body2">{displayText}</Typography>;
};

export default FlowTimer;
