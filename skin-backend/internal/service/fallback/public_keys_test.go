package fallback_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/fallback"
	"element-skin/backend/internal/testutil"
)

func TestLongestCommonYggdrasilRootExactResults(t *testing.T) {
	tests := []struct {
		name        string
		sessionURL  string
		accountURL  string
		servicesURL string
		want        string
		ok          bool
	}{
		{
			name:        "shared nested root",
			sessionURL:  "https://skin.example/ygg/sessionserver/",
			accountURL:  "https://skin.example/ygg/api",
			servicesURL: "https://skin.example/ygg/services",
			want:        "https://skin.example/ygg/",
			ok:          true,
		},
		{
			name:        "origin root",
			sessionURL:  "https://skin.example/sessionserver",
			accountURL:  "https://skin.example/api",
			servicesURL: "https://skin.example/services",
			want:        "https://skin.example/",
			ok:          true,
		},
		{
			name:        "different origins",
			sessionURL:  "https://session.example",
			accountURL:  "https://account.example",
			servicesURL: "https://services.example",
			ok:          false,
		},
		{
			name:        "invalid URL",
			sessionURL:  "not-a-url",
			accountURL:  "https://skin.example/api",
			servicesURL: "https://skin.example/services",
			ok:          false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := fallback.LongestCommonYggdrasilRoot(test.sessionURL, test.accountURL, test.servicesURL)
			if got != test.want || ok != test.ok {
				t.Fatalf("root=(%q,%t), want (%q,%t)", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestPublicKeySourcesDeduplicateExactRequestSources(t *testing.T) {
	endpoints := []map[string]any{
		{"session_url": "https://one.example/ygg/session", "account_url": "https://one.example/ygg/account", "services_url": "https://one.example/ygg/services"},
		{"session_url": "https://one.example/ygg/session/", "account_url": "https://one.example/ygg/account/", "services_url": "https://one.example/ygg/services/"},
		{"session_url": "https://session.two.example", "account_url": "https://account.two.example", "services_url": "https://services.two.example/base"},
	}
	sources := fallback.PublicKeySources(endpoints)
	if len(sources) != 2 {
		t.Fatalf("source count=%d, want 2: %#v", len(sources), sources)
	}
	if sources[0].ID == "" || sources[0].DiscoveryURL != "https://one.example/ygg/" || sources[0].ServicesPublicURL != "https://one.example/ygg/services/publickeys/" {
		t.Fatalf("first source mismatch: %#v", sources[0])
	}
	if sources[1].ID == "" || sources[1].ID == sources[0].ID || sources[1].DiscoveryURL != "" || sources[1].ServicesPublicURL != "https://services.two.example/base/publickeys/" {
		t.Fatalf("second source mismatch: %#v", sources[1])
	}
}

func TestPublicKeyParsersNormalizeAndDeduplicateExactKeys(t *testing.T) {
	first := testutil.NewPublicKeyFixture(t)
	second := testutil.NewPublicKeyFixture(t)
	discovery := []byte(`{"signaturePublickey":` + quoteJSON(first.PEM) + `,"signaturePublickeys":[` + quoteJSON(first.PEM) + `,` + quoteJSON(second.PEM) + `]}`)
	got, err := fallback.ParseDiscoveryPublicKeys(discovery)
	if err != nil {
		t.Fatal(err)
	}
	wantEntries := []model.YggdrasilPublicKey{{PublicKey: first.DERBase64}, {PublicKey: second.DERBase64}}
	if !reflect.DeepEqual(got.ProfilePropertyKeys, wantEntries) || !reflect.DeepEqual(got.PlayerCertificateKeys, wantEntries) {
		t.Fatalf("discovery keys mismatch:\n got=%#v\nwant=%#v", got, wantEntries)
	}

	services := []byte(`{"profilePropertyKeys":[{"publicKey":` + quoteJSON(first.DERBase64) + `},{"publicKey":` + quoteJSON(first.DERBase64) + `}],"playerCertificateKeys":[{"publicKey":` + quoteJSON(second.DERBase64) + `}]}`)
	got, err = fallback.ParseServicesPublicKeys(services)
	if err != nil {
		t.Fatal(err)
	}
	want := model.YggdrasilPublicKeys{
		ProfilePropertyKeys:   []model.YggdrasilPublicKey{{PublicKey: first.DERBase64}},
		PlayerCertificateKeys: []model.YggdrasilPublicKey{{PublicKey: second.DERBase64}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("services keys mismatch:\n got=%#v\nwant=%#v", got, want)
	}
}

func TestPublicKeyParsersRejectMalformedResponsesExactly(t *testing.T) {
	tests := []struct {
		name      string
		parse     func([]byte) (model.YggdrasilPublicKeys, error)
		body      string
		wantError string
	}{
		{name: "empty discovery", parse: fallback.ParseDiscoveryPublicKeys, body: `{}`, wantError: "discovery metadata contains no signature public key"},
		{name: "multiple discovery values", parse: fallback.ParseDiscoveryPublicKeys, body: `{} {}`, wantError: "response contains multiple JSON values"},
		{name: "invalid discovery PEM", parse: fallback.ParseDiscoveryPublicKeys, body: `{"signaturePublickey":"bad"}`, wantError: "discovery metadata contains invalid PEM public key"},
		{name: "missing profile keys", parse: fallback.ParseServicesPublicKeys, body: `{"profilePropertyKeys":[],"playerCertificateKeys":[]}`, wantError: "profilePropertyKeys must contain at least one RSA public key"},
		{name: "invalid services base64", parse: fallback.ParseServicesPublicKeys, body: `{"profilePropertyKeys":[{"publicKey":"bad"}],"playerCertificateKeys":[]}`, wantError: "profilePropertyKeys contains invalid base64 public key: illegal base64 data at input byte 0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.parse([]byte(test.body))
			if err == nil || err.Error() != test.wantError || !reflect.DeepEqual(got, model.YggdrasilPublicKeys{}) {
				t.Fatalf("result=%#v err=%v, want zero result and %q", got, err, test.wantError)
			}
		})
	}
}

func quoteJSON(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
