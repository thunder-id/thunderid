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

import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Divider from '@mui/material/Divider';
import Grid from '@mui/material/Grid';
import IconButton from '@mui/material/IconButton';
import InputAdornment from '@mui/material/InputAdornment';
import InputLabel from '@mui/material/InputLabel';
import MenuItem from '@mui/material/MenuItem';
import OutlinedInput from '@mui/material/OutlinedInput';
import Paper from '@mui/material/Paper';
import Select from '@mui/material/Select';
import Typography from '@mui/material/Typography';
import GitHubIcon from '@mui/icons-material/GitHub';
import GoogleIcon from '@mui/icons-material/Google';
import Visibility from '@mui/icons-material/Visibility';
import VisibilityOff from '@mui/icons-material/VisibilityOff';
import FingerprintIcon from '@mui/icons-material/Fingerprint';
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import GradientCircularProgress from '../components/GradientCircularProgress';
import Layout from '../components/Layout';
import { FlowServerError, submitNativeAuth } from '../services/authService';

interface AuthInput {
    identifier: string;
    type: string;
    required: boolean;
    ref?: string;
    options?: string[];
}

interface ActionPrompt {
    ref: string;
    nextNode?: string;
    label?: string;
}

interface FlowErrorMessage {
    key?: string;
    defaultValue?: string;
}

interface FlowError {
    code?: string;
    message?: FlowErrorMessage;
    description?: FlowErrorMessage;
}

interface FlowResponse {
    flowStatus?: string;
    error?: FlowError;
    type?: string;
    executionId?: string;
    challengeToken?: string;
    data?: {
        inputs?: AuthInput[];
        actions?: ActionPrompt[];
        redirectURL?: string;
    };
}

const getFlowErrorMessage = (error?: FlowError, fallback?: string): string => {
    return error?.message?.defaultValue ?? error?.description?.defaultValue ?? fallback ?? 'An error occurred.';
};

const isConnectionFailure = (error: Error) => {
    const message = error.message?.toLowerCase() || '';
    return message.includes('failed to fetch') || message.includes('network error');
};

const InvitePage = () => {
    const navigate = useNavigate();
    const isInitialized = useRef(false);
    const otpInputRefs = useRef<(HTMLInputElement | null)[]>([]);

    const [loading, setLoading] = useState<boolean>(true);
    const [step, setStep] = useState<'verifying' | 'form' | 'success' | 'error'>('verifying');
    const [errorMessage, setErrorMessage] = useState<string>('');
    const [executionId, setExecutionId] = useState<string>('');
    const [challengeToken, setChallengeToken] = useState<string>('');
    const [inputs, setInputs] = useState<AuthInput[]>([]);
    const [availableActions, setAvailableActions] = useState<ActionPrompt[]>([]);
    const [formData, setFormData] = useState<Record<string, string>>({});
    const [showPassword, setShowPassword] = useState<boolean>(false);
    const [otpDigits, setOtpDigits] = useState(Array(6).fill(''));

    useEffect(() => {
        otpInputRefs.current = otpInputRefs.current.slice(0, otpDigits.length);
    }, [otpDigits.length]);

    const handleResponse = (data: FlowResponse) => {
        if (data.flowStatus === 'COMPLETE') {
            setStep('success');
            setLoading(false);
            return;
        }

        if (data.flowStatus === 'ERROR') {
            setStep('error');
            setErrorMessage(getFlowErrorMessage(data.error, 'Invite flow failed. Please request a new invite link.'));
            setLoading(false);
            return;
        }

        if (data.type === 'VIEW') {
            const nextExecutionId = data.executionId || '';
            const nextActions = data.data?.actions || [];
            const nextInputs = data.data?.inputs || [];

            if (!nextExecutionId || nextActions.length === 0) {
                setStep('error');
                setErrorMessage('Invalid invite flow response. Please request a new invite link.');
                setLoading(false);
                return;
            }

            setExecutionId(nextExecutionId);
            setChallengeToken(data.challengeToken || '');
            setAvailableActions(nextActions);
            setInputs(nextInputs);
            setFormData({});
            setShowPassword(false);
            setOtpDigits(Array(6).fill(''));
            setStep('form');
            setLoading(false);
            return;
        }

        if (data.type === 'REDIRECTION') {
            const redirectURL = data.data?.redirectURL;

            if (!redirectURL) {
                setStep('error');
                setErrorMessage('Invalid invite flow response. Please request a new invite link.');
                setLoading(false);
                return;
            }

            window.location.href = redirectURL;
        }
    };

    useEffect(() => {
        if (isInitialized.current) {
            return;
        }

        isInitialized.current = true;

        const params = new URLSearchParams(window.location.search);
        const nextExecutionId = params.get('executionId') || '';
        const inviteToken = params.get('inviteToken') || '';

        if (!nextExecutionId || !inviteToken) {
            setStep('error');
            setErrorMessage('Invalid invite link. Please request a new invite link.');
            setLoading(false);
            return;
        }

        submitNativeAuth(nextExecutionId, { inviteToken }, undefined, undefined)
            .then((result) => {
                handleResponse(result.data);
            })
            .catch((error: Error) => {
                setStep('error');
                if (isConnectionFailure(error)) {
                    setErrorMessage('Unable to connect right now. Please try again.');
                } else if (error instanceof FlowServerError) {
                    setErrorMessage(error.message || 'A server error occurred. Please try again.');
                } else {
                    setErrorMessage('Invalid or expired invite link. Please request a new invite link.');
                }
                setLoading(false);
            });
    }, []);

    const handleInputChange = (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
        const { name, value } = event.target;
        setFormData((prev) => ({ ...prev, [name]: value }));
    };

    const handleSelectChange = (name: string, value: string) => {
        setFormData((prev) => ({ ...prev, [name]: value }));
    };

    const handleTogglePasswordVisibility = () => {
        setShowPassword((prev) => !prev);
    };

    const handleMouseDownPassword = (event: React.MouseEvent<HTMLButtonElement>) => {
        event.preventDefault();
    };

    const handleOTPDigitChange = (index: number, value: string) => {
        if (value.length > 1) {
            value = value.slice(0, 1);
        }

        const nextOtpDigits = [...otpDigits];
        nextOtpDigits[index] = value;
        setOtpDigits(nextOtpDigits);
        setFormData((prev) => ({ ...prev, otp: nextOtpDigits.join('') }));

        if (value && index < otpDigits.length - 1) {
            otpInputRefs.current[index + 1]?.focus();
        }
    };

    const handleOTPKeyDown = (index: number, event: React.KeyboardEvent<HTMLInputElement>) => {
        if (event.key === 'Backspace' && !otpDigits[index] && index > 0) {
            otpInputRefs.current[index - 1]?.focus();
        }
    };

    const handlePaste = (event: React.ClipboardEvent<HTMLInputElement>) => {
        event.preventDefault();
        const pastedData = event.clipboardData.getData('text');

        if (pastedData.match(/^\d+$/) && pastedData.length <= otpDigits.length) {
            const nextOtpDigits = [...otpDigits];

            for (let i = 0; i < pastedData.length; i++) {
                nextOtpDigits[i] = pastedData[i];
            }

            setOtpDigits(nextOtpDigits);
            setFormData((prev) => ({ ...prev, otp: nextOtpDigits.join('') }));

            const nextEmptyIndex = pastedData.length < otpDigits.length ? pastedData.length : otpDigits.length - 1;
            otpInputRefs.current[nextEmptyIndex]?.focus();
        }
    };

    const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        handleActionSelection(availableActions[0]?.ref);
    };

    const handleActionSelection = (actionRef?: string, payload?: Record<string, string>) => {
        if (!executionId || !actionRef) {
            setStep('error');
            setErrorMessage('Invalid invite flow response. Please request a new invite link.');
            setLoading(false);
            return;
        }

        setLoading(true);

        const completeFormData = { ...(payload || formData) };
        inputs.forEach((input) => {
            if (!(input.identifier in completeFormData)) {
                completeFormData[input.identifier] = '';
            }
        });

        submitNativeAuth(executionId, completeFormData, actionRef, challengeToken)
            .then((result) => {
                handleResponse(result.data);
            })
            .catch((error: Error) => {
                setStep('error');
                if (isConnectionFailure(error)) {
                    setErrorMessage('Unable to connect right now. Please try again.');
                } else if (error instanceof FlowServerError) {
                    setErrorMessage(error.message || 'A server error occurred. Please try again.');
                } else {
                    setErrorMessage(error.message || 'Unable to continue. Please try again.');
                }
                setLoading(false);
            });
    };

    const getActionLabel = (actionRef: string, fallbackLabel?: string) => {
        const normalizedRef = actionRef.toLowerCase();

        if (normalizedRef.includes('signin') || normalizedRef.includes('sign_in')) {
            return 'Sign In';
        }

        if (normalizedRef.includes('signup') || normalizedRef.includes('sign_up')) {
            return 'Create Account';
        }

        if (normalizedRef.includes('passkey')) {
            return 'Continue with Passkey';
        }

        if (normalizedRef.includes('github')) {
            return 'Continue with GitHub';
        }

        if (normalizedRef.includes('google')) {
            return 'Continue with Google';
        }

        if (normalizedRef.includes('sms') || normalizedRef.includes('mobile')) {
            return 'Continue with SMS OTP';
        }

        if (fallbackLabel) {
            return fallbackLabel;
        }

        return 'Continue';
    };

    const getActionIcon = (actionRef: string) => {
        const normalizedRef = actionRef.toLowerCase();

        if (normalizedRef.includes('github')) {
            return <GitHubIcon />;
        }

        if (normalizedRef.includes('google')) {
            return <GoogleIcon />;
        }

        if (normalizedRef.includes('passkey')) {
            return <FingerprintIcon />;
        }

        return undefined;
    };

    const renderInputFields = () => {
        return inputs.map((input, index) => {
            const inputId = input.identifier || `input-${index}`;
            const isPassword = input.type === 'password' || input.type === 'PASSWORD_INPUT' || input.identifier === 'password';
            const isOTP = input.type === 'otp' || input.type === 'OTP_INPUT' || input.identifier === 'otp';
            const isDropdown = input.type === 'SELECT' || input.type === 'dropdown' || input.type === 'DROPDOWN';
            const isRequired = input.required;

            let label = input.identifier;
            if (label) {
                label = label.charAt(0).toUpperCase() + label.slice(1).replace(/_/g, ' ');
            }
            label = label.replace(/([a-z])([A-Z])/g, '$1 $2');
            if (isOTP) {
                label = 'OTP Code';
            } else if (isPassword) {
                label = 'Password';
            }

            const placeholder = `Enter your ${label.toLowerCase()}`;

            if (isDropdown && input.options) {
                return (
                    <Box key={inputId} display="flex" flexDirection="column" gap={0.5}>
                        <InputLabel htmlFor={inputId} sx={{ mb: 1 }}>{label}</InputLabel>
                        <Select
                            id={inputId}
                            name={input.identifier}
                            size="small"
                            value={formData[input.identifier] || ''}
                            onChange={(e) => handleSelectChange(input.identifier, e.target.value)}
                            required={isRequired}
                            displayEmpty
                        >
                            <MenuItem value="" disabled sx={{ color: 'text.secondary' }}>
                                Select {label.toLowerCase()}
                            </MenuItem>
                            {input.options.map((option) => (
                                <MenuItem key={option} value={option}>
                                    {option}
                                </MenuItem>
                            ))}
                        </Select>
                    </Box>
                );
            }

            if (isPassword) {
                return (
                    <Box key={inputId} display="flex" flexDirection="column" gap={0.5}>
                        <InputLabel htmlFor={inputId} sx={{ mb: 1 }}>{label}</InputLabel>
                        <OutlinedInput
                            type={showPassword ? 'text' : 'password'}
                            id={inputId}
                            name={input.identifier}
                            placeholder={placeholder}
                            size="small"
                            value={formData[input.identifier] || ''}
                            onChange={handleInputChange}
                            required={isRequired}
                            endAdornment={
                                <InputAdornment position="end">
                                    <IconButton
                                        aria-label="toggle password visibility"
                                        onClick={handleTogglePasswordVisibility}
                                        onMouseDown={handleMouseDownPassword}
                                        edge="end"
                                    >
                                        {showPassword ? <VisibilityOff /> : <Visibility />}
                                    </IconButton>
                                </InputAdornment>
                            }
                        />
                    </Box>
                );
            }

            if (isOTP) {
                return (
                    <Box key={inputId} display="flex" flexDirection="column" gap={0.5}>
                        <InputLabel htmlFor={inputId} sx={{ mb: 1 }}>{label}</InputLabel>
                        <Box
                            sx={{
                                display: 'flex',
                                gap: 1,
                                justifyContent: 'space-between',
                            }}
                        >
                            {otpDigits.map((digit, otpIndex) => (
                                <OutlinedInput
                                    key={`otp-digit-${otpIndex}`}
                                    inputRef={(element) => { otpInputRefs.current[otpIndex] = element; }}
                                    value={digit}
                                    onChange={(e) => handleOTPDigitChange(otpIndex, e.target.value)}
                                    onKeyDown={(e) => handleOTPKeyDown(otpIndex, e as React.KeyboardEvent<HTMLInputElement>)}
                                    onPaste={otpIndex === 0 ? handlePaste : undefined}
                                    inputProps={{
                                        maxLength: 1,
                                        style: { textAlign: 'center', padding: '8px 0' },
                                    }}
                                    sx={{
                                        width: '40px',
                                        height: '48px',
                                        '& input': { padding: 0 },
                                    }}
                                />
                            ))}
                        </Box>
                    </Box>
                );
            }

            return (
                <Box key={inputId} display="flex" flexDirection="column" gap={0.5}>
                    <InputLabel htmlFor={inputId} sx={{ mb: 1 }}>{label}</InputLabel>
                    <OutlinedInput
                        type={input.type || 'text'}
                        id={inputId}
                        name={input.identifier}
                        placeholder={placeholder}
                        size="small"
                        value={formData[input.identifier] || ''}
                        onChange={handleInputChange}
                        required={isRequired}
                    />
                </Box>
            );
        });
    };

    const renderInputPromptForm = () => {
        const primaryAction = availableActions[0];
        const hasPasswordInput = inputs.some((input) => input.identifier === 'password' || input.type === 'PASSWORD_INPUT');
        const hasOTPInput = inputs.some((input) => input.identifier === 'otp' || input.type === 'OTP_INPUT');
        let primaryLabel = 'Continue';

        if (primaryAction?.ref) {
            if (hasPasswordInput) {
                primaryLabel = 'Reset Password';
            } else if (hasOTPInput) {
                primaryLabel = 'Verify OTP';
            } else {
                primaryLabel = getActionLabel(primaryAction.ref, primaryAction.label);
            }
        }

        return (
            <form onSubmit={handleSubmit}>
                <Box display="flex" flexDirection="column" gap={2}>
                    {renderInputFields()}

                    {primaryAction && (
                        <Button variant="contained" color="primary" type="submit" fullWidth sx={{ mt: 2 }}>
                            {primaryLabel}
                        </Button>
                    )}

                    {availableActions.length > 1 && (
                        <Box sx={{ mt: 1 }}>
                            <Divider sx={{ my: 2 }}>or</Divider>
                            {availableActions.slice(1).map((action, index) => (
                                <Button
                                    key={`alt-action-${index}`}
                                    fullWidth
                                    variant="contained"
                                    color="secondary"
                                    type="button"
                                    onClick={() => handleActionSelection(action.ref)}
                                    sx={{ mb: 1 }}
                                    startIcon={getActionIcon(action.ref)}
                                >
                                    {getActionLabel(action.ref, action.label)}
                                </Button>
                            ))}
                        </Box>
                    )}
                </Box>
            </form>
        );
    };

    const renderContent = () => {
        if (step === 'verifying') {
            return (
                <Box sx={{ textAlign: 'center', py: 4 }}>
                    <GradientCircularProgress />
                    <Typography sx={{ mt: 2 }}>Verifying your invite link...</Typography>
                </Box>
            );
        }

        if (step === 'error') {
            return (
                <Box>
                    <Box sx={{ mb: 4 }}>
                        <Typography variant="h5" gutterBottom>
                            Unable to Continue
                        </Typography>
                    </Box>
                    <Alert severity="error" sx={{ mb: 3 }}>
                        {errorMessage}
                    </Alert>
                    <Button
                        variant="contained"
                        color="primary"
                        fullWidth
                        onClick={() => navigate('/')}
                    >
                        Back to Login
                    </Button>
                </Box>
            );
        }

        if (step === 'success') {
            return (
                <Box sx={{ textAlign: 'center', py: 2 }}>
                    <Typography variant="h3" sx={{ mb: 2 }}>✅</Typography>
                    <Typography variant="h5" gutterBottom>
                        Completed
                    </Typography>
                    <Typography sx={{ mb: 3 }}>
                        Completed successfully.
                    </Typography>
                    <Button
                        variant="contained"
                        color="primary"
                        fullWidth
                        onClick={() => navigate('/')}
                    >
                        Go to Login
                    </Button>
                </Box>
            );
        }

        return (
            <Box>
                <Box sx={{ mb: 4 }}>
                    <Typography variant="h5" gutterBottom>
                        Continue
                    </Typography>
                    <Typography>
                        Provide the requested information to continue.
                    </Typography>
                </Box>
                {renderInputPromptForm()}
            </Box>
        );
    };

    return (
        <Layout>
            {loading && step === 'verifying' ? (
                <GradientCircularProgress />
            ) : (
                <Grid size={{ xs: 12, md: 6 }}>
                    <Paper
                        sx={{
                            display: 'flex',
                            width: '100%',
                            height: '100%',
                            flexDirection: 'column',
                        }}
                    >
                        <Box
                            sx={{
                                alignItems: 'center',
                                justifyContent: 'center',
                                padding: 6,
                                width: '100%',
                                maxWidth: 500,
                                margin: 'auto',
                            }}
                        >
                            {renderContent()}
                            <Box component="footer" sx={{ mt: 6 }}>
                                <Typography sx={{ textAlign: 'center' }}>
                                    © Copyright {new Date().getFullYear()}
                                </Typography>
                            </Box>
                        </Box>
                    </Paper>
                </Grid>
            )}
        </Layout>
    );
};

export default InvitePage;
