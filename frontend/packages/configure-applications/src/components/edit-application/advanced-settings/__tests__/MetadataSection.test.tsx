// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import MetadataSection from '../MetadataSection';
import type {Application} from '@thunderid/configure-applications';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

describe('MetadataSection', () => {
  describe('Rendering', () => {
    it('should render metadata section with created and updated timestamps', () => {
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
        createdAt: '2025-01-01T00:00:00Z',
        updatedAt: '2025-01-15T12:30:00Z',
      } as Application;

      render(<MetadataSection application={application} />);

      expect(screen.getByText('applications:edit.advanced.labels.metadata')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.createdAt')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.updatedAt')).toBeInTheDocument();
    });

    it('should render only createdAt when updatedAt is missing', () => {
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
        createdAt: '2025-01-01T00:00:00Z',
      } as Application;

      render(<MetadataSection application={application} />);

      expect(screen.getByText('applications:edit.advanced.labels.metadata')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.createdAt')).toBeInTheDocument();
      expect(screen.queryByText('applications:edit.advanced.labels.updatedAt')).not.toBeInTheDocument();
    });

    it('should render only updatedAt when createdAt is missing', () => {
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
        updatedAt: '2025-01-15T12:30:00Z',
      } as Application;

      render(<MetadataSection application={application} />);

      expect(screen.getByText('applications:edit.advanced.labels.metadata')).toBeInTheDocument();
      expect(screen.queryByText('applications:edit.advanced.labels.createdAt')).not.toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.updatedAt')).toBeInTheDocument();
    });

    it('should return null when both timestamps are missing', () => {
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
      } as Application;

      const {container} = render(<MetadataSection application={application} />);

      expect(container.firstChild).toBeNull();
    });
  });

  describe('Timestamp Formatting', () => {
    it('should format createdAt timestamp as locale string', () => {
      const createdDate = '2025-01-01T10:30:45Z';
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
        createdAt: createdDate,
      } as Application;

      render(<MetadataSection application={application} />);

      const expectedFormattedDate = new Date(createdDate).toLocaleString();
      expect(screen.getByText(expectedFormattedDate)).toBeInTheDocument();
    });

    it('should format updatedAt timestamp as locale string', () => {
      const updatedDate = '2025-01-15T14:20:30Z';
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
        updatedAt: updatedDate,
      } as Application;

      render(<MetadataSection application={application} />);

      const expectedFormattedDate = new Date(updatedDate).toLocaleString();
      expect(screen.getByText(expectedFormattedDate)).toBeInTheDocument();
    });

    it('should format both timestamps correctly', () => {
      const createdDate = '2025-01-01T08:00:00Z';
      const updatedDate = '2025-01-15T16:45:00Z';
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
        createdAt: createdDate,
        updatedAt: updatedDate,
      } as Application;

      render(<MetadataSection application={application} />);

      const expectedCreatedDate = new Date(createdDate).toLocaleString();
      const expectedUpdatedDate = new Date(updatedDate).toLocaleString();

      expect(screen.getByText(expectedCreatedDate)).toBeInTheDocument();
      expect(screen.getByText(expectedUpdatedDate)).toBeInTheDocument();
    });
  });

  describe('Layout and Styling', () => {
    it('should render timestamps with correct typography variants', () => {
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
        createdAt: '2025-01-01T00:00:00Z',
        updatedAt: '2025-01-15T00:00:00Z',
      } as Application;

      render(<MetadataSection application={application} />);

      const createdLabel = screen.getByText('applications:edit.advanced.labels.createdAt');
      const updatedLabel = screen.getByText('applications:edit.advanced.labels.updatedAt');

      expect(createdLabel).toHaveClass('MuiTypography-subtitle2');
      expect(updatedLabel).toHaveClass('MuiTypography-subtitle2');
    });

    it('should render in a Stack with proper spacing', () => {
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
        createdAt: '2025-01-01T00:00:00Z',
        updatedAt: '2025-01-15T00:00:00Z',
      } as Application;

      const {container} = render(<MetadataSection application={application} />);

      const stack = container.querySelector('.MuiStack-root');
      expect(stack).toBeInTheDocument();
    });
  });

  describe('Edge Cases', () => {
    it('should handle invalid date strings gracefully', () => {
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
        createdAt: 'invalid-date',
      } as Application;

      render(<MetadataSection application={application} />);

      expect(screen.getByText('applications:edit.advanced.labels.createdAt')).toBeInTheDocument();
      expect(screen.getByText('Invalid Date')).toBeInTheDocument();
    });

    it('should handle undefined timestamp values', () => {
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
        createdAt: undefined,
        updatedAt: undefined,
      } as Application;

      const {container} = render(<MetadataSection application={application} />);

      expect(container.firstChild).toBeNull();
    });

    it('should handle empty string timestamps', () => {
      const application: Application = {
        id: 'test-app-id',
        name: 'Test Application',
        template: 'custom',
        createdAt: '',
        updatedAt: '',
      } as Application;

      const {container} = render(<MetadataSection application={application} />);

      expect(container.firstChild).toBeNull();
    });
  });
});
