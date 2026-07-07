package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewElementSkinClientNilFallbackHasTimeout(t *testing.T) {
	c := NewElementSkinClient("http://127.0.0.1:1", nil)
	if c.httpClient == nil {
		t.Fatal("NewElementSkinClient with nil httpClient: httpClient is nil")
	}
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("httpClient.Timeout = %v, want 30s", c.httpClient.Timeout)
	}
}

func TestListAllProfilesStopsAfterMaxPages(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(adminProfilesResponse{
			Items:      []AdminProfile{},
			HasNext:    true,
			NextCursor: "cursor-next",
			PageSize:   100,
		})
	}))
	defer srv.Close()

	client := NewElementSkinClient(srv.URL, srv.Client())
	_, err := client.ListAllProfiles(context.Background(), "token", "")

	if callCount != maxAdminProfilePages {
		t.Errorf("expected %d admin API calls, got %d", maxAdminProfilePages, callCount)
	}
	if err == nil {
		t.Fatal("expected error from pagination limit, got nil")
	}
	if err.Error() != "admin profiles pagination exceeded maximum pages" {
		t.Errorf("error = %q, want 'admin profiles pagination exceeded maximum pages'", err.Error())
	}
}
