package buckets

import (
	"testing"

	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/handlers/buckets/testdata"
)

//go:generate go test

func TestCreateBucket(t *testing.T) {
	cfg := &types.Config{Name: "test-user"}
	h := MakeCreateHandler(cfg)
	c := &testdata.MockGinContext{Headers: map[string]string{"Authorization": "Bearer test"}}
	h(c)
	if !c.StatusCalled && !c.JSONCalled {
		t.Error("Expected response to be set")
	}
}

func TestListBucket(t *testing.T) {
	cfg := &types.Config{Name: "test-user"}
	h := MakeListHandler(cfg)
	c := &testdata.MockGinContext{Headers: map[string]string{"Authorization": "Bearer test"}}
	h(c)
	if !c.StatusCalled && !c.JSONCalled {
		t.Error("Expected response to be set")
	}
}

func TestUpdateBucket(t *testing.T) {
	cfg := &types.Config{Name: "test-user"}
	h := MakeUpdateHandler(cfg)
	c := &testdata.MockGinContext{Headers: map[string]string{"Authorization": "Bearer test"}}
	h(c)
	if !c.StatusCalled && !c.JSONCalled {
		t.Error("Expected response to be set")
	}
}

func TestDeleteBucket(t *testing.T) {
	cfg := &types.Config{Name: "test-user"}
	h := MakeDeleteHandler(cfg)
	c := &testdata.MockGinContext{Headers: map[string]string{"Authorization": "Bearer test"}}
	h(c)
	if !c.StatusCalled && !c.JSONCalled {
		t.Error("Expected response to be set")
	}
}
