// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {$isAtNodeEnd} from '@lexical/selection';
import {ElementNode, type PointType, type RangeSelection, TextNode} from 'lexical';

function getSelectedNode(selection: RangeSelection): TextNode | ElementNode {
  const {focus}: {focus: PointType} = selection;
  const anchorNode: TextNode | ElementNode = selection.anchor.getNode();
  const focusNode: TextNode | ElementNode = selection.focus.getNode();

  if (anchorNode === focusNode) {
    return anchorNode;
  }

  const isBackward: boolean = selection.isBackward();

  if (isBackward) {
    return $isAtNodeEnd(focus) ? anchorNode : focusNode;
  }
  return $isAtNodeEnd(focus) ? focusNode : anchorNode;
}

export default getSelectedNode;
