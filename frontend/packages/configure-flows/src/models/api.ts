// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ActionTypes} from './actions';
import type {Element} from './elements';

export interface Page {
  id: string;
  nodes: string[];
}

export interface Flow {
  pages: Page[];
}

export interface ExecutorInfo {
  name: string;
  meta: Record<string, unknown>;
}

export interface ActionInfo {
  type: ActionTypes;
  executors: ExecutorInfo[];
}

export interface Action {
  id: string;
  action: ActionInfo;
  next: string[];
}

export interface Node {
  id: string;
  elements: string[];
  actions: Action[];
  data: Record<string, unknown>;
}

export interface Block {
  id: string;
  elements: string[];
}

export interface Payload {
  flow: Flow;
  nodes: Node[];
  blocks: Block[];
  elements: Element[];
}
