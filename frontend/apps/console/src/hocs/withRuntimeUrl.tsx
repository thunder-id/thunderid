// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, useQueryClient} from '@tanstack/react-query';
import {EnvironmentProvider, RuntimeProvider, useConfig, type Environment} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useEffect, useMemo, useState} from 'react';
import type {JSX, ComponentType} from 'react';

/**
 * Resolves the environments this deployment applies configuration to, which one the console shows,
 * and the base URL an application's own traffic reaches, and supplies them to the tree below through
 * `EnvironmentProvider` and `RuntimeProvider`.
 *
 * The console shows this deployment's configuration, where it is edited, or one environment, which
 * is read only. Every sign-in starts on the configuration: the choice lives for the session only.
 *
 * A deployment that only holds configuration serves no runtime endpoints itself: the OAuth2,
 * flow and passkey URLs the console displays for a developer to copy have to name the gateway
 * that will answer them. The gateways registered against this deployment are the record of
 * where that is, so this asks for them rather than taking the answer from configuration.
 *
 * The URL shown is the environment's while one is shown, and otherwise the default gateway's,
 * marked `isDefault`: other gateways only receive configuration when it is applied to them, so
 * they are not where a developer editing the configuration is expected to point.
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
    const {getServerUrl} = useConfig();
    const queryClient = useQueryClient();
    const [selectedId, setSelectedId] = useState<string | undefined>(undefined);

    // The query client outlives a session, so a sign-out has to drop the previous session's
    // gateways rather than leave them to be shown to whoever signs in next, and whoever signs in
    // next starts on the configuration.
    useEffect(() => {
      if (!isSignedIn) {
        queryClient.removeQueries({queryKey: ['gateways']});
        setSelectedId(undefined);
      }
    }, [isSignedIn, queryClient]);

    const {data: gateways} = useQuery<Environment[]>({
      queryKey: ['gateways'],
      enabled: isSignedIn,
      // A deployment that serves its own runtime answers this with a 404 or a 403. Retrying it on
      // every console load costs requests and changes nothing.
      retry: false,
      staleTime: Infinity,
      queryFn: async (): Promise<Environment[]> => {
        const response: {data: Environment[]} = await http.request({
          url: `${getServerUrl()}/gateways`,
          method: 'GET',
        } as unknown as Parameters<typeof http.request>[0]);

        return Array.isArray(response.data) ? response.data : [];
      },
    });

    const environments: Environment[] = useMemo(() => (isSignedIn && gateways ? gateways : []), [isSignedIn, gateways]);
    const runtime: Environment | undefined =
      environments.find((environment: Environment) => environment.id === selectedId) ??
      environments.find((environment: Environment) => environment.isDefault);

    return (
      <EnvironmentProvider environments={environments} selectedId={selectedId} onSelect={setSelectedId}>
        <RuntimeProvider url={runtime?.baseUrl}>
          <WrappedComponent {...props} />
        </RuntimeProvider>
      </EnvironmentProvider>
    );
  };
}
