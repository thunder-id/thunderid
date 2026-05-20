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

import { useMemo, useState, useRef, useEffect } from "react";
import {
  Alert,
  Box,
  Button,
  Container,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { useNavigate } from "react-router";
import { decodeJwt } from "../utils/jwt";
import UserTable from "../components/UserTable";
import { useStepUpAuth } from "../hooks/useStepUpAuth";
import SMSOTPStepUpModal from "../components/SMSOTPStepUpModal";
import { fetchUsers } from "../utils/api";

interface TokenPayload {
  sub?: string;
  userType?: string;
  username?: string;
  ouName?: string;
  iss?: string;
  [key: string]: unknown;
}

function DashboardPage() {
  const navigate = useNavigate();
  const [exportError, setExportError] = useState<string | null>(null);
  const [exportSuccess, setExportSuccess] = useState(false);
  const successTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Cleanup timeout on unmount
  useEffect(() => {
    return () => {
      if (successTimeoutRef.current) {
        clearTimeout(successTimeoutRef.current);
      }
    };
  }, []);

  const {
    triggerStepUp,
    showStepUp,
    userMobileNumber,
    currentAssertion,
    criticalOperationMessage,
    onStepUpSuccess,
    onStepUpClose,
  } = useStepUpAuth();

  // Prefer the latest assertion from step-up auth, fall back to the value in sessionStorage.
  const assertion = currentAssertion ?? sessionStorage.getItem("assertion");

  const decodedToken = useMemo(() => {
    if (!assertion) return null;
    return decodeJwt(assertion);
  }, [assertion]);

  if (!assertion || !decodedToken) {
    navigate("/");
    return null;
  }

  const payload = decodedToken.payload as TokenPayload;
  const displayName = payload.username || payload.sub || "User";

  const handleExportUsers = async () => {
    setExportError(null);
    setExportSuccess(false);

    try {
      await triggerStepUp(
        "Exporting user data is a critical operation that requires additional authentication.",
        async () => {
          // This will execute after successful step-up
          try {
            const users = await fetchUsers();
            const exportData = {
              exportedAt: new Date().toISOString(),
              totalUsers: users.length,
              users: users.map((user) => {
                const attributes = user.attributes ?? {};
                const username =
                  typeof attributes.username === "string"
                    ? (attributes.username as string)
                    : undefined;
                const email =
                  typeof attributes.email === "string"
                    ? (attributes.email as string)
                    : undefined;
                const given_name =
                  typeof attributes.given_name === "string"
                    ? (attributes.given_name as string)
                    : undefined;
                const family_name =
                  typeof attributes.family_name === "string"
                    ? (attributes.family_name as string)
                    : undefined;
                const mobileNumber =
                  typeof attributes.mobileNumber === "string"
                    ? (attributes.mobileNumber as string)
                    : undefined;
                return {
                  id: user.id,
                  username,
                  email,
                  given_name,
                  family_name,
                  mobileNumber,
                  type: user.type,
                  ouId: user.ouId,
                };
              }),
            };

            // Create and download JSON file
            const dataStr = JSON.stringify(exportData, null, 2);
            const dataBlob = new Blob([dataStr], { type: "application/json" });
            const url = URL.createObjectURL(dataBlob);
            const link = document.createElement("a");
            link.href = url;
            link.download = `users-export-${new Date().toISOString().split("T")[0]}.json`;
            try {
              document.body.appendChild(link);
              link.click();
            } finally {
              if (document.body.contains(link)) {
                document.body.removeChild(link);
              }
              URL.revokeObjectURL(url);
            }

            setExportSuccess(true);
            // Clear any existing timeout before setting a new one
            if (successTimeoutRef.current) {
              clearTimeout(successTimeoutRef.current);
            }
            successTimeoutRef.current = setTimeout(
              () => setExportSuccess(false),
              3000
            );
          } catch (err) {
            setExportError(
              err instanceof Error
                ? err.message
                : "Failed to export user data. Please try again."
            );
          }
        }
      );
    } catch (err) {
      setExportError(
        err instanceof Error
          ? err.message
          : "Failed to initiate step-up authentication. Please try again."
      );
    }
  };

  return (
    <Container maxWidth="lg" sx={{ px: { xs: 2, sm: 3 } }}>
      <Box sx={{ my: 4 }}>
        <Stack
          direction="row"
          justifyContent="space-between"
          alignItems="center"
        >
          <Box>
            <Typography variant="h4" component="h1" fontWeight={600}>
              Dashboard
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Welcome, {displayName}! You are successfully authenticated.
            </Typography>
          </Box>
          <Button
            variant="contained"
            color="primary"
            onClick={handleExportUsers}
            sx={{ minWidth: 150 }}
          >
            Export User Data
          </Button>
        </Stack>
      </Box>

      {exportError && (
        <Alert
          severity="error"
          sx={{ mb: 2 }}
          onClose={() => setExportError(null)}
        >
          {exportError}
        </Alert>
      )}

      {exportSuccess && (
        <Alert
          severity="success"
          sx={{ mb: 2 }}
          onClose={() => setExportSuccess(false)}
        >
          User data exported successfully!
        </Alert>
      )}

      <UserTable />

      {/* SMS OTP Step-Up Modal */}
      {showStepUp && userMobileNumber && currentAssertion && (
        <SMSOTPStepUpModal
          open={showStepUp}
          mobileNumber={userMobileNumber}
          existingAssertion={currentAssertion}
          criticalOperationMessage={criticalOperationMessage || undefined}
          onSuccess={onStepUpSuccess}
          onClose={onStepUpClose}
        />
      )}
    </Container>
  );
}

export default DashboardPage;
