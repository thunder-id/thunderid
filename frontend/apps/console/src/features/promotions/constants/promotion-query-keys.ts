// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const PromotionQueryKeys = {
  ENVIRONMENTS: 'promotion-environments',
  VERSIONS: 'promotion-versions',
  DIFF: 'promotion-diff',
  VARIABLES: 'promotion-variables',
  SECRETS: 'promotion-secrets',
  PROMOTION_PREVIEW: 'promotion-preview',
  DATA_PLANE_JOB: 'promotion-data-plane-job',
} as const;

export default PromotionQueryKeys;
