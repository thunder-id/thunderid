// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import ResourceNamesContext from '../../contexts/ResourceNames';
import ChangedValuesNotice from '../ChangedValuesNotice';

const DERIVED = 'APPLICATION__01A1B036A68C7CD2BDAAADE42462071C__CLIENT_ID';

describe('ChangedValuesNotice', () => {
  it('calls a value by the resource the version says uses it, with its own name beneath', () => {
    render(
      <ChangedValuesNotice
        changed={{
          variables: [DERIVED],
          references: [
            {
              kind: 'variable',
              name: DERIVED,
              resourceType: 'application',
              resourceId: 'app-1',
              resourceName: 'Shop',
              field: 'clientId',
            },
          ],
        }}
      />,
    );

    const notice = screen.getByTestId('changed-values');
    expect(within(notice).getByText('Shop · Client ID')).toBeInTheDocument();
    expect(within(notice).getByTestId('changed-value-name')).toHaveTextContent(DERIVED);
    expect(within(notice).queryByText(/used by/)).not.toBeInTheDocument();
  });

  it('reads a derived name back to the resource the gateway map names, or to its kind', () => {
    const names = new Map([['application/01a1b036-a68c-7cd2-bdaa-ade42462071c', 'Billing']]);
    render(
      <ResourceNamesContext.Provider value={names}>
        <ChangedValuesNotice
          changed={{variables: [DERIVED], secrets: ['APPLICATION__H0123456789ABCDEF0123456789ABCDEF__CLIENT_SECRET']}}
        />
      </ResourceNamesContext.Provider>,
    );

    expect(screen.getByText('Billing · Client ID')).toBeInTheDocument();
    expect(screen.getByText('Application · Client secret')).toBeInTheDocument();
  });

  it('shows a name someone chose as it is, once', () => {
    render(<ChangedValuesNotice changed={{variables: ['SHOP_BASE_URL']}} />);

    expect(screen.getByText('SHOP_BASE_URL')).toBeInTheDocument();
    expect(screen.queryByTestId('changed-value-name')).not.toBeInTheDocument();
  });
});
