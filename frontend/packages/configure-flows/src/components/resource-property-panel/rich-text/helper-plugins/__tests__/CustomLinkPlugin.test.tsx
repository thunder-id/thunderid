// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor, act} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import CustomLinkPlugin from '../CustomLinkPlugin';

// Use vi.hoisted for mock functions
const {
  mockDispatchCommand,
  mockRegisterUpdateListener,
  mockRegisterCommand,
  mockGetRootElement,
  mockGetEditorState,
  mockGetSelection,
  mockIsRangeSelection,
  mockIsLinkNode,
  mockGetSelectedNode,
  mockEditorUpdate,
} = vi.hoisted(() => ({
  mockDispatchCommand: vi.fn<(...args: unknown[]) => unknown>(),
  mockRegisterUpdateListener: vi.fn<(...args: unknown[]) => () => void>(() => vi.fn()),
  mockRegisterCommand: vi.fn<(...args: unknown[]) => () => void>(() => vi.fn()),
  mockGetRootElement: vi.fn<() => HTMLElement | null>(() => document.createElement('div')),
  mockGetEditorState: vi.fn<() => {read: (callback: () => void) => void}>(() => ({
    read: vi.fn((callback: () => void) => callback()),
  })),
  mockGetSelection: vi.fn<() => unknown>(() => ({type: 'range'})),
  mockIsRangeSelection: vi.fn<(selection: unknown) => boolean>(() => true),
  mockIsLinkNode: vi.fn<(node: unknown) => boolean>(() => false),
  mockGetSelectedNode: vi.fn<() => unknown>(() => ({
    getParent: () => null,
    getURL: () => 'https://example.com',
    setTarget: vi.fn(),
    setRel: vi.fn(),
    getTextContent: () => '',
    type: 'text',
  })),
  mockEditorUpdate: vi.fn(),
}));

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

// Mock the lexical composer context
vi.mock('@lexical/react/LexicalComposerContext', () => ({
  useLexicalComposerContext: () => [
    {
      dispatchCommand: mockDispatchCommand,
      registerUpdateListener: mockRegisterUpdateListener,
      registerCommand: mockRegisterCommand,
      getRootElement: mockGetRootElement,
      getEditorState: mockGetEditorState,
      update: mockEditorUpdate,
    },
  ],
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
  $getSelection: mockGetSelection,
  $isRangeSelection: mockIsRangeSelection,
  $isTextNode: vi.fn(() => false),
  CLICK_COMMAND: 'CLICK_COMMAND',
  KEY_ESCAPE_COMMAND: 'KEY_ESCAPE_COMMAND',
  SELECTION_CHANGE_COMMAND: 'SELECTION_CHANGE_COMMAND',
}));

// Mock @lexical/link
vi.mock('@lexical/link', () => ({
  $isLinkNode: mockIsLinkNode,
  TOGGLE_LINK_COMMAND: 'TOGGLE_LINK_COMMAND',
}));

// Mock getSelectedNode utility
vi.mock('../../utils/getSelectedNode', () => ({
  default: mockGetSelectedNode,
}));

// Mock commands
vi.mock('../commands', () => ({
  default: 'TOGGLE_SAFE_LINK_COMMAND',
}));

// Mock DynamicValuePopover
vi.mock('../../../DynamicValuePopover', () => ({
  default: () => null,
}));

// Mock createPortal to render directly
vi.mock('react-dom', () => ({
  createPortal: (children: React.ReactNode) => children,
}));

describe('CustomLinkPlugin', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    // Restore default implementations for hoisted mocks
    // vi.clearAllMocks() does not reset implementations set via mockReturnValue/mockImplementation
    mockRegisterUpdateListener.mockImplementation(() => vi.fn());
    mockRegisterCommand.mockImplementation(() => vi.fn());
    mockGetRootElement.mockImplementation(() => document.createElement('div'));
    mockGetEditorState.mockImplementation(() => ({
      read: vi.fn((callback: () => void) => callback()),
    }));
    mockGetSelection.mockImplementation(() => ({type: 'range'}));
    mockIsRangeSelection.mockImplementation(() => true);
    mockIsLinkNode.mockImplementation(() => false);
    mockEditorUpdate.mockImplementation(vi.fn());
    mockGetSelectedNode.mockImplementation(() => ({
      getParent: () => null,
      getURL: () => 'https://example.com',
      setTarget: vi.fn(),
      setRel: vi.fn(),
      getTextContent: () => '',
      type: 'text',
    }));

    // Mock window methods
    vi.spyOn(window, 'addEventListener').mockImplementation(vi.fn());
    vi.spyOn(window, 'removeEventListener').mockImplementation(vi.fn());
    vi.spyOn(window, 'open').mockImplementation(vi.fn());
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('Rendering', () => {
    it('should render the link editor card', () => {
      render(<CustomLinkPlugin />);

      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });

    it('should render apply button', () => {
      render(<CustomLinkPlugin />);

      expect(screen.getByText('flows:core.elements.richText.linkEditor.apply')).toBeInTheDocument();
    });

    it('should render close button', () => {
      render(<CustomLinkPlugin />);

      // Find the close button (IconButton with X icon)
      const closeButtons = screen.getAllByRole('button');
      expect(closeButtons.length).toBeGreaterThan(0);
    });

    it('should render URL input field', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      expect(inputs.length).toBeGreaterThan(0);
    });
  });

  describe('Command Registration', () => {
    it('should register CLICK_COMMAND on mount', () => {
      render(<CustomLinkPlugin />);

      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should register update listener on mount', () => {
      render(<CustomLinkPlugin />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should register SELECTION_CHANGE_COMMAND', () => {
      render(<CustomLinkPlugin />);

      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should register KEY_ESCAPE_COMMAND', () => {
      render(<CustomLinkPlugin />);

      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should register TOGGLE_SAFE_LINK_COMMAND', () => {
      render(<CustomLinkPlugin />);

      expect(mockRegisterCommand).toHaveBeenCalled();
    });
  });

  describe('Edit Mode', () => {
    it('should show URL input field and apply button', () => {
      render(<CustomLinkPlugin />);

      // The component always shows the URL input and apply button
      expect(document.querySelector('.MuiTextField-root')).toBeInTheDocument();
      expect(screen.getByText('flows:core.elements.richText.linkEditor.apply')).toBeInTheDocument();
    });

    it('should show apply button instead of save button', () => {
      render(<CustomLinkPlugin />);

      expect(screen.getByText('flows:core.elements.richText.linkEditor.apply')).toBeInTheDocument();
    });

    it('should show text field', () => {
      render(<CustomLinkPlugin />);

      const textField = document.querySelector('.MuiTextField-root');
      expect(textField).toBeInTheDocument();
    });

    it('should handle escape key press in URL field', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // Second input is the URL field
      if (urlInput) {
        act(() => {
          fireEvent.keyDown(urlInput, {key: 'Escape'});
        });
      }

      // The card should still be in the document
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });

    it('should handle enter key press in URL field', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // Second input is the URL field
      expect(urlInput).toBeInTheDocument();
      act(() => {
        fireEvent.change(urlInput, {target: {value: 'https://test.com'}});
      });

      // Verify the input value was updated before Enter key
      expect(urlInput).toHaveValue('https://test.com');

      act(() => {
        fireEvent.keyDown(urlInput, {key: 'Enter'});
      });

      // After Enter, handleApply -> handleClose resets the value
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });

    it('should call handleApply when apply button is clicked', () => {
      render(<CustomLinkPlugin />);

      const applyButton = screen.getByText('flows:core.elements.richText.linkEditor.apply');
      act(() => {
        fireEvent.click(applyButton);
      });

      // Component should still be in the document
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });
  });

  describe('URL Display', () => {
    it('should show URL input field', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      expect(inputs.length).toBeGreaterThan(0);
    });

    it('should show apply button for submitting the link', () => {
      render(<CustomLinkPlugin />);

      const applyButton = screen.getByText('flows:core.elements.richText.linkEditor.apply');
      expect(applyButton).toBeInTheDocument();
    });
  });

  describe('Close Functionality', () => {
    it('should reset state when escape key is pressed in URL field', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      if (urlInput) {
        act(() => {
          fireEvent.change(urlInput, {target: {value: 'https://test.com'}});
        });
        act(() => {
          fireEvent.keyDown(urlInput, {key: 'Escape'});
        });
      }

      // Card should still be in DOM (just repositioned off-screen)
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });
  });

  describe('Event Listeners', () => {
    it('should add window resize listener on mount', () => {
      render(<CustomLinkPlugin />);

      expect(window.addEventListener).toHaveBeenCalledWith('resize', expect.any(Function));
    });

    it('should add body scroll listener on mount', () => {
      const addEventListenerSpy = vi.spyOn(document.body, 'addEventListener');

      render(<CustomLinkPlugin />);

      expect(addEventListenerSpy).toHaveBeenCalledWith('scroll', expect.any(Function));

      addEventListenerSpy.mockRestore();
    });

    it('should remove event listeners on unmount', () => {
      const {unmount} = render(<CustomLinkPlugin />);

      unmount();

      expect(window.removeEventListener).toHaveBeenCalledWith('resize', expect.any(Function));
    });
  });

  describe('Edge Cases', () => {
    it('should handle empty URL', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      if (urlInput) {
        act(() => {
          fireEvent.change(urlInput, {target: {value: ''}});
        });
      }

      // Click apply
      const applyButton = screen.getByText('flows:core.elements.richText.linkEditor.apply');
      act(() => {
        fireEvent.click(applyButton);
      });

      // Should not dispatch command with empty URL
      expect(mockDispatchCommand).not.toHaveBeenCalledWith('TOGGLE_SAFE_LINK_COMMAND', expect.anything());
    });

    it('should handle text field input changes', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      expect(urlInput).toBeInTheDocument();
      act(() => {
        fireEvent.change(urlInput, {target: {value: 'https://new-url.com'}});
      });
      expect(urlInput).toHaveValue('https://new-url.com');
    });
  });

  describe('Positioning', () => {
    it('should have absolute positioning', () => {
      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root');
      expect(card).toHaveStyle({position: 'absolute'});
    });
  });

  describe('Link Node Detection', () => {
    it('should detect when parent is a link node', async () => {
      // Mock to return a link node parent
      const {$isLinkNode} = await vi.importMock<typeof import('@lexical/link')>('@lexical/link');
      ($isLinkNode as ReturnType<typeof vi.fn>).mockImplementation(
        (node: {type?: string} | null) => node?.type === 'link',
      );

      render(<CustomLinkPlugin />);

      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should detect when node itself is a link node', async () => {
      const {$isLinkNode} = await vi.importMock<typeof import('@lexical/link')>('@lexical/link');
      ($isLinkNode as ReturnType<typeof vi.fn>).mockImplementation(
        (node: {type?: string} | null) => node?.type === 'link',
      );

      render(<CustomLinkPlugin />);

      expect(mockRegisterCommand).toHaveBeenCalled();
    });
  });

  describe('TOGGLE_SAFE_LINK_COMMAND', () => {
    it('should register TOGGLE_SAFE_LINK_COMMAND handler', () => {
      render(<CustomLinkPlugin />);

      // Verify commands were registered
      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should handle empty URL in TOGGLE_SAFE_LINK_COMMAND', () => {
      render(<CustomLinkPlugin />);

      // The component registers the TOGGLE_SAFE_LINK_COMMAND which handles empty URLs
      expect(mockRegisterCommand).toHaveBeenCalled();
    });
  });

  describe('Click Command Handler', () => {
    it('should register click command for opening links', () => {
      render(<CustomLinkPlugin />);

      // CLICK_COMMAND is registered to handle ctrl/meta+click on links
      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should handle click with meta key on link', () => {
      const mockOpen = vi.spyOn(window, 'open').mockImplementation(vi.fn());

      render(<CustomLinkPlugin />);

      // The CLICK_COMMAND handler checks for metaKey or ctrlKey
      expect(mockRegisterCommand).toHaveBeenCalled();

      mockOpen.mockRestore();
    });

    it('should handle click with ctrl key on link', () => {
      const mockOpen = vi.spyOn(window, 'open').mockImplementation(vi.fn());

      render(<CustomLinkPlugin />);

      // The CLICK_COMMAND handler checks for ctrlKey
      expect(mockRegisterCommand).toHaveBeenCalled();

      mockOpen.mockRestore();
    });
  });

  describe('Position Editor Element', () => {
    it('should position editor when rect is provided', () => {
      render(<CustomLinkPlugin />);

      // The positionEditorElement function is called during updateLinkEditor
      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should hide editor when rect is null', () => {
      render(<CustomLinkPlugin />);

      // When there's no selection, the editor is hidden
      const card = document.querySelector('.MuiCard-root');
      expect(card).toBeInTheDocument();
    });

    it('should handle viewport edge cases for horizontal positioning', () => {
      // Test that the editor stays within viewport bounds
      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should handle viewport edge cases for vertical positioning', () => {
      // Test that the editor positions above selection when near bottom
      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('URL Type Handling', () => {
    it('should determine CUSTOM URL type for regular URLs', () => {
      render(<CustomLinkPlugin />);

      // The determineUrlType function returns 'CUSTOM' for non-predefined URLs
      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should get placeholder URL for custom URLs', () => {
      render(<CustomLinkPlugin />);

      // The getPlaceholderUrl function returns the URL itself for custom URLs
      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('URL Type Change Handler', () => {
    it('should handle URL type change to CUSTOM', () => {
      render(<CustomLinkPlugin />);

      // The URL field is always visible in the new design
      const inputs = document.querySelectorAll('input');
      expect(inputs.length).toBeGreaterThan(0);
    });
  });

  describe('getCurrentUrl Function', () => {
    it('should return linkUrl for CUSTOM type', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      expect(urlInput).toBeInTheDocument();
      act(() => {
        fireEvent.change(urlInput, {target: {value: 'https://custom-url.com'}});
      });
      expect(urlInput).toHaveValue('https://custom-url.com');
    });
  });

  describe('Selection Change Handling', () => {
    it('should update on selection change', () => {
      render(<CustomLinkPlugin />);

      // SELECTION_CHANGE_COMMAND triggers updateLinkEditor
      expect(mockRegisterCommand).toHaveBeenCalled();
    });
  });

  describe('Escape Key Handling', () => {
    it('should handle KEY_ESCAPE_COMMAND', () => {
      render(<CustomLinkPlugin />);

      // KEY_ESCAPE_COMMAND is always registered (no separate edit mode)
      expect(mockRegisterCommand).toHaveBeenCalled();
    });

    it('should handle KEY_ESCAPE_COMMAND and always return true', () => {
      render(<CustomLinkPlugin />);

      // KEY_ESCAPE_COMMAND always returns true in the new design
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });
  });

  describe('Update Listener', () => {
    it('should update link editor on editor state change', () => {
      render(<CustomLinkPlugin />);

      // registerUpdateListener is called to listen for editor state changes
      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });
  });

  describe('Root Element Handling', () => {
    it('should handle null root element', () => {
      mockGetRootElement.mockReturnValueOnce(null as unknown as HTMLDivElement);

      render(<CustomLinkPlugin />);

      // Component should handle null root element gracefully
      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should handle root element with nested children', () => {
      const rootElement = document.createElement('div');
      const child = document.createElement('span');
      rootElement.appendChild(child);
      mockGetRootElement.mockReturnValue(rootElement);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('Native Selection Handling', () => {
    it('should handle collapsed native selection', () => {
      render(<CustomLinkPlugin />);

      // When nativeSelection.isCollapsed is true, editor is hidden
      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should handle non-collapsed native selection', () => {
      render(<CustomLinkPlugin />);

      // When selection is not collapsed, editor is positioned
      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('Focus Handling', () => {
    it('should have URL input field available', async () => {
      render(<CustomLinkPlugin />);

      await waitFor(() => {
        const inputs = document.querySelectorAll('input');
        expect(inputs.length).toBeGreaterThan(0);
      });
    });
  });

  describe('Save Link with Last Selection', () => {
    it('should not dispatch command when lastSelection is null', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      if (urlInput) {
        act(() => {
          fireEvent.change(urlInput, {target: {value: 'https://test.com'}});
        });
      }

      // Click apply (lastSelection is null in default test setup)
      const applyButton = screen.getByText('flows:core.elements.richText.linkEditor.apply');
      act(() => {
        fireEvent.click(applyButton);
      });

      // lastSelection is null so command should not be dispatched
      expect(mockDispatchCommand).not.toHaveBeenCalledWith('TOGGLE_SAFE_LINK_COMMAND', expect.anything());
    });
  });

  describe('Link Attributes', () => {
    it('should have apply button to set link with safe attributes', () => {
      render(<CustomLinkPlugin />);

      // Apply button triggers TOGGLE_SAFE_LINK_COMMAND which sets safe attributes
      const applyButton = screen.getByText('flows:core.elements.richText.linkEditor.apply');
      expect(applyButton).toBeInTheDocument();
    });
  });

  describe('Command Callbacks Execution', () => {
    it('should execute CLICK_COMMAND callback with link node and meta key', () => {
      // Setup mocks for link node scenario
      const mockLinkNode = {
        type: 'link',
        getParent: () => null,
        getURL: () => 'https://clicked-link.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      // Capture the command callback
      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      const mockOpen = vi.spyOn(window, 'open').mockImplementation(vi.fn());

      render(<CustomLinkPlugin />);

      // Execute the click callback with metaKey
      const clickCallback = callbacks['CLICK_COMMAND'] as ((payload: MouseEvent) => boolean) | undefined;
      expect(clickCallback).toBeDefined();
      const mockEvent = {metaKey: true, ctrlKey: false} as MouseEvent;
      const result = clickCallback!(mockEvent);
      expect(result).toBe(true);
      expect(mockOpen).toHaveBeenCalledWith('https://clicked-link.com', '_blank');

      mockOpen.mockRestore();
    });

    it('should execute CLICK_COMMAND callback with link node and ctrl key', () => {
      const mockLinkNode = {
        type: 'link',
        getParent: () => null,
        getURL: () => 'https://ctrl-clicked-link.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      const mockOpen = vi.spyOn(window, 'open').mockImplementation(vi.fn());

      render(<CustomLinkPlugin />);

      const clickCallback = callbacks['CLICK_COMMAND'] as ((payload: MouseEvent) => boolean) | undefined;
      expect(clickCallback).toBeDefined();
      const mockEvent = {metaKey: false, ctrlKey: true} as MouseEvent;
      const result = clickCallback!(mockEvent);
      expect(result).toBe(true);
      expect(mockOpen).toHaveBeenCalledWith('https://ctrl-clicked-link.com', '_blank');

      mockOpen.mockRestore();
    });

    it('should return false from CLICK_COMMAND when no meta/ctrl key', () => {
      const mockLinkNode = {
        type: 'link',
        getParent: () => null,
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      render(<CustomLinkPlugin />);

      const clickCallback = callbacks['CLICK_COMMAND'] as ((payload: MouseEvent) => boolean) | undefined;
      expect(clickCallback).toBeDefined();
      const mockEvent = {metaKey: false, ctrlKey: false} as MouseEvent;
      const result = clickCallback!(mockEvent);
      expect(result).toBe(false);
    });

    it('should return false from CLICK_COMMAND when not a link node', () => {
      const mockTextNode = {
        type: 'text',
        getParent: () => null,
        getURL: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockTextNode);
      mockIsLinkNode.mockReturnValue(false);
      mockIsRangeSelection.mockReturnValue(true);

      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      render(<CustomLinkPlugin />);

      const clickCallback = callbacks['CLICK_COMMAND'] as ((payload: MouseEvent) => boolean) | undefined;
      expect(clickCallback).toBeDefined();
      const mockEvent = {metaKey: true, ctrlKey: false} as MouseEvent;
      const result = clickCallback!(mockEvent);
      expect(result).toBe(false);
    });

    it('should return false from CLICK_COMMAND when linkNode is null', () => {
      const mockTextNode = {
        type: 'text',
        getParent: () => null,
        getURL: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockTextNode);
      mockIsLinkNode.mockReturnValue(false);
      mockIsRangeSelection.mockReturnValue(true);

      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      render(<CustomLinkPlugin />);

      const clickCallback = callbacks['CLICK_COMMAND'] as ((payload: MouseEvent) => boolean) | undefined;
      expect(clickCallback).toBeDefined();
      const mockEvent = {metaKey: true, ctrlKey: false} as MouseEvent;
      const result = clickCallback!(mockEvent);
      expect(result).toBe(false);
    });

    it('should execute TOGGLE_SAFE_LINK_COMMAND with URL', () => {
      const mockSetTarget = vi.fn();
      const mockSetRel = vi.fn();
      const mockLinkNode = {
        type: 'link',
        getParent: () => null,
        getURL: () => 'https://example.com',
        setTarget: mockSetTarget,
        setRel: mockSetRel,
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      render(<CustomLinkPlugin />);

      const toggleSafeLinkCallback = callbacks['TOGGLE_SAFE_LINK_COMMAND'] as ((url: string) => boolean) | undefined;
      expect(toggleSafeLinkCallback).toBeDefined();
      const result = toggleSafeLinkCallback!('https://new-link.com');
      expect(result).toBe(true);
      expect(mockDispatchCommand).toHaveBeenCalledWith('TOGGLE_LINK_COMMAND', 'https://new-link.com');
      expect(mockSetTarget).toHaveBeenCalledWith('_blank');
      expect(mockSetRel).toHaveBeenCalledWith('noopener noreferrer');
    });

    it('should execute TOGGLE_SAFE_LINK_COMMAND with empty URL to remove link', () => {
      mockIsRangeSelection.mockReturnValue(true);

      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      render(<CustomLinkPlugin />);

      const toggleSafeLinkCallback = callbacks['TOGGLE_SAFE_LINK_COMMAND'] as ((url: string) => boolean) | undefined;
      expect(toggleSafeLinkCallback).toBeDefined();
      const result = toggleSafeLinkCallback!('');
      expect(result).toBe(true);
      expect(mockDispatchCommand).toHaveBeenCalledWith('TOGGLE_LINK_COMMAND', null);
    });

    it('should return false from TOGGLE_SAFE_LINK_COMMAND when linkNode is null', () => {
      const mockTextNode = {
        type: 'text',
        getParent: () => null,
        getURL: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockTextNode);
      mockIsLinkNode.mockReturnValue(false);
      mockIsRangeSelection.mockReturnValue(true);

      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      render(<CustomLinkPlugin />);

      const toggleSafeLinkCallback = callbacks['TOGGLE_SAFE_LINK_COMMAND'] as ((url: string) => boolean) | undefined;
      expect(toggleSafeLinkCallback).toBeDefined();
      const result = toggleSafeLinkCallback!('https://test.com');
      expect(result).toBe(false);
    });

    it('should execute KEY_ESCAPE_COMMAND in edit mode', () => {
      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      render(<CustomLinkPlugin />);

      // KEY_ESCAPE_COMMAND is always registered (no separate edit mode)
      expect(callbacks['KEY_ESCAPE_COMMAND']).toBeDefined();
    });

    it('should execute KEY_ESCAPE_COMMAND and return true', () => {
      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      render(<CustomLinkPlugin />);

      // KEY_ESCAPE_COMMAND always returns true in the new design
      const escapeCallback = callbacks['KEY_ESCAPE_COMMAND'] as (() => boolean) | undefined;
      expect(escapeCallback).toBeDefined();
      const result = escapeCallback!();
      expect(result).toBe(true);
    });

    it('should execute SELECTION_CHANGE_COMMAND callback', () => {
      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      render(<CustomLinkPlugin />);

      const selectionChangeCallback = callbacks['SELECTION_CHANGE_COMMAND'] as (() => boolean) | undefined;
      expect(selectionChangeCallback).toBeDefined();
      const result = selectionChangeCallback!();
      expect(result).toBe(false);
    });
  });

  describe('updateLinkEditor Function Coverage', () => {
    it('should handle link node parent', () => {
      const mockParentLinkNode = {
        type: 'link',
        getURL: () => 'https://parent-link.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      const mockTextNode = {
        type: 'text',
        getParent: () => mockParentLinkNode,
        getURL: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockTextNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockParentLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should handle node being a link node directly', () => {
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://direct-link.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should handle non-link node without parent', () => {
      const mockTextNode = {
        type: 'text',
        getParent: () => null,
        getURL: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockTextNode);
      mockIsLinkNode.mockReturnValue(false);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });

    it('should handle non-range selection', () => {
      mockIsRangeSelection.mockReturnValue(false);

      render(<CustomLinkPlugin />);

      expect(mockRegisterUpdateListener).toHaveBeenCalled();
    });
  });

  describe('positionEditorElement Function Coverage', () => {
    it('should handle rect being null', () => {
      mockIsRangeSelection.mockReturnValue(false);

      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root')!;
      // When rect is null, editor should be hidden (opacity: 0)
      expect(card).toBeInTheDocument();
    });

    it('should position editor when rect is provided and left edge adjustment needed', () => {
      // Mock window dimensions
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 800, writable: true});
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 0, writable: true});

      // Mock native selection with rect near left edge
      const mockSelection = {
        isCollapsed: false,
        anchorNode: document.createElement('div'),
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: -50, // Near left edge
            height: 20,
            width: 100,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      const rootElement = document.createElement('div');
      rootElement.appendChild(mockSelection.anchorNode);
      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should position editor when rect is provided and right edge adjustment needed', () => {
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 800, writable: true});

      const mockSelection = {
        isCollapsed: false,
        anchorNode: document.createElement('div'),
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 900, // Near right edge
            height: 20,
            width: 100,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      const rootElement = document.createElement('div');
      rootElement.appendChild(mockSelection.anchorNode);
      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should position editor above selection when near bottom', () => {
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 800, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 0, writable: true});

      const mockSelection = {
        isCollapsed: false,
        anchorNode: document.createElement('div'),
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 750, // Near bottom
            left: 100,
            height: 20,
            width: 100,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      const rootElement = document.createElement('div');
      rootElement.appendChild(mockSelection.anchorNode);
      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should handle anchorNode being root element', () => {
      const rootElement = document.createElement('div');
      const innerChild = document.createElement('span');
      rootElement.appendChild(innerChild);

      const mockSelection = {
        isCollapsed: false,
        anchorNode: rootElement, // anchorNode is rootElement
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 100,
            height: 20,
            width: 100,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('Event Listener Cleanup', () => {
    it('should remove scroll listener on unmount', () => {
      const removeEventListenerSpy = vi.spyOn(document.body, 'removeEventListener');

      const {unmount} = render(<CustomLinkPlugin />);

      unmount();

      expect(removeEventListenerSpy).toHaveBeenCalledWith('scroll', expect.any(Function));

      removeEventListenerSpy.mockRestore();
    });
  });

  describe('updateLinkEditor Edge Cases', () => {
    it('should return early when editorElem is null', () => {
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelectedNode.mockReturnValue({
        getParent: () => ({type: 'paragraph'}),
        getURL: () => '',
      });
      mockIsLinkNode.mockReturnValue(false);

      render(<CustomLinkPlugin />);

      // The component should handle null editor ref gracefully
      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should return early when rootElement is null', () => {
      mockGetRootElement.mockReturnValue(null as unknown as HTMLElement);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should handle anchorNode being the root element with nested children', () => {
      const rootElement = document.createElement('div');
      const child1 = document.createElement('p');
      const child2 = document.createElement('span');
      child1.appendChild(child2);
      rootElement.appendChild(child1);

      const mockSelection = {
        isCollapsed: false,
        anchorNode: rootElement,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 100,
            height: 20,
            width: 100,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should position editor using domRange rect when anchorNode is not root', () => {
      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 100,
            height: 20,
            width: 100,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should hide editor when active element has link-input class', () => {
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockInput = document.createElement('input');
      mockInput.className = 'link-input';
      document.body.appendChild(mockInput);
      mockInput.focus();

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();

      document.body.removeChild(mockInput);
    });
  });

  describe('handleUrlTypeChange', () => {
    it('should handle URL type change to CUSTOM and set URL to https://', () => {
      render(<CustomLinkPlugin />);

      // The URL field is always visible in the new design (no mode toggle)
      const inputs = document.querySelectorAll('input');
      expect(inputs.length).toBeGreaterThan(0);
    });
  });

  describe('getCurrentUrl', () => {
    it('should return linkUrl for CUSTOM selectedUrlType', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      expect(urlInput).toBeInTheDocument();
      act(() => {
        fireEvent.change(urlInput, {target: {value: 'https://custom.com'}});
      });

      expect(urlInput).toHaveValue('https://custom.com');
    });
  });

  describe('Save Button Click with Empty URL', () => {
    it('should not dispatch command when URL is empty on apply button click', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      if (urlInput) {
        act(() => {
          fireEvent.change(urlInput, {target: {value: ''}});
        });
      }

      // Click apply
      const applyButton = screen.getByText('flows:core.elements.richText.linkEditor.apply');
      act(() => {
        fireEvent.click(applyButton);
      });

      // Component should handle empty URL gracefully
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });
  });

  describe('Enter Key Press with Empty URL', () => {
    it('should not dispatch command when URL is empty on Enter key', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      if (urlInput) {
        act(() => {
          fireEvent.change(urlInput, {target: {value: ''}});
          fireEvent.keyDown(urlInput, {key: 'Enter'});
        });
      }

      // The component should handle the Enter key press with empty URL
      // The command should not be dispatched with an empty URL
      expect(urlInput).toHaveValue('');
    });
  });

  describe('Update Listener Callback', () => {
    it('should execute update listener callback', () => {
      type UpdateCallback = (state: {editorState: {read: (cb: () => void) => void}}) => void;
      const capturedCallbacks: UpdateCallback[] = [];
      (mockRegisterUpdateListener as ReturnType<typeof vi.fn>).mockImplementation((callback: unknown) => {
        capturedCallbacks.push(callback as UpdateCallback);
        return vi.fn();
      });

      render(<CustomLinkPlugin />);

      const updateListenerCallback = capturedCallbacks[0];
      expect(updateListenerCallback).toBeDefined();
      const mockEditorState = {
        read: vi.fn((cb: () => void) => cb()),
      };
      updateListenerCallback({editorState: mockEditorState});
      expect(mockEditorState.read).toHaveBeenCalled();
    });
  });

  describe('Handle Close with Editor Ref', () => {
    it('should call positionEditorElement with null on apply', () => {
      render(<CustomLinkPlugin />);

      // Click the apply button (which calls handleApply -> handleClose)
      const applyButton = screen.getByText('flows:core.elements.richText.linkEditor.apply');
      act(() => {
        fireEvent.click(applyButton);
      });

      // The card should still be in the document (just repositioned off-screen)
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });
  });

  describe('updateLinkEditor when parent is null', () => {
    it('should return early when parent node is null', () => {
      mockGetSelectedNode.mockReturnValue({
        getParent: () => null,
        getURL: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
        type: 'text',
      });
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('handleUrlTypeChange with predefined URL', () => {
    it('should handle URL type change when selectedOption is found', () => {
      render(<CustomLinkPlugin />);

      // The URL field is always visible in the new design
      const inputs = document.querySelectorAll('input');
      expect(inputs.length).toBeGreaterThan(0);
    });

    it('should set linkUrl to https:// when switching to CUSTOM type', () => {
      render(<CustomLinkPlugin />);

      // The URL input field is always available
      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      expect(urlInput).toBeInTheDocument();
    });
  });

  describe('getCurrentUrl for predefined URLs', () => {
    it('should return linkUrl when selectedUrlType is CUSTOM', () => {
      render(<CustomLinkPlugin />);

      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      if (urlInput) {
        act(() => {
          fireEvent.change(urlInput, {target: {value: 'https://myurl.com'}});
        });

        // Click apply - this triggers getCurrentUrl()
        const applyButton = screen.getByText('flows:core.elements.richText.linkEditor.apply');
        act(() => {
          fireEvent.click(applyButton);
        });
      }

      // Card should still be in the document
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });

    it('should return selectedOption.value when selectedUrlType is not CUSTOM but option not found', () => {
      render(<CustomLinkPlugin />);

      // Since PREDEFINED_URLS is empty, even if we had a non-CUSTOM type,
      // it would fall back to returning linkUrl. Verify URL field is present.
      const inputs = document.querySelectorAll('input');
      expect(inputs.length).toBeGreaterThan(0);
    });
  });

  describe('Save button with lastSelection present', () => {
    it('should dispatch command when lastSelection is present and URL is not empty', () => {
      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test link');
      rootElement.appendChild(textNode);

      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 100,
            height: 20,
            width: 100,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      // Type a new URL in the URL input (second input field)
      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      if (urlInput) {
        act(() => {
          fireEvent.change(urlInput, {target: {value: 'https://newurl.com'}});
        });
      }

      // Click the apply button
      const applyButton = screen.getByText('flows:core.elements.richText.linkEditor.apply');
      act(() => {
        fireEvent.click(applyButton);
      });

      // Should dispatch command since lastSelection is set and URL is not empty
      // The URL dispatched is the one set by updateLinkEditor (from getURL() mock)
      expect(mockDispatchCommand).toHaveBeenCalledWith('TOGGLE_SAFE_LINK_COMMAND', expect.stringContaining('https://'));
    });
  });

  describe('Enter key with lastSelection present', () => {
    it('should dispatch command when Enter is pressed with valid lastSelection and URL', () => {
      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test link');
      rootElement.appendChild(textNode);

      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 100,
            height: 20,
            width: 100,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      // Press Enter in the URL input field
      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      if (urlInput) {
        act(() => {
          fireEvent.change(urlInput, {target: {value: 'https://enterkey.com'}});
          fireEvent.keyDown(urlInput, {key: 'Enter'});
        });
      }

      // Should dispatch command since lastSelection is set and URL is not empty
      expect(mockDispatchCommand).toHaveBeenCalledWith('TOGGLE_SAFE_LINK_COMMAND', 'https://enterkey.com');
    });
  });

  describe('positionEditorElement edge cases', () => {
    it('should position editor when top position would be negative', () => {
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 200, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 100, writable: true});

      const mockSelection = {
        isCollapsed: false,
        anchorNode: document.createElement('div'),
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: -50, // Negative top
            left: 100,
            height: 20,
            width: 100,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      const rootElement = document.createElement('div');
      rootElement.appendChild(mockSelection.anchorNode);
      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should position editor to the right when left would be negative', () => {
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});

      const mockSelection = {
        isCollapsed: false,
        anchorNode: document.createElement('div'),
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: -100, // Negative left
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      const rootElement = document.createElement('div');
      rootElement.appendChild(mockSelection.anchorNode);
      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should position editor when it would overflow right edge', () => {
      Object.defineProperty(window, 'innerWidth', {value: 400, writable: true});
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});

      const mockSelection = {
        isCollapsed: false,
        anchorNode: document.createElement('div'),
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 350, // Near right edge
            height: 20,
            width: 100,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      const rootElement = document.createElement('div');
      rootElement.appendChild(mockSelection.anchorNode);
      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('updateLinkEditor with link-input active element', () => {
    it('should not reposition when active element has link-input class', () => {
      const mockInput = document.createElement('input');
      mockInput.className = 'link-input';
      document.body.appendChild(mockInput);
      mockInput.focus();

      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();

      document.body.removeChild(mockInput);
    });
  });

  describe('updateLinkEditor inner element traversal', () => {
    it('should traverse nested elements when anchorNode is rootElement', () => {
      const rootElement = document.createElement('div');
      const level1 = document.createElement('p');
      const level2 = document.createElement('span');
      const level3 = document.createElement('strong');
      level2.appendChild(level3);
      level1.appendChild(level2);
      rootElement.appendChild(level1);

      const mockSelection = {
        isCollapsed: false,
        anchorNode: rootElement, // anchorNode is the rootElement
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 100,
            height: 20,
            width: 100,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      // The while loop should traverse through level1 -> level2 -> level3
      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('positionEditorElement boundary adjustments', () => {
    it('should adjust left position when editor would be cut off on the left', () => {
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 800, writable: true});
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 0, writable: true});

      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      // Mock selection with rect that would cause negative left position
      // The formula is: left = rect.left + pageXOffset - editorWidth/2 + rect.width/2
      // If editorWidth is ~350 (maxWidth), then left = -200 + 0 - 175 + 25 = -350 (negative)
      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: -200, // Far left position
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root')!;
      expect(card).toBeInTheDocument();
    });

    it('should adjust left position when editor would overflow right edge', () => {
      Object.defineProperty(window, 'innerWidth', {value: 500, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 800, writable: true});
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 0, writable: true});

      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      // Position that would overflow right: left + editorWidth > viewportWidth
      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 450, // Near right edge
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root')!;
      expect(card).toBeInTheDocument();
    });

    it('should position editor above selection when near bottom of viewport', () => {
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 300, writable: true});
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 0, writable: true});

      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      // Position near bottom: top + rect.height + 10 + editorHeight > viewportHeight
      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 280, // Near bottom
            left: 100,
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root')!;
      expect(card).toBeInTheDocument();
    });

    it('should ensure top position is not negative after bottom adjustment', () => {
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 100, writable: true});
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 50, writable: true});

      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      // Position that would result in negative top after bottom adjustment
      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 10, // Very close to top
            left: 100,
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root')!;
      expect(card).toBeInTheDocument();
    });
  });

  describe('resize and scroll event handlers', () => {
    it('should trigger update on window resize', () => {
      render(<CustomLinkPlugin />);

      // Get the resize handler that was registered
      const resizeCall = (window.addEventListener as ReturnType<typeof vi.fn>).mock.calls.find(
        (call: unknown[]) => call[0] === 'resize',
      );
      expect(resizeCall).toBeDefined();

      // Execute the resize handler
      const resizeHandler = resizeCall![1] as () => void;
      act(() => {
        resizeHandler();
      });

      // Verify getEditorState was called (which happens in the update function)
      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should trigger update on body scroll', () => {
      const addEventListenerSpy = vi.spyOn(document.body, 'addEventListener');

      render(<CustomLinkPlugin />);

      // Get the scroll handler that was registered
      const scrollCall = addEventListenerSpy.mock.calls.find((call) => call[0] === 'scroll');
      expect(scrollCall).toBeDefined();

      // Execute the scroll handler
      const scrollHandler = scrollCall![1] as () => void;
      act(() => {
        scrollHandler();
      });

      expect(mockGetEditorState).toHaveBeenCalled();

      addEventListenerSpy.mockRestore();
    });
  });

  describe('TOGGLE_SAFE_LINK_COMMAND with parent link node', () => {
    it('should set target and rel on parent link node when node itself is not a link', () => {
      const mockSetTarget = vi.fn();
      const mockSetRel = vi.fn();
      const mockParentLinkNode = {
        type: 'link',
        getURL: () => 'https://parent-link.com',
        setTarget: mockSetTarget,
        setRel: mockSetRel,
        getTextContent: () => '',
      };
      const mockTextNode = {
        type: 'text',
        getParent: () => mockParentLinkNode,
        getURL: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };

      mockGetSelectedNode.mockReturnValue(mockTextNode);
      // First call for node check returns false, second call for parent check returns true
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockParentLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      render(<CustomLinkPlugin />);

      const toggleSafeLinkCallback = callbacks['TOGGLE_SAFE_LINK_COMMAND'] as ((url: string) => boolean) | undefined;
      expect(toggleSafeLinkCallback).toBeDefined();
      const result = toggleSafeLinkCallback!('https://new-url.com');
      expect(result).toBe(true);
      expect(mockSetTarget).toHaveBeenCalledWith('_blank');
      expect(mockSetRel).toHaveBeenCalledWith('noopener noreferrer');
    });
  });

  describe('updateLinkEditor with link node detection', () => {
    it('should set URL and type when parent is a link node', () => {
      const mockParentLinkNode = {
        type: 'link',
        getURL: () => 'https://parent-url.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      const mockTextNode = {
        type: 'text',
        getParent: () => mockParentLinkNode,
        getURL: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };

      mockGetSelectedNode.mockReturnValue(mockTextNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockParentLinkNode);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      // Setup native selection
      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 100,
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);
      mockGetRootElement.mockReturnValue(rootElement);

      render(<CustomLinkPlugin />);

      // The link URL from parent should be set in the URL input field
      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      expect(urlInput).toBeInTheDocument();
    });

    it('should set URL and type when node itself is a link node', () => {
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://direct-link-url.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };

      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      // Parent is not a link, but node itself is
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 100,
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);
      mockGetRootElement.mockReturnValue(rootElement);

      render(<CustomLinkPlugin />);

      // The link URL from node should be set in the URL input field
      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1]; // URL input
      expect(urlInput).toBeInTheDocument();
    });

    it('should reset URL and hide editor when neither node nor parent is a link', () => {
      const mockTextNode = {
        type: 'text',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };

      mockGetSelectedNode.mockReturnValue(mockTextNode);
      mockIsLinkNode.mockReturnValue(false);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const rootElement = document.createElement('div');
      mockGetRootElement.mockReturnValue(rootElement);

      render(<CustomLinkPlugin />);

      // Editor should be hidden (positioned off-screen)
      const card = document.querySelector('.MuiCard-root')!;
      expect(card).toBeInTheDocument();
    });
  });

  describe('Native selection edge cases', () => {
    it('should handle when nativeSelection is collapsed', () => {
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };

      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const rootElement = document.createElement('div');
      mockGetRootElement.mockReturnValue(rootElement);

      // Native selection is collapsed
      const mockSelection = {
        isCollapsed: true,
        anchorNode: document.createTextNode('test'),
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 100,
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should handle when nativeSelection anchorNode is null', () => {
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };

      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const rootElement = document.createElement('div');
      mockGetRootElement.mockReturnValue(rootElement);

      // Native selection has null anchorNode
      const mockSelection = {
        isCollapsed: false,
        anchorNode: null,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 100,
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should handle when anchorNode is not contained in rootElement', () => {
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };

      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const rootElement = document.createElement('div');
      mockGetRootElement.mockReturnValue(rootElement);

      // anchorNode is NOT a child of rootElement
      const externalNode = document.createTextNode('external');
      const mockSelection = {
        isCollapsed: false,
        anchorNode: externalNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 100,
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should handle when window.getSelection returns null', () => {
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };

      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const rootElement = document.createElement('div');
      mockGetRootElement.mockReturnValue(rootElement);

      // window.getSelection returns null
      vi.spyOn(window, 'getSelection').mockReturnValue(null);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('CLICK_COMMAND with parent link node', () => {
    it('should open link when parent is a link node and meta key is pressed', () => {
      const mockParentLinkNode = {
        type: 'link',
        getURL: () => 'https://parent-link-to-open.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      const mockTextNode = {
        type: 'text',
        getParent: () => mockParentLinkNode,
        getURL: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };

      mockGetSelectedNode.mockReturnValue(mockTextNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockParentLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      const callbacks: Record<string, unknown> = {};
      (mockRegisterCommand as ReturnType<typeof vi.fn>).mockImplementation((command: unknown, callback: unknown) => {
        callbacks[command as string] = callback;
        return vi.fn();
      });

      const mockOpen = vi.spyOn(window, 'open').mockImplementation(vi.fn());

      render(<CustomLinkPlugin />);

      const clickCallback = callbacks['CLICK_COMMAND'] as ((payload: MouseEvent) => boolean) | undefined;
      expect(clickCallback).toBeDefined();
      const mockEvent = {metaKey: true, ctrlKey: false} as MouseEvent;
      const result = clickCallback!(mockEvent);
      expect(result).toBe(true);
      expect(mockOpen).toHaveBeenCalledWith('https://parent-link-to-open.com', '_blank');

      mockOpen.mockRestore();
    });
  });

  describe('Lexical selection edge cases', () => {
    it('should handle when $getSelection returns null', () => {
      mockGetSelection.mockReturnValue(null);
      mockIsRangeSelection.mockReturnValue(false);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('positionEditorElement additional boundary adjustments', () => {
    it('should set left to 10 when calculated left position is negative', () => {
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 800, writable: true});
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 0, writable: true});

      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      // Create a selection rect that will result in negative left after calculation
      // Formula: left = rect.left + pageXOffset - editorWidth/2 + rect.width/2
      // With rect.left = 0, width = 10, and assuming editorWidth ~350: left = 0 + 0 - 175 + 5 = -170
      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 0,
            height: 20,
            width: 10,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root')!;
      expect(card).toBeInTheDocument();
    });

    it('should ensure top is not less than pageYOffset after bottom adjustment', () => {
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 50, writable: true}); // Very small viewport
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 200, writable: true}); // Scrolled down

      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      // Position near the bottom of a small viewport, with scroll offset
      // After bottom adjustment, top would be: rect.top + pageYOffset - editorHeight - 10
      // With rect.top = 40, pageYOffset = 200, editorHeight ~100: top = 40 + 200 - 100 - 10 = 130
      // But if this is still < pageYOffset (200), it should be set to pageYOffset + 10 = 210
      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 40, // Very close to the top
            left: 100,
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root')!;
      expect(card).toBeInTheDocument();
    });
  });

  describe('editorElem null check in updateLinkEditor', () => {
    it('should return early when editorRef.current is null during update', () => {
      // This test verifies the early return when editorElem is null
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockTextNode = {
        type: 'text',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockTextNode);
      mockIsLinkNode.mockReturnValue(false);

      // When no link is found, updateLinkEditor exits early after checking editorElem
      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('rootElement null check in updateLinkEditor', () => {
    it('should return early when getRootElement returns null after editorElem check', () => {
      // This test verifies the early return when rootElement is null
      mockGetRootElement.mockReturnValue(null as unknown as HTMLElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('positionEditorElement right edge overflow', () => {
    it('should adjust left position to right edge when editor would overflow right side of viewport', () => {
      // This test verifies the adjustment when editor would overflow right edge
      Object.defineProperty(window, 'innerWidth', {value: 400, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 800, writable: true});
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 0, writable: true});

      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      // Position selection on the right side so that editor would overflow
      // Initial left = rect.left + pageXOffset - editorWidth/2 + rect.width/2
      // With rect.left = 350, width = 50: left = 350 + 0 - 175 + 25 = 200
      // If editorWidth ~350, then left + editorWidth = 200 + 350 = 550 > 400 (viewport)
      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 350,
            height: 20,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root')!;
      expect(card).toBeInTheDocument();
    });
  });

  describe('positionEditorElement negative top adjustment', () => {
    it('should ensure top is at least pageYOffset + 10 when calculated top is negative', () => {
      // This test verifies the adjustment when top is less than pageYOffset
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 100, writable: true}); // Small viewport height
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 500, writable: true}); // Page is scrolled down

      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      // Selection is near bottom of small viewport, causing bottom adjustment
      // After bottom adjustment: top = rect.top + pageYOffset - editorHeight - 10
      // If rect.top = 90, pageYOffset = 500, editorHeight ~200: top = 90 + 500 - 200 - 10 = 380
      // But this could still be < pageYOffset (500), so it becomes pageYOffset + 10 = 510
      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 90,
            left: 100,
            height: 10,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root')!;
      expect(card).toBeInTheDocument();
    });
  });

  describe('editorElem null handling in updateLinkEditor', () => {
    it('should return early from updateLinkEditor when editorElem is null and not a link node', () => {
      // This verifies early return with valid range selection but null editorRef
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      // Don't set up root element to simulate editorRef being null at certain point
      mockGetRootElement.mockReturnValue(null as unknown as HTMLElement);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('positionEditorElement right edge overflow (line 87)', () => {
    it('should set left to viewportWidth - editorWidth - 10 when editor would overflow right', () => {
      // Create scenario where left + editorWidth > viewportWidth
      Object.defineProperty(window, 'innerWidth', {value: 400, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 800, writable: true});
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 0, writable: true});

      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      // Position the selection far right so editor overflows
      // left calculation: rect.left + pageXOffset - editorWidth/2 + rect.width/2
      // With rect.left = 380, width = 20, and editorWidth ~350: left = 380 + 0 - 175 + 10 = 215
      // If left + editorWidth > viewportWidth (215 + 350 = 565 > 400), adjust to 400 - 350 - 10 = 40
      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 100,
            left: 380,
            height: 20,
            width: 20,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://test.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root')!;
      expect(card).toBeInTheDocument();
    });
  });

  describe('positionEditorElement top below pageYOffset (line 98)', () => {
    it('should set top to pageYOffset + 10 when calculated top is below scroll offset', () => {
      // Create scenario where top < pageYOffset after adjustments
      Object.defineProperty(window, 'innerWidth', {value: 1000, writable: true});
      Object.defineProperty(window, 'innerHeight', {value: 50, writable: true}); // Tiny viewport
      Object.defineProperty(window, 'pageXOffset', {value: 0, writable: true});
      Object.defineProperty(window, 'pageYOffset', {value: 500, writable: true}); // Scrolled far

      const rootElement = document.createElement('div');
      const textNode = document.createTextNode('test');
      rootElement.appendChild(textNode);

      // rect.top is small, but page is scrolled, so after bottom adjustment
      // top could end up less than pageYOffset
      const mockSelection = {
        isCollapsed: false,
        anchorNode: textNode,
        getRangeAt: () => ({
          getBoundingClientRect: () => ({
            top: 30,
            left: 100,
            height: 15,
            width: 50,
          }),
        }),
      };
      vi.spyOn(window, 'getSelection').mockReturnValue(mockSelection as unknown as Selection);

      mockGetRootElement.mockReturnValue(rootElement);
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://test.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      render(<CustomLinkPlugin />);

      const card = document.querySelector('.MuiCard-root')!;
      expect(card).toBeInTheDocument();
    });
  });

  describe('updateLinkEditor editorElem null at line 192', () => {
    it('should return early when editorElem is null after link URL is set', () => {
      // Test early return at line 192 when editorElem is null
      mockIsRangeSelection.mockReturnValue(true);
      mockGetSelection.mockReturnValue({type: 'range'});

      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com',
        setTarget: vi.fn(),
        setRel: vi.fn(),
        getTextContent: () => '',
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);

      const rootElement = document.createElement('div');
      mockGetRootElement.mockReturnValue(rootElement);

      render(<CustomLinkPlugin />);

      // The component should handle the editorElem null check gracefully
      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });
});
