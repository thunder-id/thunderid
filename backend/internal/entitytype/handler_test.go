package entitytype

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
)

// InlineStubEntityTypeService satisfies the service interface contract cleanly.
type InlineStubEntityTypeService struct {
	OnCreateEntityType func(
		ctx context.Context, cat TypeCategory, req CreateEntityTypeRequestWithID,
	) (*EntityType, *tidcommon.ServiceError)
	OnUpdateEntityType func(
		ctx context.Context, cat TypeCategory, id string, req UpdateEntityTypeRequest,
	) (*EntityType, *tidcommon.ServiceError)
	OnGetEntityType func(
		ctx context.Context, cat TypeCategory, id string, inc bool,
	) (*EntityType, *tidcommon.ServiceError)
	OnDeleteEntityType func(ctx context.Context, cat TypeCategory, id string) *tidcommon.ServiceError
}

func (s *InlineStubEntityTypeService) CreateEntityType(
	ctx context.Context, cat TypeCategory, req CreateEntityTypeRequestWithID,
) (*EntityType, *tidcommon.ServiceError) {
	if s.OnCreateEntityType != nil {
		return s.OnCreateEntityType(ctx, cat, req)
	}
	return &EntityType{ID: "type-123", Name: req.Name}, nil
}

func (s *InlineStubEntityTypeService) UpdateEntityType(
	ctx context.Context, cat TypeCategory, id string, req UpdateEntityTypeRequest,
) (*EntityType, *tidcommon.ServiceError) {
	if s.OnUpdateEntityType != nil {
		return s.OnUpdateEntityType(ctx, cat, id, req)
	}
	return &EntityType{ID: id, Name: req.Name}, nil
}

func (s *InlineStubEntityTypeService) GetEntityType(
	ctx context.Context, cat TypeCategory, id string, inc bool,
) (*EntityType, *tidcommon.ServiceError) {
	if s.OnGetEntityType != nil {
		return s.OnGetEntityType(ctx, cat, id, inc)
	}
	return &EntityType{ID: id}, nil
}

func (s *InlineStubEntityTypeService) DeleteEntityType(
	ctx context.Context, cat TypeCategory, id string,
) *tidcommon.ServiceError {
	if s.OnDeleteEntityType != nil {
		return s.OnDeleteEntityType(ctx, cat, id)
	}
	return nil
}

func (s *InlineStubEntityTypeService) GetEntityTypeList(
	ctx context.Context, cat TypeCategory, limit, offset int, inc bool,
) (*EntityTypeListResponse, *tidcommon.ServiceError) {
	return &EntityTypeListResponse{Types: []EntityTypeListItem{}}, nil
}

func (s *InlineStubEntityTypeService) GetAttributes(
	ctx context.Context, cat TypeCategory, id string, f1, f2, f3 bool,
) ([]AttributeInfo, *tidcommon.ServiceError) {
	return []AttributeInfo{}, nil
}

func (s *InlineStubEntityTypeService) GetDisplayAttributesByNames(
	ctx context.Context, cat TypeCategory, names []string,
) (map[string]string, *tidcommon.ServiceError) {
	return map[string]string{}, nil
}

func (s *InlineStubEntityTypeService) GetEntityTypeByName(
	ctx context.Context, cat TypeCategory, name string,
) (*EntityType, *tidcommon.ServiceError) {
	return &EntityType{Name: name}, nil
}

func (s *InlineStubEntityTypeService) GetUniqueAttributes(
	ctx context.Context, cat TypeCategory, name string,
) ([]string, *tidcommon.ServiceError) {
	return []string{}, nil
}

func (s *InlineStubEntityTypeService) ResolveEntityTypeHandles(
	ctx context.Context, entityType *EntityType,
) *tidcommon.ServiceError {
	return nil
}

func (s *InlineStubEntityTypeService) IsEntityTypeExists(
	ctx context.Context, cat TypeCategory, name string,
) (bool, *tidcommon.ServiceError) {
	return true, nil
}

func (s *InlineStubEntityTypeService) ValidateEntity(
	ctx context.Context, cat TypeCategory, name string, schema json.RawMessage, flag bool,
) (bool, *tidcommon.ServiceError) {
	return true, nil
}

func (s *InlineStubEntityTypeService) ValidateEntityUniqueness(
	ctx context.Context, cat TypeCategory, name string, schema json.RawMessage,
	eval func(map[string]interface{}) (bool, error),
) (bool, *tidcommon.ServiceError) {
	return true, nil
}

// --- POST ENDPOINT TESTS ---

func TestHandleEntityTypePostRequest_Success(t *testing.T) {
	stub := &InlineStubEntityTypeService{}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	goodJSON := `{"name": "CustomUserType", "ouId": "ou-123", "schema": {}}`
	req := httptest.NewRequest(http.MethodPost, "/user-types", bytes.NewBufferString(goodJSON))
	w := httptest.NewRecorder()

	handler.HandleEntityTypePostRequest(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestHandleEntityTypePostRequest_ValidationError(t *testing.T) {
	stub := &InlineStubEntityTypeService{}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	badJSON := `{"name": "ab"}`
	req := httptest.NewRequest(http.MethodPost, "/user-types", bytes.NewBufferString(badJSON))
	w := httptest.NewRecorder()

	handler.HandleEntityTypePostRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleEntityTypePostRequest_MalformedJSON(t *testing.T) {
	stub := &InlineStubEntityTypeService{}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	req := httptest.NewRequest(http.MethodPost, "/user-types", bytes.NewBufferString(`{bad-json`))
	w := httptest.NewRecorder()

	handler.HandleEntityTypePostRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleEntityTypePostRequest_ConflictError(t *testing.T) {
	stub := &InlineStubEntityTypeService{
		OnCreateEntityType: func(
			ctx context.Context, cat TypeCategory, req CreateEntityTypeRequestWithID,
		) (*EntityType, *tidcommon.ServiceError) {
			return nil, &tidcommon.ServiceError{
				Type: tidcommon.ClientErrorType,
				Code: "ALREADY_EXISTS",
			}
		},
	}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	goodJSON := `{"name": "DuplicateUserType", "ouId": "ou-123", "schema": {}}`
	req := httptest.NewRequest(http.MethodPost, "/user-types", bytes.NewBufferString(goodJSON))
	w := httptest.NewRecorder()

	handler.HandleEntityTypePostRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleEntityTypePostRequest_ServiceError(t *testing.T) {
	stub := &InlineStubEntityTypeService{
		OnCreateEntityType: func(
			ctx context.Context, cat TypeCategory, req CreateEntityTypeRequestWithID,
		) (*EntityType, *tidcommon.ServiceError) {
			return nil, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType}
		},
	}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	goodJSON := `{"name": "CustomUserType", "ouId": "ou-123", "schema": {}}`
	req := httptest.NewRequest(http.MethodPost, "/user-types", bytes.NewBufferString(goodJSON))
	w := httptest.NewRecorder()

	handler.HandleEntityTypePostRequest(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- PUT ENDPOINT TESTS ---

func TestHandleEntityTypePutRequest_Success(t *testing.T) {
	stub := &InlineStubEntityTypeService{}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	goodJSON := `{"name": "UpdatedUserType", "ouId": "ou-123", "schema": {}}`
	req := httptest.NewRequest(http.MethodPut, "/user-types/type-123", bytes.NewBufferString(goodJSON))
	req.SetPathValue("id", "type-123")
	w := httptest.NewRecorder()

	handler.HandleEntityTypePutRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleEntityTypePutRequest_MissingID(t *testing.T) {
	stub := &InlineStubEntityTypeService{}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	req := httptest.NewRequest(http.MethodPut, "/user-types/", bytes.NewBufferString(`{}`))
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleEntityTypePutRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleEntityTypePutRequest_NotFound(t *testing.T) {
	stub := &InlineStubEntityTypeService{
		OnUpdateEntityType: func(
			ctx context.Context, cat TypeCategory, id string, req UpdateEntityTypeRequest,
		) (*EntityType, *tidcommon.ServiceError) {
			return nil, &tidcommon.ServiceError{
				Type: tidcommon.ClientErrorType,
				Code: "NOT_FOUND",
			}
		},
	}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	goodJSON := `{"name": "MissingUserType", "ouId": "ou-123", "schema": {}}`
	req := httptest.NewRequest(http.MethodPut, "/user-types/missing-123", bytes.NewBufferString(goodJSON))
	req.SetPathValue("id", "missing-123")
	w := httptest.NewRecorder()

	handler.HandleEntityTypePutRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleEntityTypePutRequest_ServiceError(t *testing.T) {
	stub := &InlineStubEntityTypeService{
		OnUpdateEntityType: func(
			ctx context.Context, cat TypeCategory, id string, req UpdateEntityTypeRequest,
		) (*EntityType, *tidcommon.ServiceError) {
			return nil, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType}
		},
	}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	goodJSON := `{"name": "UpdatedUserType", "ouId": "ou-123", "schema": {}}`
	req := httptest.NewRequest(http.MethodPut, "/user-types/type-123", bytes.NewBufferString(goodJSON))
	req.SetPathValue("id", "type-123")
	w := httptest.NewRecorder()

	handler.HandleEntityTypePutRequest(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- GET ENDPOINT TESTS ---

func TestHandleEntityTypeGetRequest_Success(t *testing.T) {
	stub := &InlineStubEntityTypeService{}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	req := httptest.NewRequest(http.MethodGet, "/user-types/type-123", nil)
	req.SetPathValue("id", "type-123")
	w := httptest.NewRecorder()

	handler.HandleEntityTypeGetRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleEntityTypeGetRequest_MissingID(t *testing.T) {
	stub := &InlineStubEntityTypeService{}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	req := httptest.NewRequest(http.MethodGet, "/user-types/", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleEntityTypeGetRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleEntityTypeGetRequest_NotFound(t *testing.T) {
	stub := &InlineStubEntityTypeService{
		OnGetEntityType: func(
			ctx context.Context, cat TypeCategory, id string, inc bool,
		) (*EntityType, *tidcommon.ServiceError) {
			return nil, &tidcommon.ServiceError{
				Type: tidcommon.ClientErrorType,
				Code: "NOT_FOUND",
			}
		},
	}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	req := httptest.NewRequest(http.MethodGet, "/user-types/missing-123", nil)
	req.SetPathValue("id", "missing-123")
	w := httptest.NewRecorder()

	handler.HandleEntityTypeGetRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleEntityTypeGetRequest_ServiceError(t *testing.T) {
	stub := &InlineStubEntityTypeService{
		OnGetEntityType: func(
			ctx context.Context, cat TypeCategory, id string, inc bool,
		) (*EntityType, *tidcommon.ServiceError) {
			return nil, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType}
		},
	}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	req := httptest.NewRequest(http.MethodGet, "/user-types/type-123", nil)
	req.SetPathValue("id", "type-123")
	w := httptest.NewRecorder()

	handler.HandleEntityTypeGetRequest(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- DELETE ENDPOINT TESTS ---

func TestHandleEntityTypeDeleteRequest_Success(t *testing.T) {
	stub := &InlineStubEntityTypeService{}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	req := httptest.NewRequest(http.MethodDelete, "/user-types/type-123", nil)
	req.SetPathValue("id", "type-123")
	w := httptest.NewRecorder()

	handler.HandleEntityTypeDeleteRequest(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHandleEntityTypeDeleteRequest_MissingID(t *testing.T) {
	stub := &InlineStubEntityTypeService{}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	req := httptest.NewRequest(http.MethodDelete, "/user-types/", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleEntityTypeDeleteRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleEntityTypeDeleteRequest_NotFound(t *testing.T) {
	stub := &InlineStubEntityTypeService{
		OnDeleteEntityType: func(ctx context.Context, cat TypeCategory, id string) *tidcommon.ServiceError {
			return &tidcommon.ServiceError{
				Type: tidcommon.ClientErrorType,
				Code: "NOT_FOUND",
			}
		},
	}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	req := httptest.NewRequest(http.MethodDelete, "/user-types/missing-123", nil)
	req.SetPathValue("id", "missing-123")
	w := httptest.NewRecorder()

	handler.HandleEntityTypeDeleteRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleEntityTypeDeleteRequest_ServiceError(t *testing.T) {
	stub := &InlineStubEntityTypeService{
		OnDeleteEntityType: func(ctx context.Context, cat TypeCategory, id string) *tidcommon.ServiceError {
			return &tidcommon.ServiceError{Type: tidcommon.ServerErrorType}
		},
	}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	req := httptest.NewRequest(http.MethodDelete, "/user-types/type-123", nil)
	req.SetPathValue("id", "type-123")
	w := httptest.NewRecorder()

	handler.HandleEntityTypeDeleteRequest(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- LIST ENDPOINT TESTS ---

func TestHandleEntityTypeListRequest_Success(t *testing.T) {
	stub := &InlineStubEntityTypeService{}
	handler := newEntityTypeHandler(stub, TypeCategoryUser)
	req := httptest.NewRequest(http.MethodGet, "/user-types", nil)
	w := httptest.NewRecorder()

	handler.HandleEntityTypeListRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
