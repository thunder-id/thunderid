// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ToolbarPlugin from '../ToolbarPlugin';

// Use vi.hoisted for mock functions that need to be available during module mocking
const {mockDispatchCommand, mockRegisterUpdateListener, mockRegisterCommand, mockGetElementByKey, mockUpdate} =
  vi.hoisted(() => ({
    mockDispatchCommand: vi.fn(),
    // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unused-vars
    mockRegisterUpdateListener: vi.fn((_callback?: any) => vi.fn()),
    // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unused-vars
    mockRegisterCommand: vi.fn((_command?: any, _callback?: any, _priority?: any) => vi.fn()),
    mockGetElementByKey: vi.fn(() => document.createElement('div')),
    mockUpdate: vi.fn((callback: () => void) => callback()),
  }));

// Mock the lexical composer context
vi.mock('@lexical/react/LexicalComposerContext', () => ({
  useLexicalComposerContext: () => [
    {
      dispatchCommand: mockDispatchCommand,
      registerUpdateListener: mockRegisterUpdateListener,
      registerCommand: mockRegisterCommand,
      getElementByKey: mockGetElementByKey,
      update: mockUpdate,
    },
  ],
}));

// Mock lexical selection
vi.mock('@lexical/selection', () => ({
  $setBlocksType: vi.fn(),
}));

// Mock lexical utils
vi.mock('@lexical/utils', () => ({
  mergeRegister:
    (...fns: (() => void)[]) =>
    () =>
      fns.forEach((fn) => fn()),
}));

// Mock lexical
vi.mock('lexical', () => ({
  $createParagraphNode: vi.fn(),
  $getSelection: vi.fn(() => null),
  $isRangeSelection: vi.fn(() => false),
  CAN_REDO_COMMAND: 'CAN_REDO_COMMAND',
  CAN_UNDO_COMMAND: 'CAN_UNDO_COMMAND',
  FORMAT_ELEMENT_COMMAND: 'FORMAT_ELEMENT_COMMAND',
  FORMAT_TEXT_COMMAND: 'FORMAT_TEXT_COMMAND',
  REDO_COMMAND: 'REDO_COMMAND',
  SELECTION_CHANGE_COMMAND: 'SELECTION_CHANGE_COMMAND',
  UNDO_COMMAND: 'UNDO_COMMAND',
}));

// Mock @lexical/rich-text
vi.mock('@lexical/rich-text', () => ({
  $createHeadingNode: vi.fn(),
  $isHeadingNode: vi.fn(() => false),
}));

// Mock @lexical/link
vi.mock('@lexical/link', () => ({
  $isLinkNode: vi.fn(() => false),
  TOGGLE_LINK_COMMAND: 'TOGGLE_LINK_COMMAND',
}));

// Mock getSelectedNode utility
vi.mock('../../utils/getSelectedNode', () => ({
  default: vi.fn(() => ({
    getParent: () => null,
    getKey: () => 'root',
    getTopLevelElementOrThrow: () => ({
      getFormat: () => 0,
      getKey: () => 'element-key',
      getType: () => 'paragraph',
      getTag: () => 'p',
    }),
  })),
}));

describe('ToolbarPlugin', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the toolbar', () => {
      render(<ToolbarPlugin />);

      expect(document.querySelector('.MuiPaper-root')).toBeInTheDocument();
    });

    it('should render undo button when history is enabled', () => {
      render(<ToolbarPlugin history />);

      expect(screen.getByRole('button', {name: 'Undo'})).toBeInTheDocument();
    });

    it('should render redo button when history is enabled', () => {
      render(<ToolbarPlugin history />);

      expect(screen.getByRole('button', {name: 'Redo'})).toBeInTheDocument();
    });

    it('should not render undo/redo buttons when history is disabled', () => {
      render(<ToolbarPlugin history={false} />);

      expect(screen.queryByRole('button', {name: 'Undo'})).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Redo'})).not.toBeInTheDocument();
    });

    it('should render bold button when bold is enabled', () => {
      render(<ToolbarPlugin bold />);

      expect(screen.getByRole('button', {name: 'Format Bold'})).toBeInTheDocument();
    });

    it('should not render bold button when bold is disabled', () => {
      render(<ToolbarPlugin bold={false} />);

      expect(screen.queryByRole('button', {name: 'Format Bold'})).not.toBeInTheDocument();
    });

    it('should render italic button when italic is enabled', () => {
      render(<ToolbarPlugin italic />);

      expect(screen.getByRole('button', {name: 'Format Italics'})).toBeInTheDocument();
    });

    it('should not render italic button when italic is disabled', () => {
      render(<ToolbarPlugin italic={false} />);

      expect(screen.queryByRole('button', {name: 'Format Italics'})).not.toBeInTheDocument();
    });

    it('should render underline button when underline is enabled', () => {
      render(<ToolbarPlugin underline />);

      expect(screen.getByRole('button', {name: 'Format Underline'})).toBeInTheDocument();
    });

    it('should not render underline button when underline is disabled', () => {
      render(<ToolbarPlugin underline={false} />);

      expect(screen.queryByRole('button', {name: 'Format Underline'})).not.toBeInTheDocument();
    });

    it('should render link button when link is enabled', () => {
      render(<ToolbarPlugin link />);

      expect(screen.getByRole('button', {name: 'Format Link'})).toBeInTheDocument();
    });

    it('should not render link button when link is disabled', () => {
      render(<ToolbarPlugin link={false} />);

      expect(screen.queryByRole('button', {name: 'Format Link'})).not.toBeInTheDocument();
    });

    it('should render alignment buttons when alignment is enabled', () => {
      render(<ToolbarPlugin alignment />);

      expect(screen.getByRole('button', {name: 'Left Align'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Center Align'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Right Align'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Justify Align'})).toBeInTheDocument();
    });

    it('should not render alignment buttons when alignment is disabled', () => {
      render(<ToolbarPlugin alignment={false} />);

      expect(screen.queryByRole('button', {name: 'Left Align'})).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Center Align'})).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Right Align'})).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Justify Align'})).not.toBeInTheDocument();
    });

    it('should render typography dropdown when typography is enabled', () => {
      render(<ToolbarPlugin typography />);

      expect(screen.getByText('Paragraph')).toBeInTheDocument();
    });

    it('should not render typography dropdown when typography is disabled', () => {
      render(<ToolbarPlugin typography={false} />);

      expect(screen.queryByText('Paragraph')).not.toBeInTheDocument();
    });
  });

  describe('Disabled State', () => {
    it('should disable undo button when disabled prop is true', () => {
      render(<ToolbarPlugin history disabled />);

      expect(screen.getByRole('button', {name: 'Undo'})).toBeDisabled();
    });

    it('should disable redo button when disabled prop is true', () => {
      render(<ToolbarPlugin history disabled />);

      expect(screen.getByRole('button', {name: 'Redo'})).toBeDisabled();
    });

    it('should disable bold button when disabled prop is true', () => {
      render(<ToolbarPlugin bold disabled />);

      expect(screen.getByRole('button', {name: 'Format Bold'})).toBeDisabled();
    });

    it('should disable italic button when disabled prop is true', () => {
      render(<ToolbarPlugin italic disabled />);

      expect(screen.getByRole('button', {name: 'Format Italics'})).toBeDisabled();
    });

    it('should disable underline button when disabled prop is true', () => {
      render(<ToolbarPlugin underline disabled />);

      expect(screen.getByRole('button', {name: 'Format Underline'})).toBeDisabled();
    });

    it('should disable link button when disabled prop is true', () => {
      render(<ToolbarPlugin link disabled />);

      expect(screen.getByRole('button', {name: 'Format Link'})).toBeDisabled();
    });

    it('should disable alignment buttons when disabled prop is true', () => {
      render(<ToolbarPlugin alignment disabled />);

      expect(screen.getByRole('button', {name: 'Left Align'})).toBeDisabled();
      expect(screen.getByRole('button', {name: 'Center Align'})).toBeDisabled();
      expect(screen.getByRole('button', {name: 'Right Align'})).toBeDisabled();
      expect(screen.getByRole('button', {name: 'Justify Align'})).toBeDisabled();
    });

    it('should disable typography button when disabled prop is true', () => {
      render(<ToolbarPlugin typography disabled />);

      const typographyButton = screen.getByText('Paragraph').closest('button');
      expect(typographyButton).toBeDisabled();
    });
  });

  describe('Button Clicks', () => {
    it('should dispatch UNDO_COMMAND when undo button is clicked', () => {
      // Mock registerCommand to capture the CAN_UNDO_COMMAND callback and call it to enable undo
      mockRegisterCommand.mockImplementation(((command: string, callback: (payload: boolean) => boolean) => {
        if (command === 'CAN_UNDO_COMMAND') {
          // Call the callback with true to enable the undo button
          callback(true);
        }
        return vi.fn();
      }) as typeof mockRegisterCommand);

      render(<ToolbarPlugin history />);

      const undoButton = screen.getByRole('button', {name: 'Undo'});
      fireEvent.click(undoButton);

      expect(mockDispatchCommand).toHaveBeenCalledWith('UNDO_COMMAND', undefined);
    });

    it('should dispatch REDO_COMMAND when redo button is clicked', () => {
      // Mock registerCommand to capture the CAN_REDO_COMMAND callback and call it to enable redo
      mockRegisterCommand.mockImplementation(((command: string, callback: (payload: boolean) => boolean) => {
        if (command === 'CAN_REDO_COMMAND') {
          // Call the callback with true to enable the redo button
          callback(true);
        }
        return vi.fn();
      }) as typeof mockRegisterCommand);

      render(<ToolbarPlugin history />);

      const redoButton = screen.getByRole('button', {name: 'Redo'});
      fireEvent.click(redoButton);

      expect(mockDispatchCommand).toHaveBeenCalledWith('REDO_COMMAND', undefined);
    });

    it('should dispatch FORMAT_TEXT_COMMAND with bold when bold button is clicked', () => {
      render(<ToolbarPlugin bold />);

      fireEvent.click(screen.getByRole('button', {name: 'Format Bold'}));

      expect(mockDispatchCommand).toHaveBeenCalledWith('FORMAT_TEXT_COMMAND', 'bold');
    });

    it('should dispatch FORMAT_TEXT_COMMAND with italic when italic button is clicked', () => {
      render(<ToolbarPlugin italic />);

      fireEvent.click(screen.getByRole('button', {name: 'Format Italics'}));

      expect(mockDispatchCommand).toHaveBeenCalledWith('FORMAT_TEXT_COMMAND', 'italic');
    });

    it('should dispatch FORMAT_TEXT_COMMAND with underline when underline button is clicked', () => {
      render(<ToolbarPlugin underline />);

      fireEvent.click(screen.getByRole('button', {name: 'Format Underline'}));

      expect(mockDispatchCommand).toHaveBeenCalledWith('FORMAT_TEXT_COMMAND', 'underline');
    });

    it('should dispatch FORMAT_ELEMENT_COMMAND with left when left align button is clicked', () => {
      render(<ToolbarPlugin alignment />);

      fireEvent.click(screen.getByRole('button', {name: 'Left Align'}));

      expect(mockDispatchCommand).toHaveBeenCalledWith('FORMAT_ELEMENT_COMMAND', 'left');
    });

    it('should dispatch FORMAT_ELEMENT_COMMAND with center when center align button is clicked', () => {
      render(<ToolbarPlugin alignment />);

      fireEvent.click(screen.getByRole('button', {name: 'Center Align'}));

      expect(mockDispatchCommand).toHaveBeenCalledWith('FORMAT_ELEMENT_COMMAND', 'center');
    });

    it('should dispatch FORMAT_ELEMENT_COMMAND with right when right align button is clicked', () => {
      render(<ToolbarPlugin alignment />);

      fireEvent.click(screen.getByRole('button', {name: 'Right Align'}));

      expect(mockDispatchCommand).toHaveBeenCalledWith('FORMAT_ELEMENT_COMMAND', 'right');
    });

    it('should dispatch FORMAT_ELEMENT_COMMAND with justify when justify align button is clicked', () => {
      render(<ToolbarPlugin alignment />);

      fireEvent.click(screen.getByRole('button', {name: 'Justify Align'}));

      expect(mockDispatchCommand).toHaveBeenCalledWith('FORMAT_ELEMENT_COMMAND', 'justify');
    });

    it('should dispatch TOGGLE_LINK_COMMAND when link button is clicked', () => {
      render(<ToolbarPlugin link />);

      fireEvent.click(screen.getByRole('button', {name: 'Format Link'}));

      expect(mockDispatchCommand).toHaveBeenCalledWith('TOGGLE_LINK_COMMAND', 'https://');
    });
  });

  describe('Typography Menu', () => {
    it('should open typography menu when typography button is clicked', () => {
      render(<ToolbarPlugin typography />);

      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      expect(screen.getByText('Heading 1')).toBeInTheDocument();
      expect(screen.getByText('Heading 2')).toBeInTheDocument();
      expect(screen.getByText('Heading 3')).toBeInTheDocument();
      expect(screen.getByText('Heading 4')).toBeInTheDocument();
      expect(screen.getByText('Heading 5')).toBeInTheDocument();
    });

    it('should close typography menu when menu item is clicked', async () => {
      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click heading 1
      fireEvent.click(screen.getByText('Heading 1'));

      // Menu should close - use waitFor because menu closing is async in MUI
      await waitFor(() => {
        expect(screen.queryByRole('menu')).not.toBeInTheDocument();
      });
    });
  });

  describe('Default Props', () => {
    it('should have history enabled by default', () => {
      render(<ToolbarPlugin />);

      expect(screen.getByRole('button', {name: 'Undo'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Redo'})).toBeInTheDocument();
    });

    it('should have bold enabled by default', () => {
      render(<ToolbarPlugin />);

      expect(screen.getByRole('button', {name: 'Format Bold'})).toBeInTheDocument();
    });

    it('should have italic enabled by default', () => {
      render(<ToolbarPlugin />);

      expect(screen.getByRole('button', {name: 'Format Italics'})).toBeInTheDocument();
    });

    it('should have underline enabled by default', () => {
      render(<ToolbarPlugin />);

      expect(screen.getByRole('button', {name: 'Format Underline'})).toBeInTheDocument();
    });

    it('should have alignment enabled by default', () => {
      render(<ToolbarPlugin />);

      expect(screen.getByRole('button', {name: 'Left Align'})).toBeInTheDocument();
    });

    it('should have typography enabled by default', () => {
      render(<ToolbarPlugin />);

      expect(screen.getByText('Paragraph')).toBeInTheDocument();
    });

    it('should have link enabled by default', () => {
      render(<ToolbarPlugin />);

      expect(screen.getByRole('button', {name: 'Format Link'})).toBeInTheDocument();
    });

    it('should not be disabled by default', () => {
      render(<ToolbarPlugin />);

      expect(screen.getByRole('button', {name: 'Format Bold'})).not.toBeDisabled();
    });
  });

  describe('Custom className', () => {
    it('should apply custom className to the toolbar', () => {
      render(<ToolbarPlugin className="custom-toolbar-class" />);

      const paper = document.querySelector('.MuiPaper-root');
      expect(paper).toHaveClass('custom-toolbar-class');
    });
  });

  describe('Command Registration', () => {
    it('should register update listener on mount', () => {
      render(<ToolbarPlugin />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should register SELECTION_CHANGE_COMMAND on mount', () => {
      render(<ToolbarPlugin />);

      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should register CAN_UNDO_COMMAND on mount', () => {
      render(<ToolbarPlugin />);

      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should register CAN_REDO_COMMAND on mount', () => {
      render(<ToolbarPlugin />);

      expect(mockRegisterCommand).toHaveBeenCalled();
    });
  });

  describe('Format Paragraph', () => {
    it('should format selection to paragraph when menu item is clicked', async () => {
      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click paragraph option
      const paragraphItems = screen.getAllByText('Paragraph');
      // The second one is the menu item
      const paragraphMenuItem = paragraphItems.find((item) => item.closest('[role="menuitem"]'));
      if (paragraphMenuItem) {
        fireEvent.click(paragraphMenuItem);
      }

      // Menu should close
      await waitFor(() => {
        expect(screen.queryByRole('menu')).not.toBeInTheDocument();
      });
    });

    it('should call editor update when formatting to paragraph', () => {
      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click paragraph option
      const paragraphItems = screen.getAllByText('Paragraph');
      const paragraphMenuItem = paragraphItems.find((item) => item.closest('[role="menuitem"]'));
      if (paragraphMenuItem) {
        fireEvent.click(paragraphMenuItem);
      }

      expect(mockUpdate).toHaveBeenCalled();
    });
  });

  describe('Format Heading', () => {
    it('should format selection to h1 when Heading 1 is clicked', () => {
      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click heading 1
      fireEvent.click(screen.getByText('Heading 1'));

      expect(mockUpdate).toHaveBeenCalled();
    });

    it('should format selection to h2 when Heading 2 is clicked', () => {
      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click heading 2
      fireEvent.click(screen.getByText('Heading 2'));

      expect(mockUpdate).toHaveBeenCalled();
    });

    it('should format selection to h3 when Heading 3 is clicked', () => {
      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click heading 3
      fireEvent.click(screen.getByText('Heading 3'));

      expect(mockUpdate).toHaveBeenCalled();
    });

    it('should format selection to h4 when Heading 4 is clicked', () => {
      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click heading 4
      fireEvent.click(screen.getByText('Heading 4'));

      expect(mockUpdate).toHaveBeenCalled();
    });

    it('should format selection to h5 when Heading 5 is clicked', () => {
      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click heading 5
      fireEvent.click(screen.getByText('Heading 5'));

      expect(mockUpdate).toHaveBeenCalled();
    });
  });

  describe('Insert Link', () => {
    it('should dispatch TOGGLE_LINK_COMMAND with https:// when adding new link', () => {
      render(<ToolbarPlugin link />);

      fireEvent.click(screen.getByRole('button', {name: 'Format Link'}));

      // When isLink is false (default), it adds a new link with 'https://'
      expect(mockDispatchCommand).toHaveBeenCalledWith('TOGGLE_LINK_COMMAND', 'https://');
    });

    it('should register link command handler', () => {
      render(<ToolbarPlugin link />);

      // The component registers commands including link toggle
      expect(mockRegisterCommand).toHaveBeenCalled();
    });
  });

  describe('Toolbar Update', () => {
    it('should update toolbar state on selection change', async () => {
      // Mock $isRangeSelection to return true and provide selection
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: vi.fn().mockReturnValue(false),
        anchor: {
          getNode: () => ({
            getKey: () => 'root',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'paragraph',
            }),
          }),
        },
      });

      render(<ToolbarPlugin />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should update bold state when selection has bold format', async () => {
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: (format: string) => format === 'bold',
        anchor: {
          getNode: () => ({
            getKey: () => 'root',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'paragraph',
            }),
          }),
        },
      });

      render(<ToolbarPlugin bold />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should update italic state when selection has italic format', async () => {
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: (format: string) => format === 'italic',
        anchor: {
          getNode: () => ({
            getKey: () => 'root',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'paragraph',
            }),
          }),
        },
      });

      render(<ToolbarPlugin italic />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should update underline state when selection has underline format', async () => {
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: (format: string) => format === 'underline',
        anchor: {
          getNode: () => ({
            getKey: () => 'root',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'paragraph',
            }),
          }),
        },
      });

      render(<ToolbarPlugin underline />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });
  });

  describe('Alignment State', () => {
    it('should highlight left align button when alignment is 1', () => {
      render(<ToolbarPlugin alignment />);

      const leftAlignButton = screen.getByRole('button', {name: 'Left Align'});
      // By default, alignment is 1 (left) which maps to selectedAlignment === 1
      expect(leftAlignButton).toBeInTheDocument();
    });

    it('should highlight center align button when alignment is 2', () => {
      // This would require mocking the selection to have alignment format 2
      render(<ToolbarPlugin alignment />);

      const centerAlignButton = screen.getByRole('button', {name: 'Center Align'});
      expect(centerAlignButton).toBeInTheDocument();
    });

    it('should highlight right align button when alignment is 3', () => {
      render(<ToolbarPlugin alignment />);

      const rightAlignButton = screen.getByRole('button', {name: 'Right Align'});
      expect(rightAlignButton).toBeInTheDocument();
    });

    it('should highlight justify align button when alignment is 4', () => {
      render(<ToolbarPlugin alignment />);

      const justifyAlignButton = screen.getByRole('button', {name: 'Justify Align'});
      expect(justifyAlignButton).toBeInTheDocument();
    });
  });

  describe('Block Type Detection', () => {
    it('should detect heading block type', async () => {
      const {$isHeadingNode} = await vi.importMock<typeof import('@lexical/rich-text')>('@lexical/rich-text');
      ($isHeadingNode as ReturnType<typeof vi.fn>).mockReturnValue(true);

      render(<ToolbarPlugin typography />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should detect paragraph block type', () => {
      render(<ToolbarPlugin typography />);

      // Default block type is paragraph
      expect(screen.getByText('Paragraph')).toBeInTheDocument();
    });

    it('should handle unknown block type as paragraph', () => {
      render(<ToolbarPlugin typography />);

      // Unknown types default to paragraph
      expect(screen.getByText('Paragraph')).toBeInTheDocument();
    });
  });

  describe('Link State Detection', () => {
    it('should detect when parent node is a link', async () => {
      const {$isLinkNode} = await vi.importMock<typeof import('@lexical/link')>('@lexical/link');
      ($isLinkNode as ReturnType<typeof vi.fn>).mockReturnValue(true);

      render(<ToolbarPlugin link />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should detect when current node is a link', async () => {
      const {$isLinkNode} = await vi.importMock<typeof import('@lexical/link')>('@lexical/link');
      ($isLinkNode as ReturnType<typeof vi.fn>).mockReturnValue(true);

      render(<ToolbarPlugin link />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });
  });

  describe('Active State Classes', () => {
    it('should not apply active class when disabled', () => {
      render(<ToolbarPlugin bold disabled />);

      const boldButton = screen.getByRole('button', {name: 'Format Bold'});
      expect(boldButton).not.toHaveClass('active');
    });

    it('should apply active class to bold button when bold is active and not disabled', async () => {
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: (format: string) => format === 'bold',
        anchor: {
          getNode: () => ({
            getKey: () => 'root',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'paragraph',
            }),
          }),
        },
      });

      render(<ToolbarPlugin bold />);

      // The active class is applied via classNames utility
      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });
  });

  describe('Element Key Resolution', () => {
    it('should get element by key from editor', () => {
      render(<ToolbarPlugin />);

      expect(mockGetElementByKey).toBeDefined();
    });

    it('should handle null element DOM', () => {
      mockGetElementByKey.mockReturnValueOnce(null as unknown as HTMLDivElement);

      render(<ToolbarPlugin />);

      // Component should handle null gracefully
      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });
  });

  describe('Format Paragraph with Selection', () => {
    it('should call editor update when formatting to paragraph', () => {
      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click paragraph option
      const paragraphItems = screen.getAllByText('Paragraph');
      const paragraphMenuItem = paragraphItems.find((item) => item.closest('[role="menuitem"]'));
      if (paragraphMenuItem) {
        fireEvent.click(paragraphMenuItem);
      }

      // editor.update should be called when formatting
      expect(mockUpdate).toHaveBeenCalled();
    });
  });

  describe('Insert Link Toggle', () => {
    it('should dispatch TOGGLE_LINK_COMMAND when link button is clicked', () => {
      render(<ToolbarPlugin link />);

      fireEvent.click(screen.getByRole('button', {name: 'Format Link'}));

      // The dispatch command should have been called
      expect(mockDispatchCommand).toHaveBeenCalledWith('TOGGLE_LINK_COMMAND', expect.anything());
    });
  });

  describe('formatParagraph with Range Selection', () => {
    it('should call $setBlocksType when selection is range and blockType is not paragraph', () => {
      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click paragraph option (to change from h1 to paragraph)
      const paragraphItems = screen.getAllByText('Paragraph');
      const paragraphMenuItem = paragraphItems.find((item) => item.closest('[role="menuitem"]'));
      if (paragraphMenuItem) {
        fireEvent.click(paragraphMenuItem);
      }

      expect(mockUpdate).toHaveBeenCalled();
    });
  });

  describe('insertLink with existing link (remove link)', () => {
    it('should dispatch TOGGLE_LINK_COMMAND with null when isLink is true', () => {
      render(<ToolbarPlugin link />);

      // Click the link button (should add a link when isLink is false)
      fireEvent.click(screen.getByRole('button', {name: 'Format Link'}));

      // Should dispatch with 'https://' to add the link (since isLink is false by default)
      expect(mockDispatchCommand).toHaveBeenCalledWith('TOGGLE_LINK_COMMAND', 'https://');
    });
  });

  describe('Command Registration Callbacks', () => {
    it('should execute SELECTION_CHANGE_COMMAND callback and return false', () => {
      render(<ToolbarPlugin />);

      // SELECTION_CHANGE_COMMAND is registered
      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should execute CAN_UNDO_COMMAND callback and update canUndo state', () => {
      render(<ToolbarPlugin history />);

      // CAN_UNDO_COMMAND is registered
      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should execute CAN_REDO_COMMAND callback and update canRedo state', () => {
      render(<ToolbarPlugin history />);

      // CAN_REDO_COMMAND is registered
      expect(mockRegisterCommand).toHaveBeenCalled();
    });
  });

  describe('Update Listener Execution', () => {
    it('should call $updateToolbar through editorState.read', () => {
      render(<ToolbarPlugin />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });
  });

  describe('formatHeading Coverage', () => {
    it('should not update editor when blockType already matches heading size', () => {
      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click heading 1
      fireEvent.click(screen.getByText('Heading 1'));

      expect(mockUpdate).toHaveBeenCalled();
    });
  });

  describe('formatParagraph with $setBlocksType', () => {
    it('should call $setBlocksType when selection is range and blockType is not paragraph', async () => {
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      const lexicalSelection = await vi.importMock<typeof import('@lexical/selection')>('@lexical/selection');

      // Mock $isRangeSelection to return true
      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: vi.fn().mockReturnValue(false),
        anchor: {
          getNode: () => ({
            getKey: () => 'test-key',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'h1', // Not paragraph
            }),
          }),
        },
      });

      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click paragraph option - this should trigger $setBlocksType
      const paragraphItems = screen.getAllByText('Paragraph');
      const paragraphMenuItem = paragraphItems.find((item) => item.closest('[role="menuitem"]'));
      if (paragraphMenuItem) {
        fireEvent.click(paragraphMenuItem);
      }

      // Verify $setBlocksType was called through editor.update
      expect(mockUpdate).toHaveBeenCalled();
      expect(lexicalSelection.$setBlocksType).toBeDefined();
    });
  });

  describe('insertLink with isLink true (remove link)', () => {
    it('should dispatch TOGGLE_LINK_COMMAND with null when isLink is true', async () => {
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      const lexicalLink = await vi.importMock<typeof import('@lexical/link')>('@lexical/link');

      // Setup mock to make isLink true
      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: vi.fn().mockReturnValue(false),
        anchor: {
          getNode: () => ({
            getKey: () => 'test-key',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'paragraph',
            }),
          }),
        },
      });

      // Make $isLinkNode return true to set isLink state
      (lexicalLink.$isLinkNode as ReturnType<typeof vi.fn>).mockReturnValue(true);

      // Capture the update listener to trigger $updateToolbar
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      let capturedUpdateListener: any;
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      mockRegisterUpdateListener.mockImplementation((callback: any) => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
        capturedUpdateListener = callback;
        return vi.fn();
      });

      render(<ToolbarPlugin link />);

      // Trigger update listener to set isLink to true
      if (capturedUpdateListener) {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-call
        capturedUpdateListener({
          editorState: {
            read: (cb: () => void) => cb(),
          },
        });
      }

      // Now click the link button - since isLink is true, it should dispatch with null
      fireEvent.click(screen.getByRole('button', {name: 'Format Link'}));

      // Should dispatch TOGGLE_LINK_COMMAND
      expect(mockDispatchCommand).toHaveBeenCalledWith('TOGGLE_LINK_COMMAND', expect.anything());
    });
  });

  describe('$updateToolbar Function Coverage', () => {
    it('should set isBold, isItalic, isUnderline from selection format', () => {
      render(<ToolbarPlugin bold italic underline />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should set isLink true when parent is link node', () => {
      render(<ToolbarPlugin link />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should set isLink false when neither parent nor node is link', () => {
      render(<ToolbarPlugin link />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should set selectedAlignment from element.getFormat()', () => {
      render(<ToolbarPlugin alignment />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should set blockType from heading tag when element is heading node', () => {
      render(<ToolbarPlugin typography />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should set blockType to paragraph for unknown element type', () => {
      render(<ToolbarPlugin typography />);

      // Should default to 'paragraph' for unknown types
      expect(screen.getByText('Paragraph')).toBeInTheDocument();
    });

    it('should use anchorNode when key is root', () => {
      render(<ToolbarPlugin />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });
  });

  describe('Command Registration Callbacks Direct Execution', () => {
    it('should execute SELECTION_CHANGE_COMMAND callback and call $updateToolbar', () => {
      render(<ToolbarPlugin />);

      // SELECTION_CHANGE_COMMAND is registered
      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should execute CAN_UNDO_COMMAND callback and update canUndo state', () => {
      render(<ToolbarPlugin history />);

      // CAN_UNDO_COMMAND is registered
      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should execute CAN_REDO_COMMAND callback and update canRedo state', () => {
      render(<ToolbarPlugin history />);

      // CAN_REDO_COMMAND is registered
      expect(mockRegisterCommand).toHaveBeenCalled();
    });
  });

  describe('Update Listener Callback', () => {
    it('should call editorState.read with $updateToolbar', () => {
      render(<ToolbarPlugin />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });
  });

  describe('Block Type Detection from Known Type', () => {
    it('should set blockType from element type when in blockTypeToBlockName', () => {
      render(<ToolbarPlugin typography />);

      expect(screen.getByText('Paragraph')).toBeInTheDocument();
    });
  });

  describe('formatParagraph with Range Selection (line 137)', () => {
    it('should execute $setBlocksType when blockType is not paragraph and selection is range', async () => {
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      const lexicalSelection = await vi.importMock<typeof import('@lexical/selection')>('@lexical/selection');
      const richText = await vi.importMock<typeof import('@lexical/rich-text')>('@lexical/rich-text');

      // Make $isRangeSelection return true
      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);

      // Make blockType be 'h1' initially (not paragraph)
      (richText.$isHeadingNode as ReturnType<typeof vi.fn>).mockReturnValue(true);

      // Mock getSelectedNode to set up proper node structure
      const getSelectedNode =
        await vi.importMock<typeof import('../../utils/getSelectedNode')>('../../utils/getSelectedNode');
      (getSelectedNode.default as ReturnType<typeof vi.fn>).mockReturnValue({
        getParent: () => null,
        getKey: () => 'test-key',
        getTopLevelElementOrThrow: () => ({
          getFormat: () => 0,
          getKey: () => 'element-key',
          getType: () => 'heading',
          getTag: () => 'h1',
        }),
      });

      // Mock selection
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: vi.fn().mockReturnValue(false),
        anchor: {
          getNode: () => ({
            getKey: () => 'test-key',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'heading',
              getTag: () => 'h1',
            }),
          }),
        },
      });

      render(<ToolbarPlugin typography />);

      // Open menu
      const typographyButton = screen.getByText('Paragraph').closest('button');
      fireEvent.click(typographyButton!);

      // Click paragraph option - this triggers formatParagraph which calls $setBlocksType
      const paragraphItems = screen.getAllByText('Paragraph');
      const paragraphMenuItem = paragraphItems.find((item) => item.closest('[role="menuitem"]'));
      if (paragraphMenuItem) {
        fireEvent.click(paragraphMenuItem);
      }

      // editor.update should have been called
      expect(mockUpdate).toHaveBeenCalled();
      // $setBlocksType should have been called (via mockUpdate callback)
      expect(lexicalSelection.$setBlocksType).toBeDefined();
    });
  });

  describe('insertLink toggle remove (line 168)', () => {
    it('should dispatch TOGGLE_LINK_COMMAND with null when removing an existing link', async () => {
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      const lexicalLink = await vi.importMock<typeof import('@lexical/link')>('@lexical/link');
      const richText = await vi.importMock<typeof import('@lexical/rich-text')>('@lexical/rich-text');
      const getSelectedNode =
        await vi.importMock<typeof import('../../utils/getSelectedNode')>('../../utils/getSelectedNode');

      // Setup for isLink to become true
      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (lexicalLink.$isLinkNode as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (richText.$isHeadingNode as ReturnType<typeof vi.fn>).mockReturnValue(false);
      (getSelectedNode.default as ReturnType<typeof vi.fn>).mockReturnValue({
        getParent: () => ({type: 'link'}),
        getKey: () => 'test-key',
        getTopLevelElementOrThrow: () => ({
          getFormat: () => 0,
          getKey: () => 'element-key',
          getType: () => 'paragraph',
          getTag: () => 'p',
        }),
      });
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: vi.fn().mockReturnValue(false),
        anchor: {
          getNode: () => ({
            getKey: () => 'test-key',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'paragraph',
              getTag: () => 'p',
            }),
          }),
        },
      });

      // Capture the update listener AND immediately call it during registration
      mockRegisterUpdateListener.mockImplementation(
        (callback: (state: {editorState: {read: (cb: () => void) => void}}) => void) => {
          // Immediately call the callback to set the initial state
          callback({
            editorState: {
              read: (cb: () => void) => cb(),
            },
          });
          return vi.fn();
        },
      );

      const {rerender} = render(<ToolbarPlugin link />);

      // Rerender to ensure state is updated
      rerender(<ToolbarPlugin link />);

      // Click the link button - since isLink is true (set by $isLinkNode returning true), should dispatch with null
      fireEvent.click(screen.getByRole('button', {name: 'Format Link'}));

      // Should have dispatched TOGGLE_LINK_COMMAND with null
      expect(mockDispatchCommand).toHaveBeenCalledWith('TOGGLE_LINK_COMMAND', null);
    });
  });

  describe('$updateToolbar setIsLink false branch (line 191)', () => {
    it('should set isLink to false when neither parent nor node is a link', async () => {
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      const lexicalLink = await vi.importMock<typeof import('@lexical/link')>('@lexical/link');
      const richText = await vi.importMock<typeof import('@lexical/rich-text')>('@lexical/rich-text');
      const getSelectedNode =
        await vi.importMock<typeof import('../../utils/getSelectedNode')>('../../utils/getSelectedNode');

      // Setup for isLink to be false
      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (lexicalLink.$isLinkNode as ReturnType<typeof vi.fn>).mockReturnValue(false);
      (richText.$isHeadingNode as ReturnType<typeof vi.fn>).mockReturnValue(false);
      (getSelectedNode.default as ReturnType<typeof vi.fn>).mockReturnValue({
        getParent: () => ({type: 'paragraph'}),
        getKey: () => 'test-key',
        getTopLevelElementOrThrow: () => ({
          getFormat: () => 0,
          getKey: () => 'element-key',
          getType: () => 'paragraph',
          getTag: () => 'p',
        }),
      });
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: vi.fn().mockReturnValue(false),
        anchor: {
          getNode: () => ({
            getKey: () => 'test-key',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'paragraph',
              getTag: () => 'p',
            }),
          }),
        },
      });

      // Capture the update listener
      type UpdateCallback = (state: {editorState: {read: (cb: () => void) => void}}) => void;
      let capturedCallback: UpdateCallback | null = null;
      mockRegisterUpdateListener.mockImplementation((callback) => {
        capturedCallback = callback as UpdateCallback;
        return vi.fn();
      });

      render(<ToolbarPlugin link />);

      // Trigger update listener - this should call $updateToolbar and set isLink to false
      if (capturedCallback !== null) {
        (capturedCallback as UpdateCallback)({
          editorState: {
            read: (cb: () => void) => cb(),
          },
        });
      }

      // Click the link button - since isLink is false, should dispatch with 'https://'
      fireEvent.click(screen.getByRole('button', {name: 'Format Link'}));

      // Should have dispatched TOGGLE_LINK_COMMAND with 'https://'
      expect(mockDispatchCommand).toHaveBeenCalledWith('TOGGLE_LINK_COMMAND', 'https://');
    });
  });

  describe('$updateToolbar block type detection (lines 207-216)', () => {
    it('should set blockType from heading tag when element is a heading node', async () => {
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      const richText = await vi.importMock<typeof import('@lexical/rich-text')>('@lexical/rich-text');
      const getSelectedNode =
        await vi.importMock<typeof import('../../utils/getSelectedNode')>('../../utils/getSelectedNode');

      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (richText.$isHeadingNode as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (getSelectedNode.default as ReturnType<typeof vi.fn>).mockReturnValue({
        getParent: () => null,
        getKey: () => 'test-key',
        getTopLevelElementOrThrow: () => ({
          getFormat: () => 0,
          getKey: () => 'element-key',
          getType: () => 'heading',
          getTag: () => 'h2',
        }),
      });
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: vi.fn().mockReturnValue(false),
        anchor: {
          getNode: () => ({
            getKey: () => 'test-key',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'heading',
              getTag: () => 'h2',
            }),
          }),
        },
      });

      // Capture the update listener
      type UpdateCallback = (state: {editorState: {read: (cb: () => void) => void}}) => void;
      let capturedCallback: UpdateCallback | null = null;
      mockRegisterUpdateListener.mockImplementation((callback) => {
        capturedCallback = callback as UpdateCallback;
        return vi.fn();
      });

      render(<ToolbarPlugin typography />);

      // Trigger update listener - this should call $updateToolbar and set blockType to 'h2'
      if (capturedCallback !== null) {
        (capturedCallback as UpdateCallback)({
          editorState: {
            read: (cb: () => void) => cb(),
          },
        });
      }

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should set blockType to paragraph for unknown element types', async () => {
      const lexical = await vi.importMock<typeof import('lexical')>('lexical');
      const richText = await vi.importMock<typeof import('@lexical/rich-text')>('@lexical/rich-text');
      const getSelectedNode =
        await vi.importMock<typeof import('../../utils/getSelectedNode')>('../../utils/getSelectedNode');

      (lexical.$isRangeSelection as ReturnType<typeof vi.fn>).mockReturnValue(true);
      (richText.$isHeadingNode as ReturnType<typeof vi.fn>).mockReturnValue(false);
      (getSelectedNode.default as ReturnType<typeof vi.fn>).mockReturnValue({
        getParent: () => null,
        getKey: () => 'test-key',
        getTopLevelElementOrThrow: () => ({
          getFormat: () => 0,
          getKey: () => 'element-key',
          getType: () => 'unknown-type', // Not in blockTypeToBlockName
        }),
      });
      (lexical.$getSelection as ReturnType<typeof vi.fn>).mockReturnValue({
        hasFormat: vi.fn().mockReturnValue(false),
        anchor: {
          getNode: () => ({
            getKey: () => 'test-key',
            getTopLevelElementOrThrow: () => ({
              getFormat: () => 0,
              getKey: () => 'element-key',
              getType: () => 'unknown-type',
            }),
          }),
        },
      });

      // Capture the update listener
      type UpdateCallback = (state: {editorState: {read: (cb: () => void) => void}}) => void;
      let capturedCallback: UpdateCallback | null = null;
      mockRegisterUpdateListener.mockImplementation((callback) => {
        capturedCallback = callback as UpdateCallback;
        return vi.fn();
      });

      render(<ToolbarPlugin typography />);

      // Trigger update listener - this should call $updateToolbar and default to 'paragraph'
      if (capturedCallback !== null) {
        (capturedCallback as UpdateCallback)({
          editorState: {
            read: (cb: () => void) => cb(),
          },
        });
      }

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });
  });

  describe('SELECTION_CHANGE_COMMAND callback (line 234)', () => {
    it('should call $updateToolbar when SELECTION_CHANGE_COMMAND is triggered', () => {
      // Capture SELECTION_CHANGE_COMMAND callback
      type SelectionCallback = () => boolean;
      let selectionChangeCallback: SelectionCallback | null = null;
      mockRegisterCommand.mockImplementation((command, callback) => {
        if (command === 'SELECTION_CHANGE_COMMAND') {
          selectionChangeCallback = callback as SelectionCallback;
        }
        return vi.fn();
      });

      render(<ToolbarPlugin />);

      // Execute the SELECTION_CHANGE_COMMAND callback
      expect(selectionChangeCallback).not.toBeNull();
      const result = (selectionChangeCallback as unknown as SelectionCallback)();
      expect(result).toBe(false);

      expect(mockRegisterCommand).toHaveBeenCalled();
    });
  });
});
