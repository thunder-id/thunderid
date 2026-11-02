// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {screen, fireEvent, renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import AccessChoiceCards from '../AccessChoiceCards';
import GrantedOrganizationUnitList from '../GrantedOrganizationUnitList';
import OrganizationUnitChipList from '../OrganizationUnitChipList';

const unitsById = new Map<string, OrganizationUnit>([
  ['retail', {id: 'retail', handle: 'acme-retail', name: 'Acme Retail'}],
]);

describe('GrantedOrganizationUnitList', () => {
  it('shows the empty label when nothing is granted', () => {
    renderWithProviders(
      <GrantedOrganizationUnitList
        units={[]}
        unitsById={unitsById}
        readOnly={false}
        emptyLabel="Nothing yet"
        onRemove={vi.fn()}
      />,
    );

    expect(screen.getByText('Nothing yet')).toBeInTheDocument();
  });

  it('revokes a unit and falls back to its id while its name loads', () => {
    const onRemove = vi.fn();
    renderWithProviders(
      <GrantedOrganizationUnitList
        units={[{ouId: 'unknown'}]}
        unitsById={unitsById}
        readOnly={false}
        emptyLabel=""
        fixedReachLabel="Top unit only"
        onRemove={onRemove}
      />,
    );

    expect(screen.getByText(/Top unit only/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Revoke access for unknown'}));
    expect(onRemove).toHaveBeenCalledWith('unknown');
  });
});

describe('OrganizationUnitChipList', () => {
  it('removes a unit and opens the picker', () => {
    const onRemove = vi.fn();
    const onAdd = vi.fn();
    renderWithProviders(
      <OrganizationUnitChipList
        ouIds={['retail']}
        unitsById={unitsById}
        emptyLabel="None"
        addLabel="Add exception"
        onRemove={onRemove}
        onAdd={onAdd}
      />,
    );

    fireEvent.click(screen.getByLabelText('Remove Acme Retail from exceptions'));
    expect(onRemove).toHaveBeenCalledWith('retail');
    fireEvent.click(screen.getByText('Add exception'));
    expect(onAdd).toHaveBeenCalled();
  });

  it('shows the empty label when read-only and empty', () => {
    renderWithProviders(<OrganizationUnitChipList ouIds={[]} unitsById={unitsById} emptyLabel="No exceptions." />);

    expect(screen.getByText('No exceptions.')).toBeInTheDocument();
  });
});

describe('AccessChoiceCards', () => {
  it('reports the picked choice without changing the selection itself', () => {
    const onChange = vi.fn();
    renderWithProviders(
      <AccessChoiceCards
        label="Choices"
        value="a"
        choices={[
          {value: 'a', title: 'First', description: 'One'},
          {value: 'b', title: 'Second', description: 'Two'},
        ]}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByRole('radio', {name: /Second/}));

    expect(onChange).toHaveBeenCalledWith('b');
    expect(screen.getByRole('radio', {name: /First/})).toBeChecked();
  });

  it('dims every option but the chosen one when read-only', () => {
    renderWithProviders(
      <AccessChoiceCards
        label="Choices"
        value="a"
        disabled
        choices={[
          {value: 'a', title: 'First', description: 'One'},
          {value: 'b', title: 'Second', description: 'Two'},
        ]}
        onChange={vi.fn()}
      />,
    );

    const card = (title: string) => screen.getByText(title).closest('label')!;
    expect(screen.getByRole('radio', {name: /Second/})).toBeDisabled();
    expect(card('First')).toHaveStyle({opacity: '1'});
    expect(card('Second')).toHaveStyle({opacity: '0.5'});
  });
});
