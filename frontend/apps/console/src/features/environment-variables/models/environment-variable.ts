// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A non-secret value substituted into declarative configuration when it is applied to a Data Plane,
 * such as an application's redirect URLs. Unlike a secret, the value is readable.
 */
export interface EnvironmentVariable {
  id: string;
  key: string;
  value: string;
  description?: string;
  createdAt: string;
  updatedAt: string;
}

export interface EnvironmentVariableListResponse {
  totalResults: number;
  count: number;
  environmentVariables: EnvironmentVariable[];
}

export interface CreateEnvironmentVariableRequest {
  key: string;
  value: string;
  description?: string;
}

export interface UpdateEnvironmentVariableRequest {
  value: string;
  description?: string;
}

export interface UpdateEnvironmentVariableVariables {
  id: string;
  data: UpdateEnvironmentVariableRequest;
}

export interface EnvironmentVariableListParams {
  limit?: number;
  offset?: number;
}
