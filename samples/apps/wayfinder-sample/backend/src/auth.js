import { createPublicKey, createVerify } from "node:crypto";

const textDecoder = new TextDecoder();
let jwksCache = null;

function base64UrlToBuffer(value) {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const padded = normalized.padEnd(normalized.length + ((4 - (normalized.length % 4)) % 4), "=");

  return Buffer.from(padded, "base64");
}

function parseJwt(token) {
  const parts = token.split(".");

  if (parts.length !== 3) {
    throw new Error("Invalid token format");
  }

  const header = JSON.parse(textDecoder.decode(base64UrlToBuffer(parts[0])));
  const payload = JSON.parse(textDecoder.decode(base64UrlToBuffer(parts[1])));

  return {
    header,
    payload,
    signingInput: `${parts[0]}.${parts[1]}`,
    signature: base64UrlToBuffer(parts[2])
  };
}

function getIssuer() {
  if (process.env.THUNDER_ISSUER) {
    return process.env.THUNDER_ISSUER;
  }

  return process.env.THUNDER_BASE_URL;
}

async function getJwks() {
  if (jwksCache) {
    return jwksCache;
  }

  if (!process.env.THUNDER_BASE_URL) {
    throw new Error("THUNDER_BASE_URL is required when API_REQUIRE_AUTH=true");
  }

  const response = await fetch(`${process.env.THUNDER_BASE_URL}/oauth2/jwks`);

  if (!response.ok) {
    throw new Error("Unable to load ThunderID JWKS");
  }

  jwksCache = await response.json();

  return jwksCache;
}

function verifySignature(token, jwk) {
  const key = createPublicKey({
    key: jwk,
    format: "jwk"
  });
  const verifier = createVerify("RSA-SHA256");

  verifier.update(token.signingInput);
  verifier.end();

  return verifier.verify(key, token.signature);
}

function validateClaims(payload) {
  const now = Math.floor(Date.now() / 1000);
  const issuer = getIssuer();
  const audience = process.env.THUNDER_AUDIENCE;

  if (payload.iss !== issuer) {
    throw new Error("Invalid token issuer");
  }

  if (payload.exp && payload.exp < now) {
    throw new Error("Token has expired");
  }

  if (payload.nbf && payload.nbf > now) {
    throw new Error("Token is not active yet");
  }

  if (audience) {
    const tokenAudience = Array.isArray(payload.aud) ? payload.aud : [payload.aud];

    if (!tokenAudience.includes(audience)) {
      throw new Error("Invalid token audience");
    }
  }
}

export async function getAuthenticatedUser(request) {
  const authHeader = request.headers.authorization || "";
  const token = authHeader.startsWith("Bearer ") ? authHeader.slice(7) : null;

  if (!token) {
    throw new Error("Missing bearer token");
  }

  const parsedToken = parseJwt(token);

  if (parsedToken.header.alg !== "RS256") {
    throw new Error("Unsupported token algorithm");
  }

  const jwks = await getJwks();
  const jwk = jwks.keys?.find((key) => key.kid === parsedToken.header.kid);

  if (!jwk) {
    throw new Error("Signing key not found");
  }

  if (!verifySignature(parsedToken, jwk)) {
    throw new Error("Invalid token signature");
  }

  validateClaims(parsedToken.payload);

  const scopes = typeof parsedToken.payload.scope === "string"
    ? parsedToken.payload.scope.split(" ").filter(Boolean)
    : [];

  return {
    id: parsedToken.payload.sub,
    username: parsedToken.payload.username || parsedToken.payload.preferred_username,
    email: parsedToken.payload.email,
    givenName: parsedToken.payload.given_name,
    familyName: parsedToken.payload.family_name,
    scopes,
    rawClaims: parsedToken.payload
  };
}

export async function resolveUser(request) {
  if (process.env.API_REQUIRE_AUTH === "false") {
    return {
      id: "local-demo-user",
      username: "local.traveler",
      email: "local.traveler@example.com",
      givenName: "Local",
      familyName: "Traveler",
      scopes: ["booking:read", "booking:create", "booking:cancel", "booking:recommend"]
    };
  }

  return getAuthenticatedUser(request);
}

export function requireScope(user, scope) {
  const scopes = user?.scopes || [];

  if (!scopes.includes(scope)) {
    const error = new Error(`Missing required scope: ${scope}`);

    error.statusCode = 403;
    throw error;
  }
}
