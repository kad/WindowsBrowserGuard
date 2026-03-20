package monitor

import (
	"context"
	"errors"
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

							blocklistConfirmed := false
							plannedBlockedIDs := make(PlannedBlockedIDs)
							for _, valueData := range allValues {
								extensionID := detection.ExtractExtensionIDFromValue(valueData)
								if extensionID != "" {
									telemetry.Printf(ctx, "  🔍 Extension ID: %s\n", extensionID)
									trackPlannedBlockedID(plannedBlockedIDs, blocklistKeyPath, extensionID)

									telemetry.Printf(ctx, "  📝 Adding to blocklist: %s\n", blocklistKeyPath)
									err := registry.AddToBlocklist(keyPath, blocklistKeyPath, extensionID, !canWrite)
									if err != nil {
										telemetry.Printf(ctx, "  ⚠️  Failed to add to blocklist: %v\n", err)
									} else if canWrite {
										blocklistConfirmed = true
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

							if blocklistConfirmed {
								// Keep newState in sync with the registry only after confirming
								// the blocklist key exists following a successful write.
								newState.Subkeys[blocklistKeyPath] = true
							}

							// Post-process: verify blocklist/allowlist consistency
							// across all known allowlists in newState. Each comparison
							// remains browser-local (Chrome vs Chrome, Edge vs Edge).
							EnforceBlockAllowlistConsistency(ctx, keyPath, newState, canWrite, plannedBlockedIDs)
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

// PlannedBlockedIDs maps a blocklist key path to extension IDs that are
// expected to be added during the current enforcement pass but may not yet
// exist in the live registry (for example during dry-run).
type PlannedBlockedIDs map[string]map[string]bool

func trackPlannedBlockedID(planned PlannedBlockedIDs, blocklistPath, extensionID string) {
	if planned == nil || blocklistPath == "" || extensionID == "" {
		return
	}
	if planned[blocklistPath] == nil {
		planned[blocklistPath] = make(map[string]bool)
	}
	planned[blocklistPath][extensionID] = true
}

// CollectPlannedBlockedIDs scans Chromium forcelists in the captured state and
// returns the blocklist entries that would be created from them.
func CollectPlannedBlockedIDs(state *registry.RegState) PlannedBlockedIDs {
	planned := make(PlannedBlockedIDs)
	for valuePath, value := range state.Values {
		if !detection.IsChromeExtensionForcelist(valuePath) {
			continue
		}
		parentPath, ok := pathutils.GetParentPath(valuePath)
		if !ok {
			continue
		}
		extensionID := detection.ExtractExtensionIDFromValue(value.Data)
		if extensionID == "" {
			continue
		}
		trackPlannedBlockedID(planned, detection.GetBlocklistKeyPath(parentPath), extensionID)
	}
	return planned
}

// EnforceBlockAllowlistConsistency ensures that no extension ID present in a
// Chromium browser's blocklist also appears in that same browser's allowlist.
// This pass only applies to policies that use ExtensionInstallAllowlist and
// ExtensionInstallBlocklist (for example Chrome and Edge). Firefox extension
// policies are handled separately and are not part of this allowlist cleanup.
//
// For every ExtensionInstallAllowlist key found in state the function derives
// the corresponding ExtensionInstallBlocklist path (same browser, same base
// path) and reads it live from the registry. During dry-run it can also merge
// planned blocklist additions so the reported allowlist removals reflect the
// writes that would happen. Any allowlist value whose extension ID is present
// in that browser's blocklist is removed; if the allowlist key is empty
// afterwards it is deleted entirely.
func EnforceBlockAllowlistConsistency(ctx context.Context, keyPath string, state *registry.RegState, canWrite bool, plannedBlockedIDs PlannedBlockedIDs) {
	ctx, span := telemetry.StartSpan(ctx, "monitor.EnforceBlockAllowlistConsistency",
		attribute.String("key-path", keyPath),
		attribute.Bool("can-write", canWrite),
	)
	defer span.End()

	if !canWrite {
		telemetry.Println(ctx, "\n========================================")
		telemetry.Println(ctx, "Enforcing blocklist/allowlist consistency...")
		telemetry.Println(ctx, "(DRY-RUN MODE - showing planned operations)")
		telemetry.Println(ctx, "========================================")
	} else if !admin.IsAdmin() {
		telemetry.Println(ctx, "\n⚠️  Not running as Administrator - skipping blocklist/allowlist consistency check")
		return
	} else {
		telemetry.Println(ctx, "\n========================================")
		telemetry.Println(ctx, "Enforcing blocklist/allowlist consistency...")
		telemetry.Println(ctx, "========================================")
	}

	detectedConflicts := 0
	resolvedConflicts := 0

	for subkeyPath := range state.Subkeys {
		if !pathutils.Contains(subkeyPath, "ExtensionInstallAllowlist") {
			continue
		}

		// Derive the same-browser blocklist path. The path component replacement
		// preserves the full browser prefix (e.g. "Microsoft\Edge\"), so Chrome
		// blocklist is never compared against Edge allowlist and vice-versa.
		blocklistPath := pathutils.ReplacePathComponent(
			subkeyPath, "ExtensionInstallAllowlist", "ExtensionInstallBlocklist",
		)

		browser := detection.GetBrowserFromPath(subkeyPath)
		telemetry.Printf(ctx, "\n[%s] Checking allowlist: %s\n", browser, subkeyPath)
		telemetry.Printf(ctx, "[%s] Against blocklist:  %s\n", browser, blocklistPath)

		// Read the blocklist live, then merge any planned dry-run additions.
		blocklistValues, err := registry.ReadKeyValues(keyPath, blocklistPath)
		if err != nil {
			if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
				blocklistValues = map[string]string{}
			} else {
				telemetry.Printf(ctx, "  ❌ Failed to read %s blocklist at HKLM\\%s\\%s: %v\n", browser, keyPath, blocklistPath, err)
				telemetry.RecordError(ctx, err)
				continue
			}
		}
		if blocklistValues == nil {
			blocklistValues = map[string]string{}
		}

		blockedIDs := make(map[string]bool, len(blocklistValues))
		for _, v := range blocklistValues {
			if id := detection.ExtractExtensionIDFromValue(v); id != "" {
				blockedIDs[id] = true
			}
		}
		if !canWrite {
			for plannedID := range plannedBlockedIDs[blocklistPath] {
				blockedIDs[plannedID] = true
			}
		}
		if len(blockedIDs) == 0 {
			telemetry.Printf(ctx, "  ℹ️  Blocklist is empty or absent - nothing to enforce\n")
			continue
		}
		telemetry.Printf(ctx, "  📋 %d blocked ID(s) in %s blocklist\n", len(blockedIDs), browser)

		// Read allowlist values live for accurate comparison.
		allowlistValues, err := registry.ReadKeyValues(keyPath, subkeyPath)
		if err != nil {
			if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
				telemetry.Printf(ctx, "  ✓ Allowlist is empty or absent - no conflicts possible\n")
				continue
			}
			telemetry.Printf(ctx, "  ❌ Failed to read %s allowlist at HKLM\\%s\\%s: %v\n", browser, keyPath, subkeyPath, err)
			telemetry.RecordError(ctx, err)
			continue
		}
		if len(allowlistValues) == 0 {
			telemetry.Printf(ctx, "  ✓ Allowlist is empty - no conflicts possible\n")
			continue
		}

		conflicts := 0
		initialAllowlistCount := len(allowlistValues)
		conflictingValueNames := make([]string, 0, len(allowlistValues))
		conflictingIDs := make([]string, 0, len(allowlistValues))
		for valueName, valueData := range allowlistValues {
			extID := detection.ExtractExtensionIDFromValue(valueData)
			if extID == "" || !blockedIDs[extID] {
				continue
			}
			conflicts++
			detectedConflicts++
			conflictingValueNames = append(conflictingValueNames, valueName)
			conflictingIDs = append(conflictingIDs, extID)
			telemetry.Printf(ctx, "  ⚠️  Conflict: %s is blocked but present in %s allowlist\n", extID, browser)
		}

		if conflicts == 0 {
			telemetry.Printf(ctx, "  ✓ No conflicts in %s allowlist\n", browser)
			continue
		}

		deletedValueNames, err := registry.RemoveAllowlistValueNames(keyPath, subkeyPath, conflictingValueNames, !canWrite)
		if err != nil {
			telemetry.Printf(ctx, "  ❌ Failed to remove conflicting allowlist entries: %v\n", err)
			continue
		}

		deletedNames := make(map[string]bool, len(deletedValueNames))
		for _, valueName := range deletedValueNames {
			deletedNames[valueName] = true
		}
		if canWrite {
			for _, valueName := range deletedValueNames {
				delete(state.Values, pathutils.BuildPath(subkeyPath, valueName))
			}
		}
		for i, valueName := range conflictingValueNames {
			if !deletedNames[valueName] {
				continue
			}
			resolvedConflicts++
			if canWrite {
				telemetry.Printf(ctx, "  ✓ Removed %s from %s allowlist\n", conflictingIDs[i], browser)
			} else {
				telemetry.Printf(ctx, "  ✓ Would remove %s from %s allowlist (dry run)\n", conflictingIDs[i], browser)
			}
		}

		if !canWrite {
			if conflicts == initialAllowlistCount {
				telemetry.Printf(ctx, "  ✓ Would delete empty %s allowlist key (dry run)\n", browser)
			}
			continue
		}

		// Delete the key if it is now empty to leave no orphan keys behind.
		remaining, err := registry.ReadKeyValues(keyPath, subkeyPath)
		if err != nil {
			telemetry.Printf(ctx, "  ❌ Failed to confirm whether %s allowlist is empty at HKLM\\%s\\%s: %v\n", browser, keyPath, subkeyPath, err)
			telemetry.RecordError(ctx, err)
			continue
		}
		if len(remaining) == 0 {
			telemetry.Printf(ctx, "  🗑️  Allowlist empty after conflict removal, deleting: %s\n", subkeyPath)
			if err := registry.DeleteRegistryKeyRecursive(keyPath, subkeyPath, !canWrite); err != nil {
				telemetry.Printf(ctx, "  ❌ Failed to delete empty allowlist key: %v\n", err)
			} else {
				telemetry.Printf(ctx, "  ✓ Deleted empty %s allowlist key\n", browser)
				registry.RemoveSubtreeFromState(state, subkeyPath)
				delete(state.Subkeys, subkeyPath)
			}
		}
	}

	if detectedConflicts == 0 {
		telemetry.Println(ctx, "✓ No blocklist/allowlist conflicts found")
	} else {
		if canWrite {
			telemetry.Printf(ctx, "✓ Resolved %d of %d blocklist/allowlist conflict(s)\n", resolvedConflicts, detectedConflicts)
		} else {
			telemetry.Printf(ctx, "✓ Would resolve %d of %d blocklist/allowlist conflict(s) (dry run)\n", resolvedConflicts, detectedConflicts)
		}
	}
	telemetry.Println(ctx, "========================================")
	telemetry.Println(ctx, "")
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
