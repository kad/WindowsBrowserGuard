package pathutils

import (
	"strings"
)

// ============================================================================
// PATH UTILITIES - Optimized string operations
// ============================================================================

// SplitPath splits a Windows registry path by backslash
func SplitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "\\")
}

// GetParentPath returns the parent path of a registry key
// Example: "Google\Chrome\Extensions" -> "Google\Chrome"
func GetParentPath(path string) (string, bool) {
	lastSlash := strings.LastIndex(path, "\\")
	if lastSlash == -1 {
		return "", false
	}
	return path[:lastSlash], true
}

// GetKeyName returns the final component of a path
// Example: "Google\Chrome\Extensions" -> "Extensions"
func GetKeyName(path string) string {
	lastSlash := strings.LastIndex(path, "\\")
	if lastSlash == -1 {
		return path
	}
	return path[lastSlash+1:]
}

// ContainsIgnoreCase performs case-insensitive substring check
func ContainsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// Contains is a case-insensitive contains check (wrapper for ContainsIgnoreCase)
func Contains(s, substr string) bool {
	return ContainsIgnoreCase(s, substr)
}

// HasPathComponent checks if path contains a specific component
// More efficient than case-insensitive contains for path matching
func HasPathComponent(path, component string) bool {
	parts := SplitPath(path)
	componentLower := strings.ToLower(component)
	for _, part := range parts {
		if strings.ToLower(part) == componentLower {
			return true
		}
	}
	return false
}

// ExtractExtensionIDFromPath extracts extension ID from various path formats
// Handles: ...\\extensions\\{id}, ...\\ExtensionSettings\\{id}, etc.
func ExtractExtensionIDFromPath(path, afterComponent string) string {
	parts := SplitPath(path)
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], afterComponent) {
			return parts[i+1]
		}
	}
	return ""
}

// BuildPath efficiently constructs a registry path from components
func BuildPath(components ...string) string {
	// Filter out empty components
	nonEmpty := make([]string, 0, len(components))
	for _, c := range components {
		if c != "" {
			nonEmpty = append(nonEmpty, c)
		}
	}

	if len(nonEmpty) == 0 {
		return ""
	}

	var builder strings.Builder
	totalLen := 0
	for _, c := range nonEmpty {
		totalLen += len(c)
	}
	totalLen += len(nonEmpty) - 1 // for separators
	builder.Grow(totalLen)

	for i, component := range nonEmpty {
		if i > 0 {
			builder.WriteString("\\")
		}
		builder.WriteString(component)
	}
	return builder.String()
}

// ReplacePathComponent replaces a path component with another
// Example: ReplacePathComponent(path, "ExtensionInstallForcelist", "ExtensionInstallBlocklist")
func ReplacePathComponent(path, old, new string) string {
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
