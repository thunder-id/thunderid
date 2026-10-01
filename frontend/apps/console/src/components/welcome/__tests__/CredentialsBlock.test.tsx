// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {afterEach, beforeAll, beforeEach, describe, expect, it, vi} from 'vitest';

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    Check: () => <span data-testid="icon-check" />,
    Copy: () => <span data-testid="icon-copy" />,
    Eye: () => <span data-testid="icon-eye" />,
    EyeOff: () => <span data-testid="icon-eye-off" />,
  };
});

import CredentialsBlock from '../CredentialsBlock';

describe('CredentialsBlock', () => {
  it('renders the username', () => {
    render(<CredentialsBlock username="john.doe" password="john.doe" />);
    expect(screen.getByText('john.doe')).toBeInTheDocument();
  });

  it('masks the password by default', () => {
    render(<CredentialsBlock username="john.doe" password="secret" />);
    expect(screen.getByText('••••••••')).toBeInTheDocument();
    expect(screen.queryByText('secret')).not.toBeInTheDocument();
  });

  it('shows the password when the eye button is clicked', () => {
    render(<CredentialsBlock username="john.doe" password="secret" />);
    fireEvent.click(screen.getByRole('button', {name: /show password/i}));
    expect(screen.getByText('secret')).toBeInTheDocument();
  });

  it('hides the password again when eye-off is clicked', () => {
    render(<CredentialsBlock username="john.doe" password="secret" />);
    fireEvent.click(screen.getByRole('button', {name: /show password/i}));
    fireEvent.click(screen.getByRole('button', {name: /hide password/i}));
    expect(screen.getByText('••••••••')).toBeInTheDocument();
  });

  describe('clipboard', () => {
    let writeTextSpy: ReturnType<typeof vi.fn>;

    beforeAll(() => {
      Object.defineProperty(navigator, 'clipboard', {
        value: {writeText: vi.fn()},
        writable: true,
        configurable: true,
      });
    });

    beforeEach(() => {
      writeTextSpy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);
    });

    afterEach(() => {
      vi.restoreAllMocks();
    });

    it('copies username on copy button click', () => {
      render(<CredentialsBlock username="john.doe" password="secret" />);
      fireEvent.click(screen.getByRole('button', {name: /copy username/i}));
      expect(writeTextSpy).toHaveBeenCalledWith('john.doe');
    });

    it('copies password on copy button click', () => {
      render(<CredentialsBlock username="john.doe" password="secret" />);
      fireEvent.click(screen.getByRole('button', {name: /copy password/i}));
      expect(writeTextSpy).toHaveBeenCalledWith('secret');
    });
  });
});
