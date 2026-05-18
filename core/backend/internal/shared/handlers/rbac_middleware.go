// Package handlers — RBAC middleware now lives in platform/http/gin/go (v0.2.0+).
// Local types are kept as aliases so existing call sites in pymes-core do not
// need to import ginmw directly. New code should prefer the canonical import:
// github.com/devpablocristo/platform/http/gin/go.
package handlers

import (
	ginmw "github.com/devpablocristo/platform/http/gin/go"
)

// PermissionChecker re-exports the agnostic interface from platform/http/gin/go.
// Existing pymes implementations (e.g. rbac.UseCases) satisfy this interface
// structurally via the same method signature.
type PermissionChecker = ginmw.PermissionChecker

// RBACMiddleware re-exports the agnostic middleware from platform/http/gin/go.
type RBACMiddleware = ginmw.RBACMiddleware

// NewRBACMiddleware wires the platform middleware.
func NewRBACMiddleware(checker PermissionChecker) *RBACMiddleware {
	return ginmw.NewRBACMiddleware(checker)
}
