package yggdrasil_test

import (
	"testing"

	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestNewLoadsSignerAndStoresDependencies(t *testing.T) {
	cfg := testutil.TestConfig()
	ygg, err := yggdrasil.New(nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if ygg.Cfg.PublicKeyPath != cfg.PublicKeyPath || ygg.Signer == nil {
		t.Fatalf("New should retain config and load signer: %#v", ygg)
	}
}
