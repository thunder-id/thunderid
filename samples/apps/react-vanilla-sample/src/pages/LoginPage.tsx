/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
import InputAdornment from '@mui/material/InputAdornment';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import Checkbox from '@mui/material/Checkbox';
import Divider from '@mui/material/Divider';
import FormControlLabel from '@mui/material/FormControlLabel';
import Grid from '@mui/material/Grid';
import IconButton from '@mui/material/IconButton';
import InputLabel from '@mui/material/InputLabel';
import OutlinedInput from '@mui/material/OutlinedInput';
import Paper from '@mui/material/Paper';
import Typography from '@mui/material/Typography';
import Link from '@mui/material/Link';
import MenuItem from '@mui/material/MenuItem';
import Select from '@mui/material/Select';
import GoogleIcon from '@mui/icons-material/Google';
import GitHubIcon from '@mui/icons-material/GitHub';
import Visibility from '@mui/icons-material/Visibility';
import VisibilityOff from '@mui/icons-material/VisibilityOff';
import AccountCircleIcon from '@mui/icons-material/AccountCircle';
import FingerprintIcon from '@mui/icons-material/Fingerprint';
import { useEffect, useRef, useState, useCallback } from 'react';
import Layout from '../components/Layout';
import ConnectionErrorModal from '../components/ConnectionErrorModal';
import PasskeyRegPrompt from '../components/PasskeyRegPrompt';
import PasskeyAuthPrompt from '../components/PasskeyAuthPrompt';
import {
    NativeAuthSubmitType,
    FlowServerError,
    initiateNativeAuthFlow,
    initiateNativeAuthFlowWithData,
    submitNativeAuth,
    submitAuthDecision
} from '../services/authService';
import type { PasskeyCredentialResponse, PasskeyAssertionResponse } from '../services/authService';
import useAuth from '../hooks/useAuth';

// Define the interface for the authentication input
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

// Define the interface for the authentication response
interface AuthResponse {
    flowStatus?: string;
    assertion?: string;
    challengeToken?: string;
    error?: FlowError;
    type?: string;
    data?: {
        actions?: ActionPrompt[];
        inputs?: AuthInput[];
        redirectURL?: string;
        additionalData?: {
            idpName?: string;
            passkeyCreationOptions?: string;
            passkeyChallenge?: string;
            emailSent?: string;
        };
    };
    executionId?: string;
}

const getFlowErrorMessage = (error?: FlowError, fallback?: string): string => {
    return error?.message?.defaultValue ?? error?.description?.defaultValue ?? fallback ?? 'An error occurred.';
};

const isConnectionFailure = (error: Error) => {
    const message = error.message?.toLowerCase() || '';
    return message.includes('failed to fetch') || message.includes('network error');
};

/**
 * LoginPage component renders the login page with dynamic options based on the server response.
 */
const LoginPage = () => {

    const START_INIT_KEY = 'startInit';
    const EXECUTION_ID_KEY = 'executionId';
    const CHALLENGE_TOKEN_KEY = 'challengeToken';
    const SIGNUP_MODE_KEY = 'isSignupMode';

    const isComponentReMount = useRef(false);
    const initRef = useRef<((mode?: boolean) => void) | null>(null);
    const initRecoveryRef = useRef<((resetErrorState?: boolean) => void) | null>(null);
    const { setToken, clearToken } = useAuth();

    const [showRememberMe] = useState<boolean>(false);
    const [isRecoveryMode, setIsRecoveryMode] = useState<boolean>(false);
    const [emailSent, setEmailSent] = useState<boolean>(false);
    const [error, setError] = useState<boolean>(false);
    const [errorMessage, setErrorMessage] = useState<string>('');
    const [connectionError, setConnectionError] = useState<boolean>(false);

    const [loading, setLoading] = useState<boolean>(true);
    const [retryCount, setRetryCount] = useState<number>(0);
    const [executionId, setExecutionId] = useState<string>(sessionStorage.getItem(EXECUTION_ID_KEY) || '');
    const [challengeToken, setChallengeToken] = useState<string>(sessionStorage.getItem(CHALLENGE_TOKEN_KEY) || '');
    const [startInit] = useState<boolean>(JSON.parse(sessionStorage.getItem(START_INIT_KEY) || 'true'));

    // Unified form data state
    const [formData, setFormData] = useState<Record<string, string>>({});
    const [inputs, setInputs] = useState<AuthInput[]>([]);
    const [showPassword, setShowPassword] = useState(false);

    // Add new state variable to track redirection URL
    const [redirectURL, setRedirectURL] = useState<string | null>(null);
    const [socialIdpName, setSocialIdpName] = useState<string>('');

    // Add new state variables to track auth flow
    const [needsDecision, setNeedsDecision] = useState<boolean>(false);
    const [availableActions, setAvailableActions] = useState<ActionPrompt[]>([]);
    const [selectedAction, setSelectedAction] = useState<string | null>(null);
    
    const [isSignupMode, setIsSignupMode] = useState<boolean>(
        sessionStorage.getItem(START_INIT_KEY) === 'false' &&
        sessionStorage.getItem(SIGNUP_MODE_KEY) === 'true'
    );
    const [regOnlySuccess, setRegOnlySuccess] = useState<boolean>(false);
    const [promptRegistration, setPromptRegistration] = useState<boolean>(false);
    const showForgotPassword = !isSignupMode && !isRecoveryMode;
    
    // Passkey registration state
    const [passkeyCreationOptions, setPasskeyCreationOptions] = useState<string | null>(null);
    
    // Passkey authentication state
    const [passkeyChallenge, setPasskeyChallenge] = useState<string | null>(null);
    
    const GradientCircularProgress = () => {
        return (
          <>
            <svg width={0} height={0}>
              <defs>
                <linearGradient id="my_gradient" x1="0%" y1="0%" x2="0%" y2="100%">
                  <stop offset="0%" stopColor="#fc4700" />
                  <stop offset="100%" stopColor="#f87643" />
                </linearGradient>
              </defs>
            </svg>
            <CircularProgress sx={{ 'svg circle': { stroke: 'url(#my_gradient)' } }} />
          </>
        );
    }

    // OTP input handling
    const otpLength = 6;
    const [otpDigits, setOtpDigits] = useState(Array(otpLength).fill(''));
    const otpInputRefs = useRef<(HTMLInputElement | null)[]>([]);
    
    useEffect(() => {
        // Initialize refs array
        otpInputRefs.current = otpInputRefs.current.slice(0, otpDigits.length);
    }, [otpDigits.length]);
    
    const handleOTPDigitChange = (index: number, value: string) => {
        // Only allow a single digit
        if (value.length > 1) {
            value = value.slice(0, 1);
        }
        
        // Update the digit at the specified index
        const newOtpDigits = [...otpDigits];
        newOtpDigits[index] = value;
        setOtpDigits(newOtpDigits);
        
        // Combine digits for the form data
        const combinedOtp = newOtpDigits.join('');
        setFormData(prev => ({ ...prev, otp: combinedOtp }));
        
        // Auto-advance to next input if a digit was entered
        if (value && index < otpDigits.length - 1) {
            otpInputRefs.current[index + 1]?.focus();
        }
    };
    
    const handleOTPKeyDown = (index: number, e: React.KeyboardEvent<HTMLInputElement>) => {
        // Handle backspace to go to previous input
        if (e.key === 'Backspace' && !otpDigits[index] && index > 0) {
            otpInputRefs.current[index - 1]?.focus();
        }
    };
    
    const handlePaste = (e: React.ClipboardEvent<HTMLInputElement>) => {
        e.preventDefault();
        const pastedData = e.clipboardData.getData('text');
        
        // Only process if the pasted content looks like a valid OTP
        if (pastedData.match(/^\d+$/) && pastedData.length <= otpDigits.length) {
            const newOtpDigits = [...otpDigits];
            
            // Fill in the digits from the pasted content
            for (let i = 0; i < pastedData.length; i++) {
                newOtpDigits[i] = pastedData[i];
            }
            
            setOtpDigits(newOtpDigits);
            setFormData(prev => ({ ...prev, otp: newOtpDigits.join('') }));
            
            // Focus the next empty digit or the last digit if all filled
            const nextEmptyIndex = pastedData.length < otpDigits.length ? pastedData.length : otpDigits.length - 1;
            otpInputRefs.current[nextEmptyIndex]?.focus();
        }
    };

    // Effect to focus on the first OTP input when available.
    useEffect(() => {
        const hasOTPInput = inputs.some(input => input.type === "otp" || input.type === "OTP_INPUT" || input.identifier === "otp");
        
        if (hasOTPInput && otpInputRefs.current && otpInputRefs.current.length > 0) {
            setTimeout(() => {
                if (otpInputRefs.current[0]) {
                    otpInputRefs.current[0].focus();
                }
            }, 100);
        }
    }, [inputs]);

    // Single handler for all input changes
    const handleInputChange = (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
        const { name, value } = event.target;
        setFormData(prev => ({ ...prev, [name]: value }));
    };

    // Handler for select/dropdown changes
    const handleSelectChange = (name: string, value: string) => {
        setFormData(prev => ({ ...prev, [name]: value }));
    };

    const handleTogglePasswordVisibility = () => {
        setShowPassword((prev) => !prev);
    };

    // To prevent focus loss of show/hide password toggle button
    const handleMouseDownPassword = (event: React.MouseEvent<HTMLButtonElement>) => {
        event.preventDefault(); // Prevent focus loss
    };

    const handleSocialLoginClick = useCallback((redirectURL: string, newChallengeToken?: string) => {
        setLoading(true);
        sessionStorage.setItem(EXECUTION_ID_KEY, executionId);
        const tokenToSave = newChallengeToken !== undefined ? newChallengeToken : challengeToken;
        if (tokenToSave) {
            sessionStorage.setItem(CHALLENGE_TOKEN_KEY, tokenToSave);
        } else {
            sessionStorage.removeItem(CHALLENGE_TOKEN_KEY);
        }
        sessionStorage.setItem(SIGNUP_MODE_KEY, String(isSignupMode));
        sessionStorage.setItem(START_INIT_KEY, "false");
        window.location.href = redirectURL;
    }, [executionId, challengeToken, isSignupMode]);

    // Process authentication response
    const processAuthResponse = useCallback((data: AuthResponse, selectedAction?: string, autoRedirect: boolean = false) => {
        const isCameFromDecision = needsDecision || autoRedirect;
        const isMobileLogin = !!selectedAction && (selectedAction.includes('mobile') || selectedAction.includes('sms'));

        setExecutionId(data.executionId || '');
        setChallengeToken(data.challengeToken || '');
        if (data.flowStatus && data.flowStatus == 'ERROR') {
            const flowErrorMsg = getFlowErrorMessage(data.error);
            if (isMobileLogin && flowErrorMsg.includes("User not found")) {
                console.log("User not found, prompting registration");
                setPromptRegistration(true);
                setError(false);
                setErrorMessage('');
                setLoading(false);
                return;
            }

            const defaultMessage = isRecoveryMode
                ? 'Recovery failed. Please try again.'
                : isSignupMode
                    ? 'Registration failed. Please check your information.'
                    : 'Login failed. Please check your credentials.';
            setError(true);
            setErrorMessage(getFlowErrorMessage(data.error, defaultMessage));
            setLoading(false);

            // The server invalidates the flow session on ERROR, so clear stale state
            // and restart to get a fresh session with valid executionId/challengeToken.
            sessionStorage.removeItem(EXECUTION_ID_KEY);
            sessionStorage.removeItem(CHALLENGE_TOKEN_KEY);
            if (isRecoveryMode) {
                initRecoveryRef.current?.(false);
            } else {
                initRef.current?.(isSignupMode);
            }
            return;
        }

        // Clear previous state, but preserve error if it exists in the new response
        clearToken();
        const hasNewError = !!data.error;

        if (hasNewError) {
             setError(true);
             setErrorMessage(getFlowErrorMessage(data.error));
        } else {
             setError(false);
             setConnectionError(false);
        }
        
        setNeedsDecision(false);
        setFormData({});
        setAvailableActions([]);
        setInputs([]);
        setRedirectURL(null);
        setSocialIdpName('');
        setRegOnlySuccess(false);
        setPasskeyCreationOptions(null);
        setPasskeyChallenge(null);

        if (data.flowStatus && data.flowStatus === 'COMPLETE') {
            setError(false);
            if (data.assertion) {
                setToken(data.assertion);
            } else {
                setRegOnlySuccess(true);
            }
        } else if (data.type === "VIEW") {
            // Check for passkey creation options in additionalData - check this first
            if (data.data?.additionalData?.passkeyCreationOptions) {
                setPasskeyCreationOptions(data.data.additionalData.passkeyCreationOptions);
            }
            // Check for passkey authentication challenge in additionalData - check this first
            if (data.data?.additionalData?.passkeyChallenge) {
                setPasskeyChallenge(data.data.additionalData.passkeyChallenge);
            }

            if (data.data?.additionalData?.emailSent === "true") {
                setEmailSent(true);
                setLoading(false);
                return;
            }

            const hasHiddenInviteToken = data.data?.inputs?.some(
                (input: AuthInput) => input.identifier === 'inviteToken' && input.type === 'HIDDEN'
            );
            if (hasHiddenInviteToken) {
                setEmailSent(true);
                setLoading(false);
                return;
            }

            // Check if this is an input prompt (has inputs to collect)
            if (data.data?.inputs && data.data.inputs.length > 0) {
                // This is an input prompt - show input fields
                setNeedsDecision(false);
                data.data.inputs.forEach((input: AuthInput) => {
                    setInputs(prev => [...prev, input]);
                });
                // Also store actions for form submission
                if (data.data?.actions) {
                    setAvailableActions(data.data.actions);
                }
            } else if (data.data?.actions && data.data.actions.length > 1) {
                // This is a decision screen - multiple actions to choose from
                setNeedsDecision(true);
                setAvailableActions(data.data.actions);
            } else if (data.data?.actions && data.data.actions.length === 1 && !data.error) {
                // Single action without inputs - auto-execute it to continue the flow
                // This handles intermediate steps like "send_sms" that don't need user input
                const singleAction = data.data.actions[0];
                setLoading(true);
                submitAuthDecision(data.executionId || '', singleAction.ref, undefined, data.challengeToken)
                    .then((result) => {
                        processAuthResponse(result.data, singleAction.ref);
                    })
                    .catch((error) => {
                        console.error("Error auto-executing single action:", error);
                        setError(true);
                        setErrorMessage('An error occurred. Please try again.');
                        setLoading(false);
                    });
                return; // Don't set loading to false yet
            }
        } else if (data.type === "REDIRECTION") {
            // Handle redirection for social logins
            const url = data.data?.redirectURL;
            const idpName = data.data?.additionalData?.idpName || 'Social Login';

            if (isCameFromDecision) {
                // If this is a decision screen, handle the social login click.
                // Pass the new challenge token directly — the React state update for
                // challengeToken hasn't flushed yet, so the closed-over value is stale.
                handleSocialLoginClick(url || '', data.challengeToken || '');
                return;
            }
            
            if (url) {
                // Store the redirect URL instead of redirecting immediately
                setRedirectURL(url);
                setSocialIdpName(idpName);
            }
        }

        setLoading(false);
    }, [needsDecision, isRecoveryMode, isSignupMode, clearToken, setToken, handleSocialLoginClick]);

    // Handle when user selects an authentication option
    const handleAuthOptionSelection = (actionId: string) => {
        setLoading(true);
        setSelectedAction(actionId);

        submitAuthDecision(executionId, actionId, undefined, challengeToken)
            .then((result) => {
                processAuthResponse(result.data, undefined, true);
            })
            .catch((error) => {
                console.error("Error during authentication decision:", error);
                setError(true);
                setErrorMessage(error.message || 'Error processing your selection');
                setLoading(false);
            });
    };

    const init = useCallback((isSignupMode: boolean = false) => {
        clearToken();
        sessionStorage.removeItem(SIGNUP_MODE_KEY);
        setConnectionError(false);
        setNeedsDecision(false);
        setAvailableActions([]);
        setSelectedAction(null);
        setFormData({});
        setInputs([]);
        // Reset redirect URL
        setRedirectURL(null);
        setSocialIdpName('');
        setRegOnlySuccess(false);
        setPasskeyCreationOptions(null);
        setPasskeyChallenge(null);
        setIsRecoveryMode(false);
        setEmailSent(false);

        initiateNativeAuthFlow(isSignupMode ? 'REGISTRATION' : 'LOGIN')
            .then((result) => {
                const data = result.data;

                if (data.flowStatus && data.flowStatus === 'COMPLETE' && data.assertion) {
                    setToken(data.assertion);
                    setError(false);
                } else if (data.flowStatus && data.flowStatus === 'ERROR') {
                    const defaultMessage = isSignupMode
                        ? 'Registration failed. Please check your information.'
                        : 'Login failed. Please check your credentials.';
                    setError(true);
                    setErrorMessage(getFlowErrorMessage(data.error, defaultMessage));
                } else if (data.type === "VIEW") {
                    // Check for passkey creation options in additionalData - check this first
                    if (data.data?.additionalData?.passkeyCreationOptions) {
                        setPasskeyCreationOptions(data.data.additionalData.passkeyCreationOptions);
                    }
                    // Check for passkey authentication challenge in additionalData - check this first
                    if (data.data?.additionalData?.passkeyChallenge) {
                        setPasskeyChallenge(data.data.additionalData.passkeyChallenge);
                    }

                    // Handle the VIEW response
                    // Check if this is an input prompt (has inputs to collect)
                    if (data.data?.inputs && data.data.inputs.length > 0) {
                        // This is an input prompt - show input fields
                        setNeedsDecision(false);
                        data.data.inputs.forEach((input: AuthInput) => {
                            setInputs(prev => [...prev, input]);
                        });
                        // Also store actions for form submission
                        if (data.data?.actions) {
                            setAvailableActions(data.data.actions);
                        }
                    } else if (data.data?.actions && data.data.actions.length > 1) {
                        // This is a decision screen - multiple actions to choose from
                        setNeedsDecision(true);
                        setAvailableActions(data.data.actions);
                    } else if (data.data?.actions && data.data.actions.length === 1) {
                        // Single action without inputs - auto-execute it
                        const singleAction = data.data.actions[0];
                        submitAuthDecision(data.executionId, singleAction.ref, undefined, data.challengeToken)
                            .then((result) => {
                                processAuthResponse(result.data, singleAction.ref);
                            })
                            .catch((error) => {
                                console.error("Error auto-executing single action:", error);
                                setError(true);
                                setErrorMessage('An error occurred. Please try again.');
                                setLoading(false);
                            });
                        return; // Don't set loading to false yet
                    }
                } else if (data.type === "REDIRECTION") {
                    // Handle redirection for social logins
                    const url = data.data?.redirectURL;
                    const idpName = data.data?.additionalData?.idpName || 'Social Login';

                    if (url) {
                        // Store the redirect URL instead of redirecting immediately
                        setRedirectURL(url);
                        setSocialIdpName(idpName);
                    }
                }

                setExecutionId(data.executionId);
                setChallengeToken(data.challengeToken || '');
                setLoading(false);
            }).catch((error) => {
                const errorType = isSignupMode ? "registration" : "auth";
                console.error(`Error during ${errorType} initialization:`, error);
                setConnectionError(true);
                setLoading(false);
            });
    }, [clearToken, processAuthResponse, setToken]);

    const initRecovery = useCallback((resetErrorState: boolean = true) => {
        clearToken();
        sessionStorage.removeItem(SIGNUP_MODE_KEY);
        setConnectionError(false);
        setNeedsDecision(false);
        setAvailableActions([]);
        setSelectedAction(null);
        setFormData({});
        setInputs([]);
        setRedirectURL(null);
        setSocialIdpName('');
        setRegOnlySuccess(false);
        setPasskeyCreationOptions(null);
        setPasskeyChallenge(null);
        setIsSignupMode(false);
        setIsRecoveryMode(true);
        setEmailSent(false);
        setLoading(true);

        if (resetErrorState) {
            setError(false);
            setErrorMessage('');
        }

        initiateNativeAuthFlow('RECOVERY')
            .then((result) => {
                const data = result.data;

                if (data.flowStatus === 'ERROR') {
                    setError(true);
                    setErrorMessage(getFlowErrorMessage(data.error, 'Failed to start recovery. Please try again.'));
                } else if (data.type === 'VIEW' && data.data?.inputs) {
                    data.data.inputs.forEach((input: AuthInput) => {
                        setInputs(prev => [...prev, input]);
                    });
                    if (data.data.actions) {
                        setAvailableActions(data.data.actions);
                    }
                }

                setExecutionId(data.executionId);
                setChallengeToken(data.challengeToken || '');
                setLoading(false);
            })
            .catch((nextError: Error) => {
                console.error('Error during recovery initialization:', nextError);
                if (isConnectionFailure(nextError)) {
                    setConnectionError(true);
                } else {
                    setError(true);
                    setErrorMessage(nextError.message || 'Failed to start recovery. Please try again.');
                }
                setLoading(false);
            });
    }, [clearToken]);

    useEffect(() => { initRef.current = init; }, [init]);
    useEffect(() => { initRecoveryRef.current = initRecovery; }, [initRecovery]);

    // Initialize the prompt signup decision action
    const initPromptSignupDecision = () => {
        clearToken();
        setConnectionError(false);
        setNeedsDecision(false);
        setAvailableActions([]);
        // Reset redirect URL
        setRedirectURL(null);
        setSocialIdpName('');
        setRegOnlySuccess(false);
        setPromptRegistration(false);
        setPasskeyCreationOptions(null);
        setPasskeyChallenge(null);

        // Ensure all input fields are present in formData, even if empty
        const completeFormData = { ...formData };
        inputs.forEach(input => {
            if (!(input.identifier in completeFormData)) {
                completeFormData[input.identifier] = '';
            }
        });

        initiateNativeAuthFlowWithData('REGISTRATION', selectedAction, completeFormData)
            .then((result) => {
                setInputs([]);
                const data = result.data;

                if (data.flowStatus && data.flowStatus === 'COMPLETE' && data.assertion) {
                    setToken(data.assertion);
                    setError(false);
                } else if (data.flowStatus && data.flowStatus === 'ERROR') {
                    setError(true);
                    setErrorMessage(getFlowErrorMessage(data.error, 'Registration failed. Please check your information.'));
                } else if (data.type === "VIEW") {
                    // Check for passkey creation options in additionalData - check this first
                    if (data.data?.additionalData?.passkeyCreationOptions) {
                        setPasskeyCreationOptions(data.data.additionalData.passkeyCreationOptions);
                    }
                    // Check for passkey authentication challenge in additionalData - check this first
                    if (data.data?.additionalData?.passkeyChallenge) {
                        setPasskeyChallenge(data.data.additionalData.passkeyChallenge);
                    }

                    // Handle the VIEW response
                    // Check if this is an input prompt (has inputs to collect)
                    if (data.data?.inputs && data.data.inputs.length > 0) {
                        // This is an input prompt - show input fields
                        setNeedsDecision(false);
                        data.data.inputs.forEach((input: AuthInput) => {
                            setInputs(prev => [...prev, input]);
                        });
                        // Also store actions for form submission
                        if (data.data?.actions) {
                            setAvailableActions(data.data.actions);
                        }
                    } else if (data.data?.actions && data.data.actions.length > 1) {
                        // This is a decision screen - multiple actions to choose from
                        setNeedsDecision(true);
                        setAvailableActions(data.data.actions);
                    } else if (data.data?.actions && data.data.actions.length === 1) {
                        // Single action without inputs - auto-execute it
                        const singleAction = data.data.actions[0];
                        submitAuthDecision(data.executionId, singleAction.ref, undefined, data.challengeToken)
                            .then((result) => {
                                processAuthResponse(result.data, singleAction.ref);
                            })
                            .catch((error) => {
                                console.error("Error auto-executing single action:", error);
                                setError(true);
                                setErrorMessage('An error occurred. Please try again.');
                                setLoading(false);
                            });
                        return; // Don't set loading to false yet
                    }
                } else if (data.type === "REDIRECTION") {
                    // Handle redirection for social logins
                    const url = data.data?.redirectURL;
                    const idpName = data.data?.additionalData?.idpName || 'Social Login';

                    if (url) {
                        // Store the redirect URL instead of redirecting immediately
                        setRedirectURL(url);
                        setSocialIdpName(idpName);
                    }
                }

                setExecutionId(data.executionId);
                setChallengeToken(data.challengeToken || '');
                setLoading(false);
            }).catch((error) => {
                console.error(`Error during user registration:`, error);
                setInputs([]);
                setConnectionError(true);
                setLoading(false);
            });
    };

    // Unified form submission handler that works for both decisions and direct inputs
    const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        setLoading(true);

        // Ensure all input fields are present in formData, even if empty
        const completeFormData = { ...formData };
        inputs.forEach(input => {
            if (!(input.identifier in completeFormData)) {
                completeFormData[input.identifier] = '';
            }
        });

        const isMobileInput = inputs.some(input => input.identifier === "mobileNumber");

        if (needsDecision) {
            // This is a decision submission - identify the action from form data
            const formAction = event.currentTarget.getAttribute('data-action-id');
            if (formAction) {
                setSelectedAction(formAction);
                submitAuthDecision(executionId, formAction, completeFormData, challengeToken)
                    .then((result) => {
                        processAuthResponse(result.data, formAction);
                    })
                    .catch((error) => {
                        console.error("Error during authentication decision:", error);
                        handleSubmissionError(error);
                    });
            }
        } else {
            // This is a direct input submission - include action if available
            const actionRef = availableActions.length > 0 ? availableActions[0].ref : undefined;
            submitNativeAuth(executionId, completeFormData, actionRef, challengeToken)
                .then((result) => {
                    if (isMobileInput) {
                        processAuthResponse(result.data, "mobile");
                    } else {
                        processAuthResponse(result.data);
                    }
                })
                .catch((error) => {
                    console.error("Error during authentication:", error);
                    handleSubmissionError(error);
                });
        }
    };

    const handleSubmissionError = useCallback((error: Error) => {
        if (isConnectionFailure(error)) {
            setConnectionError(true);
            setLoading(false);
        } else if (error instanceof FlowServerError) {
            setError(true);
            setErrorMessage(error.message || 'An unexpected server error occurred. Please try again.');
            sessionStorage.removeItem(EXECUTION_ID_KEY);
            sessionStorage.removeItem(CHALLENGE_TOKEN_KEY);
            sessionStorage.removeItem(SIGNUP_MODE_KEY);
            sessionStorage.setItem(START_INIT_KEY, 'true');
            setLoading(false);
            if (isRecoveryMode) {
                initRecoveryRef.current?.(false);
            } else {
                initRef.current?.(isSignupMode);
            }
        } else {
            setError(true);
            setErrorMessage(error.message || 'Error during authentication');
            setLoading(false);
        }
    }, [isRecoveryMode, isSignupMode]);

    // Handler for passkey credential creation
    const handlePasskeyCredentialCreated = (credential: PasskeyCredentialResponse) => {
        setLoading(true);
        setPasskeyCreationOptions(null);

        // Submit passkey credential to complete the flow
        const passkeyInputs = {
            credentialId: credential.credentialId,
            clientDataJSON: credential.clientDataJSON,
            attestationObject: credential.attestationObject,
        };

        const actionRef = availableActions.length > 0 ? availableActions[0].ref : undefined;
        submitNativeAuth(executionId, passkeyInputs, actionRef, challengeToken)
            .then((result) => {
                processAuthResponse(result.data);
            })
            .catch((error) => {
                console.error("Error submitting passkey credential:", error);
                setConnectionError(false);
                setNeedsDecision(false);
                setAvailableActions([]);
                handleSubmissionError(error);
            });
    };

    // Handler for passkey creation error
    const handlePasskeyError = (errorMessage: string) => {
        setError(true);
        setErrorMessage(errorMessage);
        setLoading(false);
    };

    // Handler for passkey authentication (assertion) completion
    const handlePasskeyAssertionCompleted = (assertion: PasskeyAssertionResponse) => {
        setLoading(true);
        setPasskeyChallenge(null);

        // Submit passkey assertion to complete the authentication flow
        const passkeyInputs = {
            credentialId: assertion.credentialId,
            clientDataJSON: assertion.clientDataJSON,
            authenticatorData: assertion.authenticatorData,
            signature: assertion.signature,
            userHandle: assertion.userHandle,
        };

        // Include action ref if available (consistent with other direct submissions)
        const actionRef = availableActions.length > 0 ? availableActions[0].ref : undefined;
        submitNativeAuth(executionId, passkeyInputs, actionRef, challengeToken)
            .then((result) => {
                processAuthResponse(result.data);
            })
            .catch((error) => {
                console.error("Error submitting passkey assertion:", error);
                setNeedsDecision(false);
                setAvailableActions([]);
                setSelectedAction(null);
                setFormData({});
                setInputs([]);
                handleSubmissionError(error);
            });
    };

    const handleRetry = () => {
        setTimeout(() => {
            if (isRecoveryMode) {
                initRecoveryRef.current?.(false);
            } else {
                init();
            }
        }, 500);
    };
    
    // Helper function to get appropriate icon for social login
    const getSocialLoginIcon = (idpName: string) => {
        const lowerIdpName = idpName.toLowerCase();
        
        if (lowerIdpName.includes('github')) {
            return <GitHubIcon />;
        } else if (lowerIdpName.includes('google')) {
            return <GoogleIcon />;
        } else {
            return <AccountCircleIcon />;
        }
    };

    // Get social login button text
    const getSocialLoginText = (actionId: string) => {
        const prefix = isSignupMode ? 'Sign up' : 'Continue';
        
        if (actionId.includes('google')) {
            return `${prefix} with Google`;
        } else if (actionId.includes('github')) {
            return `${prefix} with GitHub`;
        } else if (actionId.includes('mobile') || actionId.includes('sms')) {
            return `${prefix} with SMS OTP`;
        } else {
            const idpText = actionId.split('_').map(word => 
                word.charAt(0).toUpperCase() + word.slice(1)
            ).join(' ');
            return `${prefix} with ${idpText}`;
        }
    };

    const isMobileAction = (action?: ActionPrompt) =>
        !!action?.ref && (action.ref.includes("mobile") || action.ref.includes("sms"));

    const usesMobileNumberField = (action?: ActionPrompt) =>
        !!action && (action.ref.includes("prompt_mobile") || action.nextNode === "prompt_mobile");

    // This effect is to handle initial component mount
    useEffect(() => {
        // Prevent double mount due to React Strict Mode
        if (isComponentReMount.current) return;
        isComponentReMount.current = true;

        if (startInit) {
            // Initialize login execution flow if fresh start
            init();
        } else {
            // This effect is to handle when return from federated IDP login
            const params = new URLSearchParams(window.location.search);
            const code = params.get('code');

            if (code) {
                // Clear query parameters to avoid re-submission
                window.history.replaceState({}, document.title, window.location.pathname);

                submitNativeAuth(executionId, { type: NativeAuthSubmitType.SOCIAL, code: code }, undefined, challengeToken)
                    .then((result) => {
                        processAuthResponse(result.data);
                    }).catch((error) => {
                        console.error("Error during social authentication:", error);
                        handleSubmissionError(error);
                    });
            } else {
                setError(true);
                setLoading(false);
            }

            sessionStorage.setItem(START_INIT_KEY, "true");
        }
    }, [startInit, init, executionId, challengeToken, processAuthResponse, handleSubmissionError]);

    // Render input fields based on the current inputs array
    const renderInputFields = () => {
        return inputs.map((input, index) => {
            const inputId = input.identifier || `input-${index}`;
            const isPassword = input.type === "password" || input.type === "PASSWORD_INPUT" || input.identifier === "password";
            const isOTP = input.type === "otp" || input.type === "OTP_INPUT" || input.identifier === "otp";
            const isDropdown = input.type === "SELECT" || input.type === "dropdown" || input.type === "DROPDOWN";
            const isRequired = input.required;
            
            // Determine appropriate label
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
            } else if (isPassword) {
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
            } else if (isOTP) {
                return (
                    <Box key={inputId} display="flex" flexDirection="column" gap={0.5}>
                        <InputLabel htmlFor={inputId} sx={{ mb: 1 }}>{label}</InputLabel>
                        <Box 
                            sx={{ 
                                display: 'flex', 
                                gap: 1,
                                justifyContent: 'space-between'
                            }}
                        >
                            {otpDigits.map((digit, index) => (
                                <OutlinedInput
                                    key={`otp-digit-${index}`}
                                    inputRef={el => otpInputRefs.current[index] = el}
                                    value={digit}
                                    onChange={(e) => handleOTPDigitChange(index, e.target.value)}
                                    onKeyDown={(e) => handleOTPKeyDown(index, e as React.KeyboardEvent<HTMLInputElement>)}
                                    onPaste={index === 0 ? handlePaste : undefined}
                                    inputProps={{
                                        maxLength: 1,
                                        style: { textAlign: 'center', padding: '8px 0' }
                                    }}
                                    sx={{
                                        width: '40px',
                                        height: '48px',
                                        '& input': { padding: 0 }
                                    }}
                                />
                            ))}
                        </Box>
                    </Box>
                );
            } else {
                return (
                    <Box key={inputId} display="flex" flexDirection="column" gap={0.5}>
                        <InputLabel htmlFor={inputId} sx={{ mb: 1 }}>{label}</InputLabel>
                        <OutlinedInput
                            type={input.type || "text"}
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
            }
        });
    };

    // Render the login form with side-by-side layout based on the available actions
    const renderSideBySideLoginForm = () => {
        const basicAuthAction = availableActions.find(action => action.ref?.includes("basic_auth"));
        const mobileAuthActions = availableActions.filter(isMobileAction);
        
        const hasSocialAuth = availableActions.some(action => 
            action.ref?.includes("google") || action.ref?.includes("github")
        );
        const hasMobileAuth = mobileAuthActions.length > 0;
        
        const socialAuthActions = availableActions.filter(action =>
            action.ref?.includes("google") || action.ref?.includes("github")
        );

        const otherActions = availableActions.filter(action =>
            action !== basicAuthAction &&
            !socialAuthActions.includes(action) &&
            !mobileAuthActions.includes(action)
        );

        return (
            <Box sx={{ my: 4 }}>
                <Box display="flex" gap={4}>
                    {/* Left: Basic Login */}
                    <Box sx={{ flex: 1 }}>
                        <form onSubmit={handleSubmit} data-action-id={basicAuthAction?.ref}>
                            <Box display="flex" flexDirection="column" gap={2}  sx={{ mb: 2, mt: 6.8 }}>
                            </Box>
                            <Box display="flex" flexDirection="column" gap={2} sx={{ mt: 3 }}>
                                <Box display="flex" flexDirection="column" gap={0.5}>
                                    <InputLabel htmlFor="username">Username</InputLabel>
                                    <OutlinedInput
                                        type="text"
                                        id="username"
                                        name="username"
                                        placeholder="Enter your username"
                                        size="small"
                                        value={formData.username || ''}
                                        onChange={handleInputChange}
                                        required
                                    />
                                </Box>
                                <Box display="flex" flexDirection="column" gap={0.5} sx={{ mt: 1 }}>
                                    <InputLabel htmlFor="password">Password</InputLabel>
                                    <OutlinedInput
                                        type={showPassword ? 'text' : 'password'}
                                        id="password"
                                        name="password"
                                        placeholder="Enter your password"
                                        size="small"
                                        value={formData.password || ''}
                                        onChange={handleInputChange}
                                        required
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

                                {(showRememberMe || showForgotPassword) && (
                                <Box display="flex" justifyContent="space-between" alignItems="center">
                                    {showRememberMe && (
                                        <FormControlLabel
                                            control={<Checkbox name="remember-me-checkbox" />}
                                            label="Remember me"
                                        />
                                    )}
                                    {showForgotPassword && (
                                        <Link href="#" onClick={(e) => { e.preventDefault(); initRecovery(); }} underline="hover">
                                            Forgot your password?
                                        </Link>
                                    )}
                                </Box>
                                )}

                                <Button
                                    variant="contained"
                                    color="primary"
                                    type="submit"
                                    fullWidth
                                    sx={{ mt: 3 }}
                                    >
                                    {isSignupMode ? 'Create Account' : 'Sign In'}
                                </Button>
                            </Box>
                        </form>
                    </Box>

                    {/* Vertical Divider */}
                    <Divider orientation="vertical" flexItem sx={{ mx: 2 }} />

                    {/* Right: Social Auth and SMS Options */}
                    <Box sx={{ flex: 1 }}>
                        {/* Social auth options */}
                        {hasSocialAuth && (
                            <Box>
                                {socialAuthActions.map((action, index) => (
                                    <Button
                                        key={`social-action-${index}`}
                                        fullWidth
                                        variant="contained"
                                        color="secondary"
                                        onClick={() => handleAuthOptionSelection(action.ref)}
                                        sx={{ my: 1 }}
                                        startIcon={getSocialLoginIcon(action.ref || '')}
                                    >
                                        {getSocialLoginText(action.ref || '')}
                                    </Button>
                                ))}
                            </Box>
                        )}

                        {/* Show divider if we have both social and sms auth options */}
                        {hasMobileAuth && hasSocialAuth && (
                            <Divider sx={{ my: 3 }}>or</Divider>
                        )}

                        {/* SMS OTP Auth */}
                        {hasMobileAuth && (
                            <form
                                onSubmit={handleSubmit}
                                data-action-id={mobileAuthActions[0]?.ref}
                            >
                                <Box display="flex" flexDirection="column" gap={2}>
                                    <Box display="flex" flexDirection="column" gap={0.5}>
                                        <InputLabel htmlFor={usesMobileNumberField(mobileAuthActions[0]) ? "mobileNumber" : "username"}>
                                            {usesMobileNumberField(mobileAuthActions[0]) ? "Mobile Number" : "Username"}
                                        </InputLabel>
                                        <OutlinedInput
                                            type="text"
                                            id={usesMobileNumberField(mobileAuthActions[0]) ? "mobileNumber" : "username"}
                                            name={usesMobileNumberField(mobileAuthActions[0]) ? "mobileNumber" : "username"}
                                            placeholder={`Enter your ${usesMobileNumberField(mobileAuthActions[0]) ? "mobile number" : "username"}`}
                                            size="small"
                                            value={formData[usesMobileNumberField(mobileAuthActions[0]) ? "mobileNumber" : "username"] || ''}
                                            onChange={handleInputChange}
                                            required
                                        />
                                    </Box>
                                    <Button
                                        variant="contained"
                                        color="primary"
                                        type="submit"
                                        fullWidth
                                        sx={{ mt: 2 }}
                                    >
                                        Continue with SMS OTP
                                    </Button>
                                </Box>
                            </form>
                        )}

                        {otherActions.length > 0 && (
                            <Box>
                                {(hasSocialAuth || hasMobileAuth) && (
                                    <Divider sx={{ my: 3 }}>or</Divider>
                                )}
                                {otherActions.map((action, index) => (
                                    <Button
                                        key={`other-action-${index}`}
                                        fullWidth
                                        variant="contained"
                                        onClick={() => handleAuthOptionSelection(action.ref)}
                                        sx={{ my: 1 }}
                                    >
                                        {action.ref || 'Continue'}
                                    </Button>
                                ))}
                            </Box>
                        )}
                    </Box>
                </Box>
            </Box>
        );
    }

    // Render the regular login form with options stacked vertically
    const renderRegularLoginForm = () => {
        const basicAuthAction = availableActions.find(action => action.ref?.includes("basic_auth"));
        const mobileAuthActions = availableActions.filter(isMobileAction);
        
        const hasBasicAuth = !!basicAuthAction;
        const hasSocialAuth = availableActions.some(action => 
            action.ref?.includes("google") || action.ref?.includes("github")
        );
        const hasMobileAuth = mobileAuthActions.length > 0;
        
        const socialAuthActions = availableActions.filter(action =>
            action.ref?.includes("google") || action.ref?.includes("github")
        );

        const otherActions = availableActions.filter(action =>
            action !== basicAuthAction &&
            !socialAuthActions.includes(action) &&
            !mobileAuthActions.includes(action)
        );

        return (
            <Box sx={{ my: 2 }}>
                {/* Social auth options */}
                {hasSocialAuth && (
                    <Box>
                        {socialAuthActions.map((action, index) => (
                            <Button
                                key={`social-action-${index}`}
                                fullWidth
                                variant="contained"
                                color="secondary"
                                onClick={() => handleAuthOptionSelection(action.ref)}
                                sx={{ my: 1 }}
                                startIcon={getSocialLoginIcon(action.ref || '')}
                            >
                                {getSocialLoginText(action.ref || '')}
                            </Button>
                        ))}
                    </Box>
                )}
                
                {/* Show divider if we have multiple auth options */}
                {((hasSocialAuth && hasBasicAuth) || (hasSocialAuth && hasMobileAuth)) && (
                    <Divider sx={{ my: 3 }}>or</Divider>
                )}
                
                {/* Basic auth form */}
                {hasBasicAuth && (
                    <form onSubmit={handleSubmit} data-action-id={basicAuthAction?.ref}>
                        <Box display="flex" flexDirection="column" gap={2}>
                            <Box display="flex" flexDirection="column" gap={0.5}>
                                <InputLabel htmlFor="username">Username</InputLabel>
                                <OutlinedInput
                                    type="text"
                                    id="username"
                                    name="username"
                                    placeholder="Enter your username"
                                    size="small"
                                    value={formData.username || ''}
                                    onChange={handleInputChange}
                                    required
                                />
                            </Box>
                            <Box display="flex" flexDirection="column" gap={0.5}>
                                <InputLabel htmlFor="password">Password</InputLabel>
                                <OutlinedInput
                                    type={showPassword ? 'text' : 'password'}
                                    id="password"
                                    name="password"
                                    placeholder="Enter your password"
                                    size="small"
                                    value={formData.password || ''}
                                    onChange={handleInputChange}
                                    required
                                    endAdornment={
                                        <InputAdornment position="end">
                                            <IconButton
                                                aria-label="toggle password visibility"
                                                onClick={handleTogglePasswordVisibility}
                                                onMouseDown={handleMouseDownPassword}
                                                edge="end"
                                            >
                                                {showPassword ? 
                                                    <VisibilityOff /> : <Visibility />
                                                }
                                            </IconButton>
                                        </InputAdornment>
                                    }
                                />
                            </Box>
                            {(showRememberMe || showForgotPassword) && (
                                <Box
                                    sx={{
                                    display: 'flex',
                                    justifyContent: 'space-between',
                                    alignItems: 'center',
                                    }}
                                >
                                    { showRememberMe && (
                                        <FormControlLabel
                                            control={<Checkbox name="remember-me-checkbox" />} 
                                            label="Remember me" />
                                    )}
                                    {showForgotPassword && (
                                        <Link href="#" onClick={(e) => { e.preventDefault(); initRecovery(); }} underline="hover">
                                            Forgot your password?
                                        </Link>
                                    )}
                                </Box>
                            )}
                            <Button
                                variant="contained"
                                color="primary"
                                type="submit"
                                fullWidth
                                sx={{ mt: 2 }}
                            >
                                {isSignupMode ? 'Create Account' : 'Sign In'}
                            </Button>
                        </Box>
                    </form>
                )}

                {/* Show divider if we have multiple auth options */}
                {(hasBasicAuth && hasMobileAuth) && (
                    <Divider sx={{ my: 3 }}>or</Divider>
                )}

                {/* SMS OTP auth form */}
                {hasMobileAuth && (
                    <form
                        onSubmit={handleSubmit}
                        data-action-id={mobileAuthActions[0]?.ref}
                    >
                        <Box display="flex" flexDirection="column" gap={2}>
                            <Box display="flex" flexDirection="column" gap={0.5}>
                                <InputLabel htmlFor={usesMobileNumberField(mobileAuthActions[0]) ? "mobileNumber" : "username"}>
                                    {usesMobileNumberField(mobileAuthActions[0]) ? "Mobile Number" : "Username"}
                                </InputLabel>
                                <OutlinedInput
                                    type="text"
                                    id={usesMobileNumberField(mobileAuthActions[0]) ? "mobileNumber" : "username"}
                                    name={usesMobileNumberField(mobileAuthActions[0]) ? "mobileNumber" : "username"}
                                    placeholder={`Enter your ${usesMobileNumberField(mobileAuthActions[0]) ? "mobile number" : "username"}`}
                                    size="small"
                                    value={formData[usesMobileNumberField(mobileAuthActions[0]) ? "mobileNumber" : "username"] || ''}
                                    onChange={handleInputChange}
                                    required
                                />
                            </Box>
                            <Button
                                variant="contained"
                                color="primary"
                                type="submit"
                                fullWidth
                                sx={{ mt: 2 }}
                            >
                                Continue with SMS OTP
                            </Button>
                        </Box>
                    </form>
                )}

                {otherActions.length > 0 && (
                    <Box>
                        {(hasBasicAuth || hasSocialAuth || hasMobileAuth) && (
                            <Divider sx={{ my: 3 }}>or</Divider>
                        )}
                        {otherActions.map((action, index) => (
                            <Button
                                key={`other-action-${index}`}
                                fullWidth
                                variant="contained"
                                onClick={() => handleAuthOptionSelection(action.ref)}
                                sx={{ my: 1 }}
                            >
                                {action.ref || 'Continue'}
                            </Button>
                        ))}
                    </Box>
                )}
            </Box>
        );
    }

    const renderInputPromptForm = () => {
        return (
            <form onSubmit={handleSubmit}>
                <Box display="flex" flexDirection="column" gap={2}>
                    {renderInputFields()}
                    
                    {(showRememberMe || showForgotPassword) && (
                        <Box
                            sx={{
                            display: 'flex',
                            justifyContent: 'space-between',
                            alignItems: 'center',
                            }}
                        >
                            { showRememberMe && (
                                <FormControlLabel
                                    control={<Checkbox name="remember-me-checkbox" />} 
                                    label="Remember me" />
                            )}
                            {showForgotPassword && (
                                <Link href="#" onClick={(e) => { e.preventDefault(); initRecovery(); }} underline="hover">
                                    Forgot your password?
                                </Link>
                            )}
                        </Box>
                    )}

                    {inputs.length > 0 && (() => {
                        const primaryRef = availableActions[0]?.ref?.toLowerCase() || '';
                        let label: string;
                        if (primaryRef.includes('signin') || primaryRef.includes('sign_in')) {
                            label = 'Sign In';
                        } else if (primaryRef.includes('signup') || primaryRef.includes('sign_up')) {
                            label = 'Create Account';
                        } else if (inputs.some(input => input.identifier === 'password' || input.type === 'PASSWORD_INPUT')) {
                            label = isRecoveryMode ? 'Reset Password' : isSignupMode ? 'Create Account' : 'Sign In';
                        } else if (inputs.some(input => input.identifier === 'otp' || input.type === 'OTP_INPUT')) {
                            label = 'Verify OTP';
                        } else {
                            label = 'Continue';
                        }
                        return (
                            <Button variant="contained" color="primary" type="submit" fullWidth sx={{ mt: 2 }}>
                                {label}
                            </Button>
                        );
                    })()}

                    {/* Render alternative actions if available (e.g. Passkey) */}
                    {availableActions.length > 1 && (
                         <Box sx={{ mt: 1 }}>
                            <Divider sx={{ my: 2 }}>or</Divider>
                            {availableActions.slice(1).map((action, index) => {
                                const isSocial = action.ref.includes("google") || action.ref.includes("github");
                                const isPasskey = action.ref.includes("passkey");
                                const isMobile = action.ref.includes("sms") || action.ref.includes("mobile");

                                let label = action.ref || "Continue";
                                if (isSocial) {
                                    label = getSocialLoginText(action.ref || '');
                                } else if (isMobile) {
                                    label = getSocialLoginText(action.ref || '');
                                } else if (isPasskey) {
                                    label = `${isSignupMode ? 'Sign up' : 'Continue'} with Passkey`;
                                } else if (action.label) {
                                    label = action.label;
                                }

                                return (
                                    <Button
                                        key={`alt-action-${index}`}
                                        fullWidth
                                        variant="contained"
                                        color="secondary"
                                        onClick={() => handleAuthOptionSelection(action.ref)}
                                        sx={{ mb: 1 }}
                                        startIcon={
                                            isSocial ? getSocialLoginIcon(action.ref || '')
                                            : isPasskey ? <FingerprintIcon />
                                            : undefined
                                        }
                                    >
                                        {label}
                                    </Button>
                                );
                            })}
                         </Box>
                    )}
                </Box>
            </form>
        );
    }

    // Render function for redirection scenarios
    const renderRedirectLoginButton = () => {
        if (!redirectURL) return null;
        
        const buttonText = socialIdpName ? 
            `Continue with ${socialIdpName}` :
            'Continue with Social Login';
        
        const icon = getSocialLoginIcon(socialIdpName);
        
        return (
            <Box sx={{ my: 2 }}>
                <Button
                    fullWidth
                    variant="contained"
                    color="secondary"
                    onClick={() => handleSocialLoginClick(redirectURL)}
                    sx={{ my: 1 }}
                    startIcon={icon}
                >
                    {buttonText}
                </Button>
            </Box>
        );
    };

    // Calculate appropriate grid size based on layout complexity
    const gridMdSize = needsDecision 
        && !promptRegistration
        && availableActions.some(action => action.ref?.includes("basic_auth"))
        && availableActions.some(action => action.ref?.includes("mobile") || action.ref?.includes("sms"))
        ? 10 : 6;
    const containerBoxMaxWidth = gridMdSize === 10 ? 1000 : 500;

    const basicAuthAction = availableActions.find(action => action.ref?.includes("basic_auth"));
    const mobileAuthActions = availableActions.filter(action =>
        action.ref?.includes("mobile") || action.ref?.includes("sms")
    );
    
    const hasBasicAuth = !!basicAuthAction;
    const hasMobileAuth = mobileAuthActions.length > 0;

    return (
        <Layout>
            { loading ? (
                <GradientCircularProgress />
            ) : (
                <Grid size={{ xs: 12, md: gridMdSize }}>
                    <Paper
                        sx={{
                            display: "flex",
                            width: "100%",
                            height: "100%",
                            flexDirection: "column",
                        }}
                    >
                        <Box
                            sx={{
                                alignItems: "center",
                                justifyContent: "center",
                                padding: 6,
                                width: "100%",
                                maxWidth: containerBoxMaxWidth,
                                margin: "auto",
                            }}
                        >
                            <Box>
                                {promptRegistration ? (
                                    <Box sx={{ mb: 4 }}>
                                        <Typography variant="h5" gutterBottom>
                                            We couldn&apos;t find your account
                                        </Typography>
                                        <Typography>
                                            No account matched your details. You can try again or sign up below.
                                        </Typography>
                                    </Box>
                                ) : regOnlySuccess ? (
                                    <Box sx={{ mb: 4 }}>
                                        <Typography variant="h5" gutterBottom>
                                            {isRecoveryMode ? 'Password Reset Successful' : 'Registration Successful'}
                                        </Typography>
                                        <Typography>
                                            You can now log in to your account.
                                        </Typography>
                                    </Box>
                                ) : !emailSent ? (
                                    <Box sx={{ mb: 4 }}>
                                        <Typography variant="h5" gutterBottom>
                                            {isRecoveryMode ? 'Reset Password' : isSignupMode ? 'Create Account' : 'Login to Account'}
                                        </Typography>

                                        <Typography>
                                            {isRecoveryMode ? (
                                                <>
                                                    Remember your password?{' '}
                                                    <Link
                                                        href="#"
                                                        onClick={(e) => {
                                                            e.preventDefault();
                                                            setError(false);
                                                            setErrorMessage('');
                                                            init(false);
                                                        }}
                                                        underline="hover"
                                                    >
                                                        Sign in!
                                                    </Link>
                                                </>
                                            ) : isSignupMode ? (
                                                <>
                                                    Already have an account?{' '}
                                                    <Link 
                                                        href="#" 
                                                        onClick={(e) => {
                                                            e.preventDefault();
                                                            setIsSignupMode(false);
                                                            setError(false);
                                                            setErrorMessage('');
                                                            init(false);
                                                        }}
                                                        underline="hover"
                                                    >
                                                        Sign in!
                                                    </Link>
                                                </>
                                            ) : (
                                                <>
                                                    Don&apos;t have an account?{' '}
                                                    <Link 
                                                        href="#" 
                                                        onClick={(e) => {
                                                            e.preventDefault();
                                                            setIsSignupMode(true);
                                                            setError(false);
                                                            setErrorMessage('');
                                                            init(true);
                                                        }}
                                                        underline="hover"
                                                    >
                                                        Sign up!
                                                    </Link>
                                                </>
                                            )}
                                        </Typography>
                                    </Box>
                                ) : (
                                    <Box sx={{ mb: 4 }} />
                                )}
                                
                                {connectionError && (
                                    <ConnectionErrorModal 
                                        onRetry={handleRetry}
                                        retryCount={retryCount}
                                        onRetryCountIncrement={() => setRetryCount(prev => prev + 1)}
                                    />
                                )}

                                {error && !connectionError && (
                                    <Alert severity="error" sx={{ my: 2 }}>
                                        {errorMessage}
                                    </Alert>
                                )}

                                {!connectionError && emailSent ? (
                                    <Box sx={{ textAlign: 'center', py: 2 }}>
                                        <Typography variant="h3" sx={{ mb: 2 }}>✉️</Typography>
                                        <Typography variant="h5" gutterBottom>
                                            Check Your Email
                                        </Typography>
                                        <Typography sx={{ mb: 3 }}>
                                            If an account with that username exists, we&apos;ve sent a password reset link to the associated email address.
                                        </Typography>
                                        <Button
                                            variant="outlined"
                                            fullWidth
                                            onClick={() => {
                                                setError(false);
                                                setErrorMessage('');
                                                init(false);
                                            }}
                                        >
                                            Back to Login
                                        </Button>
                                    </Box>
                                ) : !connectionError && (
                                    promptRegistration ? (
                                        <Box sx={{ mb: 4 }}>
                                            <Button
                                                variant="contained"
                                                color="primary"
                                                type="submit"
                                                fullWidth
                                                sx={{ mt: 2 }}
                                                onClick={(e) => {
                                                    e.preventDefault();
                                                    setIsSignupMode(true);
                                                    setError(false);
                                                    setErrorMessage('');
                                                    initPromptSignupDecision();
                                                }}
                                            >
                                                Sign Up
                                            </Button>
                                            <Button
                                                variant="contained"
                                                color="secondary"
                                                type="submit"
                                                fullWidth
                                                sx={{ mt: 2 }}
                                                onClick={(e) => {
                                                    e.preventDefault();
                                                    setPromptRegistration(false);
                                                    setError(false);
                                                    setErrorMessage('');
                                                    handleRetry();
                                                }}
                                            >
                                                Retry
                                            </Button>
                                        </Box>
                                    ) : !regOnlySuccess ? (
                                    <>
                                        {/* First check if we have a redirect URL */}
                                        {redirectURL ? (
                                            renderRedirectLoginButton()
                                        ) : passkeyCreationOptions ? (
                                            /* Show passkey creation prompt */
                                            <PasskeyRegPrompt
                                                passkeyCreationOptionsJson={passkeyCreationOptions}
                                                onCredentialCreated={handlePasskeyCredentialCreated}
                                                onError={handlePasskeyError}
                                                isLoading={loading}
                                            />
                                        ) : passkeyChallenge ? (
                                            /* Show passkey authentication prompt */
                                            <PasskeyAuthPrompt
                                                passkeyRequestOptionsJson={passkeyChallenge}
                                                onAuthenticated={handlePasskeyAssertionCompleted}
                                                onError={handlePasskeyError}
                                                isLoading={loading}
                                            />
                                        ) : needsDecision ? (
                                            /* If not redirect but needs decision */
                                            <>
                                                { hasBasicAuth && hasMobileAuth ? (
                                                    renderSideBySideLoginForm()
                                                ) : (
                                                    renderRegularLoginForm()
                                                )}
                                            </>
                                        ) : (
                                            /* If not redirect and not decision, it's an input prompt */
                                            renderInputPromptForm()
                                        )}
                                    </>
                                    ) : (
                                        <Button
                                            variant="contained"
                                            color="primary"
                                            onClick={() => {
                                                setIsSignupMode(false);
                                                setError(false);
                                                setErrorMessage('');
                                                init(false);
                                            }}
                                            fullWidth
                                        >
                                            Go to Login
                                        </Button>
                                    )
                                )}
                                
                                <Box component="footer" sx={{ mt: 6 }}>
                                    <Typography sx={{ textAlign: "center" }}>
                                        © Copyright {new Date().getFullYear()}
                                    </Typography>
                                </Box>
                            </Box>
                        </Box>
                    </Paper>
                </Grid>
            )}
        </Layout>
    );
};

export default LoginPage;
