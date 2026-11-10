// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCapabilitiesContext} from '@thunderid/contexts';
import type {ReactNode} from 'react';
import ForbiddenNotice from './ForbiddenNotice';
import PageLoader from '../PageLoader/PageLoader';

export type CapabilityGuardProps = {children: ReactNode} & (
  | {
      /** The catalog resource the page acts on (`app`, `user`, ...). */
      resource: string;
      /** The action the page performs. Defaults to `view`. */
      action?: string;
      root?: never;
    }
  | {
      /** The page needs the root `system` permission instead of a catalog action. */
      root: true;
      resource?: never;
      action?: never;
    }
);

/**
 * Renders a page only for admins who may perform `action` on `resource`, so a page opened by its URL is
 * gated like the button that leads to it. The server still decides every request.
 */
export default function CapabilityGuard({resource, action = 'view', root, children}: CapabilityGuardProps): ReactNode {
  const {can, canManageRoot, isLoading} = useCapabilitiesContext();

  if (isLoading) {
    return <PageLoader />;
  }

  if (root ? !canManageRoot : !can(resource, action)) {
    return <ForbiddenNotice />;
  }

  return children;
}
