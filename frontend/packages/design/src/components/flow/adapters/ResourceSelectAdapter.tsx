// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {FetchSelectPage} from '@thunderid/react';
import {useEffect, type JSX} from 'react';
import SelectAdapter from './SelectAdapter';
import useResourceSelectOptions from './useResourceSelectOptions';
import type {FlowFieldProps} from '../../../models/flow';

export interface ResourceSelectAdapterProps extends FlowFieldProps {
  /** Selected once when the field is empty, for example the signed-in user as the default owner. */
  defaultValue?: string;
  /** Label shown for `defaultValue` until the page that holds it has loaded. */
  defaultLabel?: string;
  onError?: (error: Error) => void;
  /** Options per request, from 1 to 100. Defaults to 30. */
  pageSize?: number;
}

interface ResourceSelectAdapterInternalProps extends ResourceSelectAdapterProps {
  fetchPage: FetchSelectPage;
  filter?: string;
}

/**
 * A `SelectAdapter` that loads its options a page at a time from a management API, as the user opens
 * the list and scrolls to the end of it. The resource adapters wrap this with their own loader.
 */
export default function ResourceSelectAdapter({
  defaultLabel = undefined,
  defaultValue = undefined,
  fetchPage,
  filter = undefined,
  onError = undefined,
  pageSize = undefined,
  ...fieldProps
}: ResourceSelectAdapterInternalProps): JSX.Element | null {
  const {component, onInputChange, values} = fieldProps;
  const {ref} = component;
  const {hasMore, loading, onLoadMore, onOpen, options} = useResourceSelectOptions({
    component,
    fetchPage,
    filter,
    onError,
    pageSize,
  });

  const currentValue = typeof ref === 'string' ? values[ref] : undefined;

  // A field the form never set has no entry, while one the user cleared holds an empty string. Only the
  // first gets the default, so clearing it sticks and a restarted flow starts from the default again.
  useEffect(() => {
    if (typeof ref !== 'string' || currentValue !== undefined || !defaultValue) return;

    onInputChange(ref, defaultValue);
  }, [ref, currentValue, defaultValue, onInputChange]);

  return (
    <SelectAdapter
      {...fieldProps}
      options={options}
      loading={loading}
      onOpen={onOpen}
      onLoadMore={hasMore ? onLoadMore : undefined}
      selectedOption={defaultValue && defaultLabel ? {label: defaultLabel, value: defaultValue} : undefined}
    />
  );
}
