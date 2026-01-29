package detection

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/kad/WindowsBrowserGuard/pkg/pathutils"
	"golang.org/x/sys/windows"
)

// ============================================================================
// DETECTION LOGIC - Pure functions for parsing and detection (no registry I/O)
// ============================================================================

// ExtractExtensionIDFromValue extracts the extension ID from a forcelist value
// Format: "extensionid;https://update-url"
func ExtractExtensionIDFromValue(value string) string {
	// Extension ID is the string before the first ';'
	idx := strings.Index(value, ";")
	if idx >= 0 {
		return strings.TrimSpace(value[:idx])
	}
	return strings.TrimSpace(value)
}

// FormatRegValue formats a registry value based on its type
func FormatRegValue(valueType uint32, data []byte) string {
	switch valueType {
	case windows.REG_SZ, windows.REG_EXPAND_SZ:
		if len(data) < 2 {
			return ""
		}
		u16 := make([]uint16, len(data)/2)
		for i := 0; i < len(u16); i++ {
			u16[i] = uint16(data[i*2]) | uint16(data[i*2+1])<<8
		}
		return syscall.UTF16ToString(u16)
	case windows.REG_DWORD:
		if len(data) >= 4 {
			return fmt.Sprintf("0x%08x", uint32(data[0])|uint32(data[1])<<8|uint32(data[2])<<16|uint32(data[3])<<24)
		}
	case windows.REG_QWORD:
		if len(data) >= 8 {
			return fmt.Sprintf("0x%016x", uint64(data[0])|uint64(data[1])<<8|uint64(data[2])<<16|uint64(data[3])<<24|
				uint64(data[4])<<32|uint64(data[5])<<40|uint64(data[6])<<48|uint64(data[7])<<56)
		}
	case windows.REG_BINARY, windows.REG_MULTI_SZ:
		return fmt.Sprintf("%d bytes", len(data))
	}
	return fmt.Sprintf("Unknown type %d", valueType)
}

// IsChromeExtensionForcelist checks if a path is a Chrome forcelist path
func IsChromeExtensionForcelist(path string) bool {
	return pathutils.Contains(path, "ExtensionInstallForcelist")
}

// IsFirefoxExtensionSettings checks if a path is a Firefox extension settings path
func IsFirefoxExtensionSettings(path string) bool {
	return pathutils.Contains(path, "Mozilla\\Firefox\\ExtensionSettings") || 
	       pathutils.Contains(path, "Firefox\\ExtensionSettings")
}

// IsEdgeExtensionForcelist checks if a path is an Edge forcelist path
func IsEdgeExtensionForcelist(path string) bool {
	return pathutils.Contains(path, "Microsoft\\Edge\\ExtensionInstallForcelist") ||
	       pathutils.Contains(path, "Edge\\ExtensionInstallForcelist")
}

// IsChromeExtensionBlocklist checks if a path is a Chrome blocklist path
func IsChromeExtensionBlocklist(path string) bool {
	return pathutils.Contains(path, "ExtensionInstallBlocklist")
}

// IsExtensionSettingsPath checks if a path is an ExtensionSettings path
func IsExtensionSettingsPath(path string) bool {
	return pathutils.Contains(path, "ExtensionSettings")
}

// Is3rdPartyExtensionsPath checks if a path is a 3rdparty extensions path
func Is3rdPartyExtensionsPath(path string) bool {
	return pathutils.Contains(path, "3rdparty\\extensions")
}

// GetBlocklistKeyPath converts a forcelist path to a blocklist path
func GetBlocklistKeyPath(forcelistPath string) string {
	// Replace "ExtensionInstallForcelist" with "ExtensionInstallBlocklist"
	return pathutils.ReplacePathComponent(forcelistPath, "ExtensionInstallForcelist", "ExtensionInstallBlocklist")
}

// GetAllowlistKeyPath converts a forcelist path to an allowlist path
func GetAllowlistKeyPath(forcelistPath string) string {
	// Replace "ExtensionInstallForcelist" with "ExtensionInstallAllowlist"
	return pathutils.ReplacePathComponent(forcelistPath, "ExtensionInstallForcelist", "ExtensionInstallAllowlist")
}

// ExtractFirefoxExtensionID extracts the extension ID from a Firefox extension path
// Path format: Mozilla\Firefox\ExtensionSettings\{extension-id}\installation_mode
func ExtractFirefoxExtensionID(valuePath string) string {
	parts := pathutils.SplitPath(valuePath)
	
	// Find ExtensionSettings and get the next part (extension ID)
	for i := 0; i < len(parts); i++ {
		if parts[i] == "ExtensionSettings" && i+1 < len(parts) {
			extID := parts[i+1]
			// Extension IDs for Firefox are typically {guid} format or name@domain
			if len(extID) > 0 && (extID[0] == '{' || pathutils.ContainsIgnoreCase(extID, "@")) {
				return extID
			}
		}
	}
	return ""
}

// GetFirefoxBlocklistPath returns the Firefox blocklist path for an extension ID
func GetFirefoxBlocklistPath(extensionID string) string {
	// Firefox blocklist path: Mozilla\Firefox\ExtensionSettings\{extension-id}\installation_mode
	return "Mozilla\\Firefox\\ExtensionSettings\\" + extensionID
}

// ExtractExtensionIDFromPath extracts extension ID from various path formats
// Supports:
//   - ExtensionSettings\{id}\policy
//   - 3rdparty\extensions\{id}\policy
//   - ExtensionInstallForcelist (ID in value data)
func ExtractExtensionIDFromPath(path string) string {
	// Try Firefox format first
	if IsFirefoxExtensionSettings(path) {
		return ExtractFirefoxExtensionID(path)
	}
	
	// Try Chrome ExtensionSettings format
	// Path: Google\Chrome\ExtensionSettings\{extension-id}
	if IsExtensionSettingsPath(path) {
		parts := pathutils.SplitPath(path)
		for i := 0; i < len(parts); i++ {
			if parts[i] == "ExtensionSettings" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	
	// Try 3rdparty extensions format
	// Path: Google\Chrome\3rdparty\extensions\{extension-id}
	if Is3rdPartyExtensionsPath(path) {
		parts := pathutils.SplitPath(path)
		for i := 0; i < len(parts); i++ {
			if parts[i] == "extensions" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	
	return ""
}

// IsExtensionPolicy checks if a path or value represents an extension policy
func IsExtensionPolicy(path string) bool {
	return IsChromeExtensionForcelist(path) ||
	       IsFirefoxExtensionSettings(path) ||
	       IsEdgeExtensionForcelist(path) ||
	       IsExtensionSettingsPath(path) ||
	       Is3rdPartyExtensionsPath(path)
}

// ShouldBlockPath determines if a registry path should be blocked
func ShouldBlockPath(path string) bool {
	// Block ExtensionInstallForcelist paths for Chrome and Edge
	if pathutils.Contains(path, "ExtensionInstallForcelist") {
		return true
	}
	
	// Block Firefox forced extension installs
	if IsFirefoxExtensionSettings(path) && pathutils.Contains(path, "installation_mode") {
		return true
	}
	
	return false
}

// ParseForcelistValues parses all values from a forcelist and returns extension IDs
func ParseForcelistValues(values map[string]string) []string {
	var extensionIDs []string
	
	for _, valueData := range values {
		extID := ExtractExtensionIDFromValue(valueData)
		if extID != "" {
			extensionIDs = append(extensionIDs, extID)
		}
	}
	
	return extensionIDs
}

// GetBrowserFromPath determines which browser a path belongs to
func GetBrowserFromPath(path string) string {
	pathLower := strings.ToLower(path)
	
	if strings.Contains(pathLower, "google\\chrome") || strings.Contains(pathLower, "chrome\\") {
		return "Chrome"
	}
	if strings.Contains(pathLower, "microsoft\\edge") || strings.Contains(pathLower, "edge\\") {
		return "Edge"
	}
	if strings.Contains(pathLower, "mozilla\\firefox") || strings.Contains(pathLower, "firefox\\") {
		return "Firefox"
	}
	
	return "Unknown"
}

// ValidateExtensionID checks if an extension ID has a valid format
func ValidateExtensionID(extID string) bool {
	if extID == "" {
		return false
	}
	
	// Chrome/Edge extension IDs are typically 32 lowercase letters (a-p)
	// Example: afdpoidmelmfapkoikmenejmcdpgecfe
	if len(extID) == 32 {
		for _, c := range extID {
			if c < 'a' || c > 'p' {
				return false
			}
		}
		return true
	}
	
	// Firefox extension IDs can be:
	// - {guid} format: {12345678-1234-1234-1234-123456789012}
	// - name@domain format: extension@developer.org
	if len(extID) > 0 {
		if extID[0] == '{' && extID[len(extID)-1] == '}' {
			return len(extID) >= 10 // Rough check for GUID format
		}
		if strings.Contains(extID, "@") {
			return len(extID) >= 5 // Rough check for name@domain format
		}
	}
	
	return false
}


