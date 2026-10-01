// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import AgentTypeResolverProperties from '../AgentTypeResolverProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.agentTypeResolver.description': 'Configure which agent types are allowed.',
        'flows:core.executions.agentTypeResolver.allowedAgentTypes.label': 'Allowed agent types',
        'flows:core.executions.agentTypeResolver.allowedAgentTypes.placeholder': 'agent-a, agent-b',
        'flows:core.executions.agentTypeResolver.allowedAgentTypes.hint': 'Comma-separated agent type ids.',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('AgentTypeResolverProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and field label', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<AgentTypeResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByText('Configure which agent types are allowed.')).toBeInTheDocument();
    expect(screen.getByText('Allowed agent types')).toBeInTheDocument();
  });

  it('should render an empty field when the node has no allowed agent types', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<AgentTypeResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Allowed agent types')).toHaveValue('');
  });

  it('should join the configured allowed agent types into a comma-separated field', () => {
    const resource = {data: {properties: {allowedAgentTypes: ['support-bot', 'billing-bot']}}} as unknown as Resource;

    render(<AgentTypeResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Allowed agent types')).toHaveValue('support-bot, billing-bot');
  });

  it('should commit the parsed list on blur', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<AgentTypeResolverProperties resource={resource} onChange={mockOnChange} />);

    const input = screen.getByLabelText('Allowed agent types');
    fireEvent.change(input, {target: {value: 'support-bot, billing-bot'}});
    fireEvent.blur(input);

    expect(mockOnChange).toHaveBeenCalledWith(
      'data.properties.allowedAgentTypes',
      ['support-bot', 'billing-bot'],
      resource,
    );
  });
});
