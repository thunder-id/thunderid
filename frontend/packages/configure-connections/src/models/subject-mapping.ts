// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

export interface SubjectAttributeRow {
  key: number;
  attribute: string;
  pdpAttribute: string;
}

export interface SubjectTypeOption {
  id: string;
  name: string;
  category: 'agent' | 'user';
}

export interface SubjectMappingTypeLists {
  userTypes: SubjectTypeOption[];
  agentTypes: SubjectTypeOption[];
}

export interface SubjectMappingGroup {
  key: number;
  category: '' | 'agent' | 'user';
  userType: string;
  rows: SubjectAttributeRow[];
}

export interface SubjectRowsState {
  groups: SubjectMappingGroup[];
  seq: number;
  syncedAttributeMappings: string;
}
