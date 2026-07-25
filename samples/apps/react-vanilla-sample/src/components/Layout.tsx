/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import Avatar from '@mui/material/Avatar';
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Divider from '@mui/material/Divider';
import Grid from "@mui/material/Grid";
import ListItemIcon from '@mui/material/ListItemIcon';
import Menu from '@mui/material/Menu';
import MenuItem from '@mui/material/MenuItem';
import Typography from '@mui/material/Typography';
import AccountCircleOutlinedIcon from '@mui/icons-material/AccountCircleOutlined';
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown';
import LogoutRoundedIcon from '@mui/icons-material/LogoutRounded';
import PersonOutlineRoundedIcon from '@mui/icons-material/PersonOutlineRounded';
import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import ThemeToggle from "../theme/ThemeToggle";
import useAuth from "../hooks/useAuth";

/**
 * PageLayout component serves as a wrapper for the main content of the application.
 */
const PageLayout = ({ children }: { children: React.ReactNode }) => {
    const { token, clearToken, userProfile } = useAuth();
    const navigate = useNavigate();
    const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
    const isMenuOpen = Boolean(menuAnchorEl);

    const userLabel = useMemo(() => {
        const username = userProfile?.attributes.username;
        const display = userProfile?.display;
        const email = userProfile?.attributes.email;

        if (typeof username === 'string' && username.trim()) {
            return username;
        }

        if (typeof display === 'string' && display.trim()) {
            return display;
        }

        if (typeof email === 'string' && email.trim()) {
            return email;
        }

        return 'Profile';
    }, [userProfile]);

    const avatarUrl = useMemo(() => {
        const picture = userProfile?.attributes.picture;
        return typeof picture === 'string' && picture.trim() ? picture : '';
    }, [userProfile]);

    const avatarFallback = useMemo(() => {
        const trimmed = userLabel.trim();
        if (!trimmed) {
            return '';
        }

        return trimmed.slice(0, 1).toUpperCase();
    }, [userLabel]);
    
    const handleLogout = () => {
        sessionStorage.removeItem("isSignupMode");
        sessionStorage.removeItem("startInit");
        sessionStorage.removeItem("executionId");
        sessionStorage.removeItem("challengeToken");
        clearToken();
        setMenuAnchorEl(null);
        navigate('/');
    };

    const handleOpenMenu = (event: React.MouseEvent<HTMLElement>) => {
        setMenuAnchorEl(event.currentTarget);
    };

    const handleCloseMenu = () => {
        setMenuAnchorEl(null);
    };

    const handleProfileClick = () => {
        navigate('/profile');
        setMenuAnchorEl(null);
    };

    return (
        <Box sx={{ height: "100vh", display: "flex", flexDirection: "column" }}>
            <Box
                sx={{
                    pt: 4,
                    pr: 4,
                    pl: 4,
                    flex: 0,
                    display: 'flex',
                    justifyContent: 'flex-end',
                    alignItems: 'center',
                    gap: 2,
                }}
            >
                <ThemeToggle />
                { token &&
                    <>
                        <Button
                            variant="text"
                            endIcon={<KeyboardArrowDownIcon />}
                            onClick={handleOpenMenu}
                            sx={{
                                px: 1,
                                py: 0.75,
                                minWidth: 180,
                                borderRadius: 999,
                                textTransform: 'none',
                                color: 'text.primary',
                                backgroundColor: 'background.paper',
                                border: '1px solid',
                                borderColor: 'divider',
                                boxShadow: '0 10px 30px rgba(15, 23, 42, 0.08)',
                                '&:hover': {
                                    backgroundColor: 'action.hover',
                                },
                            }}
                        >
                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25 }}>
                                <Avatar
                                    src={avatarUrl || undefined}
                                    sx={{
                                        width: 34,
                                        height: 34,
                                        bgcolor: 'action.selected',
                                        color: 'text.primary',
                                        fontSize: 16,
                                        boxShadow: 'none',
                                        border: '1px solid',
                                        borderColor: 'divider',
                                    }}
                                >
                                    {avatarUrl ? null : avatarFallback || <AccountCircleOutlinedIcon fontSize="small" />}
                                </Avatar>
                                <Box sx={{ textAlign: 'left', lineHeight: 1.1 }}>
                                    <Typography variant="body2" sx={{ fontWeight: 700 }}>
                                        {userLabel}
                                    </Typography>
                                </Box>
                            </Box>
                        </Button>
                        <Menu
                            anchorEl={menuAnchorEl}
                            open={isMenuOpen}
                            onClose={handleCloseMenu}
                            anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
                            transformOrigin={{ vertical: 'top', horizontal: 'right' }}
                            slotProps={{
                                paper: {
                                    sx: {
                                        mt: 1,
                                        width: menuAnchorEl ? Math.max(menuAnchorEl.clientWidth, 220) : 220,
                                        borderRadius: 3,
                                        border: '1px solid',
                                        borderColor: 'divider',
                                        boxShadow: '0 20px 45px rgba(15, 23, 42, 0.12)',
                                        overflow: 'hidden',
                                    },
                                },
                                list: {
                                    sx: {
                                        p: 0,
                                    },
                                },
                            }}
                        >
                            <Box sx={{ px: 2, py: 1.5, backgroundColor: 'action.hover' }}>
                                <Typography variant="body2" sx={{ fontWeight: 700 }}>
                                    {userLabel}
                                </Typography>
                                <Typography variant="caption" color="text.secondary">
                                    Account options
                                </Typography>
                            </Box>
                            <Divider />
                            <MenuItem onClick={handleProfileClick} sx={{ py: 1.5 }}>
                                <ListItemIcon>
                                    <PersonOutlineRoundedIcon fontSize="small" />
                                </ListItemIcon>
                                <Box>
                                    <Typography variant="body2" sx={{ fontWeight: 600 }}>
                                        Profile
                                    </Typography>
                                </Box>
                            </MenuItem>
                            <MenuItem onClick={handleLogout} sx={{ py: 1.5 }}>
                                <ListItemIcon>
                                    <LogoutRoundedIcon fontSize="small" />
                                </ListItemIcon>
                                <Box>
                                    <Typography variant="body2" sx={{ fontWeight: 600 }}>
                                        Logout
                                    </Typography>
                                </Box>
                            </MenuItem>
                        </Menu>
                    </>
                }
            </Box>
            <Box sx={{ flex: 1, display: "flex", alignItems: "center" }}>
                <Grid
                    container
                    sx={{
                        flex: 1,
                        alignSelf: "center",
                        alignItems: "center",
                        justifyContent: "center",
                        width: "100%",
                        maxWidth: 1000,
                        margin: "auto",
                    }}
                >
                    {children}
                </Grid>
            </Box>
        </Box>
    );
};

export default PageLayout;
