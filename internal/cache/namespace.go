package cache

import "encoding/json"

// Namespace identifies a cache partition for a model and tenant context.
type Namespace struct {
	Model            string
	TenantID         string
	SystemPromptHash string
}

// NamespaceKey builds a deterministic key for a namespace.
func NamespaceKey(ns Namespace) string {
	key, _ := json.Marshal([3]string{ns.Model, ns.TenantID, ns.SystemPromptHash})
	return string(key)
}
