// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// APIs
export {default as useCreateCredentialOffer} from './api/useCreateCredentialOffer';
export {default as useCreateVerifiableCredential} from './api/useCreateVerifiableCredential';
export {default as useCreateVerifiablePresentation} from './api/useCreateVerifiablePresentation';
export {default as useDeleteVerifiableCredential} from './api/useDeleteVerifiableCredential';
export {default as useDeleteVerifiablePresentation} from './api/useDeleteVerifiablePresentation';
export {default as useGetTrustAnchors} from './api/useGetTrustAnchors';
export {default as useGetVerifiableCredential} from './api/useGetVerifiableCredential';
export {default as useGetVerifiableCredentials} from './api/useGetVerifiableCredentials';
export {default as useGetVerifiablePresentation} from './api/useGetVerifiablePresentation';
export {default as useGetVerifiablePresentations} from './api/useGetVerifiablePresentations';
export {default as useInitiateVerification} from './api/useInitiateVerification';
export {default as useUpdateVerifiableCredential} from './api/useUpdateVerifiableCredential';
export {default as useUpdateVerifiablePresentation} from './api/useUpdateVerifiablePresentation';
export {default as useVerificationStatus} from './api/useVerificationStatus';

// Components
export {default as CredentialOfferDialog} from './components/CredentialOfferDialog';
export {default as VerifiableCredentialDeleteDialog} from './components/VerifiableCredentialDeleteDialog';
export {default as VerifiableCredentialForm} from './components/VerifiableCredentialForm';
export {default as VerifiableCredentialsList} from './components/VerifiableCredentialsList';
export {default as VerifiablePresentationDeleteDialog} from './components/VerifiablePresentationDeleteDialog';
export {default as VerifiablePresentationForm} from './components/VerifiablePresentationForm';
export {default as VerifiablePresentationsList} from './components/VerifiablePresentationsList';
export {default as VerificationDialog} from './components/VerificationDialog';

// Constants
export {default as VerifiableCredentialQueryKeys} from './constants/vc-query-keys';
export {default as VerifiablePresentationQueryKeys} from './constants/vp-query-keys';

// Models
export * from './models/vc';
export * from './models/vp';
export type {CreateVerifiableCredentialRequest, UpdateVerifiableCredentialRequest} from './models/credential-requests';
export type {
  CreateVerifiablePresentationRequest,
  UpdateVerifiablePresentationRequest,
} from './models/presentation-requests';

// Pages
export {default as VerifiableCredentialCreatePage} from './pages/VerifiableCredentialCreatePage';
export {default as VerifiableCredentialEditPage} from './pages/VerifiableCredentialEditPage';
export {default as VerifiableCredentialsListPage} from './pages/VerifiableCredentialsListPage';
export type {VerifiableCredentialsListPageProps} from './pages/VerifiableCredentialsListPage';
export {default as VerifiablePresentationCreatePage} from './pages/VerifiablePresentationCreatePage';
export {default as VerifiablePresentationEditPage} from './pages/VerifiablePresentationEditPage';
export {default as VerifiablePresentationsListPage} from './pages/VerifiablePresentationsListPage';

// Routes
export type {VerifiableCredentialRoutePaths} from './hooks/useVerifiableCredentialRoutes';
export {
  defaultVerifiableCredentialRoutePaths,
  default as useVerifiableCredentialRoutes,
} from './hooks/useVerifiableCredentialRoutes';
