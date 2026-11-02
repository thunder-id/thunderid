// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import NotificationTemplateDeleteDialog from '@/components/NotificationTemplateDeleteDialog';

const {mockUseDelete, mockMutate} = vi.hoisted(() => ({
  mockUseDelete: vi.fn(),
  mockMutate: vi.fn(),
}));

vi.mock('@/api/useDeleteNotificationTemplate', () => ({
  default: mockUseDelete,
}));

const defaultProps = {
  open: true,
  channel: 'email' as const,
  templateId: 'template-1',
  templateName: 'OTP Verification',
  onClose: vi.fn(),
  onSuccess: vi.fn(),
};

describe('NotificationTemplateDeleteDialog', () => {
  let t: (key: string, options?: Record<string, unknown>) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseDelete.mockReturnValue({mutate: mockMutate, isPending: false});
  });

  describe('Rendering', () => {
    it('renders the dialog title', () => {
      render(<NotificationTemplateDeleteDialog {...defaultProps} />);

      expect(screen.getByText(t('notificationTemplates:delete.title'))).toBeInTheDocument();
    });

    it('renders the named confirmation message when a template name is given', () => {
      render(<NotificationTemplateDeleteDialog {...defaultProps} />);

      expect(
        screen.getByText(t('notificationTemplates:delete.messageNamed', {name: 'OTP Verification'})),
      ).toBeInTheDocument();
    });

    it('renders the generic confirmation message when no template name is given', () => {
      render(<NotificationTemplateDeleteDialog {...defaultProps} templateName={undefined} />);

      expect(screen.getByText(t('notificationTemplates:delete.message'))).toBeInTheDocument();
    });

    it('renders cancel and delete buttons', () => {
      render(<NotificationTemplateDeleteDialog {...defaultProps} />);

      expect(screen.getByText(t('common:actions.cancel'))).toBeInTheDocument();
      expect(screen.getByText(t('common:actions.delete'))).toBeInTheDocument();
    });
  });

  describe('Cancel', () => {
    it('calls onClose when cancel is clicked', async () => {
      const onClose = vi.fn();
      const user = userEvent.setup();
      render(<NotificationTemplateDeleteDialog {...defaultProps} onClose={onClose} />);

      await user.click(screen.getByText(t('common:actions.cancel')));

      expect(onClose).toHaveBeenCalled();
    });
  });

  describe('Delete', () => {
    it('calls deleteTemplate.mutate with the channel and id when delete is clicked', async () => {
      const user = userEvent.setup();
      render(<NotificationTemplateDeleteDialog {...defaultProps} />);

      await user.click(screen.getByText(t('common:actions.delete')));

      expect(mockMutate).toHaveBeenCalledWith(
        {channel: 'email', id: 'template-1'},
        expect.objectContaining({
          onSuccess: expect.any(Function) as unknown as () => void,
          onError: expect.any(Function) as unknown as () => void,
        }),
      );
    });

    it('does not call mutate when templateId is null', async () => {
      const user = userEvent.setup();
      render(<NotificationTemplateDeleteDialog {...defaultProps} templateId={null} />);

      await user.click(screen.getByText(t('common:actions.delete')));

      expect(mockMutate).not.toHaveBeenCalled();
    });

    it('calls onClose and onSuccess on successful deletion', async () => {
      const onClose = vi.fn();
      const onSuccess = vi.fn();
      mockMutate.mockImplementation((_vars: unknown, opts: {onSuccess: () => void}) => {
        opts.onSuccess();
      });
      const user = userEvent.setup();
      render(<NotificationTemplateDeleteDialog {...defaultProps} onClose={onClose} onSuccess={onSuccess} />);

      await user.click(screen.getByText(t('common:actions.delete')));

      expect(onClose).toHaveBeenCalled();
      expect(onSuccess).toHaveBeenCalled();
    });

    it('shows a resolved error alert on deletion failure, not raw server text', async () => {
      mockMutate.mockImplementation((_vars: unknown, opts: {onError: (err: Error) => void}) => {
        opts.onError(new Error('raw server text'));
      });
      const user = userEvent.setup();
      render(<NotificationTemplateDeleteDialog {...defaultProps} />);

      await user.click(screen.getByText(t('common:actions.delete')));

      expect(screen.getByText('Failed to delete template. Please try again.')).toBeInTheDocument();
      expect(screen.queryByText('raw server text')).not.toBeInTheDocument();
    });
  });

  describe('Pending state', () => {
    it('shows the deleting label and disables the actions while the deletion is in flight', () => {
      mockUseDelete.mockReturnValue({mutate: mockMutate, isPending: true});
      render(<NotificationTemplateDeleteDialog {...defaultProps} />);

      const deleteButton = screen.getByText(t('common:status.deleting')).closest('button');
      const cancelButton = screen.getByText(t('common:actions.cancel')).closest('button');
      expect(deleteButton).toBeDisabled();
      expect(cancelButton).toBeDisabled();
    });
  });
});
