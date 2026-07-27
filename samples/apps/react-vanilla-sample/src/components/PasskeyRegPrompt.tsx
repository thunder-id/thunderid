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
    PasskeyCreationOptions, 
    PasskeyCredentialResponse 
} from '../services/authService';
import { createPasskeyCredential } from '../services/authService';

interface PasskeyRegPromptProps {
    passkeyCreationOptionsJson: string;
    onCredentialCreated: (credential: PasskeyCredentialResponse) => void;
    onError: (error: string) => void;
    isLoading?: boolean;
}

/**
 * PasskeyRegPrompt component handles passkey credential creation via WebAuthn API.
 * It displays a button to create a passkey and handles the WebAuthn ceremony.
 */
const PasskeyRegPrompt = ({
    passkeyCreationOptionsJson,
    onCredentialCreated,
    onError,
    isLoading = false,
}: PasskeyRegPromptProps) => {
    const [creating, setCreating] = useState(false);

    const handleCreatePasskey = async () => {
        setCreating(true);

        try {
            const options: PasskeyCreationOptions = JSON.parse(passkeyCreationOptionsJson);
            const credential = await createPasskeyCredential(options);
            onCredentialCreated(credential);
        } catch (err) {
            // createPasskeyCredential maps WebAuthn/browser errors to clean messages,
            // so the error message can be surfaced directly.
            onError(err instanceof Error ? err.message : 'Failed to create passkey');
        } finally {
            setCreating(false);
        }
    };

    const isDisabled = isLoading || creating;

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
                Create a Passkey
            </Typography>
            
            <Typography variant="body2" color="text.secondary" textAlign="center" sx={{ maxWidth: 300 }}>
                Use your device's biometric authentication (fingerprint, face, or PIN) to create a secure passkey.
            </Typography>


            <Button
                variant="contained"
                color="primary"
                size="large"
                onClick={handleCreatePasskey}
                disabled={isDisabled}
                startIcon={creating ? <CircularProgress size={20} color="inherit" /> : <FingerprintIcon />}
                sx={{ mt: 2, minWidth: 200 }}
            >
                {creating ? 'Creating Passkey...' : 'Create Passkey'}
            </Button>
        </Box>
    );
};

export default PasskeyRegPrompt;
