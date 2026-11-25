// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

export const AUTH_CONFIG = {
  isRedirectBased: import.meta.env.VITE_AUTH_IS_REDIRECT_BASED !== "false",
  isVerbose: import.meta.env.VITE_AUTH_IS_VERBOSE === "true",
};

const AI_FEATURES_ENABLED = import.meta.env.VITE_AI_FEATURES_ENABLED === "true";

export const BOOKING_SCOPES = [
  "openid",
  "profile",
  "email",
  "ou",
  "booking:read",
  "booking:create",
  "booking:cancel",
];

export const CHAT_SCOPES = AI_FEATURES_ENABLED ? ["agent:access"] : [];

export const SCOPES = BOOKING_SCOPES;

// The sign-in token is bound to the booking resource server, so a password change asks the
// System resource server for its own token.
export const SYSTEM_RESOURCE =
  import.meta.env.VITE_THUNDERID_SYSTEM_RESOURCE || "https://localhost:8090/mcp";

export const SELF_SERVICE_SCOPES = ["self:credentials:update"];
