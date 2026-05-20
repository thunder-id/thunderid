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

import { useState } from "react";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  Paper,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Link, useNavigate } from "react-router";
import { getConfig } from "../config";

interface AuthResponse {
  id: string;
  type: string;
  ouId?: string;
  assertion: string;
}

interface ApiError {
  code: string;
  message: { defaultValue?: string };
  description?: { defaultValue?: string };
}

function SignInPage() {
  const navigate = useNavigate();
  const [formData, setFormData] = useState({
    username: "",
    password: "",
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
    setError(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      const { baseUrl } = getConfig();

      const response = await fetch(`${baseUrl}/auth/credentials/authenticate`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          identifiers: {
            username: formData.username,
          },
          credentials: {
            password: formData.password,
          },
        }),
      });

      if (!response.ok) {
        const errorData: ApiError = await response.json();
        throw new Error(
          errorData.message?.defaultValue || "Authentication failed"
        );
      }

      const authResponse: AuthResponse = await response.json();

      // Store assertion token
      if (authResponse.assertion) {
        sessionStorage.setItem("assertion", authResponse.assertion);
      }

      // Navigate to dashboard
      navigate("/dashboard");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Sign in failed. Please try again."
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box
      sx={{
        position: "absolute",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <Container maxWidth="xs">
        <Paper elevation={2} sx={{ p: 4 }}>
          <Typography
            variant="h5"
            component="h1"
            gutterBottom
            fontWeight={600}
            textAlign="center"
          >
            Sign In
          </Typography>
          <Typography
            variant="body2"
            color="text.secondary"
            textAlign="center"
            sx={{ mb: 3 }}
          >
            Sign in with your credentials to continue
          </Typography>

          {error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {error}
            </Alert>
          )}

          <Box component="form" onSubmit={handleSubmit}>
            <Stack spacing={2}>
              <TextField
                fullWidth
                label="Username"
                name="username"
                type="text"
                value={formData.username}
                onChange={handleChange}
                required
                disabled={loading}
              />

              <TextField
                fullWidth
                label="Password"
                name="password"
                type="password"
                value={formData.password}
                onChange={handleChange}
                required
                disabled={loading}
              />

              <Button
                type="submit"
                variant="contained"
                color="primary"
                size="large"
                fullWidth
                sx={{ mt: 2 }}
                disabled={loading}
              >
                {loading ? (
                  <CircularProgress size={24} color="inherit" />
                ) : (
                  "Sign In"
                )}
              </Button>

              <Typography variant="body2" textAlign="center" sx={{ mt: 2 }}>
                Don&apos;t have an account?{" "}
                <Link to="/sign-up" style={{ color: "inherit" }}>
                  Sign up here
                </Link>
              </Typography>

              <Button
                component={Link}
                to="/"
                variant="text"
                size="small"
                sx={{ mt: 1 }}
                disabled={loading}
              >
                Back to Home
              </Button>
            </Stack>
          </Box>
        </Paper>
      </Container>
    </Box>
  );
}

export default SignInPage;
