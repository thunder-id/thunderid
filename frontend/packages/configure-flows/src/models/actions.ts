// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Element} from './elements';

export interface Executor {
  name: string;
  meta: Record<string, unknown>;
}

export type Actions = Partial<Element>[];

export const ActionTypes = {
  Next: 'NEXT',
  Previous: 'PREVIOUS',
  Incomplete: 'INCOMPLETE',
  Executor: 'EXECUTOR',
} as const;

export type ActionTypes = (typeof ActionTypes)[keyof typeof ActionTypes];
