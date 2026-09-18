// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	kmcommon "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// testSecret is the key a test registration carries.
const testSecret = "shhh"

// testCreatedAt stands in for the timestamp a database assigns on insert.
var testCreatedAt = time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)

// reversingCrypto stands in for the deployment's configuration key. It only has to be reversible
// and to leave the ciphertext different from the plaintext.
type reversingCrypto struct{}

func (reversingCrypto) Encrypt(_ context.Context, content []byte) ([]byte, error) {
	return append([]byte("enc:"), content...), nil
}

func (reversingCrypto) Decrypt(_ context.Context, content []byte) ([]byte, error) {
	return content[len("enc:"):], nil
}

var _ kmcommon.ConfigCryptoProvider = reversingCrypto{}

type fakeStore struct {
	gateways []Gateway
	err      error
	// createRefuses makes Create report that the write was refused while Count still reports
	// capacity. That is what the database does when another registration commits in between, and it
	// is the only way to exercise the path where the write, not the count, enforces the limit.
	createRefuses bool
}

func (f *fakeStore) List(context.Context) ([]Gateway, error) { return f.gateways, f.err }

// GetByID returns a copy, as the real store does: it builds a fresh struct from the row. Handing
// back a pointer into the slice would let a caller that blanks the key blank the stored one too.
func (f *fakeStore) GetByID(_ context.Context, id string) (*Gateway, error) {
	for i := range f.gateways {
		if f.gateways[i].ID == id {
			found := f.gateways[i]
			return &found, nil
		}
	}
	return nil, f.err
}

//nolint:dupl // The three lookups differ only in the field matched; sharing them would hide that.
func (f *fakeStore) GetByName(_ context.Context, name string) (*Gateway, error) {
	for i := range f.gateways {
		if f.gateways[i].Name == name {
			found := f.gateways[i]
			return &found, nil
		}
	}
	return nil, f.err
}

func (f *fakeStore) GetByDataPlaneID(_ context.Context, dataPlaneID string) (*Gateway, error) {
	if f.err != nil {
		return nil, f.err
	}
	for i := range f.gateways {
		if f.gateways[i].DataPlaneID == dataPlaneID {
			found := f.gateways[i]
			return &found, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) Count(context.Context) (int, error) { return len(f.gateways), f.err }

// Create mirrors the real store: the limit is enforced by the write, and the row it stores carries
// the timestamps a database would have set, so a caller reading back gets what it would in practice.
// Create mirrors the real store: the limit is enforced by the write, and it hands back the row it
// stored, carrying the timestamps a database would have assigned.
func (f *fakeStore) Create(_ context.Context, gw *Gateway, limit int) (*Gateway, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.createRefuses || len(f.gateways) >= limit {
		return nil, nil
	}
	stored := *gw
	stored.CreatedAt = testCreatedAt
	stored.UpdatedAt = testCreatedAt
	f.gateways = append(f.gateways, stored)

	written := stored
	return &written, nil
}

func (f *fakeStore) Update(_ context.Context, gw *Gateway) error {
	if f.err != nil {
		return f.err
	}
	for i := range f.gateways {
		if f.gateways[i].ID == gw.ID {
			f.gateways[i] = *gw
			return nil
		}
	}
	return nil
}

func (f *fakeStore) Delete(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	for i := range f.gateways {
		if f.gateways[i].ID == id {
			f.gateways = append(f.gateways[:i], f.gateways[i+1:]...)
			return nil
		}
	}
	return nil
}

func validRequest(name string) RegisterRequest {
	return RegisterRequest{
		Name:        name,
		BaseURL:     "https://dp.example.test",
		DataPlaneID: "dp-1", Key: "shhh",
	}
}

func newTestService(t *testing.T, store storeInterface) ServiceInterface {
	t.Helper()
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })
	return newService(store)
}

func TestRegisterRecordsTheGateway(t *testing.T) {
	store := &fakeStore{}
	gw, svcErr := newTestService(t, store).Register(context.Background(), validRequest("dev"))
	if svcErr != nil {
		t.Fatalf("register: %v", svcErr)
	}
	if gw.ID == "" || gw.Name != "dev" {
		t.Fatalf("unexpected gateway: %+v", gw)
	}
	if len(store.gateways) != 1 {
		t.Fatalf("expected the gateway to be stored, got %d", len(store.gateways))
	}
}

// A deployment pairs with one data plane unless it is configured otherwise, so the second
// registration is refused rather than quietly accepted.
func TestRegisterRefusesMoreGatewaysThanConfigured(t *testing.T) {
	store := &fakeStore{}
	svc := newTestService(t, store)

	if _, svcErr := svc.Register(context.Background(), validRequest("dev")); svcErr != nil {
		t.Fatalf("first register: %v", svcErr)
	}
	_, svcErr := svc.Register(context.Background(), validRequest("prod"))
	if svcErr == nil || svcErr.Code != ErrorGatewayLimitReached.Code {
		t.Fatalf("expected the limit to be enforced, got %v", svcErr)
	}
	if len(store.gateways) != 1 {
		t.Fatalf("expected the second registration not to be stored, got %d", len(store.gateways))
	}
}

// The credential must not be readable in the database, so what is stored is not the plaintext.
func TestRegisterStoresTheCredentialEncrypted(t *testing.T) {
	store := &fakeStore{}
	if _, svcErr := newTestService(t, store).Register(context.Background(), validRequest("dev")); svcErr != nil {
		t.Fatalf("register: %v", svcErr)
	}

	stored := store.gateways[0].Key
	if stored == "" {
		t.Fatal("expected a stored credential")
	}
	if stored == "shhh" {
		t.Fatal("the management token was stored in the clear")
	}

	// It must still be recoverable, or nothing could ever authenticate to the data plane.
	props, err := cmodels.DeserializePropertiesFromJSON(stored)
	if err != nil || len(props) != 1 {
		t.Fatalf("stored credential is unreadable: %v", err)
	}
	value, err := props[0].GetValue()
	if err != nil || value != "shhh" {
		t.Fatalf("expected the credential to decrypt, got %q err=%v", value, err)
	}
}

// A credential must never travel back out of the API.
func TestTheAPINeverReturnsTheCredential(t *testing.T) {
	store := &fakeStore{}
	svc := newTestService(t, store)
	created, _ := svc.Register(context.Background(), validRequest("dev"))
	if created.Key != "" {
		t.Error("register returned the credential")
	}

	listed, _ := svc.List(context.Background())
	if len(listed) != 1 || listed[0].Key != "" {
		t.Error("list returned the credential")
	}

	got, _ := svc.Get(context.Background(), created.ID)
	if got.Key != "" {
		t.Error("get returned the credential")
	}
}

func TestRegisterRefusesADuplicateName(t *testing.T) {
	store := &fakeStore{gateways: []Gateway{{ID: "a", Name: "dev"}}}
	_, svcErr := newTestService(t, store).Register(context.Background(), validRequest("dev"))
	if svcErr == nil {
		t.Fatal("expected a refusal")
	}
}

// Without all three the gateway cannot be reached, and a registration that cannot be applied to is
// worse than none because it looks configured.
func TestRegisterRefusesAnUnreachableGateway(t *testing.T) {
	svc := newTestService(t, &fakeStore{})
	for _, req := range []RegisterRequest{
		{Name: "dev", DataPlaneID: "dp-1", Key: "b"},
		{Name: "dev", BaseURL: "https://dp", Key: "b"},
		{Name: "dev", BaseURL: "https://dp", DataPlaneID: "a"},
	} {
		if _, svcErr := svc.Register(context.Background(), req); svcErr == nil ||
			svcErr.Code != ErrorGatewayConnectionRequired.Code {
			t.Fatalf("expected a connection refusal for %+v, got %v", req, svcErr)
		}
	}
}

func TestRegisterRequiresAName(t *testing.T) {
	req := validRequest("   ")
	if _, svcErr := newTestService(t, &fakeStore{}).Register(context.Background(), req); svcErr == nil ||
		svcErr.Code != ErrorGatewayNameRequired.Code {
		t.Fatalf("expected a name refusal, got %v", svcErr)
	}
}

func TestGetReportsAnUnknownGateway(t *testing.T) {
	_, svcErr := newTestService(t, &fakeStore{}).Get(context.Background(), "missing")
	if svcErr == nil || svcErr.Code != ErrorGatewayNotFound.Code {
		t.Fatalf("expected not found, got %v", svcErr)
	}
}

func TestDeleteRemovesTheGateway(t *testing.T) {
	store := &fakeStore{gateways: []Gateway{{ID: "a", Name: "dev"}}}
	if svcErr := newTestService(t, store).Delete(context.Background(), "a"); svcErr != nil {
		t.Fatalf("delete: %v", svcErr)
	}
	if len(store.gateways) != 0 {
		t.Fatal("expected the gateway to be removed")
	}
}

func TestListReportsAStoreFailure(t *testing.T) {
	_, svcErr := newTestService(t, &fakeStore{err: errors.New("database is down")}).List(context.Background())
	if svcErr == nil {
		t.Fatal("expected a failure to surface")
	}
}

// One data plane registers once. Two rows for the same data plane would be applied to
// independently and drift apart, and the second registration is usually the same one being added
// under a different hostname.
func TestRegisterRefusesADataPlaneThatIsAlreadyRegistered(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	// The limit is raised so this test exercises the uniqueness rule rather than max_gateways,
	// which by default admits only one and would refuse the second registration first.
	allowSeveralGateways(t)

	store := &fakeStore{gateways: []Gateway{
		{ID: "gw-1", Name: "production", DataPlaneID: "dp-1", BaseURL: "https://internal.dp"},
	}}
	svc := newService(store)

	_, svcErr := svc.Register(context.Background(), RegisterRequest{
		Name:        "production-external",
		DataPlaneID: "dp-1",
		BaseURL:     "https://external.dp",
		Key:         "some-key",
	})

	if svcErr == nil {
		t.Fatal("expected the second registration of a data plane to be refused")
	}
	if svcErr.Code != ErrorDataPlaneAlreadyRegistered.Code {
		t.Fatalf("expected %s, got %s", ErrorDataPlaneAlreadyRegistered.Code, svcErr.Code)
	}
}

// A registration is incomplete without the three things needed to reach the data plane.
func TestRegisterRequiresWhatItTakesToReachTheDataPlane(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	for name, req := range map[string]RegisterRequest{
		"no data plane id": {Name: "dev", BaseURL: "https://dp", Key: "k"},
		"no base url":      {Name: "dev", DataPlaneID: "dp-1", Key: "k"},
		"no key":           {Name: "dev", DataPlaneID: "dp-1", BaseURL: "https://dp"},
	} {
		svc := newService(&fakeStore{})
		if _, svcErr := svc.Register(context.Background(), req); svcErr == nil {
			t.Fatalf("%s: expected the registration to be refused", name)
		}
	}
}

// The key is never returned, by a read of one gateway or of the list. A read that returned it would
// hand every operator of this control plane the keys to every data plane it administers.
func TestAReadNeverReturnsTheKey(t *testing.T) {
	store := &fakeStore{gateways: []Gateway{
		{ID: "gw-1", Name: "dev", DataPlaneID: "dp-1", Key: "sealed-key-material"},
	}}
	svc := newService(store)

	gw, svcErr := svc.Get(context.Background(), "gw-1")
	if svcErr != nil {
		t.Fatalf("unexpected error: %v", svcErr)
	}
	if gw.Key != "" {
		t.Fatalf("a read returned the key: %q", gw.Key)
	}

	listed, svcErr := svc.List(context.Background())
	if svcErr != nil {
		t.Fatalf("unexpected error: %v", svcErr)
	}
	for _, g := range listed {
		if g.Key != "" {
			t.Fatalf("the listing returned a key: %q", g.Key)
		}
	}
}

// allowSeveralGateways lifts the registration bound for a test and restores it afterwards. The
// default is one, which would otherwise refuse a second registration before the rule under test is
// reached.
func allowSeveralGateways(t *testing.T) {
	t.Helper()
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{MaxGateways: 5},
	})
	t.Cleanup(config.ResetServerRuntime)
}

// A registration answers with the timestamps the database assigned, not the zero values the struct
// was built with. The handler serializes exactly this object, so year-one timestamps would reach
// the client.
func TestRegisterReturnsTheStoredTimestamps(t *testing.T) {
	allowSeveralGateways(t)

	gw, svcErr := newTestService(t, &fakeStore{}).Register(context.Background(), validRequest("dev"))
	if svcErr != nil {
		t.Fatalf("register: %v", svcErr)
	}
	if gw.CreatedAt.IsZero() || gw.UpdatedAt.IsZero() {
		t.Fatalf("registration returned unset timestamps: %+v", gw)
	}
	if !gw.CreatedAt.Equal(testCreatedAt) {
		t.Fatalf("expected the stored timestamp, got %v", gw.CreatedAt)
	}
}

// The limit is enforced by the write. A count taken beforehand cannot be trusted on its own, so the
// store reports that it wrote nothing and the service turns that into the limit error.
func TestRegisterRefusesWhenTheWriteReportsTheLimitWasReached(t *testing.T) {
	// Capacity is available as far as the count is concerned, so the pre-check passes and the
	// registration reaches the write. The write is what refuses it, which is the case where another
	// registration committed in between.
	allowSeveralGateways(t)

	store := &fakeStore{createRefuses: true}
	_, svcErr := newTestService(t, store).Register(context.Background(), validRequest("second"))

	if svcErr == nil {
		t.Fatal("expected the registration to be refused")
	}
	if svcErr.Code != ErrorGatewayLimitReached.Code {
		t.Fatalf("expected %s, got %s", ErrorGatewayLimitReached.Code, svcErr.Code)
	}
	if len(store.gateways) != 0 {
		t.Fatalf("expected nothing to be written, got %d rows", len(store.gateways))
	}
}

// A declared gateway may not take a data plane another gateway already owns. Without this the unique
// constraint refuses the update and the caller is told only that something went wrong.
func TestAdoptRefusesADataPlaneOwnedByAnotherGateway(t *testing.T) {
	allowSeveralGateways(t)

	store := &fakeStore{gateways: []Gateway{
		{ID: "gw-1", Name: "dev", DataPlaneID: "dp-1"},
		{ID: "gw-2", Name: "staging", DataPlaneID: "dp-2"},
	}}

	svcErr := newTestService(t, store).Adopt(context.Background(), RegisterRequest{
		Name:        "staging",
		DataPlaneID: "dp-1", // already owned by gw-1
		BaseURL:     "https://dp",
		Key:         "k",
	})

	if svcErr == nil {
		t.Fatal("expected the adoption to be refused")
	}
	if svcErr.Code != ErrorDataPlaneAlreadyRegistered.Code {
		t.Fatalf("expected %s, got %s", ErrorDataPlaneAlreadyRegistered.Code, svcErr.Code)
	}
}

// Re-reading the same declaration keeps its own data plane, which is what makes the load idempotent.
func TestAdoptKeepsItsOwnDataPlane(t *testing.T) {
	allowSeveralGateways(t)

	store := &fakeStore{gateways: []Gateway{{ID: "gw-1", Name: "dev", DataPlaneID: "dp-1"}}}

	svcErr := newTestService(t, store).Adopt(context.Background(), RegisterRequest{
		Name:        "dev",
		DataPlaneID: "dp-1",
		BaseURL:     "https://dp",
		Key:         "k",
	})

	if svcErr != nil {
		t.Fatalf("re-reading a declaration must not fail: %v", svcErr)
	}
}

// A base URL that trims to nothing usable is refused where the caller can still see which field was
// wrong, rather than stored as an address no call can reach.
func TestRegisterRefusesABaseURLThatIsNotOne(t *testing.T) {
	allowSeveralGateways(t)

	for name, raw := range map[string]string{
		"just a slash": "/",
		"scheme only":  "https://",
		"no scheme":    "dp.example.com:8090",
		"not a url":    "not a url",
		"wrong scheme": "ftp://dp.example.com",
		"slashes only": "///",
	} {
		req := validRequest("dev")
		req.BaseURL = raw
		store := &fakeStore{}

		_, svcErr := newTestService(t, store).Register(context.Background(), req)

		if svcErr == nil {
			t.Fatalf("%s: expected %q to be refused", name, raw)
		}
		if svcErr.Code != ErrorInvalidBaseURL.Code && svcErr.Code != ErrorGatewayConnectionRequired.Code {
			t.Fatalf("%s: unexpected error for %q: %s", name, raw, svcErr.Code)
		}
		if len(store.gateways) != 0 {
			t.Fatalf("%s: %q was stored anyway", name, raw)
		}
	}
}

// A usable base URL is stored without its trailing slash, and otherwise unchanged.
func TestRegisterStoresTheBaseURLNormalized(t *testing.T) {
	allowSeveralGateways(t)

	req := validRequest("dev")
	req.BaseURL = "  https://dp.example.com:8090/  "
	store := &fakeStore{}

	gw, svcErr := newTestService(t, store).Register(context.Background(), req)

	if svcErr != nil {
		t.Fatalf("register: %v", svcErr)
	}
	if gw.BaseURL != "https://dp.example.com:8090" {
		t.Fatalf("stored base URL is %q", gw.BaseURL)
	}
}

// Every service method reports a store failure as an internal error rather than as a client mistake
// or, worse, as an empty result.
func TestAStoreFailureIsAnInternalError(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })
	allowSeveralGateways(t)

	for name, call := range map[string]func(ServiceInterface) *tidcommon.ServiceError{
		"list": func(s ServiceInterface) *tidcommon.ServiceError {
			_, err := s.List(context.Background())
			return err
		},
		"get": func(s ServiceInterface) *tidcommon.ServiceError {
			_, err := s.Get(context.Background(), "gw-1")
			return err
		},
		"delete": func(s ServiceInterface) *tidcommon.ServiceError {
			return s.Delete(context.Background(), "gw-1")
		},
		"register": func(s ServiceInterface) *tidcommon.ServiceError {
			_, err := s.Register(context.Background(), validRequest("dev"))
			return err
		},
		"adopt": func(s ServiceInterface) *tidcommon.ServiceError {
			return s.Adopt(context.Background(), validRequest("dev"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			svc := newService(&fakeStore{err: errors.New("database is down")})

			svcErr := call(svc)

			if svcErr == nil {
				t.Fatal("expected the failure to be reported")
			}
			if svcErr.Code != tidcommon.InternalServerError.Code {
				t.Fatalf("expected an internal error, got %s", svcErr.Code)
			}
		})
	}
}

// An id that is not one is refused before the store is asked.
func TestAnEmptyIDIsRefused(t *testing.T) {
	svc := newService(&fakeStore{})

	if _, svcErr := svc.Get(context.Background(), "   "); svcErr == nil {
		t.Fatal("expected a blank id to be refused by Get")
	}
	if svcErr := svc.Delete(context.Background(), "   "); svcErr == nil {
		t.Fatal("expected a blank id to be refused by Delete")
	}
}

// Deleting one that is not registered says so rather than reporting success.
func TestDeletingAGatewayThatIsNotRegistered(t *testing.T) {
	svc := newService(&fakeStore{})

	svcErr := svc.Delete(context.Background(), "absent")

	if svcErr == nil || svcErr.Code != ErrorGatewayNotFound.Code {
		t.Fatalf("expected not-found, got %v", svcErr)
	}
}

// A declaration missing its name is refused by Adopt as well as by the file validator.
func TestAdoptRefusesADeclarationWithoutAName(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	svcErr := newService(&fakeStore{}).Adopt(context.Background(), RegisterRequest{
		DataPlaneID: "dp-1", BaseURL: "https://dp", Key: "k"})

	if svcErr == nil || svcErr.Code != ErrorGatewayNameRequired.Code {
		t.Fatalf("expected a name to be required, got %v", svcErr)
	}
}

// If the credential cannot be sealed, nothing is stored. A gateway registered with an unreadable key
// would look configured and fail at the first call.
func TestNothingIsStoredWhenTheKeyCannotBeSealed(t *testing.T) {
	allowSeveralGateways(t)
	// No crypto provider, so sealing fails the way it would if the key manager were unavailable.
	cmodels.SetConfigCryptoProvider(nil)

	store := &fakeStore{}
	_, svcErr := newService(store).Register(context.Background(), validRequest("dev"))

	if svcErr == nil {
		t.Fatal("expected registration to fail when the credential cannot be sealed")
	}
	if len(store.gateways) != 0 {
		t.Fatalf("a gateway was stored despite the sealing failure: %+v", store.gateways)
	}
}
