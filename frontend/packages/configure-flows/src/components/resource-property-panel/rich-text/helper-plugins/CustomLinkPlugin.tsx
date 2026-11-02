// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {$isLinkNode, TOGGLE_LINK_COMMAND} from '@lexical/link';
import {useLexicalComposerContext} from '@lexical/react/LexicalComposerContext';
import {mergeRegister} from '@lexical/utils';
import {Box, Button, Card, IconButton, InputAdornment, TextField, Tooltip} from '@wso2/oxygen-ui';
import {AlignLeft, Link2, SquareFunction} from '@wso2/oxygen-ui-icons-react';
import {
  $getSelection,
  $isRangeSelection,
  $isTextNode,
  CLICK_COMMAND,
  KEY_ESCAPE_COMMAND,
  SELECTION_CHANGE_COMMAND,
} from 'lexical';
import type {CommandListenerPriority, EditorState, ElementNode, BaseSelection, TextNode} from 'lexical';
import {type ChangeEvent, type KeyboardEvent, type ReactElement, useCallback, useEffect, useRef, useState} from 'react';
import {createPortal} from 'react-dom';
import {useTranslation} from 'react-i18next';
import TOGGLE_SAFE_LINK_COMMAND from './commands';
import DynamicValuePopover from '../../DynamicValuePopover';
import getSelectedNode from '../utils/getSelectedNode';

const LowPriority: CommandListenerPriority = 1;
const HighPriority: CommandListenerPriority = 3;

// Predefined URL options
interface PredefinedUrlOption {
  label: string;
  placeholder: string;
  value: string;
}

const PREDEFINED_URLS: PredefinedUrlOption[] = [];

/**
 * Positions the editor element based on the selection rectangle.
 * @param editor - The editor element to position.
 * @param rect - The bounding rectangle of the selection.
 */
const positionEditorElement = (editorElement: HTMLDivElement, rect: DOMRect | null): void => {
  if (rect === null) {
    editorElement.style.opacity = '0';
    editorElement.style.top = '-1000px';
    editorElement.style.left = '-1000px';
  } else {
    editorElement.style.opacity = '1';

    // Get viewport dimensions.
    const viewportWidth: number = window.innerWidth;
    const viewportHeight: number = window.innerHeight;

    // Get editor dimensions.
    const editorWidth: number = editorElement.offsetWidth;
    const editorHeight: number = editorElement.offsetHeight;

    // Calculate initial position (centered below the selection).
    let top: number = rect.top + rect.height + window.pageYOffset + 10;
    let left: number = rect.left + window.pageXOffset - editorWidth / 2 + rect.width / 2;

    // Adjust horizontal position to keep editor within viewport.
    if (left < 0) {
      // If editor would be cut off on the left, align it to the left edge.
      left = 10;
    } else if (left + editorWidth > viewportWidth) {
      // If editor would be cut off on the right, align it to the right edge.
      left = viewportWidth - editorWidth - 10;
    }

    // Adjust vertical position to keep editor within viewport.
    if (top + editorHeight > viewportHeight + window.pageYOffset) {
      // If editor would be cut off at the bottom, position it above the selection.
      top = rect.top + window.pageYOffset - editorHeight - 10;
    }

    // Ensure top position is not negative.
    if (top < window.pageYOffset) {
      top = window.pageYOffset + 10;
    }

    editorElement.style.top = `${top}px`;
    editorElement.style.left = `${left}px`;
  }
};

/**
 * Determines the URL type based on the URL content.
 */
const determineUrlType = (url: string): string => {
  const predefinedUrl: PredefinedUrlOption | undefined = PREDEFINED_URLS.find(
    (option: PredefinedUrlOption) => option.value === url,
  );

  return predefinedUrl ? predefinedUrl.value : 'CUSTOM';
};

/**
 * Gets the placeholder URL for a given URL.
 * @param url - The URL to get the placeholder for.
 * @returns The placeholder URL if found, otherwise an empty string.
 */
const getPlaceholderUrl = (url: string): string => {
  const selectedType: string = determineUrlType(url);

  if (selectedType !== 'CUSTOM') {
    const selectedOption: PredefinedUrlOption | undefined = PREDEFINED_URLS.find(
      (option: PredefinedUrlOption) => option.value === url,
    );

    return selectedOption ? selectedOption.placeholder : '';
  }

  // Template URLs doesn't need the `http(s)://` prefix, so we return the raw URL which may contain the template.
  const templateMatch: RegExpExecArray | null = /(\{\{(?:meta|t)\([^)]+\)\}\})/.exec(url);

  if (templateMatch) {
    return templateMatch[1];
  }

  return url;
};

/**
 * Link editor component for managing links in the rich text editor.
 */
function LinkEditor(): ReactElement {
  const [editor] = useLexicalComposerContext();
  const editorRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [linkUrl, setLinkUrl] = useState('');
  const [linkText, setLinkText] = useState('');
  const [lastSelection, setLastSelection] = useState<BaseSelection | null>(null);
  const [isDynamicValuePopoverOpen, setIsDynamicValuePopoverOpen] = useState<boolean>(false);
  const [dynamicValueBtnEl, setDynamicValueBtnEl] = useState<HTMLButtonElement | null>(null);

  const {t} = useTranslation();

  /**
   * Updates the link editor position and state based on the current selection.
   */
  const updateLinkEditor: () => void = useCallback(() => {
    const selection: BaseSelection | null = $getSelection();
    const editorElem: HTMLDivElement | null = editorRef.current;

    if ($isRangeSelection(selection)) {
      const node: TextNode | ElementNode = getSelectedNode(selection);
      const parent: ElementNode | null = node.getParent();

      if (!parent) {
        return;
      }

      if ($isLinkNode(parent)) {
        const url: string = parent.getURL();

        setLinkUrl(getPlaceholderUrl(url));
        setLinkText(parent.getTextContent());
      } else if ($isLinkNode(node)) {
        const url: string = node.getURL();

        setLinkUrl(getPlaceholderUrl(url));
        setLinkText(node.getTextContent());
      } else {
        setLinkUrl('');
        setLinkText('');
        if (editorElem) {
          positionEditorElement(editorElem, null);
        }

        return;
      }
    }

    const nativeSelection: Selection | null = window.getSelection();
    const {activeElement} = document;

    if (editorElem === null) {
      return;
    }

    const rootElement: HTMLElement | null = editor.getRootElement();

    if (!rootElement) {
      return;
    }

    if (
      selection !== null &&
      nativeSelection !== null &&
      !nativeSelection.isCollapsed &&
      rootElement !== null &&
      nativeSelection.anchorNode &&
      rootElement.contains(nativeSelection.anchorNode)
    ) {
      const domRange: Range = nativeSelection.getRangeAt(0);
      let rect: DOMRect;

      if (nativeSelection.anchorNode === rootElement) {
        let inner: HTMLElement = rootElement;

        while (inner.firstElementChild != null) {
          inner = inner.firstElementChild as HTMLElement;
        }
        rect = inner.getBoundingClientRect();
      } else {
        rect = domRange.getBoundingClientRect();
      }

      positionEditorElement(editorElem, rect);
      setLastSelection(selection);
    } else if (activeElement?.className !== 'link-input') {
      if (rootElement !== null) {
        positionEditorElement(editorElem, null);
      }
      setLastSelection(null);
      setLinkUrl('');
      setLinkText('');
    }
  }, [editor]);

  /**
   * Sets up event listeners for window resize and scroll to update the link editor position.
   */
  useEffect(() => {
    const scrollerElem: HTMLElement = document.body;

    const update = (): void => {
      editor.getEditorState().read(() => {
        updateLinkEditor();
      });
    };

    window.addEventListener('resize', update);
    scrollerElem.addEventListener('scroll', update);

    return () => {
      window.removeEventListener('resize', update);
      scrollerElem.removeEventListener('scroll', update);
    };
  }, [editor, updateLinkEditor]);

  /**
   * Registers commands and listeners for the link editor.
   */
  useEffect(
    () =>
      mergeRegister(
        editor.registerUpdateListener(({editorState}: {editorState: EditorState}) => {
          editorState.read(() => {
            updateLinkEditor();
          });
        }),
        editor.registerCommand(
          SELECTION_CHANGE_COMMAND,
          () => {
            updateLinkEditor();

            return false;
          },
          LowPriority,
        ),
        editor.registerCommand(
          KEY_ESCAPE_COMMAND,
          () => {
            if (editorRef.current) {
              positionEditorElement(editorRef.current, null);
            }
            setLastSelection(null);
            setLinkUrl('');
            setLinkText('');

            return true;
          },
          LowPriority,
        ),
        editor.registerCommand(
          TOGGLE_SAFE_LINK_COMMAND,
          (url: string) => {
            if (url) {
              // First use the default command to handle the link creation/update.
              editor.dispatchCommand(TOGGLE_LINK_COMMAND, url);

              // Then update the link attributes to include safe properties.
              const selection: BaseSelection | null = $getSelection();

              if ($isRangeSelection(selection)) {
                const node: TextNode | ElementNode = getSelectedNode(selection);
                const linkNode: ElementNode | null = $isLinkNode(node) ? node : node.getParent();

                if (!linkNode) {
                  return false;
                }

                if ($isLinkNode(linkNode)) {
                  // Update the link node with safe attributes.
                  linkNode.setTarget('_blank');
                  linkNode.setRel('noopener noreferrer');
                }
              }
            } else {
              // If no URL, remove the link (same as TOGGLE_LINK_COMMAND with null).
              editor.dispatchCommand(TOGGLE_LINK_COMMAND, null);
            }

            return true;
          },
          HighPriority,
        ),
      ),
    [editor, updateLinkEditor],
  );

  /**
   * Updates the link editor position.
   */
  useEffect(() => {
    editor.getEditorState().read(() => {
      updateLinkEditor();
    });
  }, [editor, updateLinkEditor]);

  const handleClose = useCallback(() => {
    if (editorRef.current) {
      positionEditorElement(editorRef.current, null);
    }
    setLastSelection(null);
    setLinkUrl('');
    setLinkText('');
  }, []);

  const handleApply = useCallback(() => {
    if (lastSelection !== null && linkUrl !== '') {
      editor.dispatchCommand(TOGGLE_SAFE_LINK_COMMAND, linkUrl);
      // Update the display text of the link node if it changed.
      if (linkText) {
        editor.update(() => {
          const selection: BaseSelection | null = $getSelection();

          if ($isRangeSelection(selection)) {
            const node: TextNode | ElementNode = getSelectedNode(selection);
            const linkNode: ElementNode | null = $isLinkNode(node) ? node : node.getParent();

            if ($isLinkNode(linkNode)) {
              const firstChild = linkNode.getFirstChild();

              if ($isTextNode(firstChild)) {
                firstChild.setTextContent(linkText);
              }
            }
          }
        });
      }
    }
    handleClose();
  }, [editor, handleClose, lastSelection, linkUrl, linkText]);

  return (
    <Card
      ref={editorRef}
      elevation={0}
      sx={{
        position: 'absolute',
        right: 1.25,
        zIndex: 1200,
        width: 280,
        padding: 1,
        display: 'flex',
        flexDirection: 'column',
        gap: 1,
      }}
    >
      <TextField
        fullWidth
        size="small"
        value={linkText}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
          setLinkText(event.target.value);
        }}
        placeholder={t('flows:core.elements.richText.linkEditor.textPlaceholder')}
        InputProps={{
          startAdornment: (
            <InputAdornment position="start">
              <AlignLeft size={16} />
            </InputAdornment>
          ),
        }}
      />
      <Box display="flex" gap={1} alignItems="center">
        <TextField
          inputRef={inputRef}
          fullWidth
          size="small"
          value={linkUrl}
          onChange={(event: ChangeEvent<HTMLInputElement>) => {
            setLinkUrl(event.target.value);
          }}
          placeholder={t('flows:core.elements.richText.linkEditor.placeholder')}
          onKeyDown={(event: KeyboardEvent<HTMLInputElement>) => {
            if (event.key === 'Enter') {
              event.preventDefault();
              handleApply();
            } else if (event.key === 'Escape') {
              event.preventDefault();
              handleClose();
            }
          }}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <Link2 size={16} />
              </InputAdornment>
            ),
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip title={t('flows:core.elements.textPropertyField.tooltip.configureDynamicValue')}>
                  <IconButton
                    ref={setDynamicValueBtnEl}
                    onClick={() => setIsDynamicValuePopoverOpen((prev) => !prev)}
                    size="small"
                    edge="end"
                  >
                    <SquareFunction size={16} />
                  </IconButton>
                </Tooltip>
              </InputAdornment>
            ),
          }}
        />
        <Button size="small" variant="outlined" onClick={handleApply}>
          {t('flows:core.elements.richText.linkEditor.apply')}
        </Button>
      </Box>
      <DynamicValuePopover
        open={isDynamicValuePopoverOpen}
        anchorEl={dynamicValueBtnEl}
        propertyKey="linkUrl"
        onClose={() => setIsDynamicValuePopoverOpen(false)}
        value={linkUrl}
        onChange={(newValue: string) => setLinkUrl(newValue)}
      />
    </Card>
  );
}

/**
 * Custom link plugin that handles link editing in the rich text editor.
 */
const CustomLinkPlugin = (): ReactElement => {
  const [editor] = useLexicalComposerContext();

  useEffect(
    () =>
      mergeRegister(
        editor.registerCommand(
          CLICK_COMMAND,
          (payload: MouseEvent) => {
            const selection: BaseSelection | null = $getSelection();

            if ($isRangeSelection(selection)) {
              const node: TextNode | ElementNode = getSelectedNode(selection);
              const linkNode: ElementNode | null = $isLinkNode(node) ? node : node.getParent();

              if (!linkNode) {
                return false;
              }

              if ($isLinkNode(linkNode) && (payload.metaKey || payload.ctrlKey)) {
                window.open(linkNode.getURL(), '_blank');

                return true;
              }
            }

            return false;
          },
          LowPriority,
        ),
      ),
    [editor],
  );

  // TODO: Refactor this to use `Popover` from Oxygen UI instead.
  return createPortal(<LinkEditor />, document.body);
};

export default CustomLinkPlugin;
