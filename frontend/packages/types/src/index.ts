// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Concept Types: These types represent core domain concepts and are intended for use across multiple packages to ensure consistency in how key entities are represented.
export {type User} from './concepts/user';
export {
  type AccessStatus,
  type AccessHoldReason,
  type AccessLockedMethod,
  type AccessStatusDetails,
  type AccessStatusReport,
} from './concepts/access-status';

// Primitive Types: These types represent fundamental data structures or patterns that can be widely reused across the codebase, such as error representations or utility types.
export {type ApiError, type I18nMessage} from './primitives/ApiError';
export {type ApiFilteringParams} from './primitives/ApiFilteringParams';
export {type ApiPaginationLink} from './primitives/ApiPaginationLink';
export {type RecursivePartial} from './primitives/RecursivePartial';
