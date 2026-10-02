// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  GATEWAY_VALUE_DESCRIPTION_MAX_LENGTH,
  GATEWAY_VALUE_MAX_LENGTH,
  GATEWAY_VALUE_NAME_MAX_LENGTH,
  GATEWAY_VALUE_NAME_PATTERN,
} from '../constants/gateway-values';

/**
 * What is wrong with a field of a variable or a secret, by the rules the gateway's store applies.
 */
export type GatewayValueProblem = 'required' | 'pattern' | 'tooLong';

export function validateGatewayValueName(name: string): GatewayValueProblem | undefined {
  if (!name) return 'required';
  if (name.length > GATEWAY_VALUE_NAME_MAX_LENGTH) return 'tooLong';
  if (!GATEWAY_VALUE_NAME_PATTERN.test(name)) return 'pattern';
  return undefined;
}

export function validateGatewayValueValue(value: string): GatewayValueProblem | undefined {
  if (!value) return 'required';
  if (value.length > GATEWAY_VALUE_MAX_LENGTH) return 'tooLong';
  return undefined;
}

export function validateGatewayValueDescription(description: string): GatewayValueProblem | undefined {
  return description.length > GATEWAY_VALUE_DESCRIPTION_MAX_LENGTH ? 'tooLong' : undefined;
}
