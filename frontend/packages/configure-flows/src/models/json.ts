// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

type JsonValue = string | number | boolean | null | JsonObject | JsonArray;

interface JsonObject {
  [key: string]: JsonValue;
}

type JsonArray = JsonValue[];

export type {JsonArray, JsonObject, JsonValue};
