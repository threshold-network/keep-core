package clientinfo

import (
	"encoding/json"
)

// RegisterDiagnosticSource registers diagnostics source callback with a given name.
// Name will be used as a key and callback result as a value in JSON object
// during composing diagnostics JSON.
// Note: function will override existing diagnostics source on attempt
// to register another one with the same name.
func (r *Registry) RegisterDiagnosticSource(name string, source func() string) {
	r.diagnosticsMutex.Lock()
	defer r.diagnosticsMutex.Unlock()

	r.diagnosticsSources[name] = source
}

// Exposes all registered diagnostics sources in a single JSON document.
func (r *Registry) exposeDiagnostics() string {
	r.diagnosticsMutex.RLock()
	defer r.diagnosticsMutex.RUnlock()

	diagnostics := make(map[string]interface{})
	for _, sourceName := range sortedKeys(r.diagnosticsSources) {
		var jsonString = r.diagnosticsSources[sourceName]()
		var jsonObject interface{}
		err := json.Unmarshal([]byte(jsonString), &jsonObject)
		if err == nil {
			diagnostics[sourceName] = jsonObject
		}
	}

	bytes, err := json.MarshalIndent(diagnostics, "", "  ")
	if err != nil {
		logger.Errorf("diagnostics JSON serialization error: [%v]", err)
		return ""
	}

	return string(bytes)
}
