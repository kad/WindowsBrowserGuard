package main

import (
	"strings"
)

// ============================================================================
// PATH UTILITIES - Optimized string operations
// ============================================================================

// splitPath splits a Windows registry path by backslash
func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "\\")
}

// getParentPath returns the parent path of a registry key
// Example: "Google\Chrome\Extensions" -> "Google\Chrome"
func getParentPath(path string) (string, bool) {
	lastSlash := strings.LastIndex(path, "\\")
	if lastSlash == -1 {
		return "", false
	}
	return path[:lastSlash], true
}

// getKeyName returns the final component of a path
// Example: "Google\Chrome\Extensions" -> "Extensions"
func getKeyName(path string) string {
	lastSlash := strings.LastIndex(path, "\\")
	if lastSlash == -1 {
		return path
	}
	return path[lastSlash+1:]
}

// containsIgnoreCase performs case-insensitive substring check
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// hasPathComponent checks if path contains a specific component
// More efficient than case-insensitive contains for path matching
func hasPathComponent(path, component string) bool {
	parts := splitPath(path)
	componentLower := strings.ToLower(component)
	for _, part := range parts {
		if strings.ToLower(part) == componentLower {
			return true
		}
	}
	return false
}

// extractExtensionIDFromPath extracts extension ID from various path formats
// Handles: ...\\extensions\\{id}, ...\\ExtensionSettings\\{id}, etc.
func extractExtensionIDFromPath(path, afterComponent string) string {
	parts := splitPath(path)
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], afterComponent) {
			return parts[i+1]
		}
	}
	return ""
}

// buildPath efficiently constructs a registry path from components
func buildPath(components ...string) string {
	var builder strings.Builder
	totalLen := 0
	for _, c := range components {
		totalLen += len(c)
	}
	totalLen += len(components) - 1 // for separators
	builder.Grow(totalLen)
	
	for i, component := range components {
		if i > 0 && component != "" {
			builder.WriteString("\\")
		}
		builder.WriteString(component)
	}
	return builder.String()
}

// replacePathComponent replaces a path component with another
// Example: replacePathComponent(path, "ExtensionInstallForcelist", "ExtensionInstallBlocklist")
func replacePathComponent(path, old, new string) string {
	// For case-insensitive replacement in registry paths
	oldLower := strings.ToLower(old)
	result := strings.Builder{}
	remaining := path
	
	for {
		lowerRemaining := strings.ToLower(remaining)
		idx := strings.Index(lowerRemaining, oldLower)
		if idx == -1 {
			result.WriteString(remaining)
			break
		}
		
		result.WriteString(remaining[:idx])
		result.WriteString(new)
		remaining = remaining[idx+len(old):]
	}
	
	return result.String()
}
