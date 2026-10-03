package site_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestProfileRoutesRejectCaseConflictsWithExactErrorsAndUnchangedState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	owner := testutil.CreateUser(t, db, "route-case-owner@test.com", "Password123", "RouteCaseOwner", false)
	other := testutil.CreateUser(t, db, "route-case-other@test.com", "Password123", "RouteCaseOther", false)
	reserved := testutil.CreateProfile(t, db, owner.ID, "route_case_reserved", "CasePlayer")
	target := testutil.CreateProfile(t, db, other.ID, "route_case_target", "OtherPlayer")
	h := site.New(testutil.TestConfig(), db, nil)
	for _, tc := range []struct {
		name, method, path string
		call               func(http.ResponseWriter, *http.Request)
	}{
		{"create", http.MethodPost, "/v2/users/me/profiles", h.CreateProfile},
		{"rename", http.MethodPatch, "/v2/users/me/profiles/" + target.ID, h.UpdateProfile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"name":"caseplayer"}`))
			req.SetPathValue("pid", target.ID)
			req = withUserActor(req, other.ID)
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"profile_name\",\"operation\":\"reserve\",\"reason\":\"conflict\"}}\n" {
				t.Fatalf("case conflict status=%d body=%q", rec.Code, rec.Body.String())
			}
			for _, want := range []model.Profile{reserved, target} {
				got, err := db.Profiles.GetByID(t.Context(), want.ID)
				if err != nil || !reflect.DeepEqual(got, &want) {
					t.Fatalf("case conflict changed profile=%#v err=%v", got, err)
				}
			}
			if count, err := db.Profiles.CountByUser(t.Context(), other.ID); err != nil || count != 1 {
				t.Fatalf("case conflict left extra profile count=%d err=%v", count, err)
			}
		})
	}
}
