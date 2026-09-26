// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CredentialOfferDialog} from '@thunderid/configure-verifiable-credentials';
import type {JSX} from 'react';

/**
 * Renders the credential offer dialog for the verifiable credentials listing.
 *
 * Issuing an offer is a runtime operation: it is served by the OpenID4VCI endpoints, which a Data
 * Plane has and a Control Plane does not. This module is passed to the listing by the Data Plane
 * console's route tree and by nothing else, so the Control Plane console links neither the dialog
 * nor the request it makes, and never shows the action.
 */
export default function renderCredentialOffer({
  handle,
  onClose,
}: {
  handle: string | null;
  onClose: () => void;
}): JSX.Element {
  return <CredentialOfferDialog open={handle !== null} handle={handle} onClose={onClose} />;
}
