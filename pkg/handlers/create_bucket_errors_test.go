package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/grycap/oscar/v4/pkg/types"
)

func TestBucketCreationErrorStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"bucket already exists", types.ErrBucketAlreadyExists, http.StatusConflict},
		{"other bucket creation failure", types.ErrBucketCreation, http.StatusBadRequest},
		{"invalid input", errInput, http.StatusBadRequest},
		{"unrelated failure", errors.New("unrelated failure"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bucketCreationErrorStatus(tt.err); got != tt.want {
				t.Errorf("bucketCreationErrorStatus() = %d, want %d", got, tt.want)
			}
		})
	}
}
