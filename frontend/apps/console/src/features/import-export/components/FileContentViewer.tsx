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

import {Box, Button, Collapse, IconButton, Paper, Stack, Typography, useColorScheme} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronUp, FileDown} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import Editor from '@/lib/MonacoEditor';

/**
 * Props for the {@link FileContentViewer} component.
 *
 * @public
 */
export interface FileContentViewerProps {
  /**
   * File content to display
   */
  content: string;
  /**
   * File name for display and download
   */
  fileName: string;
  /**
   * Title for the section
   */
  title: string;
  /**
   * Subtitle/description
   */
  subtitle?: string | null;
  /**
   * Icon to display
   */
  icon?: ReactNode;
  /**
   * Icon background color
   */
  iconBgColor?: string;
  /**
   * Icon color
   */
  iconColor?: string;
  /**
   * Enable download button
   */
  showDownload?: boolean;
  /**
   * Maximum height for content area
   */
  maxHeight?: number;
}

/**
 * Shared file content viewer component for displaying configuration files.
 * Uses Monaco editor for syntax highlighting and better readability.
 * Used for product yml and other YAML/config files in readonly mode.
 *
 * @public
 */
export default function FileContentViewer({
  content,
  fileName,
  title,
  subtitle = null,
  icon = <FileDown size={18} />,
  iconBgColor = 'primary.lighter',
  iconColor = 'primary.main',
  showDownload = true,
  maxHeight = 400,
}: FileContentViewerProps): JSX.Element {
  const {t} = useTranslation('importExport');
  const [expanded, setExpanded] = useState(false);
  const {mode, systemMode} = useColorScheme();
  const colorMode: 'light' | 'dark' = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';

  // Determine language based on file extension
  const extension = fileName.substring(fileName.lastIndexOf('.'));
  const language = extension === '.yml' || extension === '.yaml' ? 'yaml' : 'plaintext';

  const handleDownload = async (): Promise<void> => {
    try {
      // Determine MIME type and file extension based on fileName
      const extension = fileName.substring(fileName.lastIndexOf('.'));
      const mimeType = extension === '.yml' || extension === '.yaml' ? 'text/yaml' : 'text/plain';
      const acceptTypes = extension === '.yml' || extension === '.yaml' ? ['.yml', '.yaml'] : [extension];

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
              description: 'Configuration File',
              accept: {[mimeType]: acceptTypes},
            },
          ],
        });
        const writable = await handle.createWritable();
        await writable.write(content);
        await writable.close();
      } else {
        // Fallback to traditional download
        const blob = new Blob([content], {type: `${mimeType};charset=utf-8`});
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
                bgcolor: iconBgColor,
                color: iconColor,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              {icon}
            </Box>
            <Box>
              <Typography variant="h6" fontWeight={600}>
                {title}
              </Typography>
              {subtitle && (
                <Typography variant="caption" color="text.secondary">
                  {subtitle}
                </Typography>
              )}
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
                {t('fileViewer.download', {fileName})}
              </Button>
            )}
            <IconButton size="small" onClick={() => setExpanded(!expanded)}>
              {expanded ? <ChevronUp size={20} /> : <ChevronDown size={20} />}
            </IconButton>
          </Stack>
        </Stack>
        <Collapse in={expanded} sx={{mt: expanded ? 2 : 0}}>
          <Box
            sx={{
              borderRadius: 1,
              maxHeight,
              overflow: 'hidden',
              border: '1px solid',
              borderColor: 'divider',
            }}
          >
            <Editor
              height={maxHeight}
              language={language}
              theme={colorMode === 'dark' ? 'vs-dark' : 'vs'}
              value={content}
              options={{
                readOnly: true,
                minimap: {enabled: false},
                scrollBeyondLastLine: false,
                automaticLayout: true,
                fontSize: 12,
                tabSize: 2,
                wordWrap: 'on',
                lineNumbers: 'on',
                folding: true,
                renderLineHighlight: 'none',
                contextmenu: false,
              }}
            />
          </Box>
        </Collapse>
      </Stack>
    </Paper>
  );
}
