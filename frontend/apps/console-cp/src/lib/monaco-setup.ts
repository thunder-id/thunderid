// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {loader} from '@monaco-editor/react';
import * as monaco from 'monaco-editor';
// eslint-disable-next-line import-x/default
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker';
// eslint-disable-next-line import-x/default
import cssWorker from 'monaco-editor/esm/vs/language/css/css.worker?worker';
// eslint-disable-next-line import-x/default
import jsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker';

self.MonacoEnvironment = {
  getWorker(_, label) {
    if (label === 'json') return new jsonWorker();
    if (label === 'css' || label === 'scss' || label === 'less') return new cssWorker();
    return new editorWorker();
  },
};

loader.config({monaco});

// Monaco's native EditContext input layer ships its own stylesheet
// (nativeEditContext.css), but it is not pulled into the bundle through the main
// monaco entry here. Without it, `.native-edit-context` and `.ime-text-area` stay
// `position: static`, so they take up flow space at the top of the editor and push
// the text layer down. Monaco then bakes that offset into the coordinates it uses
// for click-to-cursor mapping, so clicks land on the wrong line and values cannot be
// edited by placing the cursor (issue #4569). Inject the rules at runtime so they are
// guaranteed present before any editor initializes, independent of CSS bundling.
const NATIVE_EDIT_CONTEXT_STYLE_ID = 'monaco-native-edit-context-fix';
if (typeof document !== 'undefined' && !document.getElementById(NATIVE_EDIT_CONTEXT_STYLE_ID)) {
  const style = document.createElement('style');
  style.id = NATIVE_EDIT_CONTEXT_STYLE_ID;
  style.textContent = `
.monaco-editor .native-edit-context {
  margin: 0;
  padding: 0;
  position: absolute;
  overflow-y: scroll;
  scrollbar-width: none;
  z-index: -10;
  white-space: pre-wrap;
}
.monaco-editor .ime-text-area {
  min-width: 0;
  min-height: 0;
  margin: 0;
  padding: 0;
  position: absolute;
  outline: none !important;
  resize: none;
  border: none;
  overflow: hidden;
  color: transparent;
  background-color: transparent;
  z-index: -10;
}`;
  document.head.appendChild(style);
}
