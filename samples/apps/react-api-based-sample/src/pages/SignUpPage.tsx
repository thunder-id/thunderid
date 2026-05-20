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
import { Link } from "react-router";
import { getConfig } from "../config";
import { getCustomersOrganizationUnitId } from "../utils/api";

interface UserResponse {
  id: string;
  ouId: string;
  type: string;
  attributes: Record<string, unknown>;
}

interface ApiError {
  code: string;
  message: { defaultValue?: string };
  description?: { defaultValue?: string };
}

function SignUpPage() {
  const [formData, setFormData] = useState({
    username: "",
    given_name: "",
    family_name: "",
    email: "",
    password: "",
    confirmPassword: "",
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
    setError(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Validate passwords match
    if (formData.password !== formData.confirmPassword) {
      setError("Passwords do not match");
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const { baseUrl } = getConfig();
      const organizationUnitId = await getCustomersOrganizationUnitId();

      const response = await fetch(`${baseUrl}/users`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          ouId: organizationUnitId,
          type: "Customer",
          attributes: {
            username: formData.username,
            given_name: formData.given_name,
            family_name: formData.family_name,
            email: formData.email,
            password: formData.password,
          },
        }),
      });

      if (!response.ok) {
        const errorData: ApiError = await response.json();
        throw new Error(
          errorData.description?.defaultValue ||
            errorData.message?.defaultValue ||
            "Registration failed"
        );
      }

      const userResponse: UserResponse = await response.json();
      console.log("Registration successful:", userResponse);

      setSuccess(true);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Registration failed. Please try again."
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
        py: 4,
        overflow: "auto",
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
            Sign Up
          </Typography>
          <Typography
            variant="body2"
            color="text.secondary"
            textAlign="center"
            sx={{ mb: 3 }}
          >
            Create a new account to get started
          </Typography>

          {error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {error}
            </Alert>
          )}

          {success && (
            <Alert severity="success" sx={{ mb: 2 }}>
              Registration successful! You can now sign in with your
              credentials.
            </Alert>
          )}

          <Box component="form" onSubmit={handleSubmit}>
            <Stack spacing={2}>
              <TextField
                fullWidth
                label="Username"
                name="username"
                value={formData.username}
                onChange={handleChange}
                required
                disabled={loading || success}
              />
              <TextField
                fullWidth
                label="First Name"
                name="given_name"
                value={formData.given_name}
                onChange={handleChange}
                required
                disabled={loading || success}
              />
              <TextField
                fullWidth
                label="Last Name"
                name="family_name"
                value={formData.family_name}
                onChange={handleChange}
                required
                disabled={loading || success}
              />
              <TextField
                fullWidth
                label="Email Address"
                name="email"
                type="email"
                value={formData.email}
                onChange={handleChange}
                required
                disabled={loading || success}
              />
              <TextField
                fullWidth
                label="Password"
                name="password"
                type="password"
                value={formData.password}
                onChange={handleChange}
                required
                disabled={loading || success}
              />
              <TextField
                fullWidth
                label="Confirm Password"
                name="confirmPassword"
                type="password"
                value={formData.confirmPassword}
                onChange={handleChange}
                required
                disabled={loading || success}
              />

              <Button
                type="submit"
                variant="contained"
                color="primary"
                size="large"
                fullWidth
                sx={{ mt: 2 }}
                disabled={loading || success}
              >
                {loading ? (
                  <CircularProgress size={24} color="inherit" />
                ) : (
                  "Sign Up"
                )}
              </Button>

              <Typography variant="body2" textAlign="center" sx={{ mt: 2 }}>
                Already have an account?{" "}
                <Link to="/sign-in" style={{ color: "inherit" }}>
                  Sign in here
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

export default SignUpPage;
