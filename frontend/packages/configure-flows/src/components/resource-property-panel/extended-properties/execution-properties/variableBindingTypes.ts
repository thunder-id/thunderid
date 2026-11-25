// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {FlowVariableType} from '@thunderid/react';

/**
 * The variable types a text property can be bound to.
 */
export const TEXT_VARIABLES: FlowVariableType[] = ['TEXT_INPUT', 'EMAIL_INPUT', 'PHONE_INPUT', 'SELECT'];

/**
 * The variable types a numeric property can be bound to.
 */
export const NUMBER_VARIABLES: FlowVariableType[] = ['NUMBER_INPUT'];

/**
 * The variable types a switch property can be bound to.
 */
export const BOOLEAN_VARIABLES: FlowVariableType[] = ['BOOLEAN_INPUT'];

/**
 * The variable types a property that names a connection can be bound to.
 */
export const CONNECTION_VARIABLES: FlowVariableType[] = ['CONNECTION_SELECT'];
