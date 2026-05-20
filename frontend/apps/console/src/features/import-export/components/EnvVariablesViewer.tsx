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

import Editor from '@monaco-editor/react';
import {Alert, Box, Button, Collapse, IconButton, Paper, Stack, Typography, useColorScheme} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronUp, FileCode, FileDown} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link EnvVariablesViewer} component.
 *
 * @public
 */
export interface EnvVariablesViewerProps {
  /**
   * Environment variables content
   */
  content: string;
  /**
   * Enable download button
   */
  showDownload?: boolean;
  /**
   * Maximum height for content area
   */
  maxHeight?: number;
  /**
   * Enable editing of environment variables
   */
  editable?: boolean;
  /**
   * Callback when content changes (only used when editable=true)
   */
  onChange?: (newContent: string) => void;
  /**
   * File name used for the download
   */
  fileName: string;
}

/**
 * Shared environment variables viewer component.
 * Displays .env file content with consistent styling.
 * Optionally supports editing mode for fixing missing environment values during import.
 *
 * @public
 */
export default function EnvVariablesViewer({
  content,
  showDownload = true,
  maxHeight = 400,
  editable = false,
  onChange = undefined,
  fileName,
}: EnvVariablesViewerProps): JSX.Element {
  const {t} = useTranslation('importExport');
  const [expanded, setExpanded] = useState(false);
  const {mode, systemMode} = useColorScheme();
  const colorMode: 'light' | 'dark' = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';

  const [localContent, setLocalContent] = useState(content);
  const [hasChanges, setHasChanges] = useState(false);
  const shouldPropagateRef = useRef(false);

  // Check if there are any placeholder values
  const hasPlaceholders = content.includes('_placeholder');
  const variableCount = content.split(/\r?\n|\r/).filter((line) => line.trim() && !line.trim().startsWith('#')).length;

  const handleEditorChange = (value: string | undefined): void => {
    const newContent = value ?? '';
    setLocalContent(newContent);
    setHasChanges(newContent !== content);
    shouldPropagateRef.current = true;
  };

  useEffect(() => {
    if (!editable || !onChange || !shouldPropagateRef.current) {
      return;
    }

    const timer = window.setTimeout(() => {
      onChange(localContent);
    }, 200);

    return () => window.clearTimeout(timer);
  }, [editable, localContent, onChange]);

  const handleDownload = async (): Promise<void> => {
    const contentToDownload = editable ? localContent : content;
    try {
      // Try to use File System Access API for "Save As" dialog
      if ('showSaveFilePicker' in window) {
        const handle = await (
          window as Window & {
            showSaveFilePicker: (options?: {
              suggestedName?: string;
              types?: {description: string; accept: Record<string, string[]>}[];
            }) => Promise<{
              createWritable: () => Promise<{write: (data: string) => Promise<void>; close: () => Promise<void>}>;
            }>;
          }
        ).showSaveFilePicker({
          suggestedName: fileName,
          types: [
            {
              description: 'Environment Variables',
              accept: {'text/plain': ['.env']},
            },
          ],
        });
        const writable = await handle.createWritable();
        await writable.write(contentToDownload);
        await writable.close();
      } else {
        // Fallback to traditional download
        const blob = new Blob([contentToDownload], {type: 'text/plain;charset=utf-8'});
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = fileName;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);
      }
    } catch {
      // User cancelled or error occurred, ignore silently
    }
  };

  return (
    <Paper variant="outlined" sx={{p: 3, borderRadius: 2}}>
      <Stack>
        <Stack direction="row" alignItems="center" justifyContent="space-between">
          <Stack direction="row" alignItems="center" spacing={1.5}>
            <Box
              sx={{
                width: 32,
                height: 32,
                borderRadius: 1,
                bgcolor: 'warning.lighter',
                color: 'warning.main',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <FileCode size={18} />
            </Box>
            <Box>
              <Typography variant="h6" fontWeight={600}>
                {t('envViewer.title')}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {t('envViewer.variableCount', {count: variableCount})}
                {hasChanges && editable ? ` ${t('envViewer.modified')}` : ''}
              </Typography>
            </Box>
          </Stack>
          <Stack direction="row" spacing={1}>
            {showDownload && (
              <Button
                variant="outlined"
                size="small"
                startIcon={<FileDown size={16} />}
                onClick={() => void handleDownload()}
              >
                {t('envViewer.download')}
              </Button>
            )}
            <IconButton size="small" onClick={() => setExpanded(!expanded)}>
              {expanded ? <ChevronUp size={20} /> : <ChevronDown size={20} />}
            </IconButton>
          </Stack>
        </Stack>
        <Collapse in={expanded} sx={{mt: expanded ? 2 : 0}}>
          {editable && hasPlaceholders && (
            <Alert severity="warning" sx={{mb: 2}}>
              {t('envViewer.placeholderWarning')}
            </Alert>
          )}
          <Box
            sx={{
              borderRadius: 1,
              maxHeight,
              overflow: 'hidden',
              border: '1px solid',
              borderColor: editable && hasPlaceholders ? 'warning.main' : 'divider',
            }}
          >
            <Editor
              height={maxHeight}
              language="shell"
              theme={colorMode === 'dark' ? 'vs-dark' : 'vs'}
              value={localContent}
              onChange={handleEditorChange}
              options={{
                readOnly: !editable,
                minimap: {enabled: false},
                scrollBeyondLastLine: false,
                automaticLayout: true,
                fontSize: 12,
                tabSize: 2,
                wordWrap: 'on',
                lineNumbers: 'on',
                folding: false,
                renderLineHighlight: editable ? 'line' : 'none',
                contextmenu: editable,
              }}
            />
          </Box>
        </Collapse>
      </Stack>
    </Paper>
  );
}
