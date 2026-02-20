package monitor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sys/windows"

	"github.com/kad/WindowsBrowserGuard/pkg/admin"
	"github.com/kad/WindowsBrowserGuard/pkg/detection"
	"github.com/kad/WindowsBrowserGuard/pkg/pathutils"
	"github.com/kad/WindowsBrowserGuard/pkg/registry"
	"github.com/kad/WindowsBrowserGuard/pkg/telemetry"
)

// CaptureRegistryState captures the current state of a registry key and all its subkeys
func CaptureRegistryState(ctx context.Context, hKey windows.Handle, keyPath string) (*registry.RegState, error) {
	startTime := time.Now()
	ctx, span := telemetry.StartSpan(ctx, "monitor.CaptureRegistryState",
		attribute.String("key-path", keyPath),
	)
	defer span.End()

	state := &registry.RegState{
		Subkeys: make(map[string]bool),
		Values:  make(map[string]registry.RegValue),
	}

	err := registry.CaptureKeyRecursive(hKey, "", state, 0)
	duration := time.Since(startTime)

	if err != nil {
		telemetry.RecordError(ctx, err)
		telemetry.RecordOperationDuration(ctx, "capture_registry_state", duration)
		telemetry.RecordRegistryOperation(ctx, "capture", false)
		return nil, err
	}

	telemetry.SetAttributes(ctx,
		attribute.Int("subkeys-count", len(state.Subkeys)),
		attribute.Int("values-count", len(state.Values)),
	)

	// Record metrics
	telemetry.RecordRegistryStateSize(ctx, len(state.Subkeys), len(state.Values))
	telemetry.RecordOperationDuration(ctx, "capture_registry_state", duration)
	telemetry.RecordRegistryOperation(ctx, "capture", true)

	return state, nil
}

// PrintDiff compares two registry states and prints the differences
func PrintDiff(ctx context.Context, oldState, newState *registry.RegState, keyPath string, canWrite bool, extensionIndex *registry.ExtensionPathIndex) {
	ctx, span := telemetry.StartSpan(ctx, "monitor.PrintDiff",
		attribute.String("key-path", keyPath),
		attribute.Bool("can-write", canWrite),
	)
	defer span.End()
	telemetry.Println(ctx, "\n========== CHANGES DETECTED ==========")
	telemetry.Println(ctx, "Time:", time.Now().Format(time.RFC3339))
	telemetry.Println(ctx, "Key:", keyPath)
	telemetry.Println(ctx, "======================================")

	hasChanges := false

	for name := range newState.Subkeys {
		if !oldState.Subkeys[name] {
			telemetry.Printf(ctx, "[SUBKEY ADDED] %s\n", name)
			hasChanges = true
		}
	}
	for name := range oldState.Subkeys {
		if !newState.Subkeys[name] {
			telemetry.Printf(ctx, "[SUBKEY REMOVED] %s\n", name)
			hasChanges = true
		}
	}

	for name, newVal := range newState.Values {
		oldVal, exists := oldState.Values[name]
		if !exists {
			telemetry.Printf(ctx, "[VALUE ADDED] %s = %s (type: %d)\n", name, newVal.Data, newVal.Type)
			hasChanges = true

			if detection.IsChromeExtensionForcelist(name) {
				telemetry.Printf(ctx, "  ⚠️  DETECTED Chrome ExtensionInstallForcelist VALUE - PROCESSING...\n")

				if !admin.IsAdmin() {
					telemetry.Printf(ctx, "  ❌ Insufficient privileges. Run as Administrator.\n")
				} else {
					lastSlash := -1
					for i := len(name) - 1; i >= 0; i-- {
						if name[i] == '\\' {
							lastSlash = i
							break
						}
					}

					if lastSlash >= 0 {
						forcelistKeyPath := name[:lastSlash]

						allValues, err := registry.ReadKeyValues(keyPath, forcelistKeyPath)
						if err != nil {
							telemetry.Printf(ctx, "  ⚠️  Could not read forcelist values: %v\n", err)
						} else {
							telemetry.Printf(ctx, "  📋 Processing all extension IDs in forcelist...\n")

							blocklistKeyPath := detection.GetBlocklistKeyPath(forcelistKeyPath)
							allowlistKeyPath := detection.GetAllowlistKeyPath(forcelistKeyPath)

							for _, valueData := range allValues {
								extensionID := detection.ExtractExtensionIDFromValue(valueData)
								if extensionID != "" {
									telemetry.Printf(ctx, "  🔍 Extension ID: %s\n", extensionID)

									telemetry.Printf(ctx, "  📝 Adding to blocklist: %s\n", blocklistKeyPath)
									err := registry.AddToBlocklist(keyPath, blocklistKeyPath, extensionID, !canWrite)
									if err != nil {
										telemetry.Printf(ctx, "  ⚠️  Failed to add to blocklist: %v\n", err)
									}

									telemetry.Printf(ctx, "  🔍 Checking allowlist: %s\n", allowlistKeyPath)
									err = registry.RemoveFromAllowlist(keyPath, allowlistKeyPath, extensionID, !canWrite)
									if err != nil {
										telemetry.Printf(ctx, "  ⚠️  Failed to remove from allowlist: %v\n", err)
									}

									registry.RemoveExtensionSettingsForID(keyPath, extensionID, !canWrite, newState, extensionIndex)
								}
							}

							telemetry.Printf(ctx, "  🗑️  Deleting forcelist key: %s\n", forcelistKeyPath)
							err = registry.DeleteRegistryKeyRecursive(keyPath, forcelistKeyPath, !canWrite)
							if err != nil {
								telemetry.Printf(ctx, "  ❌ Failed to delete key: %v\n", err)
							} else {
								telemetry.Printf(ctx, "  ✓ Successfully deleted forcelist key\n")
								delete(newState.Subkeys, forcelistKeyPath)
								for valName := range newState.Values {
									if len(valName) > len(forcelistKeyPath) &&
										valName[:len(forcelistKeyPath)] == forcelistKeyPath {
										delete(newState.Values, valName)
									}
								}
							}
						}
					}
				}
			}

			if detection.IsFirefoxExtensionSettings(name) && pathutils.Contains(name, "installation_mode") {
				if newVal.Data == "force_installed" || newVal.Data == "normal_installed" {
					telemetry.Printf(ctx, "  ⚠️  DETECTED Firefox extension install policy - PROCESSING...\n")

					if !admin.IsAdmin() {
						telemetry.Printf(ctx, "  ❌ Insufficient privileges. Run as Administrator.\n")
					} else {
						extensionID := detection.ExtractFirefoxExtensionID(name)
						if extensionID != "" {
							telemetry.Printf(ctx, "  🔍 Extension ID: %s\n", extensionID)

							telemetry.Printf(ctx, "  📝 Blocking Firefox extension\n")
							err := registry.BlockFirefoxExtension(keyPath, extensionID, !canWrite)
							if err != nil {
								telemetry.Printf(ctx, "  ⚠️  Failed to block extension: %v\n", err)
							}

							lastSlash := -1
							for i := len(name) - 1; i >= 0; i-- {
								if name[i] == '\\' {
									lastSlash = i
									break
								}
							}

							if lastSlash >= 0 {
								extensionKeyPath := name[:lastSlash]
								telemetry.Printf(ctx, "  🗑️  Deleting install policy: %s\n", extensionKeyPath)
								err = registry.DeleteRegistryKeyRecursive(keyPath, extensionKeyPath, !canWrite)
								if err != nil {
									telemetry.Printf(ctx, "  ❌ Failed to delete key: %v\n", err)
								} else {
									telemetry.Printf(ctx, "  ✓ Successfully deleted install policy\n")
									delete(newState.Subkeys, extensionKeyPath)
									registry.RemoveSubtreeFromState(newState, extensionKeyPath)
								}
							}
						}
					}
				}
			}
		} else if oldVal.Data != newVal.Data || oldVal.Type != newVal.Type {
			telemetry.Printf(ctx, "[VALUE CHANGED] %s\n", name)
			telemetry.Printf(ctx, "  Old: %s (type: %d)\n", oldVal.Data, oldVal.Type)
			telemetry.Printf(ctx, "  New: %s (type: %d)\n", newVal.Data, newVal.Type)
			hasChanges = true
		}
	}

	for name := range oldState.Values {
		if _, exists := newState.Values[name]; !exists {
			telemetry.Printf(ctx, "[VALUE REMOVED] %s\n", name)
			hasChanges = true
		}
	}

	if !hasChanges {
		telemetry.Println(ctx, "(No actual changes detected - likely a metadata update)")
	}

	telemetry.Println(ctx, "======================================")
	fmt.Println()
}

// ProcessExistingPolicies scans for and processes existing extension install policies
// ProcessExistingPolicies scans for and processes existing extension install policies
func ProcessExistingPolicies(ctx context.Context, keyPath string, state *registry.RegState, canWrite bool, extensionIndex *registry.ExtensionPathIndex) {
	ctx, span := telemetry.StartSpan(ctx, "monitor.ProcessExistingPolicies",
		attribute.String("key-path", keyPath),
		attribute.Bool("can-write", canWrite),
	)
	defer span.End()

	if !canWrite && !admin.IsAdmin() {
		telemetry.Println(ctx, "\n========================================")
		telemetry.Println(ctx, "Checking for existing extension policies...")
		telemetry.Println(ctx, "(DRY-RUN MODE - showing planned operations)")
		telemetry.Println(ctx, "========================================")
	} else if !admin.IsAdmin() && canWrite {
		telemetry.Println(ctx, "\n⚠️  Not running as Administrator - skipping existing policy processing")
		return
	} else {
		telemetry.Println(ctx, "\n========================================")
		telemetry.Println(ctx, "Checking for existing extension policies...")
		telemetry.Println(ctx, "========================================")
	}

	hasExistingPolicies := false

	for valuePath, value := range state.Values {
		if detection.IsChromeExtensionForcelist(valuePath) {
			hasExistingPolicies = true
			telemetry.Printf(ctx, "\n[EXISTING CHROME POLICY DETECTED]\n")
			telemetry.Printf(ctx, "Path: %s\n", valuePath)
			telemetry.Printf(ctx, "Value: %s\n", value.Data)

			extensionID := detection.ExtractExtensionIDFromValue(value.Data)
			if extensionID != "" {
				telemetry.Printf(ctx, "🔍 Extension ID: %s\n", extensionID)

				// Determine browser from path
				browser := "chrome"
				if strings.Contains(strings.ToLower(valuePath), "edge") {
					browser = "edge"
				}

				// Record metrics
				telemetry.RecordExtensionDetected(ctx, browser, extensionID)

				lastSlash := -1
				for i := len(valuePath) - 1; i >= 0; i-- {
					if valuePath[i] == '\\' {
						lastSlash = i
						break
					}
				}

				if lastSlash >= 0 {
					forcelistKeyPath := valuePath[:lastSlash]

					allValues, err := registry.ReadKeyValues(keyPath, forcelistKeyPath)
					if err != nil {
						telemetry.Printf(ctx, "⚠️  Could not read forcelist values: %v\n", err)
					} else {
						telemetry.Printf(ctx, "📋 Processing all extension IDs in forcelist...\n")

						blocklistKeyPath := detection.GetBlocklistKeyPath(forcelistKeyPath)
						allowlistKeyPath := detection.GetAllowlistKeyPath(forcelistKeyPath)

						for _, valueData := range allValues {
							extensionID := detection.ExtractExtensionIDFromValue(valueData)
							if extensionID != "" {
								telemetry.Printf(ctx, "🔍 Extension ID: %s\n", extensionID)

								// Determine browser and record metrics
								browser := "chrome"
								if strings.Contains(strings.ToLower(forcelistKeyPath), "edge") {
									browser = "edge"
								}
								telemetry.RecordExtensionBlocked(ctx, browser, extensionID)

								telemetry.Printf(ctx, "📝 Adding to Chrome blocklist: %s\n", blocklistKeyPath)
								err := registry.AddToBlocklist(keyPath, blocklistKeyPath, extensionID, !canWrite)
								if err != nil {
									telemetry.Printf(ctx, "⚠️  Failed to add to blocklist: %v\n", err)
								}

								telemetry.Printf(ctx, "🔍 Checking Chrome allowlist: %s\n", allowlistKeyPath)
								err = registry.RemoveFromAllowlist(keyPath, allowlistKeyPath, extensionID, !canWrite)
								if err != nil {
									telemetry.Printf(ctx, "⚠️  Failed to remove from allowlist: %v\n", err)
								}

								registry.RemoveExtensionSettingsForID(keyPath, extensionID, !canWrite, state, extensionIndex)
							}
						}
					}

					telemetry.Printf(ctx, "🗑️  Deleting Chrome forcelist key: %s\n", forcelistKeyPath)
					err = registry.DeleteRegistryKeyRecursive(keyPath, forcelistKeyPath, !canWrite)
					if err != nil {
						telemetry.Printf(ctx, "❌ Failed to delete key: %v\n", err)
					} else {
						telemetry.Printf(ctx, "✓ Successfully removed forcelist key\n")
						delete(state.Subkeys, forcelistKeyPath)
						for valName := range state.Values {
							if len(valName) > len(forcelistKeyPath) &&
								valName[:len(forcelistKeyPath)] == forcelistKeyPath {
								delete(state.Values, valName)
							}
						}
					}
				}
			}
		}

		if detection.IsFirefoxExtensionSettings(valuePath) && pathutils.Contains(valuePath, "installation_mode") {
			if value.Data == "force_installed" || value.Data == "normal_installed" {
				hasExistingPolicies = true
				telemetry.Printf(ctx, "\n[EXISTING FIREFOX POLICY DETECTED]\n")
				telemetry.Printf(ctx, "Path: %s\n", valuePath)
				telemetry.Printf(ctx, "Value: %s\n", value.Data)

				extensionID := detection.ExtractFirefoxExtensionID(valuePath)
				if extensionID != "" {
					telemetry.Printf(ctx, "🔍 Extension ID: %s\n", extensionID)

					telemetry.Printf(ctx, "📝 Blocking Firefox extension\n")
					err := registry.BlockFirefoxExtension(keyPath, extensionID, !canWrite)
					if err != nil {
						telemetry.Printf(ctx, "⚠️  Failed to block extension: %v\n", err)
					}

					lastSlash := -1
					for i := len(valuePath) - 1; i >= 0; i-- {
						if valuePath[i] == '\\' {
							lastSlash = i
							break
						}
					}

					if lastSlash >= 0 {
						extensionKeyPath := valuePath[:lastSlash]
						telemetry.Printf(ctx, "🗑️  Deleting Firefox install policy: %s\n", extensionKeyPath)
						err = registry.DeleteRegistryKeyRecursive(keyPath, extensionKeyPath, !canWrite)
						if err != nil {
							telemetry.Printf(ctx, "❌ Failed to delete key: %v\n", err)
						} else {
							telemetry.Printf(ctx, "✓ Successfully removed install policy\n")
							delete(state.Subkeys, extensionKeyPath)
							registry.RemoveSubtreeFromState(state, extensionKeyPath)
						}
					}
				}
			}
		}
	}

	if !hasExistingPolicies {
		telemetry.Println(ctx, "✓ No existing extension install policies found")
	}

	telemetry.Println(ctx, "========================================")
	fmt.Println()
}

// CleanupAllowlists removes ExtensionInstallAllowlist keys
// CleanupAllowlists removes ExtensionInstallAllowlist keys
func CleanupAllowlists(ctx context.Context, keyPath string, state *registry.RegState, canWrite bool) {
	ctx, span := telemetry.StartSpan(ctx, "monitor.CleanupAllowlists",
		attribute.String("key-path", keyPath),
		attribute.Bool("can-write", canWrite),
	)
	defer span.End()

	if !admin.IsAdmin() {
		return
	}

	telemetry.Println(ctx, "Checking for ExtensionInstallAllowlist keys...")

	allowlistsFound := false
	allowlistKeys := make(map[string]bool)

	for subkeyPath := range state.Subkeys {
		if pathutils.Contains(subkeyPath, "ExtensionInstallAllowlist") {
			allowlistsFound = true
			allowlistKeys[subkeyPath] = true
		}
	}

	if !allowlistsFound {
		telemetry.Println(ctx, "✓ No ExtensionInstallAllowlist keys found")
		return
	}

	for allowlistPath := range allowlistKeys {
		telemetry.Printf(ctx, "\n[REMOVING ALLOWLIST]\n")
		telemetry.Printf(ctx, "Path: %s\n", allowlistPath)

		values, err := registry.ReadKeyValues(keyPath, allowlistPath)
		if err == nil && len(values) > 0 {
			telemetry.Printf(ctx, "Found %d extension(s) in allowlist:\n", len(values))
			for valueName, valueData := range values {
				extensionID := detection.ExtractExtensionIDFromValue(valueData)
				if extensionID != "" {
					telemetry.Printf(ctx, "  - %s: %s\n", valueName, extensionID)
				}
			}
		}

		telemetry.Printf(ctx, "🗑️  Deleting allowlist key: %s\n", allowlistPath)
		err = registry.DeleteRegistryKeyRecursive(keyPath, allowlistPath, !canWrite)
		if err != nil {
			telemetry.Printf(ctx, "❌ Failed to delete allowlist: %v\n", err)
		} else {
			telemetry.Printf(ctx, "✓ Successfully deleted allowlist\n")
			delete(state.Subkeys, allowlistPath)
			for valName := range state.Values {
				if len(valName) > len(allowlistPath) &&
					valName[:len(allowlistPath)] == allowlistPath {
					delete(state.Values, valName)
				}
			}
		}
	}

	fmt.Println()
}

// GetBlockedExtensionIDs scans the registry state for all blocked extension IDs
func GetBlockedExtensionIDs(ctx context.Context, keyPath string, state *registry.RegState) map[string]bool {
	blockedIDs := make(map[string]bool)

	telemetry.Println(ctx, "  📋 Scanning for blocked extension IDs...")

	for subkeyPath := range state.Subkeys {
		if pathutils.Contains(subkeyPath, "ExtensionInstallBlocklist") {
			telemetry.Printf(ctx, "  🔍 Found blocklist: %s\n", subkeyPath)
			values, err := registry.ReadKeyValues(keyPath, subkeyPath)
			if err == nil {
				for valueName, valueData := range values {
					extensionID := detection.ExtractExtensionIDFromValue(valueData)
					if extensionID != "" {
						telemetry.Printf(ctx, "    ├─ %s: %s\n", valueName, extensionID)
						blockedIDs[extensionID] = true
					}
				}
			} else {
				telemetry.Printf(ctx, "    ⚠️  Could not read values: %v\n", err)
			}
		}
	}

	for valuePath, value := range state.Values {
		if detection.IsFirefoxExtensionSettings(valuePath) &&
			pathutils.Contains(valuePath, "installation_mode") &&
			value.Data == "blocked" {
			extensionID := detection.ExtractFirefoxExtensionID(valuePath)
			if extensionID != "" {
				telemetry.Printf(ctx, "  🦊 Firefox blocked: %s\n", extensionID)
				blockedIDs[extensionID] = true
			}
		}
	}

	return blockedIDs
}

// CleanupExtensionSettings removes extension settings for all blocked extensions
func CleanupExtensionSettings(ctx context.Context, keyPath string, state *registry.RegState, canWrite bool, extensionIndex *registry.ExtensionPathIndex) {
	ctx, span := telemetry.StartSpan(ctx, "monitor.CleanupExtensionSettings",
		attribute.String("key-path", keyPath),
		attribute.Bool("can-write", canWrite),
	)
	defer span.End()

	if !admin.IsAdmin() {
		return
	}

	telemetry.Println(ctx, "\n========================================")
	telemetry.Println(ctx, "Cleaning up extension settings...")
	telemetry.Println(ctx, "========================================")
	telemetry.Println(ctx, "Checking for extension settings of blocked extensions...")
	telemetry.Println(ctx, "Note: This removes settings for ALL extensions in blocklists,")
	telemetry.Println(ctx, "      regardless of whether they were added via forcelist or manually.")

	blockedIDs := GetBlockedExtensionIDs(ctx, keyPath, state)

	if len(blockedIDs) == 0 {
		telemetry.Println(ctx, "✓ No blocked extensions found")
		telemetry.Println(ctx, "========================================")
		fmt.Println()
		return
	}

	telemetry.Printf(ctx, "\nFound %d blocked extension ID(s):\n", len(blockedIDs))
	for id := range blockedIDs {
		telemetry.Printf(ctx, "  - %s\n", id)
	}

	for extensionID := range blockedIDs {
		telemetry.Printf(ctx, "\n[CHECKING SETTINGS FOR BLOCKED EXTENSION]\n")
		telemetry.Printf(ctx, "Extension ID: %s\n", extensionID)
		registry.RemoveExtensionSettingsForID(keyPath, extensionID, !canWrite, state, extensionIndex)
	}

	telemetry.Println(ctx, "========================================")
	fmt.Println()
}

// WatchRegistryChanges monitors registry changes and processes them
func WatchRegistryChanges(ctx context.Context, hKey windows.Handle, keyPath string, previousState *registry.RegState, canWrite bool, extensionIndex *registry.ExtensionPathIndex) {
	ctx, span := telemetry.StartSpan(ctx, "monitor.WatchRegistryChanges",
		attribute.String("key-path", keyPath),
		attribute.Bool("can-write", canWrite),
	)
	defer span.End()

	event, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		telemetry.Println(ctx, "Error creating event:", err)
		telemetry.RecordError(ctx, err)
		return
	}
	defer func() { _ = windows.CloseHandle(event) }()

	err = windows.RegNotifyChangeKeyValue(hKey, true, windows.REG_NOTIFY_CHANGE_NAME|windows.REG_NOTIFY_CHANGE_LAST_SET, event, true)
	if err != nil {
		telemetry.Println(ctx, "Error setting up registry notification:", err)
		telemetry.RecordError(ctx, err)
		return
	}

	telemetry.Println(ctx, "Monitoring registry changes...")
	telemetry.AddEvent(ctx, "monitoring-started")

	for {
		status, err := windows.WaitForSingleObject(event, windows.INFINITE)
		if err != nil {
			telemetry.Println(ctx, "Error waiting for event:", err)
			telemetry.RecordError(ctx, err)
			return
		}

		if status == windows.WAIT_OBJECT_0 {
			telemetry.AddEvent(ctx, "registry-change-detected")

			newState, err := CaptureRegistryState(ctx, hKey, keyPath)
			if err != nil {
				telemetry.Println(ctx, "Error capturing new state:", err)
			} else {
				PrintDiff(ctx, previousState, newState, keyPath, canWrite, extensionIndex)
				previousState = newState
			}

			err = windows.RegNotifyChangeKeyValue(hKey, true, windows.REG_NOTIFY_CHANGE_NAME|windows.REG_NOTIFY_CHANGE_LAST_SET, event, true)
			if err != nil {
				telemetry.Println(ctx, "Error re-arming registry notification:", err)
				telemetry.RecordError(ctx, err)
				return
			}
		}
	}
}
