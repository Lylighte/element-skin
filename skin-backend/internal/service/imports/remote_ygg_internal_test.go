package imports

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestRemoteYggHTTPClientClonesAndBlocksRedirectsExactly(t *testing.T) {
	defaultClient := remoteYggHTTPClient(nil)
	if defaultClient.Timeout != 10*time.Second {
		t.Fatalf("default client timeout=%s; want 10s", defaultClient.Timeout)
	}

	base := &http.Client{Timeout: 3 * time.Second}
	cloned := remoteYggHTTPClient(base)
	if cloned == base || cloned.Timeout != 3*time.Second {
		t.Fatalf("base client clone mismatch: cloned=%#v base=%#v", cloned, base)
	}
	if err := cloned.CheckRedirect(&http.Request{}, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirect policy error=%v; want ErrUseLastResponse", err)
	}
	if base.CheckRedirect != nil {
		t.Fatal("base client should not be mutated")
	}
}
