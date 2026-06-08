package texture_test

import (
	"errors"
	"testing"

	"element-skin/backend/internal/database/texture"
)

func TestErrNotFoundIsStableSentinel(t *testing.T) {
	if !errors.Is(texture.ErrNotFound, texture.ErrNotFound) || texture.ErrNotFound.Error() != "not found" {
		t.Fatalf("unexpected ErrNotFound sentinel: %v", texture.ErrNotFound)
	}
}
