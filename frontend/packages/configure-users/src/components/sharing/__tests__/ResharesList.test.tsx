// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import type {SharingPolicy} from '@thunderid/configure-resource-sharing';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ResharesList from '../ResharesList';

const names: Record<string, string> = {acme: 'Acme', sales: 'Sales', 'sales-eu': 'Sales EU', 'sales-us': 'Sales US'};

vi.mock('@thunderid/configure-organization-units', () => ({
  useGetOrganizationUnitsByIds: (ids: string[]) =>
    new Map(ids.map((id) => [id, {id, handle: id, name: names[id] ?? id} as OrganizationUnit])),
}));

let mockPolicies: SharingPolicy[] = [];
vi.mock('@thunderid/configure-resource-sharing', () => ({
  useGetSharingPolicies: () => ({data: mockPolicies}),
}));

const ownerShare: SharingPolicy = {
  id: 'p-acme',
  initiatingOuId: 'acme',
  stage: 'share',
  version: 1,
  childUnits: {mode: 'selected', units: [{ouId: 'sales', reach: 'unit'}]},
};

describe('ResharesList', () => {
  beforeEach(() => {
    mockPolicies = [];
  });

  it('shows nothing when no unit passed the user on', () => {
    mockPolicies = [ownerShare];
    render(<ResharesList userName="Alice" resourcePath="users/u-1" />);

    expect(screen.queryByText('Passed on by other units')).not.toBeInTheDocument();
  });

  it('lists each unit that passed the user on, and where to', () => {
    mockPolicies = [
      ownerShare,
      {
        id: 'p-sales',
        initiatingOuId: 'sales',
        stage: 'reshare',
        parentPolicyId: 'p-acme',
        version: 1,
        childUnits: {mode: 'selected', units: [{ouId: 'sales-eu', reach: 'unit'}]},
      },
    ];
    render(<ResharesList userName="Alice" resourcePath="users/u-1" />);

    expect(screen.getByText('Passed on by other units')).toBeInTheDocument();
    expect(screen.getByText('Sales passes it on to')).toBeInTheDocument();
    expect(screen.getByText('Sales EU')).toBeInTheDocument();
    expect(screen.getByText('This unit only')).toBeInTheDocument();
  });

  it('describes a re-share to every unit beneath, with its exceptions', () => {
    mockPolicies = [
      {
        id: 'p-sales',
        initiatingOuId: 'sales',
        stage: 'reshare',
        version: 1,
        childUnits: {mode: 'all', excludedOuIds: ['sales-us']},
      },
    ];
    render(<ResharesList userName="Alice" resourcePath="users/u-1" />);

    expect(screen.getByText('All child units of Sales')).toBeInTheDocument();
    expect(screen.getByText('except Sales US')).toBeInTheDocument();
  });
});
