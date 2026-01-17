package gdpr

import "maps"

func BuildDSARPayload(employee, datasets map[string]any) map[string]any {
	payload := map[string]any{
		"employee": employee,
	}
	maps.Copy(payload, datasets)
	return payload
}
