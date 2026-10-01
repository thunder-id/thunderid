// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {isAbsoluteUrl} from '@thunderid/utils';

const resolveStaticResourcePath = (path: string): string =>
  isAbsoluteUrl(path) ? path : `${import.meta.env.BASE_URL}/${path}`;

export default resolveStaticResourcePath;
