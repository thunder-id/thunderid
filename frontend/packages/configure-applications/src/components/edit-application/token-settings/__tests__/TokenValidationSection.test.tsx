// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useForm, type Control, type FieldErrors} from 'react-hook-form';
import {describe, it, expect, vi} from 'vitest';
import TokenValidationSection from '../TokenValidationSection';

interface FormValues {
  validityPeriod: number;
  accessTokenValidity: number;
  idTokenValidity: number;
  refreshTokenValidity: number;
}

// Mock the Components
vi.mock('@thunderid/components', () => ({
  SettingsCard: ({title, description, children}: {title: string; description: string; children: React.ReactNode}) => (
    <div data-testid="settings-card">
      <div data-testid="card-title">{title}</div>
      <div data-testid="card-description">{description}</div>
      {children}
    </div>
  ),
}));

// Wrapper component for testing with react-hook-form
function TestWrapper({
  children,
  defaultValues = {},
}: {
  children: (props: {control: Control<FormValues>; errors: FieldErrors<FormValues>}) => React.ReactNode;
  defaultValues?: Partial<FormValues>;
}) {
  const {control, formState} = useForm<FormValues>({
    defaultValues: {
      validityPeriod: 3600,
      accessTokenValidity: 3600,
      idTokenValidity: 3600,
      refreshTokenValidity: 86400,
      ...defaultValues,
    },
  });

  return <>{children({control, errors: formState.errors})}</>;
}

describe('TokenValidationSection', () => {
  describe('Rendering with tokenType="shared"', () => {
    it('should render the settings card with correct title and description', () => {
      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="shared" />}
        </TestWrapper>,
      );

      expect(screen.getByTestId('card-title')).toHaveTextContent('Token Validity');
      expect(screen.getByTestId('card-description')).toHaveTextContent(
        'Configure how long tokens remain valid before expiration',
      );
    });

    it('should render the validity period field with default value', () => {
      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="shared" />}
        </TestWrapper>,
      );

      const input = document.getElementById('validityPeriod-input');
      expect(input).toBeInTheDocument();
      expect(input).toHaveValue(3600);
    });

    it('should render number input type', () => {
      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="shared" />}
        </TestWrapper>,
      );

      const input = document.getElementById('validityPeriod-input');
      expect((input as HTMLInputElement).type).toBe('number');
    });

    it('should render with custom initial value', () => {
      render(
        <TestWrapper defaultValues={{validityPeriod: 7200}}>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="shared" />}
        </TestWrapper>,
      );

      const input = document.getElementById('validityPeriod-input');
      expect(input).toHaveValue(7200);
    });

    it('should render helper text when no error', () => {
      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="shared" />}
        </TestWrapper>,
      );

      expect(screen.getByText('Token validity period in seconds (e.g., 3600 for 1 hour)')).toBeInTheDocument();
    });
  });

  describe('Rendering with tokenType="oauth" (access token tab)', () => {
    it('should render the settings card with correct title and description', () => {
      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="oauth" />}
        </TestWrapper>,
      );

      expect(screen.getByTestId('card-title')).toHaveTextContent('Token Validity');
      expect(screen.getByTestId('card-description')).toHaveTextContent(
        'Configure how long tokens remain valid before expiration',
      );
    });

    it('should render the access token validity field with default value', () => {
      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="oauth" />}
        </TestWrapper>,
      );

      const input = document.getElementById('accessTokenValidity-input');
      expect(input).toHaveValue(3600);
    });

    it('should render with custom initial access token value', () => {
      render(
        <TestWrapper defaultValues={{accessTokenValidity: 1800}}>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="oauth" />}
        </TestWrapper>,
      );

      const input = document.getElementById('accessTokenValidity-input');
      expect(input).toHaveValue(1800);
    });
  });

  describe('Rendering with tokenType="oauth" (id token tab)', () => {
    it('should render the settings card with correct title and description', () => {
      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="oauth" />}
        </TestWrapper>,
      );

      expect(screen.getByTestId('card-title')).toHaveTextContent('Token Validity');
      expect(screen.getByTestId('card-description')).toHaveTextContent(
        'Configure how long tokens remain valid before expiration',
      );
    });

    it('should render the ID token validity field with default value after switching tab', async () => {
      const user = userEvent.setup();

      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="oauth" />}
        </TestWrapper>,
      );

      // Switch to ID Token tab
      await user.click(screen.getByRole('tab', {name: /ID Token/i}));

      const input = document.getElementById('idTokenValidity-input');
      expect(input).toHaveValue(3600);
    });

    it('should render with custom initial ID token value after switching tab', async () => {
      const user = userEvent.setup();

      render(
        <TestWrapper defaultValues={{idTokenValidity: 900}}>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="oauth" />}
        </TestWrapper>,
      );

      // Switch to ID Token tab
      await user.click(screen.getByRole('tab', {name: /ID Token/i}));

      const input = document.getElementById('idTokenValidity-input');
      expect(input).toHaveValue(900);
    });
  });

  describe('Rendering with tokenType="oauth" (refresh token tab)', () => {
    it('should render the refresh token validity field with default value after switching tab', async () => {
      const user = userEvent.setup();

      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="oauth" />}
        </TestWrapper>,
      );

      await user.click(screen.getByRole('tab', {name: /Refresh Token/i}));

      const input = document.getElementById('refreshTokenValidity-input');
      expect(input).toHaveValue(86400);
    });

    it('should render with custom initial refresh token value after switching tab', async () => {
      const user = userEvent.setup();

      render(
        <TestWrapper defaultValues={{refreshTokenValidity: 172800}}>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="oauth" />}
        </TestWrapper>,
      );

      await user.click(screen.getByRole('tab', {name: /Refresh Token/i}));

      const input = document.getElementById('refreshTokenValidity-input');
      expect(input).toHaveValue(172800);
    });
  });

  describe('User Interaction', () => {
    it('should allow user to type a new validity value', async () => {
      const user = userEvent.setup();

      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="shared" />}
        </TestWrapper>,
      );

      const input = document.getElementById('validityPeriod-input') as HTMLInputElement;
      await user.clear(input);
      await user.type(input, '7200');

      expect(input).toHaveValue(7200);
    });

    it('should allow user to update existing value', async () => {
      const user = userEvent.setup();

      render(
        <TestWrapper defaultValues={{validityPeriod: 1800}}>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="shared" />}
        </TestWrapper>,
      );

      const input = document.getElementById('validityPeriod-input') as HTMLInputElement;
      await user.clear(input);
      await user.type(input, '3600');

      expect(input).toHaveValue(3600);
    });

    it('should handle numeric input for access token', async () => {
      const user = userEvent.setup();

      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="oauth" />}
        </TestWrapper>,
      );

      const input = document.getElementById('accessTokenValidity-input') as HTMLInputElement;
      await user.clear(input);
      await user.type(input, '5400');

      expect(input).toHaveValue(5400);
    });

    it('should handle numeric input for ID token', async () => {
      const user = userEvent.setup();

      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="oauth" />}
        </TestWrapper>,
      );

      // Switch to ID Token tab
      await user.click(screen.getByRole('tab', {name: /ID Token/i}));

      const input = document.getElementById('idTokenValidity-input') as HTMLInputElement;
      await user.clear(input);
      await user.type(input, '1200');

      expect(input).toHaveValue(1200);
    });

    it('should handle numeric input for refresh token', async () => {
      const user = userEvent.setup();

      render(
        <TestWrapper>
          {({control, errors}) => <TokenValidationSection control={control} errors={errors} tokenType="oauth" />}
        </TestWrapper>,
      );

      await user.click(screen.getByRole('tab', {name: /Refresh Token/i}));

      const input = document.getElementById('refreshTokenValidity-input') as HTMLInputElement;
      await user.clear(input);
      await user.type(input, '172800');

      expect(input).toHaveValue(172800);
    });
  });
});
