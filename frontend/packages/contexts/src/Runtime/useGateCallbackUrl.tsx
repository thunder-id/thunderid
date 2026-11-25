// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import useRuntimeUrl from './useRuntimeUrl';
import useConfig from '../Config/useConfig';

/**
 * React hook returning the OAuth redirect URI of the login gate, which a connection registers
 * with its external identity provider.
 *
 * The provider sends the user back to the gate of the deployment that runs the sign-in flow, so
 * without a `gate_client` configuration the URL is built from the runtime URL rather than the
 * server URL: a console managing a separate runtime would otherwise give its own address.
 *
 * @returns The gate callback URL (e.g., "https://gateway.example.com/gate/callback")
 *
 * @public
 */
export default function useGateCallbackUrl(): string {
  const {config} = useConfig();
  const runtimeUrl = useRuntimeUrl();
  const gate = config.gate_client;

  let base: string = runtimeUrl;
  if (gate?.public_url) {
    base = gate.public_url;
  } else if (gate?.hostname) {
    const scheme: string = gate.scheme ?? 'https';
    base = gate.port !== undefined ? `${scheme}://${gate.hostname}:${gate.port}` : `${scheme}://${gate.hostname}`;
  }

  return `${base.replace(/\/+$/, '')}/gate/callback`;
}
