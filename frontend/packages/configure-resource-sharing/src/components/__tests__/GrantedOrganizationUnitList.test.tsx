// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import GrantedOrganizationUnitList, {type GrantedUnitTerms} from '../GrantedOrganizationUnitList';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, defaultValue?: string, options?: Record<string, string>) =>
      Object.entries(options ?? {}).reduce(
        (text, [name, value]) => text.replace(`{{${name}}}`, value),
        defaultValue ?? _key,
      ),
  }),
}));

const units = [
  {ouId: 'retail', reach: 'unit' as const},
  {ouId: 'wholesale', reach: 'subtree' as const},
];

const renderList = (terms: GrantedUnitTerms) =>
  render(
    <GrantedOrganizationUnitList
      units={units}
      unitsById={new Map()}
      readOnly={false}
      emptyLabel="None"
      onRemove={vi.fn()}
      terms={terms}
    />,
  );

describe('GrantedOrganizationUnitList terms', () => {
  it('should draw the terms of the open unit under its row only', () => {
    renderList({
      editLabel: (name: string) => `Terms for ${name}`,
      expandedOuId: 'retail',
      onEdit: vi.fn(),
      renderPanel: (ouId: string) => <div data-testid={`panel-${ouId}`} />,
    });

    expect(screen.getByTestId('panel-retail')).toBeInTheDocument();
    expect(screen.queryByTestId('panel-wholesale')).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Terms for retail'})).toHaveTextContent('Edit terms');
    expect(screen.getByRole('button', {name: 'Terms for retail'})).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('button', {name: 'Terms for wholesale'})).toHaveAttribute('aria-expanded', 'false');
  });

  it('should ask to open the terms of a unit when its button is clicked', () => {
    const onEdit = vi.fn();

    renderList({editLabel: (name: string) => `Terms for ${name}`, onEdit});
    fireEvent.click(screen.getByRole('button', {name: 'Terms for wholesale'}));

    expect(onEdit).toHaveBeenCalledWith('wholesale', 'wholesale');
  });
});
