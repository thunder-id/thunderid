// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCopyToClipboard} from '@thunderid/hooks';
import {
  Box,
  Typography,
  Stack,
  TextField,
  IconButton,
  InputAdornment,
  Alert,
  Button,
  Divider,
  useTheme,
} from '@wso2/oxygen-ui';
import {Copy, Eye, EyeOff, AlertTriangle} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';

export interface ShowClientSecretProps {
  agentName: string;
  clientId?: string;
  clientSecret: string;
  onContinue: () => void;
}

export default function ShowClientSecret({
  agentName,
  clientId = undefined,
  clientSecret,
  onContinue,
}: ShowClientSecretProps): JSX.Element {
  const {t} = useTranslation();
  const theme = useTheme();
  const [showSecret, setShowSecret] = useState(false);
  const {copied, copy} = useCopyToClipboard({resetDelay: 2000}) as {
    copied: boolean;
    copy: (text: string) => Promise<void>;
  };

  const handleCopy = async (): Promise<void> => {
    await copy(clientSecret);
  };

  return (
    <Stack direction="column" spacing={4} sx={{width: '100%'}} data-testid="agent-show-client-secret">
      <Box
        sx={{
          width: 64,
          height: 64,
          borderRadius: 2,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          alignSelf: 'center',
        }}
      >
        <AlertTriangle size={64} color={theme.vars?.palette.warning.main} />
      </Box>

      <Stack direction="column" spacing={1} sx={{textAlign: 'center'}}>
        <Typography variant="h3" component="h1">
          {t('agents:clientSecret.saveTitle', 'Save your client secret')}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {t(
            'agents:clientSecret.saveSubtitle',
            "This secret won't be shown again. Copy it and store it somewhere safe.",
          )}
        </Typography>
      </Stack>

      <Box
        sx={{
          p: 3,
          bgcolor: 'background.paper',
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1,
        }}
      >
        <Stack direction="column" spacing={2}>
          <Box>
            <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
              {t('agents:clientSecret.agentNameLabel', 'Agent name')}
            </Typography>
            <Typography variant="body1">{agentName}</Typography>
          </Box>

          {clientId && (
            <>
              <Divider />
              <Box>
                <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
                  {t('agents:clientSecret.clientIdLabel', 'Client ID')}
                </Typography>
                <Typography variant="body1" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
                  {clientId}
                </Typography>
              </Box>
            </>
          )}

          <Divider />

          <Box>
            <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
              {t('agents:clientSecret.clientSecretLabel', 'Client Secret')}
            </Typography>
            <TextField
              fullWidth
              data-testid="agent-client-secret-value"
              type={showSecret ? 'text' : 'password'}
              value={clientSecret}
              InputProps={{
                readOnly: true,
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton onClick={() => setShowSecret(!showSecret)} edge="end" size="small">
                      {showSecret ? <EyeOff size={16} /> : <Eye size={16} />}
                    </IconButton>
                    <IconButton
                      onClick={() => {
                        handleCopy().catch(() => null);
                      }}
                      edge="end"
                      size="small"
                      sx={{ml: 0.5}}
                    >
                      <Copy size={16} />
                    </IconButton>
                  </InputAdornment>
                ),
              }}
            />
          </Box>
        </Stack>
      </Box>

      <Alert severity="warning" icon={<AlertTriangle size={20} />}>
        <Typography variant="body2" sx={{fontWeight: 'medium', mb: 1}}>
          {t('agents:clientSecret.securityReminder.title', "You won't be able to see this secret again")}
        </Typography>
        <Typography variant="body2">
          {t(
            'agents:clientSecret.securityReminder.description',
            'Store the client secret somewhere safe. If you lose it, you will need to regenerate it from the agent settings.',
          )}
        </Typography>
      </Alert>

      <Stack direction="row" spacing={2} sx={{width: '100%'}}>
        <Button
          variant="contained"
          fullWidth
          startIcon={<Copy size={16} />}
          onClick={() => {
            handleCopy().catch(() => null);
          }}
          disabled={copied}
        >
          {copied
            ? t('agents:clientSecret.copied', 'Copied')
            : t('agents:clientSecret.copySecret', 'Copy client secret')}
        </Button>
        <Button data-testid="agent-client-secret-continue" variant="outlined" fullWidth onClick={onContinue}>
          {t('common:actions.continue')}
        </Button>
      </Stack>
    </Stack>
  );
}
