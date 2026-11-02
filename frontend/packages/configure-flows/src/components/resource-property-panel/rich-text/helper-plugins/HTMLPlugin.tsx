// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {$generateHtmlFromNodes, $generateNodesFromDOM} from '@lexical/html';
import {useLexicalComposerContext} from '@lexical/react/LexicalComposerContext';
import {$getRoot, $insertNodes, type EditorState, type LexicalNode, RootNode} from 'lexical';
import {type ReactElement, useCallback, useEffect, useRef} from 'react';
import type {Resource} from '../../../../models/resources';
import {UPDATE_TYPES, type UpdateType} from '../../../../models/rich-text';

/**
 * Props interface for the HTML plugin.
 */
interface HTMLPluginProps {
  /**
   * Listener for changes in the editor state.
   */
  onChange: (value: string) => void;
  /**
   * The resource associated with the rich text editor.
   */
  resource: Resource;
  /**
   * Whether the editor is disabled.
   */
  disabled?: boolean;
}

const PRE_WRAP_STYLE_WITH_CLASS = '" style="white-space: pre-wrap;"';
const PRE_WRAP_STYLE = 'style="white-space: pre-wrap;"';
const TEXT_ALIGN_TYPES: string[] = ['left', 'right', 'center', 'justify'];
const TEXT_ALIGN_PLACEHOLDER = '{{textAlign}}';
const TEXT_ALIGN_STYLE_WITH_CLASS = `" style="text-align: ${TEXT_ALIGN_PLACEHOLDER};"`;
const TEXT_ALIGN_STYLE = `style="text-align: ${TEXT_ALIGN_PLACEHOLDER};"`;
const DIR_LTR_CLASS = '" dir="ltr"';
const DIR_LTR = 'dir="ltr"';
const CLASS_NAME_PLACEHOLDER = '{{className}}';
const ADDITIONAL_CLASSES = `class="${CLASS_NAME_PLACEHOLDER}"`;
const EMPTY_CONTENT = '<p class="rich-text-paragraph"><br></p>';

/** Identifies an anchor across an editor round trip by what the author sees and clicks. */
function anchorIdentity(anchor: HTMLAnchorElement): string {
  return `${anchor.getAttribute('href') ?? ''}|${anchor.textContent?.trim() ?? ''}`;
}

/**
 * Restores the `data-component-ref` and `data-action-ref` sentinels that Lexical drops.
 *
 * Lexical only round-trips the attributes its registered node classes declare, so both
 * sentinels are lost on every export. Both carry behaviour: the renderer hides a link whose
 * component ref is disabled for the application, and dispatches only the anchor whose action
 * ref matches the component's `action.ref`.
 *
 * Anchors are matched on href plus text so a link keeps its own ref when surrounding content is
 * edited, including when another link is inserted ahead of it. Position is the fallback, used
 * only when the identity is ambiguous or the link itself was edited, and then only while the
 * anchor count is unchanged — a shifted index would move the ref onto the wrong link.
 *
 * TODO(#4658): drop once the editor node classes declare these attributes.
 *
 * @param html - The exported HTML.
 * @param sourceLabel - The label the editor was seeded with.
 * @returns The HTML with its sentinels restored.
 */
function restoreActionSentinels(html: string, sourceLabel: string): string {
  if (!sourceLabel.includes('data-component-ref') && !sourceLabel.includes('data-action-ref')) {
    return html;
  }

  const parser: DOMParser = new DOMParser();
  const source: Document = parser.parseFromString(sourceLabel, 'text/html');
  const target: Document = parser.parseFromString(html, 'text/html');

  const componentRef: string | null =
    source.body.querySelector('[data-component-ref]')?.getAttribute('data-component-ref') ?? null;
  const firstBlock: globalThis.Element | null = target.body.firstElementChild;

  if (componentRef && firstBlock && !firstBlock.hasAttribute('data-component-ref')) {
    firstBlock.setAttribute('data-component-ref', componentRef);
  }

  const sourceAnchors: HTMLAnchorElement[] = Array.from(source.body.querySelectorAll('a'));
  const targetAnchors: HTMLAnchorElement[] = Array.from(target.body.querySelectorAll('a'));
  const countsMatch: boolean = sourceAnchors.length === targetAnchors.length;

  // Identities shared by more than one source anchor are mapped to null: neither ref can be
  // attributed with confidence, so those fall through to the positional pass. Anchors carrying
  // no ref are recorded too, otherwise a wired anchor twinned with an unwired one would read as
  // unambiguous and its ref would be copied onto both.
  const refsByIdentity: Map<string, string | null> = new Map<string, string | null>();

  sourceAnchors.forEach((anchor: HTMLAnchorElement) => {
    const attribute: string | null = anchor.getAttribute('data-action-ref');
    const actionRef: string | null = attribute !== null && attribute !== '' ? attribute : null;
    const identity: string = anchorIdentity(anchor);

    refsByIdentity.set(identity, refsByIdentity.has(identity) ? null : actionRef);
  });

  const claimedIdentities: Set<string> = new Set<string>();

  targetAnchors.forEach((anchor: HTMLAnchorElement, index: number) => {
    if (anchor.hasAttribute('data-action-ref')) {
      // An anchor that kept its ref still owns its identity, so a duplicate of it below
      // cannot be handed the same ref.
      if (anchor.getAttribute('data-action-ref')) {
        claimedIdentities.add(anchorIdentity(anchor));
      }

      return;
    }

    const identity: string = anchorIdentity(anchor);
    const identityRef: string | null = claimedIdentities.has(identity) ? null : (refsByIdentity.get(identity) ?? null);

    if (identityRef) {
      anchor.setAttribute('data-action-ref', identityRef);
      claimedIdentities.add(identity);

      return;
    }

    const positionalRef: string | null = countsMatch
      ? (sourceAnchors[index]?.getAttribute('data-action-ref') ?? null)
      : null;

    if (positionalRef) {
      anchor.setAttribute('data-action-ref', positionalRef);
    }
  });

  return target.body.innerHTML;
}

/**
 * Convert nodes tree to HTML string.
 */
function HTMLPlugin({onChange, resource, disabled = false}: HTMLPluginProps): ReactElement | null {
  const [editor] = useLexicalComposerContext();
  const updateType = useRef<UpdateType>(UPDATE_TYPES.NONE);
  // The label the editor currently holds, read at export time to restore the sentinels Lexical
  // drops. Kept in a ref so the update listener does not re-register on every resource identity.
  const sourceLabelRef = useRef<string>('');

  useEffect(() => {
    sourceLabelRef.current = (resource as Resource & {label?: string})?.label ?? '';
  }, [resource]);

  /**
   * Pre-process the HTML string to add additional classes and styles.
   *
   * @param html - The HTML string to pre-process.
   * @returns The pre-processed HTML string.
   */
  const preProcessHTML = useCallback((html: string): string => {
    let processedHtml = html.replaceAll(DIR_LTR_CLASS, '"');
    processedHtml = processedHtml.replaceAll(DIR_LTR, '');

    processedHtml = processedHtml.replaceAll(PRE_WRAP_STYLE_WITH_CLASS, ' rich-text-pre-wrap"');
    processedHtml = processedHtml.replaceAll(
      PRE_WRAP_STYLE,
      ADDITIONAL_CLASSES.replace(CLASS_NAME_PLACEHOLDER, 'rich-text-pre-wrap'),
    );

    TEXT_ALIGN_TYPES.forEach((textAlign) => {
      processedHtml = processedHtml.replaceAll(
        TEXT_ALIGN_STYLE_WITH_CLASS.replace(TEXT_ALIGN_PLACEHOLDER, textAlign),
        ` rich-text-align-${textAlign}"`,
      );
      processedHtml = processedHtml.replaceAll(
        TEXT_ALIGN_STYLE.replace(TEXT_ALIGN_PLACEHOLDER, textAlign),
        ADDITIONAL_CLASSES.replace(CLASS_NAME_PLACEHOLDER, `rich-text-align-${textAlign}`),
      );
    });

    return processedHtml;
  }, []);

  /**
   * Post-process the HTML string to reverse the transformations done by preProcessHTML.
   * This method converts processed HTML back to its original format.
   *
   * @param html - The processed HTML string to reverse.
   * @returns The original HTML string.
   */
  const postProcessHTML = (html: string): string => {
    let processedHtml = html;

    // Restore template URLs that were previously mangled by Lexical's URL formatter
    // (which prepends https:// to URLs without a protocol, e.g. {{meta(...)}} → https://{{meta(...)}}/).
    processedHtml = processedHtml.replace(/href="https?:\/\/(\{\{(?:meta|t)\([^)]+\)\}\})\/?"/g, 'href="$1"');
    // Reverse text alignment class replacements.
    TEXT_ALIGN_TYPES.forEach((textAlign) => {
      processedHtml = processedHtml.replaceAll(
        ` rich-text-align-${textAlign}"`,
        TEXT_ALIGN_STYLE_WITH_CLASS.replace(TEXT_ALIGN_PLACEHOLDER, textAlign),
      );
      processedHtml = processedHtml.replaceAll(
        ADDITIONAL_CLASSES.replace(CLASS_NAME_PLACEHOLDER, `rich-text-align-${textAlign}`),
        TEXT_ALIGN_STYLE.replace(TEXT_ALIGN_PLACEHOLDER, textAlign),
      );
    });

    // Reverse pre-wrap style replacements.
    processedHtml = processedHtml.replaceAll(' rich-text-pre-wrap"', PRE_WRAP_STYLE_WITH_CLASS);
    processedHtml = processedHtml.replaceAll(
      ADDITIONAL_CLASSES.replace(CLASS_NAME_PLACEHOLDER, 'rich-text-pre-wrap'),
      PRE_WRAP_STYLE,
    );

    return processedHtml;
  };

  useEffect(() => {
    if (!editor || !resource) {
      return;
    }

    if (updateType.current === UPDATE_TYPES.INTERNAL) {
      updateType.current = UPDATE_TYPES.NONE;

      return;
    }

    const parser: DOMParser = new DOMParser();
    const labelValue = (resource as Resource & {label?: string})?.label ?? '';
    const dom: Document = parser.parseFromString(postProcessHTML(labelValue), 'text/html');

    editor.update(() => {
      updateType.current = UPDATE_TYPES.EXTERNAL;

      const root: RootNode = $getRoot();

      root.clear(); // clear existing content if needed.

      const nodes: LexicalNode[] = $generateNodesFromDOM(editor, dom);

      $insertNodes(nodes); // insert new nodes into the editor.
    });
  }, [editor, resource]);

  /**
   * Register the update listener to process the editor state changes.
   */
  useEffect(() => {
    if (!editor || !onChange) {
      return undefined;
    }

    return editor.registerUpdateListener(({editorState}: {editorState: EditorState}) => {
      if (updateType.current === UPDATE_TYPES.EXTERNAL) {
        updateType.current = UPDATE_TYPES.NONE;

        return;
      }

      editorState.read(() => {
        updateType.current = UPDATE_TYPES.INTERNAL;

        let htmlString: string = $generateHtmlFromNodes(editor);

        // Lexical's formatUrl prepends https:// to template URLs (e.g. {{meta(...)}}) during
        // HTML serialization. Strip the prefix so the raw template is preserved in storage.
        htmlString = htmlString.replace(/href="https?:\/\/(\{\{(?:meta|t)\([^)]+\)\}\})\/?"/g, 'href="$1"');

        const processedHTML: string = preProcessHTML(htmlString);

        onChange(processedHTML === EMPTY_CONTENT ? '' : restoreActionSentinels(processedHTML, sourceLabelRef.current));
      });
    });
  }, [editor, onChange, preProcessHTML]);

  /**
   * Handle the editor's disabled state.
   */
  useEffect(() => {
    if (disabled) {
      editor.setEditable(false);
    } else if (!editor.isEditable()) {
      editor.setEditable(true);
    }
  }, [disabled, editor]);

  return null;
}

export default HTMLPlugin;
