// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

'use client';

import Alert from '@mui/material/Alert';
import Avatar from '@mui/material/Avatar';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import IconButton from '@mui/material/IconButton';
import InputAdornment from '@mui/material/InputAdornment';
import InputLabel from '@mui/material/InputLabel';
import Link from '@mui/material/Link';
import OutlinedInput from '@mui/material/OutlinedInput';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import ArrowBackRoundedIcon from '@mui/icons-material/ArrowBackRounded';
import ContentCopyRoundedIcon from '@mui/icons-material/ContentCopyRounded';
import Visibility from '@mui/icons-material/Visibility';
import VisibilityOff from '@mui/icons-material/VisibilityOff';
import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import Layout from '../components/Layout';
import useAuth from '../hooks/useAuth';
import {
    updateCurrentUserPassword,
    updateCurrentUserProfile,
} from '../services/userProfileService';

const defaultProfileFields = [
    'username',
    'name',
    'given_name',
    'family_name',
    'email',
    'phone',
    'phone_number',
    'picture',
] as const;

const fieldLabels: Record<string, string> = {
    username: 'Username',
    name: 'Display Name',
    given_name: 'First Name',
    family_name: 'Last Name',
    email: 'Email',
    phone: 'Phone',
    phone_number: 'Phone Number',
    picture: 'Picture URL',
};

const toFormValue = (value: unknown): string => {
    if (value === null || value === undefined) {
        return '';
    }

    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
        return String(value);
    }

    return '';
};

const isEditableScalar = (value: unknown) => (
    value === null ||
    value === undefined ||
    typeof value === 'string' ||
    typeof value === 'number' ||
    typeof value === 'boolean'
);

const buildFormState = (attributes: Record<string, unknown>) => {
    const nextState: Record<string, string> = {};

    defaultProfileFields.forEach((key) => {
        nextState[key] = toFormValue(attributes[key]);
    });

    Object.entries(attributes).forEach(([key, value]) => {
        if (isEditableScalar(value) && !(key in nextState)) {
            nextState[key] = toFormValue(value);
        }
    });

    return nextState;
};

const ProfilePage = () => {
    const { isAuthenticated, userProfile, refreshUserProfile } = useAuth();
    const router = useRouter();

    const [formState, setFormState] = useState<Record<string, string>>({});
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [savingPassword, setSavingPassword] = useState(false);
    const [showPassword, setShowPassword] = useState(false);
    const [isEditing, setIsEditing] = useState(false);
    const [error, setError] = useState('');
    const [successMessage, setSuccessMessage] = useState('');
    const [passwordState, setPasswordState] = useState({
        currentPassword: '',
        newPassword: '',
        confirmPassword: '',
    });

    useEffect(() => {
        if (!isAuthenticated) {
            setLoading(false);
            return;
        }

        setLoading(true);

        const loadProfile = async () => {
            try {
                const profile = userProfile ?? await refreshUserProfile();
                if (profile) {
                    setFormState(buildFormState(profile.attributes));
                } else {
                    setError('Failed to load user profile.');
                }
            } catch (loadError) {
                setError(loadError instanceof Error ? loadError.message : 'Failed to load user profile.');
            } finally {
                setLoading(false);
            }
        };

        void loadProfile();
    }, [refreshUserProfile, isAuthenticated, userProfile]);

    useEffect(() => {
        if (userProfile && !isEditing) {
            setFormState(buildFormState(userProfile.attributes));
        }
    }, [isEditing, userProfile]);

    const editableFields = useMemo(() => {
        return Object.keys(formState);
    }, [formState]);

    const complexAttributes = useMemo(() => {
        if (!userProfile) {
            return [];
        }

        return Object.entries(userProfile.attributes).filter(([, value]) => !isEditableScalar(value));
    }, [userProfile]);

    const handleChange = (key: string, value: string) => {
        setFormState((prev) => ({
            ...prev,
            [key]: value,
        }));
        setSuccessMessage('');
    };

    const handlePasswordChange = (key: 'currentPassword' | 'newPassword' | 'confirmPassword', value: string) => {
        setPasswordState((prev) => ({
            ...prev,
            [key]: value,
        }));
        setSuccessMessage('');
    };

    const handleTogglePasswordVisibility = () => {
        setShowPassword((prev) => !prev);
    };

    const handleMouseDownPassword = (event: React.MouseEvent<HTMLButtonElement>) => {
        event.preventDefault();
    };

    const handleSave = async () => {
        if (!isAuthenticated || !userProfile) {
            return;
        }

        setSaving(true);
        setError('');
        setSuccessMessage('');

        const mergedAttributes: Record<string, unknown> = {
            ...userProfile.attributes,
        };

        Object.entries(formState).forEach(([key, value]) => {
            const hadKey = Object.prototype.hasOwnProperty.call(userProfile.attributes, key);
            const existingValue = userProfile.attributes[key];

            if (!hadKey && value.trim() === '') {
                return;
            }

            if (hadKey && isEditableScalar(existingValue) && toFormValue(existingValue) === value) {
                mergedAttributes[key] = existingValue;
                return;
            }

            mergedAttributes[key] = value;
        });

        try {
            await updateCurrentUserProfile(mergedAttributes);
            const refreshedProfile = await refreshUserProfile();
            if (refreshedProfile) {
                setFormState(buildFormState(refreshedProfile.attributes));
            }
            setIsEditing(false);
            setSuccessMessage('Profile updated successfully.');
        } catch (saveError) {
            setError(saveError instanceof Error ? saveError.message : 'Failed to update user profile.');
        } finally {
            setSaving(false);
        }
    };

    const handleStartEdit = () => {
        setError('');
        setSuccessMessage('');
        setIsEditing(true);
    };

    const handleCloseEdit = () => {
        if (userProfile) {
            setFormState(buildFormState(userProfile.attributes));
        }
        setError('');
        setSuccessMessage('');
        setIsEditing(false);
    };

    const handlePasswordSave = async () => {
        if (!isAuthenticated) {
            return;
        }

        const trimmedPassword = passwordState.newPassword.trim();
        const trimmedCurrentPassword = passwordState.currentPassword.trim();

        if (!trimmedCurrentPassword) {
            setError('Enter your current password.');
            setSuccessMessage('');
            return;
        }

        if (!trimmedPassword) {
            setError('Enter a new password.');
            setSuccessMessage('');
            return;
        }

        if (trimmedPassword !== passwordState.confirmPassword.trim()) {
            setError('Password confirmation does not match.');
            setSuccessMessage('');
            return;
        }

        setSavingPassword(true);
        setError('');
        setSuccessMessage('');

        try {
            await updateCurrentUserPassword(trimmedPassword, trimmedCurrentPassword);
            setPasswordState({
                currentPassword: '',
                newPassword: '',
                confirmPassword: '',
            });
            setSuccessMessage('Password updated successfully.');
        } catch (saveError) {
            setError(saveError instanceof Error ? saveError.message : 'Failed to update password.');
        } finally {
            setSavingPassword(false);
        }
    };

    const profileHeading = (
        formState.username ||
        userProfile?.display ||
        formState.name ||
        formState.email ||
        'Your Profile'
    );
    const usernameValue = formState.username || userProfile?.display || formState.email || 'Signed-in user';

    const profileAvatarUrl = typeof userProfile?.attributes.picture === 'string' ? userProfile.attributes.picture : '';
    const avatarFallback = profileHeading.trim().slice(0, 1).toUpperCase();

    const handleCopyUsername = async () => {
        if (!usernameValue || usernameValue === 'Signed-in user') {
            return;
        }

        try {
            await navigator.clipboard.writeText(usernameValue);
            setError('');
            setSuccessMessage('Username copied.');
        } catch {
            setError('Failed to copy username.');
            setSuccessMessage('');
        }
    };

    return (
        <Layout>
            <Box sx={{ width: '100%', maxWidth: 960, px: 2, py: 4 }}>
                <Stack spacing={3}>
                    <Box>
                        <Link
                            component="button"
                            type="button"
                            underline="hover"
                            color="primary"
                            onClick={() => router.push('/')}
                            sx={{
                                px: 0,
                                mb: 1.5,
                                display: 'inline-flex',
                                alignItems: 'center',
                                gap: 0.75,
                                fontWeight: 600,
                            }}
                        >
                            <ArrowBackRoundedIcon fontSize="small" />
                            Home
                        </Link>
                    </Box>

                    {error && <Alert severity="error">{error}</Alert>}
                    {successMessage && <Alert severity="success">{successMessage}</Alert>}

                    {loading ? (
                        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
                            <CircularProgress />
                        </Box>
                    ) : !isAuthenticated ? (
                        <Alert severity="warning">No auth assertion available. Please log in again.</Alert>
                    ) : !userProfile ? (
                        <Alert severity="warning">Unable to load the current user profile.</Alert>
                    ) : (
                        <>
                            <Paper
                                elevation={0}
                                sx={{
                                    p: 3,
                                    borderRadius: 4,
                                    border: '1px solid',
                                    borderColor: 'divider',
                                    backgroundColor: 'background.paper',
                                    boxShadow: (theme) => theme.palette.mode === 'dark'
                                        ? '0 18px 42px rgba(2, 6, 23, 0.28)'
                                        : '0 18px 42px rgba(15, 23, 42, 0.06)',
                                }}
                            >
                                <Stack spacing={3}>
                                    <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2, alignItems: 'flex-start' }}>
                                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                                            <Avatar
                                                src={profileAvatarUrl || undefined}
                                                sx={{
                                                    width: 68,
                                                    height: 68,
                                                    bgcolor: 'action.selected',
                                                    color: 'text.primary',
                                                    fontSize: 28,
                                                    boxShadow: 'none',
                                                    border: '1px solid',
                                                    borderColor: 'divider',
                                                }}
                                            >
                                                {profileAvatarUrl ? null : avatarFallback}
                                            </Avatar>
                                            <Box>
                                                <Typography variant="h4" sx={{ mb: 0.5, fontWeight: 700 }}>
                                                    {usernameValue}
                                                </Typography>
                                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
                                                    <Typography
                                                        variant="body1"
                                                        color="text.secondary"
                                                        sx={{ opacity: 0.78 }}
                                                    >
                                                        {userProfile.id || '-'}
                                                    </Typography>
                                                    <IconButton
                                                        size="small"
                                                        aria-label="copy username"
                                                        onClick={() => void handleCopyUsername()}
                                                        sx={{ color: 'text.secondary' }}
                                                    >
                                                        <ContentCopyRoundedIcon fontSize="inherit" />
                                                    </IconButton>
                                                </Box>
                                            </Box>
                                        </Box>
                                        <Box sx={{ display: 'flex', gap: 1, flexShrink: 0 }}>
                                            {isEditing ? (
                                                <>
                                                    <Button
                                                        variant="contained"
                                                        color="primary"
                                                        disabled={saving || userProfile.isReadOnly}
                                                        onClick={() => void handleSave()}
                                                    >
                                                        {saving ? 'Saving...' : 'Save'}
                                                    </Button>
                                                    <Button
                                                        variant="outlined"
                                                        color="primary"
                                                        disabled={saving}
                                                        onClick={handleCloseEdit}
                                                    >
                                                        Close
                                                    </Button>
                                                </>
                                            ) : (
                                                <Button
                                                    variant="contained"
                                                    color="primary"
                                                    disabled={userProfile.isReadOnly}
                                                    onClick={handleStartEdit}
                                                >
                                                    Edit
                                                </Button>
                                            )}
                                        </Box>
                                    </Box>

                                    <Box
                                        sx={{
                                            display: 'grid',
                                            gap: 2,
                                            gridTemplateColumns: {
                                                xs: '1fr',
                                                md: 'repeat(2, minmax(0, 1fr))',
                                            },
                                        }}
                                    >
                                        <Box sx={{ minWidth: 0 }}>
                                            <Typography variant="body2" color="text.secondary">User Type</Typography>
                                            <Typography sx={{ wordBreak: 'break-word', overflowWrap: 'anywhere' }}>
                                                {userProfile.type || '-'}
                                            </Typography>
                                        </Box>
                                        <Box sx={{ minWidth: 0 }}>
                                            <Typography variant="body2" color="text.secondary">Organization Unit</Typography>
                                            <Typography sx={{ wordBreak: 'break-word', overflowWrap: 'anywhere' }}>
                                                {userProfile.ouHandle || userProfile.ouId || '-'}
                                            </Typography>
                                        </Box>
                                    </Box>

                                    <Box
                                        sx={{
                                            display: 'grid',
                                            gap: 2,
                                            gridTemplateColumns: {
                                                xs: '1fr',
                                                md: 'repeat(2, minmax(0, 1fr))',
                                            },
                                        }}
                                    >
                                        {editableFields.map((fieldKey) => (
                                            isEditing ? (
                                                <TextField
                                                    key={fieldKey}
                                                    fullWidth
                                                    label={fieldLabels[fieldKey] || fieldKey}
                                                    value={formState[fieldKey]}
                                                    onChange={(event) => handleChange(fieldKey, event.target.value)}
                                                />
                                            ) : (
                                                <Box key={fieldKey} sx={{ minWidth: 0 }}>
                                                    <Typography variant="body2" color="text.secondary">
                                                        {fieldLabels[fieldKey] || fieldKey}
                                                    </Typography>
                                                    <Typography sx={{ wordBreak: 'break-word', overflowWrap: 'anywhere' }}>
                                                        {formState[fieldKey] || '-'}
                                                    </Typography>
                                                </Box>
                                            )
                                        ))}
                                    </Box>

                                    {complexAttributes.length > 0 && (
                                        <Box>
                                            <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                                                Additional Attributes
                                            </Typography>
                                            <pre style={{ margin: 0 }}>
                                                <code>{JSON.stringify(Object.fromEntries(complexAttributes), null, 2)}</code>
                                            </pre>
                                        </Box>
                                    )}
                                </Stack>
                            </Paper>

                            <Paper
                                elevation={0}
                                sx={{
                                    p: 3,
                                    borderRadius: 4,
                                    border: '1px solid',
                                    borderColor: 'divider',
                                    backgroundColor: 'background.paper',
                                    boxShadow: (theme) => theme.palette.mode === 'dark'
                                        ? '0 18px 42px rgba(2, 6, 23, 0.35)'
                                        : '0 18px 42px rgba(15, 23, 42, 0.06)',
                                }}
                            >
                                <Stack spacing={3}>
                                    <Box>
                                        <Typography variant="h6" sx={{ mb: 1 }}>
                                            Update Password
                                        </Typography>
                                    </Box>

                                    <Box
                                        sx={{
                                            display: 'grid',
                                            gap: 2,
                                            gridTemplateColumns: {
                                                xs: '1fr',
                                                md: 'repeat(2, minmax(0, 1fr))',
                                            },
                                        }}
                                    >
                                        <Box display="flex" flexDirection="column" gap={0.5}>
                                            <InputLabel htmlFor="current-password">Current Password</InputLabel>
                                            <OutlinedInput
                                                id="current-password"
                                                type="password"
                                                autoComplete="current-password"
                                                size="small"
                                                value={passwordState.currentPassword}
                                                onChange={(event) => handlePasswordChange('currentPassword', event.target.value)}
                                            />
                                        </Box>
                                        <Box display="flex" flexDirection="column" gap={0.5}>
                                            <InputLabel htmlFor="new-password">New Password</InputLabel>
                                            <OutlinedInput
                                                id="new-password"
                                                type={showPassword ? 'text' : 'password'}
                                                size="small"
                                                value={passwordState.newPassword}
                                                onChange={(event) => handlePasswordChange('newPassword', event.target.value)}
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
                                        <Box display="flex" flexDirection="column" gap={0.5}>
                                            <InputLabel htmlFor="confirm-password">Confirm Password</InputLabel>
                                            <OutlinedInput
                                                id="confirm-password"
                                                type={showPassword ? 'text' : 'password'}
                                                size="small"
                                                value={passwordState.confirmPassword}
                                                onChange={(event) => handlePasswordChange('confirmPassword', event.target.value)}
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
                                    </Box>

                                    <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
                                        <Button
                                            variant="contained"
                                            color="primary"
                                            disabled={savingPassword || userProfile.isReadOnly}
                                            onClick={() => void handlePasswordSave()}
                                        >
                                            {savingPassword ? 'Updating...' : 'Update Password'}
                                        </Button>
                                    </Box>
                                </Stack>
                            </Paper>
                        </>
                    )}
                </Stack>
            </Box>
        </Layout>
    );
};

export default ProfilePage;
