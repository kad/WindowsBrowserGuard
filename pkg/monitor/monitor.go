package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/kad/WindowsBrowserGuard/pkg/admin"
	"github.com/kad/WindowsBrowserGuard/pkg/detection"
	"github.com/kad/WindowsBrowserGuard/pkg/pathutils"
	"github.com/kad/WindowsBrowserGuard/pkg/registry"
	"github.com/kad/WindowsBrowserGuard/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sys/windows"
)

// CaptureRegistryState captures the current state of a registry key and all its subkeys
func CaptureRegistryState(ctx context.Context, hKey windows.Handle, keyPath string) (*registry.RegState, error) {
	ctx, span := telemetry.StartSpan(ctx, "monitor.CaptureRegistryState",
		attribute.String("key-path", keyPath),
	)
	defer span.End()

	state := &registry.RegState{
		Subkeys: make(map[string]bool),
		Values:  make(map[string]registry.RegValue),
	}

	err := registry.CaptureKeyRecursive(hKey, "", state, 0)
	if err != nil {
		telemetry.RecordError(ctx, err)
		return nil, err
	}

	telemetry.SetAttributes(ctx,
		attribute.Int("subkeys-count", len(state.Subkeys)),
		attribute.Int("values-count", len(state.Values)),
	)

	return state, nil
}

// PrintDiff compares two registry states and prints the differences
func PrintDiff(ctx context.Context, oldState, newState *registry.RegState, keyPath string, canWrite bool, extensionIndex *registry.ExtensionPathIndex) {
	ctx, span := telemetry.StartSpan(ctx, "monitor.PrintDiff",
		attribute.String("key-path", keyPath),
		attribute.Bool("can-write", canWrite),
	)
	defer span.End()
	fmt.Println("\n========== CHANGES DETECTED ==========")
	fmt.Println("Time:", time.Now().Format(time.RFC3339))
	fmt.Println("Key:", keyPath)
	fmt.Println("======================================")

	hasChanges := false

	for name := range newState.Subkeys {
		if !oldState.Subkeys[name] {
			fmt.Printf("[SUBKEY ADDED] %s\n", name)
			hasChanges = true
		}
	}
	for name := range oldState.Subkeys {
		if !newState.Subkeys[name] {
			fmt.Printf("[SUBKEY REMOVED] %s\n", name)
			hasChanges = true
		}
	}

	for name, newVal := range newState.Values {
		oldVal, exists := oldState.Values[name]
		if !exists {
			fmt.Printf("[VALUE ADDED] %s = %s (type: %d)\n", name, newVal.Data, newVal.Type)
			hasChanges = true

			if detection.IsChromeExtensionForcelist(name) {
				fmt.Printf("  ⚠️  DETECTED Chrome ExtensionInstallForcelist VALUE - PROCESSING...\n")

				if !admin.IsAdmin() {
					fmt.Printf("  ❌ Insufficient privileges. Run as Administrator.\n")
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
							fmt.Printf("  ⚠️  Could not read forcelist values: %v\n", err)
						} else {
							fmt.Printf("  📋 Processing all extension IDs in forcelist...\n")

							blocklistKeyPath := detection.GetBlocklistKeyPath(forcelistKeyPath)
							allowlistKeyPath := detection.GetAllowlistKeyPath(forcelistKeyPath)

							for _, valueData := range allValues {
								extensionID := detection.ExtractExtensionIDFromValue(valueData)
								if extensionID != "" {
									fmt.Printf("  🔍 Extension ID: %s\n", extensionID)

									fmt.Printf("  📝 Adding to blocklist: %s\n", blocklistKeyPath)
									err := registry.AddToBlocklist(keyPath, blocklistKeyPath, extensionID, !canWrite)
									if err != nil {
										fmt.Printf("  ⚠️  Failed to add to blocklist: %v\n", err)
									}

									fmt.Printf("  🔍 Checking allowlist: %s\n", allowlistKeyPath)
									err = registry.RemoveFromAllowlist(keyPath, allowlistKeyPath, extensionID, !canWrite)
									if err != nil {
										fmt.Printf("  ⚠️  Failed to remove from allowlist: %v\n", err)
									}

									registry.RemoveExtensionSettingsForID(keyPath, extensionID, !canWrite, newState, extensionIndex)
								}
							}

							fmt.Printf("  🗑️  Deleting forcelist key: %s\n", forcelistKeyPath)
							err = registry.DeleteRegistryKeyRecursive(keyPath, forcelistKeyPath, !canWrite)
							if err != nil {
								fmt.Printf("  ❌ Failed to delete key: %v\n", err)
							} else {
								fmt.Printf("  ✓ Successfully deleted forcelist key\n")
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
					fmt.Printf("  ⚠️  DETECTED Firefox extension install policy - PROCESSING...\n")

					if !admin.IsAdmin() {
						fmt.Printf("  ❌ Insufficient privileges. Run as Administrator.\n")
					} else {
						extensionID := detection.ExtractFirefoxExtensionID(name)
						if extensionID != "" {
							fmt.Printf("  🔍 Extension ID: %s\n", extensionID)

							fmt.Printf("  📝 Blocking Firefox extension\n")
							err := registry.BlockFirefoxExtension(keyPath, extensionID, !canWrite)
							if err != nil {
								fmt.Printf("  ⚠️  Failed to block extension: %v\n", err)
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
								fmt.Printf("  🗑️  Deleting install policy: %s\n", extensionKeyPath)
								err = registry.DeleteRegistryKeyRecursive(keyPath, extensionKeyPath, !canWrite)
								if err != nil {
									fmt.Printf("  ❌ Failed to delete key: %v\n", err)
								} else {
									fmt.Printf("  ✓ Successfully deleted install policy\n")
									delete(newState.Subkeys, extensionKeyPath)
									registry.RemoveSubtreeFromState(newState, extensionKeyPath)
								}
							}
						}
					}
				}
			}
		} else if oldVal.Data != newVal.Data || oldVal.Type != newVal.Type {
			fmt.Printf("[VALUE CHANGED] %s\n", name)
			fmt.Printf("  Old: %s (type: %d)\n", oldVal.Data, oldVal.Type)
			fmt.Printf("  New: %s (type: %d)\n", newVal.Data, newVal.Type)
			hasChanges = true
		}
	}

	for name := range oldState.Values {
		if _, exists := newState.Values[name]; !exists {
			fmt.Printf("[VALUE REMOVED] %s\n", name)
			hasChanges = true
		}
	}

	if !hasChanges {
		fmt.Println("(No actual changes detected - likely a metadata update)")
	}

	fmt.Println("======================================\n")
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
		fmt.Println("\n========================================")
		fmt.Println("Checking for existing extension policies...")
		fmt.Println("(DRY-RUN MODE - showing planned operations)")
		fmt.Println("========================================")
	} else if !admin.IsAdmin() && canWrite {
		fmt.Println("\n⚠️  Not running as Administrator - skipping existing policy processing")
		return
	} else {
		fmt.Println("\n========================================")
		fmt.Println("Checking for existing extension policies...")
		fmt.Println("========================================")
	}

	hasExistingPolicies := false

	for valuePath, value := range state.Values {
		if detection.IsChromeExtensionForcelist(valuePath) {
			hasExistingPolicies = true
			fmt.Printf("\n[EXISTING CHROME POLICY DETECTED]\n")
			fmt.Printf("Path: %s\n", valuePath)
			fmt.Printf("Value: %s\n", value.Data)

			extensionID := detection.ExtractExtensionIDFromValue(value.Data)
			if extensionID != "" {
				fmt.Printf("🔍 Extension ID: %s\n", extensionID)

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
						fmt.Printf("⚠️  Could not read forcelist values: %v\n", err)
					} else {
						fmt.Printf("📋 Processing all extension IDs in forcelist...\n")

						blocklistKeyPath := detection.GetBlocklistKeyPath(forcelistKeyPath)
						allowlistKeyPath := detection.GetAllowlistKeyPath(forcelistKeyPath)

						for _, valueData := range allValues {
							extensionID := detection.ExtractExtensionIDFromValue(valueData)
							if extensionID != "" {
								fmt.Printf("🔍 Extension ID: %s\n", extensionID)

								fmt.Printf("📝 Adding to Chrome blocklist: %s\n", blocklistKeyPath)
								err := registry.AddToBlocklist(keyPath, blocklistKeyPath, extensionID, !canWrite)
								if err != nil {
									fmt.Printf("⚠️  Failed to add to blocklist: %v\n", err)
								}

								fmt.Printf("🔍 Checking Chrome allowlist: %s\n", allowlistKeyPath)
								err = registry.RemoveFromAllowlist(keyPath, allowlistKeyPath, extensionID, !canWrite)
								if err != nil {
									fmt.Printf("⚠️  Failed to remove from allowlist: %v\n", err)
								}

								registry.RemoveExtensionSettingsForID(keyPath, extensionID, !canWrite, state, extensionIndex)
							}
						}
					}

					fmt.Printf("🗑️  Deleting Chrome forcelist key: %s\n", forcelistKeyPath)
					err = registry.DeleteRegistryKeyRecursive(keyPath, forcelistKeyPath, !canWrite)
					if err != nil {
						fmt.Printf("❌ Failed to delete key: %v\n", err)
					} else {
						fmt.Printf("✓ Successfully removed forcelist key\n")
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
				fmt.Printf("\n[EXISTING FIREFOX POLICY DETECTED]\n")
				fmt.Printf("Path: %s\n", valuePath)
				fmt.Printf("Value: %s\n", value.Data)

				extensionID := detection.ExtractFirefoxExtensionID(valuePath)
				if extensionID != "" {
					fmt.Printf("🔍 Extension ID: %s\n", extensionID)

					fmt.Printf("📝 Blocking Firefox extension\n")
					err := registry.BlockFirefoxExtension(keyPath, extensionID, !canWrite)
					if err != nil {
						fmt.Printf("⚠️  Failed to block extension: %v\n", err)
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
						fmt.Printf("🗑️  Deleting Firefox install policy: %s\n", extensionKeyPath)
						err = registry.DeleteRegistryKeyRecursive(keyPath, extensionKeyPath, !canWrite)
						if err != nil {
							fmt.Printf("❌ Failed to delete key: %v\n", err)
						} else {
							fmt.Printf("✓ Successfully removed install policy\n")
							delete(state.Subkeys, extensionKeyPath)
							registry.RemoveSubtreeFromState(state, extensionKeyPath)
						}
					}
				}
			}
		}
	}

	if !hasExistingPolicies {
		fmt.Println("✓ No existing extension install policies found")
	}

	fmt.Println("========================================\n")
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

	fmt.Println("Checking for ExtensionInstallAllowlist keys...")

	allowlistsFound := false
	allowlistKeys := make(map[string]bool)

	for subkeyPath := range state.Subkeys {
		if pathutils.Contains(subkeyPath, "ExtensionInstallAllowlist") {
			allowlistsFound = true
			allowlistKeys[subkeyPath] = true
		}
	}

	if !allowlistsFound {
		fmt.Println("✓ No ExtensionInstallAllowlist keys found")
		return
	}

	for allowlistPath := range allowlistKeys {
		fmt.Printf("\n[REMOVING ALLOWLIST]\n")
		fmt.Printf("Path: %s\n", allowlistPath)

		values, err := registry.ReadKeyValues(keyPath, allowlistPath)
		if err == nil && len(values) > 0 {
			fmt.Printf("Found %d extension(s) in allowlist:\n", len(values))
			for valueName, valueData := range values {
				extensionID := detection.ExtractExtensionIDFromValue(valueData)
				if extensionID != "" {
					fmt.Printf("  - %s: %s\n", valueName, extensionID)
				}
			}
		}

		fmt.Printf("🗑️  Deleting allowlist key: %s\n", allowlistPath)
		err = registry.DeleteRegistryKeyRecursive(keyPath, allowlistPath, !canWrite)
		if err != nil {
			fmt.Printf("❌ Failed to delete allowlist: %v\n", err)
		} else {
			fmt.Printf("✓ Successfully deleted allowlist\n")
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
func GetBlockedExtensionIDs(keyPath string, state *registry.RegState) map[string]bool {
	blockedIDs := make(map[string]bool)

	fmt.Println("  📋 Scanning for blocked extension IDs...")

	for subkeyPath := range state.Subkeys {
		if pathutils.Contains(subkeyPath, "ExtensionInstallBlocklist") {
			fmt.Printf("  🔍 Found blocklist: %s\n", subkeyPath)
			values, err := registry.ReadKeyValues(keyPath, subkeyPath)
			if err == nil {
				for valueName, valueData := range values {
					extensionID := detection.ExtractExtensionIDFromValue(valueData)
					if extensionID != "" {
						fmt.Printf("    ├─ %s: %s\n", valueName, extensionID)
						blockedIDs[extensionID] = true
					}
				}
			} else {
				fmt.Printf("    ⚠️  Could not read values: %v\n", err)
			}
		}
	}

	for valuePath, value := range state.Values {
		if detection.IsFirefoxExtensionSettings(valuePath) &&
			pathutils.Contains(valuePath, "installation_mode") &&
			value.Data == "blocked" {
			extensionID := detection.ExtractFirefoxExtensionID(valuePath)
			if extensionID != "" {
				fmt.Printf("  🦊 Firefox blocked: %s\n", extensionID)
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

	fmt.Println("\n========================================")
	fmt.Println("Cleaning up extension settings...")
	fmt.Println("========================================")
	fmt.Println("Checking for extension settings of blocked extensions...")
	fmt.Println("Note: This removes settings for ALL extensions in blocklists,")
	fmt.Println("      regardless of whether they were added via forcelist or manually.")

	blockedIDs := GetBlockedExtensionIDs(keyPath, state)

	if len(blockedIDs) == 0 {
		fmt.Println("✓ No blocked extensions found")
		fmt.Println("========================================\n")
		return
	}

	fmt.Printf("\nFound %d blocked extension ID(s):\n", len(blockedIDs))
	for id := range blockedIDs {
		fmt.Printf("  - %s\n", id)
	}

	for extensionID := range blockedIDs {
		fmt.Printf("\n[CHECKING SETTINGS FOR BLOCKED EXTENSION]\n")
		fmt.Printf("Extension ID: %s\n", extensionID)
		registry.RemoveExtensionSettingsForID(keyPath, extensionID, !canWrite, state, extensionIndex)
	}

	fmt.Println("========================================\n")
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
		fmt.Println("Error creating event:", err)
		telemetry.RecordError(ctx, err)
		return
	}
	defer windows.CloseHandle(event)

	err = windows.RegNotifyChangeKeyValue(hKey, true, windows.REG_NOTIFY_CHANGE_NAME|windows.REG_NOTIFY_CHANGE_LAST_SET, event, true)
	if err != nil {
		fmt.Println("Error setting up registry notification:", err)
		telemetry.RecordError(ctx, err)
		return
	}

	fmt.Println("Monitoring registry changes...")
	telemetry.AddEvent(ctx, "monitoring-started")

	for {
		status, err := windows.WaitForSingleObject(event, windows.INFINITE)
		if err != nil {
			fmt.Println("Error waiting for event:", err)
			telemetry.RecordError(ctx, err)
			return
		}

		if status == windows.WAIT_OBJECT_0 {
			telemetry.AddEvent(ctx, "registry-change-detected")
			
			newState, err := CaptureRegistryState(ctx, hKey, keyPath)
			if err != nil {
				fmt.Println("Error capturing new state:", err)
			} else {
				PrintDiff(ctx, previousState, newState, keyPath, canWrite, extensionIndex)
				previousState = newState
			}

			err = windows.RegNotifyChangeKeyValue(hKey, true, windows.REG_NOTIFY_CHANGE_NAME|windows.REG_NOTIFY_CHANGE_LAST_SET, event, true)
			if err != nil {
				fmt.Println("Error re-arming registry notification:", err)
				telemetry.RecordError(ctx, err)
				return
			}
		}
	}
}
