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

import { Box, Button, Divider, Typography } from "@wso2/oxygen-ui";
import { Link } from "react-router";

function HomePage() {
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
        background:
          "linear-gradient(135deg, rgba(25, 118, 210, 0.1) 0%, rgba(25, 118, 210, 0.05) 50%, rgba(25, 118, 210, 0.1) 100%)",
      }}
    >
      <Box
        sx={{
          position: "relative",
          width: "100%",
          maxWidth: 520,
          mx: 2,
        }}
      >
        {/* Logo/Brand Section */}
        <Box sx={{ textAlign: "center", mb: 4 }}>
          <Typography
            variant="h3"
            component="h1"
            sx={{
              fontWeight: 700,
              color: "primary.main",
              letterSpacing: "-0.5px",
            }}
          >
            ThunderID Sample App
          </Typography>
          <Typography
            variant="h6"
            sx={{
              color: "text.secondary",
              fontWeight: 400,
              mt: 1,
            }}
          >
            API-Based Authentication Demo
          </Typography>
        </Box>

        {/* Main Card */}
        <Box
          sx={{
            p: { xs: 3, sm: 5 },
            borderRadius: 3,
            bgcolor: "background.paper",
            boxShadow: "0 25px 50px -12px rgba(0, 0, 0, 0.15)",
          }}
        >
          {/* Description */}
          <Typography
            variant="body1"
            sx={{
              color: "text.secondary",
              textAlign: "center",
              lineHeight: 1.7,
              mb: 4,
            }}
          >
            This sample demonstrates ThunderID&apos;s API-based authentication
            flow and Atomic APIs, allowing direct integration with sign-in and
            sign-up APIs without browser redirects.
          </Typography>

          {/* Sign Up Section */}
          <Box sx={{ mb: 3 }}>
            <Typography
              variant="overline"
              sx={{
                color: "text.secondary",
                fontWeight: 600,
                letterSpacing: 1.5,
                display: "block",
                mb: 1.5,
              }}
            >
              New User
            </Typography>
            <Button
              component={Link}
              to="/sign-up"
              variant="contained"
              color="primary"
              size="large"
              fullWidth
              sx={{
                py: 1.5,
                fontSize: "1rem",
                fontWeight: 600,
                borderRadius: 2,
                textTransform: "none",
                boxShadow: "0 4px 14px 0 rgba(25, 118, 210, 0.39)",
                "&:hover": {
                  boxShadow: "0 6px 20px 0 rgba(25, 118, 210, 0.5)",
                },
              }}
            >
              Sign Up
            </Button>
          </Box>

          <Divider sx={{ my: 3 }}>
            <Typography variant="caption" color="text.secondary">
              OR
            </Typography>
          </Divider>

          {/* Sign In Section */}
          <Box>
            <Typography
              variant="overline"
              sx={{
                color: "text.secondary",
                fontWeight: 600,
                letterSpacing: 1.5,
                display: "block",
                mb: 1.5,
              }}
            >
              Existing User
            </Typography>
            <Button
              component={Link}
              to="/sign-in"
              variant="outlined"
              color="primary"
              size="large"
              fullWidth
              sx={{
                py: 1.5,
                fontSize: "0.95rem",
                fontWeight: 500,
                borderRadius: 2,
                textTransform: "none",
                borderWidth: 2,
                "&:hover": {
                  borderWidth: 2,
                },
              }}
            >
              Sign In
            </Button>
          </Box>
        </Box>

        {/* Footer text */}
        <Box sx={{ textAlign: "center", mt: 3 }}>
          <Typography
            variant="caption"
            sx={{
              color: "text.secondary",
              display: "block",
            }}
          >
            Powered by ThunderID Identity Server
          </Typography>
        </Box>
      </Box>
    </Box>
  );
}

export default HomePage;
