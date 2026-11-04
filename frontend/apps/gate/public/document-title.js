// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Shows the configured product name in the browser tab while the application bundle loads.
document.title = window.__THUNDERID_RUNTIME_CONFIG__?.brand?.product_name || document.title;
