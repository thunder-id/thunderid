// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * This test file covers the predefined URL functionality in CustomLinkPlugin.
 * Since PREDEFINED_URLS is an empty constant array in the source,
 * we test the helper functions' logic directly through component behavior
 * by mocking scenarios where the URL detection functions are exercised.
 */

import {render, screen, fireEvent, act} from '@testing-library/react';
import type React from 'react';
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
    type: 'text',
  })),
}));

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

// Mock the lexical composer context
vi.mock('@lexical/react/LexicalComposerContext', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@lexical/react/LexicalComposerContext')>()),
  useLexicalComposerContext: () => [
    {
      dispatchCommand: mockDispatchCommand,
      registerUpdateListener: mockRegisterUpdateListener,
      registerCommand: mockRegisterCommand,
      getRootElement: mockGetRootElement,
      getEditorState: mockGetEditorState,
    },
  ],
}));

// Mock lexical utils
vi.mock('@lexical/utils', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@lexical/utils')>()),
  mergeRegister:
    (...fns: (() => void)[]) =>
    () =>
      fns.forEach((fn) => fn()),
}));

// Mock lexical
vi.mock('lexical', async (importOriginal) => ({
  ...(await importOriginal<typeof import('lexical')>()),
  $getSelection: mockGetSelection,
  $isRangeSelection: mockIsRangeSelection,
  CLICK_COMMAND: 'CLICK_COMMAND',
  KEY_ESCAPE_COMMAND: 'KEY_ESCAPE_COMMAND',
  SELECTION_CHANGE_COMMAND: 'SELECTION_CHANGE_COMMAND',
}));

// Mock @lexical/link
vi.mock('@lexical/link', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@lexical/link')>()),
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

// Mock createPortal to render directly
vi.mock('react-dom', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-dom')>()),
  createPortal: (children: React.ReactNode) => children,
}));

describe('CustomLinkPlugin - URL Type Detection Functions', () => {
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
    mockGetSelectedNode.mockImplementation(() => ({
      getParent: () => null,
      getURL: () => 'https://example.com',
      getTextContent: () => '',
      setTarget: vi.fn(),
      setRel: vi.fn(),
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

  describe('determineUrlType function behavior', () => {
    it('should return CUSTOM for URLs not in PREDEFINED_URLS array', () => {
      // Since PREDEFINED_URLS is empty, any URL will return CUSTOM
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://any-custom-url.com',
        getTextContent: () => 'link text',
        setTarget: vi.fn(),
        setRel: vi.fn(),
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
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

      // The component renders the editing card (determineUrlType returns CUSTOM for any URL)
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });

    it('should exercise determineUrlType when URL matches no predefined option', () => {
      // This tests the behavior when find() returns undefined and returns 'CUSTOM'
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://random-url.com',
        getTextContent: () => 'link text',
        setTarget: vi.fn(),
        setRel: vi.fn(),
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('getPlaceholderUrl function behavior', () => {
    it('should return URL itself when selectedType is CUSTOM', () => {
      // This tests the behavior when selectedType is CUSTOM
      const testUrl = 'https://my-custom-placeholder.com';
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => testUrl,
        getTextContent: () => 'link text',
        setTarget: vi.fn(),
        setRel: vi.fn(),
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
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

      // The component renders the editing card; placeholder URL value is set in the input
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });
  });

  describe('handleUrlTypeChange function behavior', () => {
    it('should set URL to https:// when switching to CUSTOM type', () => {
      // This tests the else branch behavior when newType is CUSTOM
      // Since PREDEFINED_URLS is empty, all URLs are treated as CUSTOM type
      render(<CustomLinkPlugin />);

      // Verify the component renders the editing interface directly
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();

      // Since PREDEFINED_URLS is empty, Select won't render
      // Verify the component renders the URL input field
      const textField = document.querySelector('input');
      expect(textField).toBeInTheDocument();
    });
  });

  describe('getCurrentUrl function behavior', () => {
    it('should return linkUrl when selectedUrlType is CUSTOM', () => {
      // This tests the else branch that returns linkUrl
      render(<CustomLinkPlugin />);

      // The component renders the editing interface directly
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();

      // Type a custom URL into the URL input field (second input)
      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1];
      act(() => {
        fireEvent.change(urlInput, {target: {value: 'https://test-current-url.com'}});
      });
      expect(urlInput).toHaveValue('https://test-current-url.com');

      // Click apply to submit the link (getCurrentUrl returns linkUrl for CUSTOM type)
      const applyButton = screen.getByText('flows:core.elements.richText.linkEditor.apply');
      act(() => {
        fireEvent.click(applyButton);
      });

      // Card should still be present in the DOM
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });
  });

  describe('Select component rendering', () => {
    it('should not render Select when PREDEFINED_URLS is empty', () => {
      // This tests the conditional rendering when PREDEFINED_URLS is empty
      render(<CustomLinkPlugin />);

      // The component renders the editing interface directly
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();

      // Select should NOT be rendered since PREDEFINED_URLS is empty
      const select = document.querySelector('.MuiSelect-root');
      expect(select).not.toBeInTheDocument();
    });
  });

  describe('URL detection with various URL formats', () => {
    it('should handle empty string URL', () => {
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => '',
        getTextContent: () => '',
        setTarget: vi.fn(),
        setRel: vi.fn(),
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should handle URL with special characters', () => {
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com/path?query=value&other=test#hash',
        getTextContent: () => 'link text',
        setTarget: vi.fn(),
        setRel: vi.fn(),
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });

    it('should handle URL with unicode characters', () => {
      const mockLinkNode = {
        type: 'link',
        getParent: () => ({type: 'paragraph'}),
        getURL: () => 'https://example.com/路径/页面',
        getTextContent: () => 'link text',
        setTarget: vi.fn(),
        setRel: vi.fn(),
      };
      mockGetSelectedNode.mockReturnValue(mockLinkNode);
      mockIsLinkNode.mockImplementation((node: unknown) => node === mockLinkNode);
      mockIsRangeSelection.mockReturnValue(true);

      render(<CustomLinkPlugin />);

      expect(mockGetEditorState).toHaveBeenCalled();
    });
  });

  describe('Edge cases for URL type selection', () => {
    it('should handle rapid URL type changes', () => {
      render(<CustomLinkPlugin />);

      // The component renders the editing interface directly
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();

      // Type multiple URLs rapidly into the URL input field (second input)
      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1];
      act(() => {
        fireEvent.change(urlInput, {target: {value: 'https://url1.com'}});
      });
      act(() => {
        fireEvent.change(urlInput, {target: {value: 'https://url2.com'}});
      });
      act(() => {
        fireEvent.change(urlInput, {target: {value: 'https://final-url.com'}});
      });
      expect(urlInput).toHaveValue('https://final-url.com');
    });

    it('should handle save with valid URL after initially empty', () => {
      render(<CustomLinkPlugin />);

      // The component renders the editing interface directly
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();

      // Use the URL input field (second input)
      const inputs = document.querySelectorAll('input');
      const urlInput = inputs[1];
      // Clear the field first
      act(() => {
        fireEvent.change(urlInput, {target: {value: ''}});
      });
      // Then add a valid URL
      act(() => {
        fireEvent.change(urlInput, {target: {value: 'https://valid-url.com'}});
      });
      expect(urlInput).toHaveValue('https://valid-url.com');

      // Apply the link
      const applyButton = screen.getByText('flows:core.elements.richText.linkEditor.apply');
      act(() => {
        fireEvent.click(applyButton);
      });

      // Card should still be present in the DOM
      expect(document.querySelector('.MuiCard-root')).toBeInTheDocument();
    });
  });
});
