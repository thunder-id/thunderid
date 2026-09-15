// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import PresentationDefinitionSelect from '../PresentationDefinitionSelect';

interface VpQueryResult {
  data: {id: string; handle: string; name?: string}[] | undefined;
  isLoading: boolean;
}

const mockUseGetVerifiablePresentations = vi.fn<() => VpQueryResult>();

vi.mock('@thunderid/configure-verifiable-credentials', () => ({
  useGetVerifiablePresentations: (): VpQueryResult => mockUseGetVerifiablePresentations(),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string) => key}),
}));

describe('PresentationDefinitionSelect', () => {
  const onChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetVerifiablePresentations.mockReturnValue({
      data: [{id: '1', handle: 'eudi-pid', name: 'EUDI PID'}],
      isLoading: false,
    });
  });

  const renderComponent = (errorMessage?: string): void => {
    render(
      <PresentationDefinitionSelect
        propertyKey="presentation_definition_id"
        value=""
        onChange={onChange}
        errorMessage={errorMessage}
      />,
    );
  };

  it('should render the field validation message when one is supplied', () => {
    renderComponent('Presentation definition is required');

    expect(screen.getByText('Presentation definition is required')).toBeInTheDocument();
  });

  it('should render no validation message when the field is valid', () => {
    renderComponent('');

    expect(screen.queryByText('Presentation definition is required')).not.toBeInTheDocument();
  });

  it('should render no validation message when the prop is omitted', () => {
    render(<PresentationDefinitionSelect propertyKey="presentation_definition_id" value="" onChange={onChange} />);

    expect(screen.getByText('verifiable-presentations:select.label')).toBeInTheDocument();
  });
});
