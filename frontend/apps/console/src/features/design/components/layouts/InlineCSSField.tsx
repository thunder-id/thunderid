/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {Box, Dialog, DialogContent, DialogTitle, IconButton, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Maximize, X} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import Editor from '@/lib/MonacoEditor';

// Shared DOM node for Monaco overflow widgets (context menu, suggest, etc.)
// Appended to <body> so they are never clipped by parent overflow.
let sharedOverflowNode: HTMLDivElement | null = null;
function getOverflowWidgetsDomNode(): HTMLDivElement {
  if (!sharedOverflowNode) {
    sharedOverflowNode = document.createElement('div');
    sharedOverflowNode.className = 'monaco-editor';
    sharedOverflowNode.style.zIndex = '9999';
    document.body.appendChild(sharedOverflowNode);
  }
  return sharedOverflowNode;
}

interface InlineCSSFieldProps {
  id: string;
  content: string;
  colorMode: 'light' | 'dark';
  onChange: (content: string) => void;
  /** Called on mount/update so the parent can flush this field's pending debounce. */
  registerFlush: (flush: (() => void) | null) => void;
}

const EDITOR_OPTIONS = {
  minimap: {enabled: false},
  scrollBeyondLastLine: false,
  automaticLayout: true,
  fontSize: 12,
  lineHeight: 18,
  tabSize: 2,
  wordWrap: 'on' as const,
  folding: false,
  fixedOverflowWidgets: true,
  overflowWidgetsDomNode: getOverflowWidgetsDomNode(),
  scrollbar: {verticalScrollbarSize: 2, horizontalScrollbarSize: 2, useShadows: false},
  overviewRulerLanes: 0,
  hideCursorInOverviewRuler: true,
  overviewRulerBorder: false,
  renderLineHighlight: 'none' as const,
  padding: {top: 4, bottom: 4},
  quickSuggestions: true,
  suggestOnTriggerCharacters: true,
  suggest: {showProperties: true, showValues: true, showColors: true, showKeywords: true},
};

function InlineCSSField({id, content, colorMode, onChange, registerFlush}: InlineCSSFieldProps): JSX.Element {
  const {t} = useTranslation('design');
  const [localContent, setLocalContent] = useState(content);
  const [expanded, setExpanded] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const localContentRef = useRef(localContent);
  const onChangeRef = useRef(onChange);

  useEffect(() => {
    localContentRef.current = localContent;
    onChangeRef.current = onChange;
  });

  const [prevContent, setPrevContent] = useState(content);
  if (prevContent !== content) {
    setPrevContent(content);
    setLocalContent(content);
  }

  // Register a flush callback so the parent can synchronously commit pending edits.
  const flush = useCallback(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
      onChangeRef.current(localContentRef.current);
    }
  }, []);

  useEffect(() => {
    registerFlush(flush);
    return () => registerFlush(null);
  }, [registerFlush, flush]);

  // Cleanup debounce on unmount
  useEffect(
    () => () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    },
    [],
  );

  const handleEditorChange = (raw: string | undefined): void => {
    const text = raw ?? '';
    setLocalContent(text);
    localContentRef.current = text;
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => onChange(text), 400);
  };

  return (
    <>
      <Box sx={{border: '1px solid', borderColor: 'divider', borderRadius: 1}}>
        <Editor
          height="220px"
          language="css"
          theme={colorMode === 'dark' ? 'vs-dark' : 'vs'}
          value={localContent}
          onChange={handleEditorChange}
          options={{
            ...EDITOR_OPTIONS,
            lineNumbers: 'on',
            glyphMargin: false,
            lineDecorationsWidth: 4,
            lineNumbersMinChars: 3,
          }}
        />
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="flex-end"
          sx={{
            borderTop: '1px solid',
            borderColor: 'divider',
            px: 0.5,
            py: 0.25,
            bgcolor: colorMode === 'dark' ? 'grey.900' : 'grey.50',
          }}
        >
          <Tooltip title={t('layouts.config.custom_css.actions.open_full_editor.tooltip', 'Open in full editor')}>
            <IconButton
              size="small"
              onClick={() => setExpanded(true)}
              sx={{p: 0.25}}
              aria-label={t('layouts.config.custom_css.actions.open_full_editor.tooltip', 'Open in full editor')}
            >
              <Maximize size={14} />
            </IconButton>
          </Tooltip>
        </Stack>
      </Box>

      <Dialog
        open={expanded}
        onClose={() => setExpanded(false)}
        maxWidth="md"
        fullWidth
        slotProps={{paper: {sx: {height: '80vh'}}}}
      >
        <DialogTitle sx={{display: 'flex', alignItems: 'center', py: 1, px: 2}}>
          <Typography component="span" variant="subtitle2" sx={{flex: 1, fontFamily: 'monospace'}}>
            {id}
          </Typography>
          <IconButton size="small" onClick={() => setExpanded(false)} aria-label={t('common.actions.close', 'Close')}>
            <X size={16} />
          </IconButton>
        </DialogTitle>
        <DialogContent sx={{p: 0}}>
          <Editor
            height="100%"
            language="css"
            theme={colorMode === 'dark' ? 'vs-dark' : 'vs'}
            value={localContent}
            onChange={handleEditorChange}
            options={{
              ...EDITOR_OPTIONS,
              lineNumbers: 'on',
              fixedOverflowWidgets: false,
              overflowWidgetsDomNode: undefined,
            }}
          />
        </DialogContent>
      </Dialog>
    </>
  );
}

export default InlineCSSField;
