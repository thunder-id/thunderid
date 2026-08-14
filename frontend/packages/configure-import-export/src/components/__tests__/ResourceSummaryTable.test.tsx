// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {UsersRound} from '@wso2/oxygen-ui-icons-react';
import {describe, expect, it, vi} from 'vitest';
import type {ConfigSummaryItem} from '../../models/import-configuration';
import ResourceSummaryTable from '../ResourceSummaryTable';

const mockT = (key: string, params?: Record<string, unknown>) => {
  if (key === 'export.table.dependencyCount' && params?.count !== undefined) {
    const count = typeof params.count === 'number' ? params.count : 0;
    return `${count} dependencies`;
  }
  if (key === 'export.table.noDependencies') {
    return 'No dependencies';
  }
  if (key === 'export.status.ready') {
    return 'Ready';
  }
  if (key === 'export.status.warning') {
    return 'Warning';
  }
  return key;
};

describe('ResourceSummaryTable', () => {
  const mockItems: ConfigSummaryItem[] = [
    {
      id: 'users',
      label: 'Users',
      value: 10,
      icon: <UsersRound size={16} />,
    },
    {
      id: 'groups',
      label: 'Groups',
      value: 5,
      icon: <UsersRound size={16} />,
    },
  ];

  describe('rendering', () => {
    it('renders table with items', () => {
      render(<ResourceSummaryTable items={mockItems} />);

      expect(screen.getByText('Users')).toBeInTheDocument();
      expect(screen.getByText('Groups')).toBeInTheDocument();
      expect(screen.getByText('10')).toBeInTheDocument();
      expect(screen.getByText('5')).toBeInTheDocument();
    });

    it('renders empty state when no items', () => {
      render(<ResourceSummaryTable items={[]} />);

      expect(screen.getByText('No resources found')).toBeInTheDocument();
    });

    it('renders table headers', () => {
      render(<ResourceSummaryTable items={mockItems} />);

      expect(screen.getByText('Resource Type')).toBeInTheDocument();
      expect(screen.getByText('Count')).toBeInTheDocument();
    });
  });

  describe('expand/collapse behavior', () => {
    it('expands row when clicked', async () => {
      const itemWithContent: ConfigSummaryItem[] = [
        {
          id: 'users',
          label: 'Users',
          value: 10,
          icon: <UsersRound size={16} />,
          content: <div>User details content</div>,
        },
      ];

      render(<ResourceSummaryTable items={itemWithContent} />);
      const row = screen.getByText('Users').closest('tr');

      await userEvent.click(row!);

      expect(screen.getByText('User details content')).toBeInTheDocument();
    });

    it('toggles expansion when icon button clicked', async () => {
      const itemWithContent: ConfigSummaryItem[] = [
        {
          id: 'users',
          label: 'Users',
          value: 10,
          icon: <UsersRound size={16} />,
          content: <div>User details content</div>,
        },
      ];

      render(<ResourceSummaryTable items={itemWithContent} />);
      const iconButton = screen.getAllByRole('button')[0];

      await userEvent.click(iconButton);
      expect(screen.getByText('User details content')).toBeInTheDocument();

      await userEvent.click(iconButton);
      // Collapse unmounts its content only after its (real, in the browser) transition finishes.
      await waitFor(() => {
        expect(screen.queryByText('User details content')).not.toBeInTheDocument();
      });
    });

    it('shows no details message when content is not provided', async () => {
      render(<ResourceSummaryTable items={mockItems} />);
      const row = screen.getByText('Users').closest('tr');

      await userEvent.click(row!);

      expect(screen.getByText('No details available')).toBeInTheDocument();
    });

    it('can expand multiple rows independently', async () => {
      const items: ConfigSummaryItem[] = [
        {
          id: 'users',
          label: 'Users',
          value: 10,
          icon: <UsersRound size={16} />,
          content: <div>User details</div>,
        },
        {
          id: 'groups',
          label: 'Groups',
          value: 5,
          icon: <UsersRound size={16} />,
          content: <div>Group details</div>,
        },
      ];

      render(<ResourceSummaryTable items={items} />);

      const userRow = screen.getByText('Users').closest('tr');
      const groupRow = screen.getByText('Groups').closest('tr');

      await userEvent.click(userRow!);
      await userEvent.click(groupRow!);

      expect(screen.getByText('User details')).toBeInTheDocument();
      expect(screen.getByText('Group details')).toBeInTheDocument();
    });
  });

  describe('status column', () => {
    it('renders status column when showStatus is true', () => {
      render(<ResourceSummaryTable items={mockItems} showStatus={true} t={mockT} />);

      expect(screen.getByText('Status')).toBeInTheDocument();
    });

    it('displays ready status chip', () => {
      const itemsWithStatus: ConfigSummaryItem[] = [
        {
          id: 'users',
          label: 'Users',
          value: 10,
          icon: <UsersRound size={16} />,
          status: 'ready',
        },
      ];

      render(<ResourceSummaryTable items={itemsWithStatus} showStatus={true} t={mockT} />);

      expect(screen.getByText('Ready')).toBeInTheDocument();
    });

    it('displays warning status chip', () => {
      const itemsWithStatus: ConfigSummaryItem[] = [
        {
          id: 'users',
          label: 'Users',
          value: 10,
          icon: <UsersRound size={16} />,
          status: 'warning',
        },
      ];

      render(<ResourceSummaryTable items={itemsWithStatus} showStatus={true} t={mockT} />);

      expect(screen.getByText('Warning')).toBeInTheDocument();
    });

    it('does not render status column when showStatus is false', () => {
      render(<ResourceSummaryTable items={mockItems} showStatus={false} />);

      expect(screen.queryByText('Status')).not.toBeInTheDocument();
    });
  });

  describe('dependencies column', () => {
    it('renders dependencies column when showDependencies is true', () => {
      render(<ResourceSummaryTable items={mockItems} showDependencies={true} t={mockT} />);

      expect(screen.getByText('Dependencies')).toBeInTheDocument();
    });

    it('displays dependency count', () => {
      const itemsWithDeps: ConfigSummaryItem[] = [
        {
          id: 'users',
          label: 'Users',
          value: 10,
          icon: <UsersRound size={16} />,
          dependencyCount: 3,
        },
      ];

      render(<ResourceSummaryTable items={itemsWithDeps} showDependencies={true} t={mockT} />);

      expect(screen.getByText('3 dependencies')).toBeInTheDocument();
    });

    it('displays no dependencies message when count is 0', () => {
      const itemsWithDeps: ConfigSummaryItem[] = [
        {
          id: 'users',
          label: 'Users',
          value: 10,
          icon: <UsersRound size={16} />,
          dependencyCount: 0,
        },
      ];

      render(<ResourceSummaryTable items={itemsWithDeps} showDependencies={true} t={mockT} />);

      expect(screen.getByText('No dependencies')).toBeInTheDocument();
    });

    it('displays no dependencies message when count is undefined', () => {
      const itemsWithDeps: ConfigSummaryItem[] = [
        {
          id: 'users',
          label: 'Users',
          value: 10,
          icon: <UsersRound size={16} />,
        },
      ];

      render(<ResourceSummaryTable items={itemsWithDeps} showDependencies={true} t={mockT} />);

      expect(screen.getByText('No dependencies')).toBeInTheDocument();
    });

    it('does not render dependencies column when showDependencies is false', () => {
      render(<ResourceSummaryTable items={mockItems} showDependencies={false} />);

      expect(screen.queryByText('Dependencies')).not.toBeInTheDocument();
    });
  });

  describe('table headers with flags', () => {
    it('shows "Item" header when showStatus or showDependencies is true', () => {
      render(<ResourceSummaryTable items={mockItems} showStatus={true} />);

      expect(screen.getByText('Item')).toBeInTheDocument();
      expect(screen.queryByText('Resource Type')).not.toBeInTheDocument();
    });

    it('shows "Resource Type" header when both flags are false', () => {
      render(<ResourceSummaryTable items={mockItems} showStatus={false} showDependencies={false} />);

      expect(screen.getByText('Resource Type')).toBeInTheDocument();
      expect(screen.queryByText('Item')).not.toBeInTheDocument();
    });

    it('does not show count column when showStatus or showDependencies is true', () => {
      render(<ResourceSummaryTable items={mockItems} showStatus={true} />);

      expect(screen.queryByText('Count')).not.toBeInTheDocument();
    });
  });

  describe('custom translation function', () => {
    it('uses provided t function for translations', () => {
      const customT = vi.fn((key: string) => `custom-${key}`);

      const itemsWithStatus: ConfigSummaryItem[] = [
        {
          id: 'users',
          label: 'Users',
          value: 10,
          icon: <UsersRound size={16} />,
          status: 'ready',
        },
      ];

      render(<ResourceSummaryTable items={itemsWithStatus} showStatus={true} t={customT} />);

      expect(customT).toHaveBeenCalledWith('export.status.ready');
    });

    it('falls back to default t function when not provided', () => {
      render(<ResourceSummaryTable items={mockItems} />);

      expect(screen.getByText('Resource Type')).toBeInTheDocument();
    });
  });

  describe('icon rendering', () => {
    it('renders icon for each item', () => {
      const {container} = render(<ResourceSummaryTable items={mockItems} />);

      const icons = container.querySelectorAll('svg');
      expect(icons.length).toBeGreaterThan(0);
    });
  });

  describe('edge cases', () => {
    it('handles items with very large values', () => {
      const items: ConfigSummaryItem[] = [
        {
          id: 'users',
          label: 'Users',
          value: 999999,
          icon: <UsersRound size={16} />,
        },
      ];

      render(<ResourceSummaryTable items={items} />);

      expect(screen.getByText('999999')).toBeInTheDocument();
    });

    it('handles items with zero value', () => {
      const items: ConfigSummaryItem[] = [
        {
          id: 'users',
          label: 'Users',
          value: 0,
          icon: <UsersRound size={16} />,
        },
      ];

      render(<ResourceSummaryTable items={items} />);

      expect(screen.getByText('0')).toBeInTheDocument();
    });

    it('handles long label names', () => {
      const items: ConfigSummaryItem[] = [
        {
          id: 'long-label',
          label: 'Very Long Resource Type Name That Might Cause Layout Issues',
          value: 5,
          icon: <UsersRound size={16} />,
        },
      ];

      render(<ResourceSummaryTable items={items} />);

      expect(screen.getByText('Very Long Resource Type Name That Might Cause Layout Issues')).toBeInTheDocument();
    });
  });
});
