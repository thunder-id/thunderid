// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCapabilitiesContext} from '@thunderid/contexts';
import type {ReactNode} from 'react';
import CapabilitiesErrorNotice from './CapabilitiesErrorNotice';
import ForbiddenNotice from './ForbiddenNotice';
import PageLoadingAnimation from '../lab/components/PageLoadingAnimation';

export type CapabilityGuardProps = {children: ReactNode} & (
  | {
      /** The catalog resource the page acts on (`app`, `user`, ...). */
      resource: string;
      /** The action the page performs. Defaults to `view`. */
      action?: string;
      anyOf?: never;
    }
  | {
      /** The page offers each of these, so an admin who may perform any one of them may open it. */
      anyOf: {resource: string; action?: string}[];
      resource?: never;
      action?: never;
    }
);

/**
 * Renders a page only for admins who may perform `action` on `resource`, so a page opened by its URL is
 * gated like the button that leads to it. The server still decides every request.
 */
export default function CapabilityGuard({resource, action = 'view', anyOf, children}: CapabilityGuardProps): ReactNode {
  const {can, isLoading, error} = useCapabilitiesContext();

  if (isLoading) {
    return <PageLoadingAnimation />;
  }
  if (error) {
    return <CapabilitiesErrorNotice />;
  }

  const allowed = anyOf
    ? anyOf.some((needed: {resource: string; action?: string}) => can(needed.resource, needed.action))
    : can(resource, action);
  if (!allowed) {
    return <ForbiddenNotice />;
  }

  return children;
}
