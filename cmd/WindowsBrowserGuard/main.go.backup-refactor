package main

import (
	"flag"
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/kad/WindowsBrowserGuard/pkg/detection"
	"github.com/kad/WindowsBrowserGuard/pkg/pathutils"
	"github.com/kad/WindowsBrowserGuard/pkg/registry"
	"golang.org/x/sys/windows"
)

var (
	shell32            = syscall.NewLazyDLL("shell32.dll")
	shellExecuteW      = shell32.NewProc("ShellExecuteW")
	
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getModuleFileNameW = kernel32.NewProc("GetModuleFileNameW")
	
	// Command line flags
	dryRun = flag.Bool("dry-run", false, "Run in read-only mode without making changes")
)

var extensionIndex *registry.ExtensionPathIndex
var metrics registry.PerfMetrics

func isAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.GetCurrentProcessToken()
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}
	return member
}

func getExecutablePath() (string, error) {
	buf := make([]uint16, windows.MAX_PATH)
	ret, _, _ := getModuleFileNameW.Call(0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ret == 0 {
		return "", fmt.Errorf("failed to get executable path")
	}
	return syscall.UTF16ToString(buf), nil
}

func elevatePrivileges() error {
	exePath, err := getExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	verbPtr, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString(exePath)
	cwdPtr, _ := syscall.UTF16PtrFromString("")

	ret, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(exePtr)),
		0,
		uintptr(unsafe.Pointer(cwdPtr)),
		uintptr(1), // SW_NORMAL
	)

	if ret <= 32 {
		return fmt.Errorf("failed to elevate privileges, error code: %d", ret)
	}

	return nil
}

func canDeleteRegistryKey(keyPath string) bool {
	key, err := syscall.UTF16PtrFromString(keyPath)
	if err != nil {
		return false
	}

	var hKey windows.Handle
	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, key, 0, windows.DELETE|windows.KEY_READ, &hKey)
	if err != nil {
		return false
	}
	windows.RegCloseKey(hKey)
	return true
}

func checkAdminAndElevate() bool {
	if *dryRun {
		fmt.Println("🔍 DRY-RUN MODE: Running in read-only mode")
		fmt.Println("   No changes will be made to the registry")
		fmt.Println("   All write/delete operations will be simulated\n")
		return false
	}
	
	if !isAdmin() {
		fmt.Println("⚠️  WARNING: Not running as Administrator")
		fmt.Println("Registry deletion requires elevated privileges.")
		fmt.Print("Attempting to elevate permissions... ")

		err := elevatePrivileges()
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
			fmt.Println("\nPlease run this program as Administrator to enable key deletion.")
			fmt.Println("Or use --dry-run flag to test in read-only mode.")
			fmt.Println("Press Enter to continue in read-only mode...")
			fmt.Scanln()
			return false
		} else {
			fmt.Println("✓ Relaunching with elevated privileges...")
			return false
		}
	} else {
		fmt.Println("✓ Running with Administrator privileges")
		return true
	}
}

func captureRegistryState(hKey windows.Handle, keyPath string) (*registry.RegState, error) {
	state := &registry.RegState{
		Subkeys: make(map[string]bool),
		Values:  make(map[string]registry.RegValue),
	}

	err := registry.CaptureKeyRecursive(hKey, "", state, 0)
	if err != nil {
		return nil, err
	}

	return state, nil
}

func printDiff(oldState, newState *registry.RegState, keyPath string, canWrite bool) {
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

				if !isAdmin() {
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
									fmt.Printf("  🔍 Extracted Chrome extension ID: %s\n", extensionID)

									fmt.Printf("  📝 Adding to Chrome blocklist: %s\n", blocklistKeyPath)
									err := registry.AddToBlocklist(keyPath, blocklistKeyPath, extensionID, !canWrite)
									if err != nil {
										fmt.Printf("  ⚠️  Failed to add %s to blocklist: %v\n", extensionID, err)
									}

									fmt.Printf("  🔍 Checking Chrome allowlist: %s\n", allowlistKeyPath)
									err = registry.RemoveFromAllowlist(keyPath, allowlistKeyPath, extensionID, !canWrite)
									if err != nil {
										fmt.Printf("  ⚠️  Failed to remove from allowlist: %v\n", err)
									}

									registry.RemoveExtensionSettingsForID(keyPath, extensionID, !canWrite, newState, extensionIndex)
								}
							}
						}

						fmt.Printf("  🗑️  Deleting Chrome forcelist key: %s\n", forcelistKeyPath)
						err = registry.DeleteRegistryKeyRecursive(keyPath, forcelistKeyPath, !canWrite)
						if err != nil {
							fmt.Printf("  ❌ Failed to delete key: %v\n", err)
						} else {
							fmt.Printf("  ✓ Successfully deleted Chrome forcelist key\n")
							delete(newState.Subkeys, forcelistKeyPath)
							registry.RemoveSubtreeFromState(newState, forcelistKeyPath)
						}
					}
				}
			}

			if detection.IsFirefoxExtensionSettings(name) {
				fmt.Printf("  ⚠️  DETECTED Firefox ExtensionSettings VALUE - PROCESSING...\n")

				if !isAdmin() {
					fmt.Printf("  ❌ Insufficient privileges. Run as Administrator.\n")
				} else {
					extensionID := detection.ExtractFirefoxExtensionID(name)
					if extensionID != "" {
						fmt.Printf("  🔍 Extracted Firefox extension ID: %s\n", extensionID)

						if pathutils.Contains(name, "installation_mode") &&
							(newVal.Data == "force_installed" || newVal.Data == "normal_installed") {
							fmt.Printf("  📝 Blocking Firefox extension: %s\n", extensionID)
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
								fmt.Printf("  🗑️  Deleting Firefox extension install policy: %s\n", extensionKeyPath)
								err = registry.DeleteRegistryKeyRecursive(keyPath, extensionKeyPath, !canWrite)
								if err != nil {
									fmt.Printf("  ❌ Failed to delete key: %v\n", err)
								} else {
									fmt.Printf("  ✓ Successfully deleted Firefox install policy\n")
									delete(newState.Subkeys, extensionKeyPath)
									registry.RemoveSubtreeFromState(newState, extensionKeyPath)
								}
							}
						}
					}
				}
			}
		} else if oldVal.Data != newVal.Data || oldVal.Type != newVal.Type {
			fmt.Printf("[VALUE MODIFIED] %s\n", name)
			fmt.Printf("  Old: %s (type: %d)\n", oldVal.Data, oldVal.Type)
			fmt.Printf("  New: %s (type: %d)\n", newVal.Data, newVal.Type)
			hasChanges = true
		}
	}
	for name, oldVal := range oldState.Values {
		if _, exists := newState.Values[name]; !exists {
			fmt.Printf("[VALUE REMOVED] %s = %s\n", name, oldVal.Data)
			hasChanges = true
		}
	}

	if !hasChanges {
		fmt.Println("(No specific changes detected)")
	}
	fmt.Println("======================================\n")
}

func processExistingPolicies(keyPath string, state *registry.RegState, canWrite bool) {
	if !canWrite && !isAdmin() {
		fmt.Println("\n========================================")
		fmt.Println("Checking for existing extension policies...")
		fmt.Println("(DRY-RUN MODE - showing planned operations)")
		fmt.Println("========================================")
	} else if !isAdmin() && canWrite {
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

func cleanupAllowlists(keyPath string, state *registry.RegState, canWrite bool) {
	if !isAdmin() {
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

func getBlockedExtensionIDs(keyPath string, state *registry.RegState) map[string]bool {
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

func cleanupExtensionSettings(keyPath string, state *registry.RegState, canWrite bool) {
	if !isAdmin() {
		return
	}

	fmt.Println("\n========================================")
	fmt.Println("Cleaning up extension settings...")
	fmt.Println("========================================")
	fmt.Println("Checking for extension settings of blocked extensions...")
	fmt.Println("Note: This removes settings for ALL extensions in blocklists,")
	fmt.Println("      regardless of whether they were added via forcelist or manually.")

	blockedIDs := getBlockedExtensionIDs(keyPath, state)

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

func watchRegistryChanges(hKey windows.Handle, keyPath string, previousState *registry.RegState, canWrite bool) {
	event, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		fmt.Println("Error creating event:", err)
		return
	}
	defer windows.CloseHandle(event)

	err = windows.RegNotifyChangeKeyValue(hKey, true, windows.REG_NOTIFY_CHANGE_NAME|windows.REG_NOTIFY_CHANGE_LAST_SET, event, true)
	if err != nil {
		fmt.Println("Error setting up registry notification:", err)
		return
	}

	fmt.Println("Monitoring registry changes...")

	for {
		status, err := windows.WaitForSingleObject(event, windows.INFINITE)
		if err != nil {
			fmt.Println("Error waiting for event:", err)
			return
		}

		if status == windows.WAIT_OBJECT_0 {
			newState, err := captureRegistryState(hKey, keyPath)
			if err != nil {
				fmt.Println("Error capturing new state:", err)
			} else {
				printDiff(previousState, newState, keyPath, canWrite)
				previousState = newState
			}

			err = windows.RegNotifyChangeKeyValue(hKey, true, windows.REG_NOTIFY_CHANGE_NAME|windows.REG_NOTIFY_CHANGE_LAST_SET, event, true)
			if err != nil {
				fmt.Println("Error re-arming registry notification:", err)
				return
			}
		}
	}
}

func main() {
	flag.Parse()
	
	hasAdmin := checkAdminAndElevate()
	canWrite := hasAdmin && !*dryRun

	keyPath := `SOFTWARE\Policies`

	if canWrite {
		canDelete := canDeleteRegistryKey(keyPath)
		if !canDelete {
			fmt.Println("⚠️  WARNING: Insufficient permissions to delete registry keys")
			fmt.Println("Key deletion features will be disabled.")
			canWrite = false
		} else {
			fmt.Println("✓ Registry deletion permissions verified")
		}
	}

	key, err := syscall.UTF16PtrFromString(keyPath)
	if err != nil {
		fmt.Println("Error converting key path:", err)
		return
	}

	var hKey windows.Handle
	// In dry-run mode, only request read permissions
	var permissions uint32 = windows.KEY_NOTIFY | windows.KEY_READ
	if canWrite {
		permissions |= windows.DELETE
	}
	
	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, key, 0, permissions, &hKey)
	if err != nil {
		fmt.Println("Error opening registry key:", err)
		return
	}
	defer windows.RegCloseKey(hKey)

	fmt.Println("Capturing initial registry state...")
	startTime := time.Now()
	previousState, err := captureRegistryState(hKey, keyPath)
	if err != nil {
		fmt.Println("Error capturing initial state:", err)
		return
	}
	scanDuration := time.Since(startTime)
	metrics.StartupTime = scanDuration
	metrics.InitialScanKeys = len(previousState.Subkeys) + len(previousState.Values)

	fmt.Printf("Initial state: %d subkeys, %d values (captured in %v)\n",
		len(previousState.Subkeys), len(previousState.Values), scanDuration)

	fmt.Println("Building extension path index...")
	indexStart := time.Now()
	extensionIndex = registry.NewExtensionPathIndex()
	extensionIndex.BuildFromState(previousState)
	indexDuration := time.Since(indexStart)
	metrics.IndexBuildTime = indexDuration

	fmt.Printf("Index built: tracking %d unique extension IDs (in %v)\n",
		extensionIndex.GetCount(), indexDuration)

	processExistingPolicies(keyPath, previousState, canWrite)
	cleanupAllowlists(keyPath, previousState, canWrite)
	cleanupExtensionSettings(keyPath, previousState, canWrite)

	watchRegistryChanges(hKey, keyPath, previousState, canWrite)
}
