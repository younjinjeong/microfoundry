package service

import (
	"context"
	"encoding/json"
	"log"

	"github.com/younjinjeong/microfoundry/pkg/secrets"
)

// VCAPServiceEntry represents a single service entry in VCAP_SERVICES.
type VCAPServiceEntry struct {
	Name        string            `json:"name"`
	Label       string            `json:"label"`
	Plan        string            `json:"plan"`
	Tags        []string          `json:"tags"`
	Credentials map[string]string `json:"credentials"`
}

// BuildVCAPServices constructs the VCAP_SERVICES JSON for the given bound service names.
// appName is used to enrich gateway credentials with the app-specific proxy URL.
func BuildVCAPServices(ctx context.Context, mgr *Manager, secretsMgr *secrets.Manager, appName string, serviceNames []string) (string, error) {
	vcap := make(map[string][]VCAPServiceEntry)

	for _, svcName := range serviceNames {
		inst, err := mgr.Get(ctx, svcName)
		if err != nil {
			log.Printf("warning: VCAP_SERVICES: skipping %q: %v", svcName, err)
			continue
		}

		svcType, ok := FindServiceType(inst.ServiceType)
		if !ok {
			log.Printf("warning: VCAP_SERVICES: unknown type %q for %q", inst.ServiceType, svcName)
			continue
		}

		// Get credentials from secret
		detail, err := secretsMgr.Get(ctx, svcName)
		if err != nil {
			log.Printf("warning: VCAP_SERVICES: no secret for %q: %v", svcName, err)
			continue
		}

		creds := make(map[string]string)
		for _, key := range detail.Keys {
			val, err := secretsMgr.GetValue(ctx, svcName, key)
			if err != nil {
				continue
			}
			creds[key] = val
		}

		// Enrich gateway credentials with app-specific proxy URL
		tmpl, hasTmpl := GetTemplate(inst.ServiceType)
		if hasTmpl && tmpl.IsGateway && appName != "" {
			if proxyURL, ok := creds["proxy_url"]; ok {
				creds["app_proxy_url"] = proxyURL + "/" + appName
			}
			if url, ok := creds["url"]; ok {
				creds["app_proxy_url"] = url + "/" + appName
			}
		}

		entry := VCAPServiceEntry{
			Name:        svcName,
			Label:       svcType.Label,
			Plan:        inst.Plan,
			Tags:        svcType.Tags,
			Credentials: creds,
		}

		vcap[svcType.Label] = append(vcap[svcType.Label], entry)
	}

	data, err := json.Marshal(vcap)
	if err != nil {
		return "{}", err
	}
	return string(data), nil
}
