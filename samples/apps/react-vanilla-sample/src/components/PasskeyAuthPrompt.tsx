/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useState } from 'react';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';
import CircularProgress from '@mui/material/CircularProgress';
import FingerprintIcon from '@mui/icons-material/Fingerprint';
import type { 
    PasskeyRequestOptions, 
    PasskeyAssertionResponse 
} from '../services/authService';
import { authenticateWithPasskey } from '../services/authService';

interface PasskeyAuthPromptProps {
    passkeyRequestOptionsJson: string;
    onAuthenticated: (assertion: PasskeyAssertionResponse) => void;
    onError: (error: string) => void;
    isLoading?: boolean;
}

/**
 * PasskeyAuthPrompt component handles passkey authentication via WebAuthn API.
 * It displays a button to authenticate with a passkey and handles the WebAuthn ceremony.
 */
const PasskeyAuthPrompt = ({
    passkeyRequestOptionsJson,
    onAuthenticated,
    onError,
    isLoading = false,
}: PasskeyAuthPromptProps) => {
    const [authenticating, setAuthenticating] = useState(false);

    const handleAuthenticate = async () => {
        setAuthenticating(true);

        try {
            const options: PasskeyRequestOptions = JSON.parse(passkeyRequestOptionsJson);

            const assertion = await authenticateWithPasskey(options);

            onAuthenticated(assertion);
        } catch (err) {
            // authenticateWithPasskey maps WebAuthn/browser errors to clean messages,
            // so the error message can be surfaced directly.
            onError(err instanceof Error ? err.message : 'Failed to authenticate with passkey');
        } finally {
            setAuthenticating(false);
        }
    };

    const isDisabled = isLoading || authenticating;

    return (
        <Box
            sx={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                gap: 2,
                py: 3,
            }}
        >
            <FingerprintIcon sx={{ fontSize: 64, color: 'primary.main' }} />
            
            <Typography variant="h6" textAlign="center">
                Authenticate with Passkey
            </Typography>
            
            <Typography variant="body2" color="text.secondary" textAlign="center" sx={{ maxWidth: 300 }}>
                Use your device's biometric authentication (fingerprint, face, or PIN) to sign in securely.
            </Typography>


            <Button
                variant="contained"
                color="primary"
                size="large"
                onClick={handleAuthenticate}
                disabled={isDisabled}
                startIcon={authenticating ? <CircularProgress size={20} color="inherit" /> : <FingerprintIcon />}
                sx={{ mt: 2, minWidth: 200 }}
            >
                {authenticating ? 'Authenticating...' : 'Use Passkey'}
            </Button>
        </Box>
    );
};

export default PasskeyAuthPrompt;
