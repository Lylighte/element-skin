package yggdrasil

import (
	"context"
	"strings"

	"element-skin/backend/internal/permission"
	fallbacksvc "element-skin/backend/internal/service/fallback"
)

func (y Yggdrasil) Metadata(ctx context.Context, actor permission.Actor) (map[string]any, error) {
	if err := requirePublicPermission(actor); err != nil {
		return nil, err
	}
	signer, err := y.signer()
	if err != nil {
		return nil, err
	}
	name, err := y.settings().Get(ctx, "site_name", "皮肤站")
	if err != nil {
		return nil, err
	}
	site := strings.TrimRight(y.Cfg.SiteURL, "/")
	host := strings.TrimPrefix(strings.TrimPrefix(site, "https://"), "http://")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	domains, err := y.DB.Fallbacks.CollectSkinDomains(ctx)
	if err != nil {
		return nil, err
	}
	domains = appendUniqueDomain(domains, host)
	publicKeys, err := y.PublicKeys(ctx, actor)
	if err != nil {
		return nil, err
	}
	signaturePublicKeys := make([]string, 0, len(publicKeys.ProfilePropertyKeys))
	for _, key := range publicKeys.ProfilePropertyKeys {
		encoded, err := fallbacksvc.PublicKeyPEM(key.PublicKey)
		if err != nil {
			return nil, err
		}
		signaturePublicKeys = append(signaturePublicKeys, encoded)
	}
	return map[string]any{
		"meta": map[string]any{
			"serverName": name, "implementationName": "element-skin", "implementationVersion": "go",
			"links":                   map[string]any{"homepage": site + "/", "register": site + "/register/"},
			"feature.non_email_login": true,
		},
		"skinDomains":         domains,
		"signaturePublickey":  signer.PublicKeyPEM(),
		"signaturePublickeys": signaturePublicKeys,
	}, nil
}

func appendUniqueDomain(domains []string, domain string) []string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return domains
	}
	for _, existing := range domains {
		if existing == domain {
			return domains
		}
	}
	return append(domains, domain)
}
