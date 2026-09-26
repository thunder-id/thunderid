// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {useState} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import EditAccessSettings from '../EditAccessSettings';
import type {Application} from '@thunderid/configure-applications';

// Mock the child components
vi.mock('../AccessSection', () => ({
  default: function MockAccessSection({
    application,
    editedApp,
  }: {
    application: Application;
    editedApp: Partial<Application>;
  }) {
    // Mimics AccessSection's local state — used to prove that a changed sectionResetKey remounts
    // (rather than just re-renders) the section.
    const [clicks, setClicks] = useState(0);
    return (
      <div data-testid="access-section">
        AccessSection - App: {application.id}, Edited URL: {editedApp.url ?? 'None'}, Clicks: {clicks}
        <button type="button" onClick={() => setClicks((c) => c + 1)}>
          Bump
        </button>
      </div>
    );
  },
}));

describe('EditAccessSettings', () => {
  const mockOnFieldChange = vi.fn();
  const mockApplication: Application = {
    id: 'app-123',
    name: 'Test App',
    url: 'https://example.com',
  } as Application;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render AccessSection', () => {
      render(<EditAccessSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByTestId('access-section')).toBeInTheDocument();
    });

    it('should pass application to child components', () => {
      render(<EditAccessSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByTestId('access-section')).toHaveTextContent('App: app-123');
    });

    it('should pass editedApp to AccessSection', () => {
      const editedApp = {url: 'https://edited.com'};

      render(
        <EditAccessSettings application={mockApplication} editedApp={editedApp} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.getByTestId('access-section')).toHaveTextContent('Edited URL: https://edited.com');
    });
  });

  describe('Access section reset', () => {
    it('remounts AccessSection, dropping its local state, when sectionResetKey changes', () => {
      const {rerender} = render(
        <EditAccessSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

      fireEvent.click(screen.getByRole('button', {name: 'Bump'}));
      expect(screen.getByTestId('access-section')).toHaveTextContent('Clicks: 1');

      rerender(
        <EditAccessSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={1}
        />,
      );

      expect(screen.getByTestId('access-section')).toHaveTextContent('Clicks: 0');
    });

    it('keeps AccessSection mounted when sectionResetKey stays the same', () => {
      const {rerender} = render(
        <EditAccessSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

      fireEvent.click(screen.getByRole('button', {name: 'Bump'}));
      expect(screen.getByTestId('access-section')).toHaveTextContent('Clicks: 1');

      rerender(
        <EditAccessSettings
          application={mockApplication}
          editedApp={{url: 'https://re-rendered.com'}}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

      expect(screen.getByTestId('access-section')).toHaveTextContent('Clicks: 1');
    });
  });

  describe('Props Propagation', () => {
    it('should pass onFieldChange to AccessSection', () => {
      const {container} = render(
        <EditAccessSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(container.querySelector('[data-testid="access-section"]')).toBeInTheDocument();
    });

    it('should pass all required props to child components', () => {
      const editedApp = {url: 'https://new.com'};

      render(
        <EditAccessSettings application={mockApplication} editedApp={editedApp} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.getByTestId('access-section')).toBeInTheDocument();
    });
  });
});
