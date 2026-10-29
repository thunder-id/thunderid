// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-return */
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent} from '../../../../models/agent';
import AttributesSummarySection from '../AttributesSummarySection';

const {mockUseGetAgentTypes, mockUseGetAgentType} = vi.hoisted(() => ({
  mockUseGetAgentTypes: vi.fn(),
  mockUseGetAgentType: vi.fn(),
}));

vi.mock('@thunderid/configure-agent-types', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-agent-types')>()),
  useGetAgentTypes: () => mockUseGetAgentTypes(),
  useGetAgentType: (id?: string) => mockUseGetAgentType(id),
}));

vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useResolveDisplayName: () => ({resolveDisplayName: (v: string) => v}),
}));

describe('AttributesSummarySection', () => {
  const baseAgent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    attributes: {email: 'a@b.com', count: 5, isAdmin: true, tags: ['a', 'b']},
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetAgentTypes.mockReturnValue({
      data: {types: [{id: 'schema-1', handle: 'default', displayName: 'Default', ouId: 'ou-1'}]},
    });
    mockUseGetAgentType.mockReturnValue({
      data: {
        id: 'schema-1',
        handle: 'default',
        displayName: 'Default',
        ouId: 'ou-1',
        schema: {email: {type: 'string', required: true}, count: {type: 'number'}},
      },
      isLoading: false,
    });
  });

  it('shows a loading spinner while the schema is loading', () => {
    mockUseGetAgentType.mockReturnValue({data: undefined, isLoading: true});
    render(<AttributesSummarySection agent={baseAgent} />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows the attribute key and formatted value for each attribute', () => {
    render(<AttributesSummarySection agent={baseAgent} />);

    expect(screen.getByText('email')).toBeInTheDocument();
    expect(screen.getByText('a@b.com')).toBeInTheDocument();
    expect(screen.getByText('count')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('isAdmin')).toBeInTheDocument();
    expect(screen.getByText('Yes')).toBeInTheDocument();
    expect(screen.getByText('tags')).toBeInTheDocument();
    expect(screen.getByText('a, b')).toBeInTheDocument();
  });

  it('shows an empty state when there are no attribute values', () => {
    render(<AttributesSummarySection agent={{...baseAgent, attributes: {}}} />);

    expect(screen.getByText('No attributes available.')).toBeInTheDocument();
  });

  it('never shows an edit action', () => {
    render(<AttributesSummarySection agent={baseAgent} />);

    expect(screen.queryByRole('button', {name: 'Edit'})).not.toBeInTheDocument();
  });

  it('shows the resolved display name for attributes when the schema defines one', () => {
    mockUseGetAgentType.mockReturnValue({
      data: {
        id: 'schema-1',
        handle: 'default',
        displayName: 'Default',
        ouId: 'ou-1',
        schema: {email: {type: 'string', displayName: 'Email Address'}},
      },
      isLoading: false,
    });
    render(<AttributesSummarySection agent={{...baseAgent, attributes: {email: 'a@b.com'}}} />);

    expect(screen.getByText('Email Address')).toBeInTheDocument();
  });

  describe('variant="bare"', () => {
    it('renders each attribute as a single "label: value" chip, with no card title', () => {
      render(<AttributesSummarySection agent={baseAgent} variant="bare" />);

      expect(screen.getByText('email: a@b.com')).toBeInTheDocument();
      expect(screen.getByText('count: 5')).toBeInTheDocument();
      expect(screen.getByText('isAdmin: Yes')).toBeInTheDocument();
      expect(screen.getByText('tags: a, b')).toBeInTheDocument();
      expect(screen.queryByText('Attributes')).not.toBeInTheDocument();
    });

    it('still shows the empty state when there are no attribute values', () => {
      render(<AttributesSummarySection agent={{...baseAgent, attributes: {}}} variant="bare" />);

      expect(screen.getByText('No attributes available.')).toBeInTheDocument();
    });

    it('adds a chip for when the lifespan ends, once one is set', () => {
      const validUntil = '2027-01-27T08:30:00Z';
      render(<AttributesSummarySection agent={{...baseAgent, validUntil}} variant="bare" />);

      const expected = new Date(validUntil).toLocaleString(undefined, {dateStyle: 'medium', timeStyle: 'short'});
      expect(screen.getByText(`Valid Until: ${expected}`)).toBeInTheDocument();
      expect(screen.getByText('email: a@b.com')).toBeInTheDocument();
    });

    it('shows no lifespan chip for an agent that does not expire', () => {
      render(<AttributesSummarySection agent={baseAgent} variant="bare" />);

      expect(screen.queryByText(/^Valid Until:/)).not.toBeInTheDocument();
    });

    it('shows the lifespan chip instead of the empty state when there are no attribute values', () => {
      render(
        <AttributesSummarySection
          agent={{...baseAgent, attributes: {}, validUntil: '2027-01-27T08:30:00Z'}}
          variant="bare"
        />,
      );

      expect(screen.getByText(/^Valid Until:/)).toBeInTheDocument();
      expect(screen.queryByText('No attributes available.')).not.toBeInTheDocument();
    });

    it('still shows a loading spinner while the schema is loading', () => {
      mockUseGetAgentType.mockReturnValue({data: undefined, isLoading: true});
      render(<AttributesSummarySection agent={baseAgent} variant="bare" />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });
  });
});
