// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {$isLinkNode, TOGGLE_LINK_COMMAND} from '@lexical/link';
import {useLexicalComposerContext} from '@lexical/react/LexicalComposerContext';
import {$createHeadingNode, $isHeadingNode, type HeadingTagType} from '@lexical/rich-text';
import {$setBlocksType} from '@lexical/selection';
import {mergeRegister} from '@lexical/utils';

import {Button, Divider, IconButton, ListItemText, Menu, MenuItem, Paper, Stack} from '@wso2/oxygen-ui';
import {
  Bold,
  ChevronDownIcon,
  Italic,
  LinkIcon,
  Redo2,
  TextAlignCenter,
  TextAlignEnd,
  TextAlignJustify,
  TextAlignStart,
  Underline,
  Undo2,
} from '@wso2/oxygen-ui-icons-react';
import classNames from 'classnames';
import {
  $createParagraphNode,
  $getSelection,
  $isRangeSelection,
  CAN_REDO_COMMAND,
  CAN_UNDO_COMMAND,
  FORMAT_ELEMENT_COMMAND,
  FORMAT_TEXT_COMMAND,
  REDO_COMMAND,
  SELECTION_CHANGE_COMMAND,
  UNDO_COMMAND,
} from 'lexical';
import type {CommandListenerPriority, EditorState, ElementNode, BaseSelection, NodeKey, TextNode} from 'lexical';
import React, {type HTMLAttributes, type ReactElement, useCallback, useEffect, useState} from 'react';
import getSelectedNode from '../utils/getSelectedNode';

const LowPriority: CommandListenerPriority = 1;

type BlockType = 'paragraph' | 'h1' | 'h2' | 'h3' | 'h4' | 'h5';

const blockTypeToBlockName: Record<BlockType, string> = {
  h1: 'Heading 1',
  h2: 'Heading 2',
  h3: 'Heading 3',
  h4: 'Heading 4',
  h5: 'Heading 5',
  paragraph: 'Paragraph',
};

/**
 * Props interface for the ToolbarPlugin component.
 *
 * The toolbar is designed with a responsive multi-row layout:
 * - Primary row: History controls, text formatting (bold, italic, underline), typography dropdown, and overflow menu
 * - Secondary row: Alignment controls (left, center, right, justify)
 * - Overflow menu: Less frequently used features like link insertion
 *
 * This layout ensures optimal usability in narrow side panels while maintaining access to all features.
 */
export interface ToolbarPluginProps extends HTMLAttributes<HTMLDivElement> {
  history?: boolean;
  bold?: boolean;
  italic?: boolean;
  underline?: boolean;
  alignment?: boolean;
  typography?: boolean;
  link?: boolean;
  disabled?: boolean;
}

/**
 * ToolbarPlugin component for rich text editor toolbar.
 */
function ToolbarPlugin({
  history = true,
  bold = true,
  italic = true,
  underline = true,
  alignment = true,
  typography = true,
  link = true,
  disabled = false,
  className,
}: ToolbarPluginProps): ReactElement {
  const [editor] = useLexicalComposerContext();

  const [canUndo, setCanUndo] = useState(false);
  const [canRedo, setCanRedo] = useState(false);
  const [isBold, setIsBold] = useState(false);
  const [isItalic, setIsItalic] = useState(false);
  const [isUnderline, setIsUnderline] = useState(false);
  const [blockType, setBlockType] = useState<BlockType>('paragraph');
  const [isLink, setIsLink] = useState(false);
  const [typographyMenu, setTypographyMenu] = useState<null | HTMLElement>(null);
  const [selectedAlignment, setSelectedAlignment] = useState<number>(1);

  const openTypographyMenu = Boolean(typographyMenu);

  const handleTypographyMenuOpen = (event: React.MouseEvent<HTMLElement>): void => {
    setTypographyMenu(event.currentTarget);
  };

  const handleTypographyMenuClose = (): void => {
    setTypographyMenu(null);
  };

  /**
   * Formats the selected text to a paragraph block type.
   * If the current block type is not a paragraph, it changes it to a paragraph.
   * Closes the typography menu after formatting.
   */
  const formatParagraph = (): void => {
    editor.update(() => {
      const selection = $getSelection();

      if ($isRangeSelection(selection) && blockType !== 'paragraph') {
        $setBlocksType(selection, () => $createParagraphNode());
      }
    });
    handleTypographyMenuClose();
  };

  /**
   * Formats the selected text to a heading block type.
   * If the current block type is not the specified heading size, it changes it to the specified heading size.
   * Closes the typography menu after formatting.
   *
   * @param headingSize - The heading size to format the selected text to.
   */
  const formatHeading = (headingSize: 'h1' | 'h2' | 'h3' | 'h4' | 'h5'): void => {
    if (blockType !== headingSize) {
      editor.update(() => {
        const selection: BaseSelection | null = $getSelection();

        $setBlocksType(selection, () => $createHeadingNode(headingSize));
      });
    }
    handleTypographyMenuClose();
  };

  /**
   * Inserts or removes a link in the selected text.
   */
  const insertLink: () => void = useCallback(() => {
    if (!isLink) {
      editor.dispatchCommand(TOGGLE_LINK_COMMAND, 'https://');
    } else {
      editor.dispatchCommand(TOGGLE_LINK_COMMAND, null);
    }
  }, [editor, isLink]);

  /**
   * Updates the toolbar state based on the current selection.
   */
  const $updateToolbar: () => void = useCallback(() => {
    const selection: BaseSelection | null = $getSelection();

    if ($isRangeSelection(selection)) {
      // Update text format.
      setIsBold(selection.hasFormat('bold'));
      setIsItalic(selection.hasFormat('italic'));
      setIsUnderline(selection.hasFormat('underline'));

      // Update link.
      const node: TextNode | ElementNode = getSelectedNode(selection);
      const parent: ElementNode | null = node.getParent();

      if ($isLinkNode(parent) || $isLinkNode(node)) {
        setIsLink(true);
      } else {
        setIsLink(false);
      }

      const anchorNode: TextNode | ElementNode = selection.anchor.getNode();
      const element: TextNode | ElementNode =
        anchorNode.getKey() === 'root' ? anchorNode : anchorNode.getTopLevelElementOrThrow();

      // Update text alignment.
      setSelectedAlignment(element.getFormat());

      const elementKey: NodeKey = element.getKey();
      const elementDOM: HTMLElement | null = editor.getElementByKey(elementKey);

      // Update block type
      if (elementDOM !== null) {
        if ($isHeadingNode(element)) {
          const tag: HeadingTagType = element.getTag();

          setBlockType(tag as BlockType);
        } else {
          const type: string = element.getType();

          if (type in blockTypeToBlockName) {
            setBlockType(type as BlockType);
          } else {
            setBlockType('paragraph');
          }
        }
      }
    }
  }, [editor]);

  useEffect(
    () =>
      mergeRegister(
        editor.registerUpdateListener(({editorState}: {editorState: EditorState}) => {
          editorState.read(() => {
            $updateToolbar();
          });
        }),
        editor.registerCommand(
          SELECTION_CHANGE_COMMAND,
          () => {
            $updateToolbar();

            return false;
          },
          LowPriority,
        ),
        editor.registerCommand(
          CAN_UNDO_COMMAND,
          (payload: boolean) => {
            setCanUndo(payload);

            return false;
          },
          LowPriority,
        ),
        editor.registerCommand(
          CAN_REDO_COMMAND,
          (payload: boolean) => {
            setCanRedo(payload);

            return false;
          },
          LowPriority,
        ),
      ),
    [editor, $updateToolbar],
  );

  return (
    <Paper className={className} variant="outlined" sx={{boxShadow: 'none !important', marginBottom: 2}}>
      <Stack direction="row" alignItems="center" justifyContent="space-evenly">
        {history && (
          <>
            <IconButton
              disabled={!canUndo || disabled}
              onClick={() => {
                editor.dispatchCommand(UNDO_COMMAND, undefined);
              }}
              aria-label="Undo"
            >
              <Undo2 size={14} />
            </IconButton>
            <IconButton
              disabled={!canRedo || disabled}
              onClick={() => {
                editor.dispatchCommand(REDO_COMMAND, undefined);
              }}
              aria-label="Redo"
            >
              <Redo2 size={14} />
            </IconButton>
          </>
        )}
        <Divider orientation="vertical" flexItem />
        {typography && (
          <>
            <Button
              disableRipple
              variant="text"
              color="secondary"
              disabled={disabled}
              onClick={handleTypographyMenuOpen}
              endIcon={<ChevronDownIcon />}
            >
              {blockTypeToBlockName[blockType]}
            </Button>
            <Menu
              open={openTypographyMenu}
              anchorEl={typographyMenu}
              onClose={handleTypographyMenuClose}
              anchorOrigin={{horizontal: 'left', vertical: 'bottom'}}
              transformOrigin={{horizontal: 'left', vertical: 'top'}}
            >
              <MenuItem onClick={() => formatHeading('h1')}>
                <ListItemText primary={<h1>{blockTypeToBlockName.h1}</h1>} />
              </MenuItem>
              <MenuItem onClick={() => formatHeading('h2')}>
                <ListItemText primary={<h2>{blockTypeToBlockName.h2}</h2>} />
              </MenuItem>
              <MenuItem onClick={() => formatHeading('h3')}>
                <ListItemText primary={<h3>{blockTypeToBlockName.h3}</h3>} />
              </MenuItem>
              <MenuItem onClick={() => formatHeading('h4')}>
                <ListItemText primary={<h4>{blockTypeToBlockName.h4}</h4>} />
              </MenuItem>
              <MenuItem onClick={() => formatHeading('h5')}>
                <ListItemText primary={<h5>{blockTypeToBlockName.h5}</h5>} />
              </MenuItem>
              <MenuItem onClick={formatParagraph}>
                <ListItemText primary={blockTypeToBlockName.paragraph} />
              </MenuItem>
            </Menu>
          </>
        )}
      </Stack>
      <Divider />
      <Stack direction="row" alignItems="center" justifyContent="space-evenly" margin={1}>
        {bold && (
          <IconButton
            disabled={disabled}
            onClick={() => {
              editor.dispatchCommand(FORMAT_TEXT_COMMAND, 'bold');
            }}
            className={classNames({active: isBold && !disabled})}
            aria-label="Format Bold"
          >
            <Bold size={14} />
          </IconButton>
        )}
        {italic && (
          <IconButton
            disabled={disabled}
            onClick={() => {
              editor.dispatchCommand(FORMAT_TEXT_COMMAND, 'italic');
            }}
            className={classNames({active: isItalic && !disabled})}
            aria-label="Format Italics"
          >
            <Italic size={14} />
          </IconButton>
        )}
        {underline && (
          <IconButton
            disabled={disabled}
            onClick={() => {
              editor.dispatchCommand(FORMAT_TEXT_COMMAND, 'underline');
            }}
            className={classNames({active: isUnderline && !disabled})}
            aria-label="Format Underline"
          >
            <Underline size={14} />
          </IconButton>
        )}
        {link && (
          <IconButton
            disabled={disabled}
            onClick={() => insertLink()}
            className={classNames({active: isLink && !disabled})}
            aria-label="Format Link"
          >
            <LinkIcon size={14} />
          </IconButton>
        )}
        {alignment && (
          <>
            <IconButton
              disabled={disabled}
              onClick={() => {
                editor.dispatchCommand(FORMAT_ELEMENT_COMMAND, 'left');
              }}
              className={classNames({active: selectedAlignment === 1 && !disabled})}
              aria-label="Left Align"
            >
              <TextAlignStart size={14} />
            </IconButton>
            <IconButton
              disabled={disabled}
              onClick={() => {
                editor.dispatchCommand(FORMAT_ELEMENT_COMMAND, 'center');
              }}
              className={classNames({active: selectedAlignment === 2 && !disabled})}
              aria-label="Center Align"
            >
              <TextAlignCenter size={14} />
            </IconButton>
            <IconButton
              disabled={disabled}
              onClick={() => {
                editor.dispatchCommand(FORMAT_ELEMENT_COMMAND, 'right');
              }}
              className={classNames({active: selectedAlignment === 3 && !disabled})}
              aria-label="Right Align"
            >
              <TextAlignEnd size={14} />
            </IconButton>
            <IconButton
              disabled={disabled}
              onClick={() => {
                editor.dispatchCommand(FORMAT_ELEMENT_COMMAND, 'justify');
              }}
              className={classNames({active: selectedAlignment === 4 && !disabled})}
              aria-label="Justify Align"
            >
              <TextAlignJustify size={14} />
            </IconButton>
          </>
        )}
      </Stack>
    </Paper>
  );
}

export default ToolbarPlugin;
