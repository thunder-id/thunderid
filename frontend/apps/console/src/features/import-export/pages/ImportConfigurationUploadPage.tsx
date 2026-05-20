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

import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {Box, Breadcrumbs, Button, IconButton, LinearProgress, Stack, Typography, Alert} from '@wso2/oxygen-ui';
import {ChevronRight, Upload, X} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import yaml from 'yaml';
import {ALLOWED_RESOURCE_TYPES, type ResourceType} from '../constants/resource-types';
import getConfigFileName from '../utils/getConfigFileName';
import getEnvFileName from '../utils/getEnvFileName';

export default function ImportConfigurationUploadPage(): JSX.Element {
  const {t} = useTranslation('importExport');
  const navigate = useNavigate();
  const logger = useLogger('ImportConfigurationUploadPage');
  const {config} = useConfig();
  const configFileName = getConfigFileName(config.brand.product_name);
  const envFileName = getEnvFileName(config.brand.product_name);
  const [dragActive, setDragActive] = useState(false);
  const [envDragActive, setEnvDragActive] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [selectedEnvFile, setSelectedEnvFile] = useState<File | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleClose = (): void => {
    void navigate('/home');
  };

  const handleCancel = (): void => {
    void navigate('/welcome');
  };

  const handleDrag = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true);
    } else if (e.type === 'dragleave') {
      setDragActive(false);
    }
  }, []);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setDragActive(false);
      setError(null);

      if (e.dataTransfer.files?.[0]) {
        const file = e.dataTransfer.files[0];
        if (file.name.endsWith('.yml') || file.name.endsWith('.yaml')) {
          setSelectedFile(file);
        } else {
          setError(t('upload.errors.uploadYaml', {configFileName}));
        }
      }
    },
    [t, configFileName],
  );

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>): void => {
    setError(null);
    if (e.target.files?.[0]) {
      const file = e.target.files[0];
      if (file.name.endsWith('.yml') || file.name.endsWith('.yaml')) {
        setSelectedFile(file);
      } else {
        setError(t('upload.errors.uploadYaml', {configFileName}));
      }
    }
  };

  const handleEnvDrag = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setEnvDragActive(true);
    } else if (e.type === 'dragleave') {
      setEnvDragActive(false);
    }
  }, []);

  const handleEnvDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setEnvDragActive(false);
      setError(null);

      if (e.dataTransfer.files?.[0]) {
        const file = e.dataTransfer.files[0];
        if (file.name.endsWith('.env') || file.name === '.env') {
          setSelectedEnvFile(file);
        } else {
          setError(t('upload.errors.uploadEnv'));
        }
      }
    },
    [t],
  );

  const handleEnvFileChange = (e: React.ChangeEvent<HTMLInputElement>): void => {
    setError(null);
    if (e.target.files?.[0]) {
      const file = e.target.files[0];
      if (file.name.endsWith('.env') || file.name === '.env') {
        setSelectedEnvFile(file);
      } else {
        setError(t('upload.errors.uploadEnv'));
      }
    }
  };

  const handleContinue = async (): Promise<void> => {
    if (!selectedFile) {
      setError(t('upload.errors.selectFile'));
      return;
    }
    if (!selectedEnvFile) {
      setError(t('upload.errors.selectEnvFile'));
      return;
    }

    try {
      let configData: unknown = null;
      let envData: unknown = null;
      let configContent: string | null = null;
      const parseErrors: {resourceType: string; fileName: string; error: string}[] = [];
      let successCount = 0;
      let failCount = 0;

      // Parse multi-document YAML file
      if (selectedFile) {
        const fileContent = await selectedFile.text();
        configContent = fileContent;

        // Split by document separator and process each section
        const sections = fileContent.split(/^---$/m);
        const resourcesByType: Record<string, unknown[]> = {};

        sections.forEach((section) => {
          const trimmedSection = section.trim();
          if (!trimmedSection) return;

          // Extract comments at the start of the section
          const lines = trimmedSection.split(/\r?\n|\r/);
          let resourceType = 'unknown';
          let fileName = 'unknown';

          // First pass: extract metadata from comment lines
          for (const line of lines) {
            if (line.startsWith('#')) {
              const resourceTypeRegex = /resource_type:\s*(\w+)/;
              const fileNameRegex = /File:\s*(.+\.yaml)/i;
              const resourceTypeMatch = resourceTypeRegex.exec(line);
              const fileNameMatch = fileNameRegex.exec(line);

              if (resourceTypeMatch) {
                resourceType = resourceTypeMatch[1];
              }
              if (fileNameMatch) {
                fileName = fileNameMatch[1].trim();
              }
            }
          }

          // Second pass: extract only non-comment, non-empty lines for YAML parsing
          const yamlLines = lines.filter((line) => {
            const trimmed = line.trim();
            // Skip comments, empty lines, paste service warnings, and template block directives
            return (
              trimmed !== '' &&
              !trimmed.startsWith('#') &&
              !trimmed.toLowerCase().includes('paste expires') &&
              !trimmed.toLowerCase().includes('public ip access') &&
              !trimmed.toLowerCase().includes('share whatever') &&
              !trimmed.startsWith('{{-') && // Skip template block directives (e.g., {{- range}}, {{- end}})
              trimmed !== '{{- end}}' &&
              trimmed !== '{{- end }}'
            );
          });

          // Replace template variables with placeholder strings to prevent YAML parsing errors
          // Templates like {{ .VAR }} or {{ t(...) }} are interpreted as objects by YAML parser
          const yamlContent = yamlLines
            .map((line) => {
              let processedLine = line;

              // Quote template variables in two scenarios:
              // 1. Standalone values: key: {{.VAR}} → key: "{{.VAR}}"
              processedLine = processedLine.replace(/:\s*(\{\{[^}]+\}\})(\s*)$/g, ': "$1"$2');

              // 2. Array items: - {{.}} → - "{{.}}"
              processedLine = processedLine.replace(/^(\s*-\s+)(\{\{[^}]+\}\})(\s*)$/g, '$1"$2"$3');

              return processedLine;
            })
            .join('\n');

          // Parse the YAML content (without comments)
          try {
            const resource = yaml.parse(yamlContent) as unknown;

            if (resource && typeof resource === 'object') {
              if (!resourcesByType[resourceType]) {
                resourcesByType[resourceType] = [];
              }

              resourcesByType[resourceType].push({
                ...resource,
                _metadata: {
                  originalFileName: fileName,
                  resourceType,
                },
              });
              successCount++;
            }
          } catch (parseError) {
            failCount++;
            const errorMessage = parseError instanceof Error ? parseError.message : String(parseError);
            parseErrors.push({
              resourceType,
              fileName,
              error: errorMessage,
            });
            logger.warn('Failed to parse YAML section', {
              resourceType,
              fileName,
              yamlPreview: yamlContent?.substring(0, 200),
              error: errorMessage,
            });
          }
        });

        const unknownTypes = Object.keys(resourcesByType).filter(
          (type) => !ALLOWED_RESOURCE_TYPES.includes(type as ResourceType) && type !== 'unknown',
        );

        if (unknownTypes.length > 0) {
          unknownTypes.forEach((unknownType) => {
            const unknownResources = resourcesByType[unknownType];
            if (Array.isArray(unknownResources)) {
              unknownResources.forEach((resource: unknown) => {
                const typedResource = resource as {_metadata?: {originalFileName?: string}};
                failCount++;
                parseErrors.push({
                  resourceType: unknownType,
                  fileName: typedResource._metadata?.originalFileName ?? 'unknown',
                  error: t('upload.errors.unknownResourceType', {
                    resourceType: unknownType,
                    allowedTypes: ALLOWED_RESOURCE_TYPES.join(', '),
                  }),
                });
              });
            }
            // Remove unknown types from the parsed data
            delete resourcesByType[unknownType];
          });
        }

        if (failCount > 0) {
          logger.warn(`Parsed ${successCount} sections successfully, ${failCount} sections failed`);
        }

        configData = resourcesByType;
      }

      // Parse .env file if provided
      if (selectedEnvFile) {
        const envContent = await selectedEnvFile.text();
        envData = envContent;
      }

      // Navigate to validation page
      await navigate('/welcome/open-project/validate', {
        state: {
          method: 'file',
          file: selectedFile,
          envFile: selectedEnvFile,
          configData,
          envData,
          parseErrors,
          parseStats: {successCount, failCount},
          configContent,
        },
      });
    } catch (error) {
      logger.error('Failed to parse configuration file', {error});
      setError(
        t('upload.errors.parseFailed', {
          message: error instanceof Error ? error.message : t('common:dictionary.unknown'),
        }),
      );
    }
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={33} sx={{height: 6}} />

      <Box
        sx={{
          p: 4,
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          flexShrink: 0,
        }}
      >
        <Stack direction="row" alignItems="center" spacing={2}>
          <IconButton
            aria-label={t('common:actions.close')}
            onClick={handleClose}
            sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
          >
            <X size={24} />
          </IconButton>
          <Breadcrumbs separator={<ChevronRight size={16} />} aria-label="breadcrumb">
            <Typography
              variant="h5"
              onClick={() => void navigate('/welcome')}
              sx={{cursor: 'pointer', '&:hover': {textDecoration: 'underline'}}}
            >
              {t('common:welcome.header')}
            </Typography>
            <Typography variant="h5" color="text.primary">
              {t('upload.breadcrumb.openProject')}
            </Typography>
          </Breadcrumbs>
        </Stack>
      </Box>

      <Box
        sx={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          py: 8,
          px: {xs: 2, sm: 3, md: 8, lg: 20},
          alignItems: 'flex-start',
        }}
      >
        <Box
          sx={{
            width: '100%',
            maxWidth: 1600,
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <Stack spacing={1} mb={4}>
            <Typography variant="h2" fontWeight={600}>
              {t('upload.title')}
            </Typography>
            <Typography variant="body1" color="text.secondary">
              {t('upload.subtitle', {configFileName})}
            </Typography>
          </Stack>

          {error && (
            <Alert severity="error" sx={{mb: 3}}>
              {error}
            </Alert>
          )}

          <Stack spacing={3} mb={4}>
            <Box
              onDragEnter={handleDrag}
              onDragLeave={handleDrag}
              onDragOver={handleDrag}
              onDrop={handleDrop}
              sx={{
                border: '2px dashed',
                borderColor: dragActive ? 'primary.main' : 'divider',
                borderRadius: 2,
                p: 6,
                textAlign: 'center',
                bgcolor: dragActive ? 'action.hover' : 'background.paper',
                transition: 'all 0.2s',
                cursor: 'pointer',
              }}
              onClick={() => document.getElementById('file-upload')?.click()}
            >
              <input
                id="file-upload"
                type="file"
                accept=".yml,.yaml,application/x-yaml,text/yaml"
                onChange={handleFileChange}
                style={{display: 'none'}}
              />
              <Stack spacing={2} alignItems="center">
                <Box
                  sx={{
                    width: 56,
                    height: 56,
                    borderRadius: '50%',
                    bgcolor: 'action.hover',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                >
                  <Upload size={24} />
                </Box>
                {selectedFile ? (
                  <>
                    <Typography variant="body1" fontWeight={600}>
                      {selectedFile.name}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {(selectedFile.size / 1024).toFixed(2)} KB
                    </Typography>
                    <Button variant="outlined" size="small">
                      {t('upload.actions.changeFile')}
                    </Button>
                  </>
                ) : (
                  <>
                    <Typography variant="body1" fontWeight={600}>
                      {t('upload.dropConfig')}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {t('upload.orClickBrowse')}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {t('upload.supportsYaml', {configFileName})}
                    </Typography>
                  </>
                )}
              </Stack>
            </Box>

            {/* Environment Variables Section - Available for both methods */}
            <>
              <Typography variant="h6" fontWeight={600} mt={2}>
                {t('upload.env.title')}
              </Typography>
              <Typography variant="body2" color="text.secondary" mb={1}>
                {t('upload.env.subtitle', {envFileName})}
              </Typography>
              <Box
                onDragEnter={handleEnvDrag}
                onDragLeave={handleEnvDrag}
                onDragOver={handleEnvDrag}
                onDrop={handleEnvDrop}
                sx={{
                  border: '2px dashed',
                  borderColor: envDragActive ? 'primary.main' : 'divider',
                  borderRadius: 2,
                  p: 4,
                  textAlign: 'center',
                  bgcolor: envDragActive ? 'action.hover' : 'background.paper',
                  transition: 'all 0.2s',
                  cursor: 'pointer',
                }}
                onClick={() => document.getElementById('env-file-upload')?.click()}
              >
                <input
                  id="env-file-upload"
                  type="file"
                  accept=".env"
                  onChange={handleEnvFileChange}
                  style={{display: 'none'}}
                />
                <Stack spacing={2} alignItems="center">
                  <Box
                    sx={{
                      width: 48,
                      height: 48,
                      borderRadius: '50%',
                      bgcolor: 'action.hover',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <Upload size={20} />
                  </Box>
                  {selectedEnvFile ? (
                    <>
                      <Typography variant="body2" fontWeight={600}>
                        {selectedEnvFile.name}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {(selectedEnvFile.size / 1024).toFixed(2)} KB
                      </Typography>
                      <Button variant="outlined" size="small">
                        {t('upload.actions.changeFile')}
                      </Button>
                    </>
                  ) : (
                    <>
                      <Typography variant="body2" fontWeight={600}>
                        {t('upload.env.dropFile', {envFileName})}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {t('upload.orClickBrowse')}
                      </Typography>
                    </>
                  )}
                </Stack>
              </Box>
            </>
          </Stack>

          <Stack direction="row" spacing={2} justifyContent="flex-start">
            <Button variant="outlined" onClick={handleCancel}>
              {t('common:actions.cancel')}
            </Button>
            <Button
              variant="contained"
              onClick={() => void handleContinue()}
              disabled={!selectedFile || !selectedEnvFile}
            >
              {t('common:actions.continue')}
            </Button>
          </Stack>
        </Box>
      </Box>
    </Box>
  );
}
