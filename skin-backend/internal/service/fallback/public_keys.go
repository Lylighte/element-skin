package fallback

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"element-skin/backend/internal/model"
)

const maxPublicKeysPerGroup = 128

type PublicKeySource struct {
	ID                string
	DiscoveryURL      string
	ServicesPublicURL string
}

type discoveryPublicKeys struct {
	SignaturePublicKey  string   `json:"signaturePublickey"`
	SignaturePublicKeys []string `json:"signaturePublickeys"`
}

func PublicKeySources(endpoints []map[string]any) []PublicKeySource {
	out := make([]PublicKeySource, 0, len(endpoints))
	seen := make(map[string]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		source := publicKeySource(endpoint)
		if source.ServicesPublicURL == "" {
			continue
		}
		if _, exists := seen[source.ID]; exists {
			continue
		}
		seen[source.ID] = struct{}{}
		out = append(out, source)
	}
	return out
}

func LongestCommonYggdrasilRoot(sessionURL, accountURL, servicesURL string) (string, bool) {
	parsed := make([]*url.URL, 0, 3)
	for _, raw := range []string{sessionURL, accountURL, servicesURL} {
		u, ok := parseEndpointURL(raw)
		if !ok {
			return "", false
		}
		parsed = append(parsed, u)
	}
	if !strings.EqualFold(parsed[0].Scheme, parsed[1].Scheme) ||
		!strings.EqualFold(parsed[0].Scheme, parsed[2].Scheme) ||
		!strings.EqualFold(parsed[0].Host, parsed[1].Host) ||
		!strings.EqualFold(parsed[0].Host, parsed[2].Host) {
		return "", false
	}
	common := splitPath(parsed[0].Path)
	for _, current := range parsed[1:] {
		parts := splitPath(current.Path)
		limit := len(common)
		if len(parts) < limit {
			limit = len(parts)
		}
		matched := 0
		for matched < limit && common[matched] == parts[matched] {
			matched++
		}
		common = common[:matched]
	}
	root := *parsed[0]
	root.RawPath = ""
	root.RawQuery = ""
	root.Fragment = ""
	root.Path = "/"
	if len(common) > 0 {
		root.Path = "/" + strings.Join(common, "/") + "/"
	}
	return root.String(), true
}

func ParseDiscoveryPublicKeys(body []byte) (model.YggdrasilPublicKeys, error) {
	var metadata discoveryPublicKeys
	if err := decodeSingleJSON(body, &metadata); err != nil {
		return model.YggdrasilPublicKeys{}, err
	}
	encoded := make([]string, 0, 1+len(metadata.SignaturePublicKeys))
	if strings.TrimSpace(metadata.SignaturePublicKey) != "" {
		encoded = append(encoded, metadata.SignaturePublicKey)
	}
	encoded = append(encoded, metadata.SignaturePublicKeys...)
	keys, err := NormalizePEMPublicKeys(encoded)
	if err != nil {
		return model.YggdrasilPublicKeys{}, err
	}
	if len(keys) == 0 {
		return model.YggdrasilPublicKeys{}, errors.New("discovery metadata contains no signature public key")
	}
	return model.YggdrasilPublicKeys{
		PlayerCertificateKeys: append([]model.YggdrasilPublicKey(nil), keys...),
		ProfilePropertyKeys:   append([]model.YggdrasilPublicKey(nil), keys...),
	}, nil
}

func ParseServicesPublicKeys(body []byte) (model.YggdrasilPublicKeys, error) {
	var keys model.YggdrasilPublicKeys
	if err := decodeSingleJSON(body, &keys); err != nil {
		return model.YggdrasilPublicKeys{}, err
	}
	return NormalizePublicKeys(keys)
}

func NormalizePublicKeys(keys model.YggdrasilPublicKeys) (model.YggdrasilPublicKeys, error) {
	profile, err := normalizeDERPublicKeys("profilePropertyKeys", keys.ProfilePropertyKeys)
	if err != nil {
		return model.YggdrasilPublicKeys{}, err
	}
	if len(profile) == 0 {
		return model.YggdrasilPublicKeys{}, errors.New("profilePropertyKeys must contain at least one RSA public key")
	}
	certificates, err := normalizeDERPublicKeys("playerCertificateKeys", keys.PlayerCertificateKeys)
	if err != nil {
		return model.YggdrasilPublicKeys{}, err
	}
	return model.YggdrasilPublicKeys{
		PlayerCertificateKeys: certificates,
		ProfilePropertyKeys:   profile,
	}, nil
}

func MergePublicKeys(groups ...model.YggdrasilPublicKeys) model.YggdrasilPublicKeys {
	return model.YggdrasilPublicKeys{
		PlayerCertificateKeys: mergePublicKeyGroups(groups, func(keys model.YggdrasilPublicKeys) []model.YggdrasilPublicKey {
			return keys.PlayerCertificateKeys
		}),
		ProfilePropertyKeys: mergePublicKeyGroups(groups, func(keys model.YggdrasilPublicKeys) []model.YggdrasilPublicKey {
			return keys.ProfilePropertyKeys
		}),
	}
}

func PublicKeyPEM(encoded string) (string, error) {
	der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", err
	}
	if _, err := parseRSAPublicKey(der); err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}

func publicKeySource(endpoint map[string]any) PublicKeySource {
	sessionURL, _ := endpoint["session_url"].(string)
	accountURL, _ := endpoint["account_url"].(string)
	servicesURL, _ := endpoint["services_url"].(string)
	discoveryURL, _ := LongestCommonYggdrasilRoot(sessionURL, accountURL, servicesURL)
	servicesPublicURL := appendURLPath(servicesURL, "publickeys/")
	identity := discoveryURL + "\x00" + servicesPublicURL
	digest := sha256.Sum256([]byte(identity))
	return PublicKeySource{
		ID:                hex.EncodeToString(digest[:]),
		DiscoveryURL:      discoveryURL,
		ServicesPublicURL: servicesPublicURL,
	}
}

func parseEndpointURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, false
	}
	u.Path = path.Clean("/" + strings.TrimPrefix(u.Path, "/"))
	if u.Path == "." {
		u.Path = "/"
	}
	u.RawPath = ""
	return u, true
}

func appendURLPath(raw, suffix string) string {
	u, ok := parseEndpointURL(raw)
	if !ok {
		return ""
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(suffix, "/")
	return u.String()
}

func splitPath(value string) []string {
	trimmed := strings.Trim(value, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func NormalizePEMPublicKeys(values []string) ([]model.YggdrasilPublicKey, error) {
	if len(values) > maxPublicKeysPerGroup {
		return nil, fmt.Errorf("signaturePublickeys contains more than %d keys", maxPublicKeysPerGroup)
	}
	out := make([]model.YggdrasilPublicKey, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		block, rest := pem.Decode([]byte(strings.TrimSpace(value)))
		if block == nil || block.Type != "PUBLIC KEY" || len(strings.TrimSpace(string(rest))) != 0 {
			return nil, errors.New("discovery metadata contains invalid PEM public key")
		}
		if _, err := parseRSAPublicKey(block.Bytes); err != nil {
			return nil, fmt.Errorf("discovery metadata contains invalid RSA public key: %w", err)
		}
		canonical := base64.StdEncoding.EncodeToString(block.Bytes)
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, model.YggdrasilPublicKey{PublicKey: canonical})
	}
	return out, nil
}

func normalizeDERPublicKeys(name string, values []model.YggdrasilPublicKey) ([]model.YggdrasilPublicKey, error) {
	if len(values) > maxPublicKeysPerGroup {
		return nil, fmt.Errorf("%s contains more than %d keys", name, maxPublicKeysPerGroup)
	}
	out := make([]model.YggdrasilPublicKey, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value.PublicKey))
		if err != nil {
			return nil, fmt.Errorf("%s contains invalid base64 public key: %w", name, err)
		}
		if _, err := parseRSAPublicKey(der); err != nil {
			return nil, fmt.Errorf("%s contains invalid RSA public key: %w", name, err)
		}
		canonical := base64.StdEncoding.EncodeToString(der)
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, model.YggdrasilPublicKey{PublicKey: canonical})
	}
	return out, nil
}

func parseRSAPublicKey(der []byte) (*rsa.PublicKey, error) {
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not RSA")
	}
	return key, nil
}

func mergePublicKeyGroups(groups []model.YggdrasilPublicKeys, selectGroup func(model.YggdrasilPublicKeys) []model.YggdrasilPublicKey) []model.YggdrasilPublicKey {
	out := make([]model.YggdrasilPublicKey, 0)
	seen := map[string]struct{}{}
	for _, group := range groups {
		for _, key := range selectGroup(group) {
			if _, exists := seen[key.PublicKey]; exists {
				continue
			}
			seen[key.PublicKey] = struct{}{}
			out = append(out, key)
		}
	}
	return out
}

func decodeSingleJSON(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("response contains multiple JSON values")
		}
		return err
	}
	return nil
}
