package policy

import (
	"path/filepath"
	"runtime"
	"strings"
)

func assessPath(req PathRequest) assessment {
	details := map[string]any{
		"path":      req.Path,
		"operation": string(req.Operation),
	}
	if req.Path == "" {
		return assessment{Risk: RiskLevelCritical, Reason: "path is required", Details: details}
	}

	absPath, err := filepath.Abs(req.Path)
	if err != nil {
		details["error"] = err.Error()
		return assessment{Risk: RiskLevelCritical, Reason: "path could not be resolved", Details: details}
	}
	absPath = filepath.Clean(absPath)
	details["resolved_path"] = absPath

	if isSecretPath(absPath) && req.Operation == PathOperationRead {
		return assessment{Risk: RiskLevelCritical, Reason: "secret files are blocked by default", Details: details}
	}

	inWorkspace := false
	if req.WorkspaceRoot != "" {
		root, err := filepath.Abs(req.WorkspaceRoot)
		if err != nil {
			details["workspace_error"] = err.Error()
			return assessment{Risk: RiskLevelCritical, Reason: "workspace root could not be resolved", Details: details}
		}
		root = filepath.Clean(root)
		details["workspace_root"] = root
		inWorkspace = isSubpath(root, absPath)
		details["in_workspace"] = inWorkspace
	}

	switch req.Operation {
	case PathOperationRead:
		if req.WorkspaceRoot != "" && !inWorkspace {
			return assessment{Risk: RiskLevelMedium, Reason: "reading outside the workspace requires review", Details: details}
		}
		return assessment{Risk: RiskLevelLow, Reason: "path read is allowed", Details: details}
	case PathOperationWrite:
		if req.WorkspaceRoot == "" {
			return assessment{Risk: RiskLevelHigh, Reason: "workspace root is required for file writes", Details: details}
		}
		if !inWorkspace {
			return assessment{Risk: RiskLevelCritical, Reason: "writing outside the workspace is blocked by default", Details: details}
		}
		return assessment{Risk: RiskLevelMedium, Reason: "workspace file write", Details: details}
	case PathOperationDelete:
		if req.WorkspaceRoot == "" {
			return assessment{Risk: RiskLevelHigh, Reason: "workspace root is required for file deletes", Details: details}
		}
		if !inWorkspace {
			return assessment{Risk: RiskLevelCritical, Reason: "deleting outside the workspace is blocked by default", Details: details}
		}
		return assessment{Risk: RiskLevelHigh, Reason: "workspace file delete requires review", Details: details}
	default:
		return assessment{Risk: RiskLevelMedium, Reason: "unknown path operation requires review", Details: details}
	}
}

func isSubpath(root, path string) bool {
	if samePath(root, path) {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func isSecretPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pem" || ext == ".key" || ext == ".p12" || ext == ".pfx" {
		return true
	}

	secretNames := map[string]struct{}{
		".env":                 {},
		".env.local":           {},
		".env.production":      {},
		".npmrc":               {},
		".pypirc":              {},
		".netrc":               {},
		"id_rsa":               {},
		"id_dsa":               {},
		"id_ecdsa":             {},
		"id_ed25519":           {},
		"credentials":          {},
		"credentials.json":     {},
		"service-account.json": {},
	}
	if _, ok := secretNames[base]; ok {
		return true
	}

	normalized := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(normalized, "/.ssh/") ||
		strings.Contains(normalized, "/.aws/credentials") ||
		strings.Contains(normalized, "/.config/gcloud/")
}
