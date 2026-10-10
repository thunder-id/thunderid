// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, useQueryClient} from '@tanstack/react-query';
import {RuntimeProvider, useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useEffect} from 'react';
import type {JSX, ComponentType} from 'react';

/**
 * The fields of a registered gateway this provider reads.
 */
interface RegisteredGateway {
  id: string;
  name: string;
  baseUrl?: string;
  /** Whether this is the default gateway. At most one is. */
  isDefault?: boolean;
}

/**
 * Resolves the base URL an application's own traffic reaches and supplies it to the tree below
 * through `RuntimeProvider`.
 *
 * A deployment that only holds configuration serves no runtime endpoints itself: the OAuth2,
 * flow and passkey URLs the console displays for a developer to copy have to name the gateway
 * that will answer them. The gateways registered against this deployment are the record of
 * where that is, so this asks for them rather than taking the answer from configuration.
 *
 * The URL shown is the default gateway's, marked `isDefault`. Other registered gateways only
 * receive configuration when it is applied to them, so they are not where a developer's
 * application is expected to point.
 *
 * Only a console in control-plane mode asks. A standalone deployment serves its own runtime, so
 * it has no gateway to look for and keeps the server URL without a request.
 *
 * No default gateway, no permission to list them, or a deployment with no gateway API at all
 * leaves the URL unset, and every consumer falls back to the server URL. That is the ordinary
 * answer for a deployment that serves its own runtime, so the failure is silent by design. A
 * failed request is left as an error rather than stored as an empty list, so it is asked again
 * on the next mount instead of being cached for the rest of the session.
 */
export default function withRuntimeUrl<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithRuntimeUrl(props: P): JSX.Element {
    const {http, isSignedIn} = useThunderID();
    const {getServerUrl, isControlPlane} = useConfig();
    const asksGateways: boolean = isSignedIn && isControlPlane();
    const queryClient = useQueryClient();

    // The query client outlives a session, so a sign-out has to drop the previous session's
    // gateways rather than leave them to be shown to whoever signs in next.
    useEffect(() => {
      if (!isSignedIn) {
        queryClient.removeQueries({queryKey: ['gateways']});
      }
    }, [isSignedIn, queryClient]);

    const {data: gateways} = useQuery<RegisteredGateway[]>({
      queryKey: ['gateways'],
      enabled: asksGateways,
      // A deployment that serves its own runtime answers this with a 404 or a 403. Retrying it on
      // every console load costs requests and changes nothing.
      retry: false,
      staleTime: Infinity,
      queryFn: async (): Promise<RegisteredGateway[]> => {
        const response: {data: RegisteredGateway[]} = await http.request({
          url: `${getServerUrl()}/gateways`,
          method: 'GET',
        } as unknown as Parameters<typeof http.request>[0]);

        return Array.isArray(response.data) ? response.data : [];
      },
    });

    const runtimeUrl: string | undefined = asksGateways
      ? gateways?.find((gateway) => gateway.isDefault && Boolean(gateway.baseUrl))?.baseUrl
      : undefined;

    return (
      <RuntimeProvider url={runtimeUrl}>
        <WrappedComponent {...props} />
      </RuntimeProvider>
    );
  };
}
