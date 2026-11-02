// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useCallback} from 'react';

export type Method = 'GET' | 'POST' | 'PUT' | 'DELETE';

export type Request = <T>(method: Method, path: string, data?: unknown) => Promise<T>;

/** A call to the control plane with the console's token, answered with the response body. */
export default function useRequest(): Request {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  return useCallback(
    async <T>(method: Method, path: string, data?: unknown): Promise<T> => {
      const response: {data: T} = await http.request({
        url: `${getServerUrl()}${path}`,
        method,
        ...(data === undefined ? {} : {data}),
      } as unknown as Parameters<typeof http.request>[0]);
      return response.data;
    },
    [http, getServerUrl],
  );
}
