package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	advapi32           = syscall.NewLazyDLL("advapi32.dll")
	regEnumValueW      = advapi32.NewProc("RegEnumValueW")
	regQueryInfoKeyW   = advapi32.NewProc("RegQueryInfoKeyW")
	regDeleteKeyW      = advapi32.NewProc("RegDeleteKeyW")
	regSetValueExW     = advapi32.NewProc("RegSetValueExW")
	regCreateKeyExW    = advapi32.NewProc("RegCreateKeyExW")
	regDeleteValueW    = advapi32.NewProc("RegDeleteValueW")
	
	shell32            = syscall.NewLazyDLL("shell32.dll")
	shellExecuteW      = shell32.NewProc("ShellExecuteW")
	
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getModuleFileNameW = kernel32.NewProc("GetModuleFileNameW")
)

type RegValue struct {
	Name  string
	Type  uint32
	Data  string
}

type RegState struct {
	Subkeys map[string]bool
	Values  map[string]RegValue
}

var extensionIndex *ExtensionPathIndex


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

func captureRegistryState(hKey windows.Handle, keyPath string) (*RegState, error) {
	state := &RegState{
		Subkeys: make(map[string]bool),
		Values:  make(map[string]RegValue),
	}

	err := captureKeyRecursive(hKey, "", state)
	if err != nil {
		return nil, err
	}

	return state, nil
}

func captureKeyRecursive(hKey windows.Handle, relativePath string, state *RegState) error {
	// Enumerate subkeys at this level
	var index uint32
	subkeyNames := []string{}
	for {
		nameBuf := getNameBuffer()
		nameLen := uint32(len(*nameBuf))
		err := windows.RegEnumKeyEx(hKey, index, &(*nameBuf)[0], &nameLen, nil, nil, nil, nil)
		if err == windows.ERROR_NO_MORE_ITEMS {
			putNameBuffer(nameBuf)
			break
		}
		if err != nil {
			putNameBuffer(nameBuf)
			return fmt.Errorf("error enumerating subkeys: %v", err)
		}
		subkeyName := syscall.UTF16ToString((*nameBuf)[:nameLen])
		putNameBuffer(nameBuf)
		
		fullPath := buildPath(relativePath, subkeyName)
		
		state.Subkeys[fullPath] = true
		subkeyNames = append(subkeyNames, subkeyName)
		index++
	}

	// Enumerate values at this level
	index = 0
	for {
		nameBuf := getNameBuffer()
		nameLen := uint32(len(*nameBuf))
		var valueType uint32
		dataBuf := getDataBuffer()
		dataLen := uint32(len(*dataBuf))

		ret, _, _ := regEnumValueW.Call(
			uintptr(hKey),
			uintptr(index),
			uintptr(unsafe.Pointer(&(*nameBuf)[0])),
			uintptr(unsafe.Pointer(&nameLen)),
			0,
			uintptr(unsafe.Pointer(&valueType)),
			uintptr(unsafe.Pointer(&(*dataBuf)[0])),
			uintptr(unsafe.Pointer(&dataLen)),
		)

		if ret == uintptr(windows.ERROR_NO_MORE_ITEMS) {
			putNameBuffer(nameBuf)
			putDataBuffer(dataBuf)
			break
		}
		if ret != 0 {
			putNameBuffer(nameBuf)
			putDataBuffer(dataBuf)
			return fmt.Errorf("error enumerating values: error code %d", ret)
		}

		valueName := syscall.UTF16ToString((*nameBuf)[:nameLen])
		valueData := formatRegValue(valueType, (*dataBuf)[:dataLen])
		putNameBuffer(nameBuf)
		putDataBuffer(dataBuf)
		
		fullPath := buildPath(relativePath, valueName)
		
		state.Values[fullPath] = RegValue{
			Name: fullPath,
			Type: valueType,
			Data: valueData,
		}
		index++
	}

	// Recursively process each subkey
	for _, subkeyName := range subkeyNames {
		subkeyPtr, err := syscall.UTF16PtrFromString(subkeyName)
		if err != nil {
			continue
		}

		var hSubKey windows.Handle
		err = windows.RegOpenKeyEx(hKey, subkeyPtr, 0, windows.KEY_READ, &hSubKey)
		if err != nil {
			// Skip keys we can't open (permission issues, etc.)
			continue
		}

		fullPath := buildPath(relativePath, subkeyName)

		captureKeyRecursive(hSubKey, fullPath, state)
		windows.RegCloseKey(hSubKey)
	}

	return nil
}

func formatRegValue(valueType uint32, data []byte) string {
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

func deleteRegistryKey(baseKeyPath, relativePath string) error {
	// Open the parent key
	fullPath := baseKeyPath
	if relativePath != "" {
		fullPath = baseKeyPath + "\\" + relativePath
	}

	keyPtr, err := syscall.UTF16PtrFromString(fullPath)
	if err != nil {
		return fmt.Errorf("error converting key path: %v", err)
	}

	var hKey windows.Handle
	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, keyPtr, 0, windows.DELETE, &hKey)
	if err != nil {
		return fmt.Errorf("error opening key for deletion: %v", err)
	}
	defer windows.RegCloseKey(hKey)

	// Delete the key
	ret, _, _ := regDeleteKeyW.Call(uintptr(hKey), uintptr(0))
	if ret != 0 {
		return fmt.Errorf("error deleting key: error code %d", ret)
	}

	return nil
}

func deleteRegistryKeyRecursive(baseKeyPath, relativePath string) error {
	fullPath := baseKeyPath
	if relativePath != "" {
		fullPath = baseKeyPath + "\\" + relativePath
	}

	keyPtr, err := syscall.UTF16PtrFromString(fullPath)
	if err != nil {
		return fmt.Errorf("error converting key path: %v", err)
	}

	var hKey windows.Handle
	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, keyPtr, 0, windows.KEY_READ|windows.DELETE, &hKey)
	if err != nil {
		return fmt.Errorf("error opening key: %v", err)
	}
	defer windows.RegCloseKey(hKey)

	// Enumerate and delete all subkeys first
	for {
		nameBuf := make([]uint16, 256)
		nameLen := uint32(len(nameBuf))
		err := windows.RegEnumKeyEx(hKey, 0, &nameBuf[0], &nameLen, nil, nil, nil, nil)
		if err == windows.ERROR_NO_MORE_ITEMS {
			break
		}
		if err != nil {
			return fmt.Errorf("error enumerating subkeys: %v", err)
		}
		subkeyName := syscall.UTF16ToString(nameBuf[:nameLen])
		
		// Recursively delete subkey
		subkeyPath := relativePath
		if subkeyPath != "" {
			subkeyPath += "\\"
		}
		subkeyPath += subkeyName
		
		err = deleteRegistryKeyRecursive(baseKeyPath, subkeyPath)
		if err != nil {
			fmt.Printf("Warning: failed to delete subkey %s: %v\n", subkeyPath, err)
		}
	}

	// Now delete this key
	windows.RegCloseKey(hKey)
	
	keyPtr, _ = syscall.UTF16PtrFromString(relativePath)
	parentPath := baseKeyPath
	if relativePath != "" {
		// Open parent and delete this key
		lastSlash := -1
		for i := len(relativePath) - 1; i >= 0; i-- {
			if relativePath[i] == '\\' {
				lastSlash = i
				break
			}
		}
		
		if lastSlash >= 0 {
			parentPath = baseKeyPath + "\\" + relativePath[:lastSlash]
			keyName := relativePath[lastSlash+1:]
			keyPtr, _ = syscall.UTF16PtrFromString(keyName)
		} else {
			keyPtr, _ = syscall.UTF16PtrFromString(relativePath)
		}
		
		parentKeyPtr, _ := syscall.UTF16PtrFromString(parentPath)
		var hParentKey windows.Handle
		err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, parentKeyPtr, 0, windows.DELETE, &hParentKey)
		if err != nil {
			return fmt.Errorf("error opening parent key: %v", err)
		}
		defer windows.RegCloseKey(hParentKey)
		
		ret, _, _ := regDeleteKeyW.Call(uintptr(hParentKey), uintptr(unsafe.Pointer(keyPtr)))
		if ret != 0 {
			return fmt.Errorf("error deleting key: error code %d", ret)
		}
	}

	return nil
}

func printDiff(oldState, newState *RegState, keyPath string) {
	fmt.Println("\n========== CHANGES DETECTED ==========")
	fmt.Println("Time:", time.Now().Format(time.RFC3339))
	fmt.Println("Key:", keyPath)
	fmt.Println("======================================")

	hasChanges := false

	// Check for added/removed subkeys
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

	// Check for added/modified/removed values
	for name, newVal := range newState.Values {
		oldVal, exists := oldState.Values[name]
		if !exists {
			fmt.Printf("[VALUE ADDED] %s = %s (type: %d)\n", name, newVal.Data, newVal.Type)
			hasChanges = true
			
			// Check if this is a Chrome ExtensionInstallForcelist value
			if isChromeExtensionForcelist(name) {
				fmt.Printf("  ⚠️  DETECTED Chrome ExtensionInstallForcelist VALUE - PROCESSING...\n")
				
				if !isAdmin() {
					fmt.Printf("  ❌ Insufficient privileges. Run as Administrator.\n")
				} else {
					// Get the key path (parent of the value)
					lastSlash := -1
					for i := len(name) - 1; i >= 0; i-- {
						if name[i] == '\\' {
							lastSlash = i
							break
						}
					}
					
					if lastSlash >= 0 {
						forcelistKeyPath := name[:lastSlash]
						
						// Read ALL values from the forcelist key before deleting
						allValues, err := readKeyValues(keyPath, forcelistKeyPath)
						if err != nil {
							fmt.Printf("  ⚠️  Could not read forcelist values: %v\n", err)
						} else {
							fmt.Printf("  📋 Processing all extension IDs in forcelist...\n")
							
							blocklistKeyPath := getBlocklistKeyPath(forcelistKeyPath)
							allowlistKeyPath := getAllowlistKeyPath(forcelistKeyPath)
							
							// Process each extension ID
							for _, valueData := range allValues {
								extensionID := extractExtensionIDFromValue(valueData)
								if extensionID != "" {
									fmt.Printf("  🔍 Extracted Chrome extension ID: %s\n", extensionID)
									
									// Add to blocklist
									fmt.Printf("  📝 Adding to Chrome blocklist: %s\n", blocklistKeyPath)
									err := addToBlocklist(keyPath, blocklistKeyPath, extensionID)
									if err != nil {
										fmt.Printf("  ⚠️  Failed to add %s to blocklist: %v\n", extensionID, err)
									}
									
									// Remove from allowlist
									fmt.Printf("  🔍 Checking Chrome allowlist: %s\n", allowlistKeyPath)
									err = removeFromAllowlist(keyPath, allowlistKeyPath, extensionID)
									if err != nil {
										fmt.Printf("  ⚠️  Failed to remove from allowlist: %v\n", err)
									}
									
									// Remove extension settings for this ID
									removeExtensionSettingsForID(keyPath, extensionID, newState)
								}
							}
						}
						
						// Now delete the entire forcelist key
						fmt.Printf("  🗑️  Deleting Chrome forcelist key: %s\n", forcelistKeyPath)
						err = deleteRegistryKeyRecursive(keyPath, forcelistKeyPath)
						if err != nil {
							fmt.Printf("  ❌ Failed to delete key: %v\n", err)
						} else {
							fmt.Printf("  ✓ Successfully deleted Chrome forcelist key\n")
							// Remove from newState
							delete(newState.Subkeys, forcelistKeyPath)
							// Remove all values under this key from newState
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
			
			// Check if this is a Firefox ExtensionSettings value
			if isFirefoxExtensionSettings(name) {
				fmt.Printf("  ⚠️  DETECTED Firefox ExtensionSettings VALUE - PROCESSING...\n")
				
				if !isAdmin() {
					fmt.Printf("  ❌ Insufficient privileges. Run as Administrator.\n")
				} else {
					// Extract extension ID from the path
					extensionID := extractFirefoxExtensionID(name)
					if extensionID != "" {
						fmt.Printf("  🔍 Extracted Firefox extension ID: %s\n", extensionID)
						
						// Check if this is an install/force_install policy
						if contains(name, "installation_mode") && 
						   (newVal.Data == "force_installed" || newVal.Data == "normal_installed") {
							fmt.Printf("  📝 Blocking Firefox extension: %s\n", extensionID)
							err := blockFirefoxExtension(keyPath, extensionID)
							if err != nil {
								fmt.Printf("  ⚠️  Failed to block extension: %v\n", err)
							}
							
							// Delete the extension's install policy
							// Find the extension's key path (parent of installation_mode)
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
								err = deleteRegistryKeyRecursive(keyPath, extensionKeyPath)
								if err != nil {
									fmt.Printf("  ❌ Failed to delete key: %v\n", err)
								} else {
									fmt.Printf("  ✓ Successfully deleted Firefox install policy\n")
									// Remove from newState
									delete(newState.Subkeys, extensionKeyPath)
									// Remove all values under this key from newState
									for valName := range newState.Values {
										if len(valName) > len(extensionKeyPath) && 
										   valName[:len(extensionKeyPath)] == extensionKeyPath {
											delete(newState.Values, valName)
										}
									}
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

func contains(s, substr string) bool {
	return containsIgnoreCase(s, substr)
}

func indexOf(s, substr string) int {
	lowerS := strings.ToLower(s)
	lowerSubstr := strings.ToLower(substr)
	return strings.Index(lowerS, lowerSubstr)
}

func extractExtensionIDFromValue(value string) string {
	// Extension ID is the string before the first ';'
	idx := strings.Index(value, ";")
	if idx >= 0 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}

func readKeyValues(baseKeyPath, relativePath string) (map[string]string, error) {
	fullPath := baseKeyPath
	if relativePath != "" {
		fullPath = baseKeyPath + "\\" + relativePath
	}

	keyPtr, err := syscall.UTF16PtrFromString(fullPath)
	if err != nil {
		return nil, fmt.Errorf("error converting key path: %v", err)
	}

	var hKey windows.Handle
	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, keyPtr, 0, windows.KEY_READ, &hKey)
	if err != nil {
		return nil, fmt.Errorf("error opening key: %v", err)
	}
	defer windows.RegCloseKey(hKey)

	values := make(map[string]string)
	var index uint32
	for {
		nameBuf := getNameBuffer()
		nameLen := uint32(len(*nameBuf))
		var valueType uint32
		dataBuf := getDataBuffer()
		dataLen := uint32(len(*dataBuf))

		ret, _, _ := regEnumValueW.Call(
			uintptr(hKey),
			uintptr(index),
			uintptr(unsafe.Pointer(&(*nameBuf)[0])),
			uintptr(unsafe.Pointer(&nameLen)),
			0,
			uintptr(unsafe.Pointer(&valueType)),
			uintptr(unsafe.Pointer(&(*dataBuf)[0])),
			uintptr(unsafe.Pointer(&dataLen)),
		)

		if ret == uintptr(windows.ERROR_NO_MORE_ITEMS) {
			putNameBuffer(nameBuf)
			putDataBuffer(dataBuf)
			break
		}
		if ret != 0 {
			putNameBuffer(nameBuf)
			putDataBuffer(dataBuf)
			return nil, fmt.Errorf("error enumerating values: error code %d", ret)
		}

		valueName := syscall.UTF16ToString((*nameBuf)[:nameLen])
		valueData := formatRegValue(valueType, (*dataBuf)[:dataLen])
		putNameBuffer(nameBuf)
		putDataBuffer(dataBuf)
		values[valueName] = valueData
		index++
	}

	return values, nil
}

func getBlocklistKeyPath(forcelistPath string) string {
	// Replace "ExtensionInstallForcelist" with "ExtensionInstallBlocklist"
	return replacePathComponent(forcelistPath, "ExtensionInstallForcelist", "ExtensionInstallBlocklist")
}

func getAllowlistKeyPath(forcelistPath string) string {
	// Replace "ExtensionInstallForcelist" with "ExtensionInstallAllowlist"
	return replacePathComponent(forcelistPath, "ExtensionInstallForcelist", "ExtensionInstallAllowlist")
}

func removeFromAllowlist(baseKeyPath, allowlistPath, extensionID string) error {
	fullPath := baseKeyPath
	if allowlistPath != "" {
		fullPath = baseKeyPath + "\\" + allowlistPath
	}

	keyPtr, err := syscall.UTF16PtrFromString(fullPath)
	if err != nil {
		return fmt.Errorf("error converting key path: %v", err)
	}

	var hKey windows.Handle
	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, keyPtr, 0, windows.KEY_READ|windows.KEY_WRITE, &hKey)
	if err != nil {
		// Allowlist key doesn't exist, which is fine
		if err == windows.ERROR_FILE_NOT_FOUND {
			return nil
		}
		return fmt.Errorf("error opening allowlist key: %v", err)
	}
	defer windows.RegCloseKey(hKey)

	// Read existing values to find matching extension IDs
	existingValues, err := readKeyValues(baseKeyPath, allowlistPath)
	if err != nil {
		return fmt.Errorf("error reading allowlist values: %v", err)
	}

	// Find and delete values containing this extension ID
	found := false
	for valueName, valueData := range existingValues {
		// Check if this value contains our extension ID
		checkID := extractExtensionIDFromValue(valueData)
		if checkID == extensionID {
			found = true
			fmt.Printf("  🔍 Found in allowlist at index %s\n", valueName)
			
			// Delete this value
			valueNamePtr, _ := syscall.UTF16PtrFromString(valueName)
			ret, _, _ := regDeleteValueW.Call(
				uintptr(hKey),
				uintptr(unsafe.Pointer(valueNamePtr)),
			)
			if ret != 0 {
				return fmt.Errorf("error deleting allowlist value: error code %d", ret)
			}
			fmt.Printf("  ✓ Removed %s from allowlist\n", extensionID)
		}
	}

	if !found {
		fmt.Printf("  ℹ️  Extension ID %s not found in allowlist\n", extensionID)
	}

	return nil
}

func isFirefoxExtensionSettings(path string) bool {
	return contains(path, "Mozilla\\Firefox\\ExtensionSettings") || 
	       contains(path, "Firefox\\ExtensionSettings")
}

func isChromeExtensionForcelist(path string) bool {
	return contains(path, "ExtensionInstallForcelist")
}

func extractFirefoxExtensionID(valuePath string) string {
	// For Firefox, the extension ID is typically in the path itself
	// Path format: Mozilla\Firefox\ExtensionSettings\{extension-id}\installation_mode
	parts := splitPath(valuePath)
	
	// Find ExtensionSettings and get the next part (extension ID)
	for i := 0; i < len(parts); i++ {
		if parts[i] == "ExtensionSettings" && i+1 < len(parts) {
			extID := parts[i+1]
			// Extension IDs for Firefox are typically {guid} format or name@domain
			if len(extID) > 0 && (extID[0] == '{' || containsIgnoreCase(extID, "@")) {
				return extID
			}
		}
	}
	return ""
}

func getFirefoxBlocklistPath(extensionID string) string {
	// Firefox blocklist path: Mozilla\Firefox\ExtensionSettings\{extension-id}\installation_mode
	return "Mozilla\\Firefox\\ExtensionSettings\\" + extensionID
}

func blockFirefoxExtension(baseKeyPath, extensionID string) error {
	blocklistPath := getFirefoxBlocklistPath(extensionID)
	fullPath := baseKeyPath
	if blocklistPath != "" {
		fullPath = baseKeyPath + "\\" + blocklistPath
	}

	keyPtr, err := syscall.UTF16PtrFromString(fullPath)
	if err != nil {
		return fmt.Errorf("error converting key path: %v", err)
	}

	var hKey windows.Handle
	var disposition uint32
	
	// Create the key for this extension
	ret, _, _ := regCreateKeyExW.Call(
		uintptr(windows.HKEY_LOCAL_MACHINE),
		uintptr(unsafe.Pointer(keyPtr)),
		0,
		0,
		0,
		uintptr(windows.KEY_READ|windows.KEY_WRITE),
		0,
		uintptr(unsafe.Pointer(&hKey)),
		uintptr(unsafe.Pointer(&disposition)),
	)
	
	if ret != 0 {
		return fmt.Errorf("error creating blocklist key: error code %d", ret)
	}
	defer windows.RegCloseKey(hKey)

	// Set installation_mode to "blocked"
	valueNamePtr, _ := syscall.UTF16PtrFromString("installation_mode")
	valueDataUTF16, _ := syscall.UTF16FromString("blocked")
	dataSize := uint32(len(valueDataUTF16) * 2)

	ret, _, _ = regSetValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(valueNamePtr)),
		0,
		uintptr(windows.REG_SZ),
		uintptr(unsafe.Pointer(&valueDataUTF16[0])),
		uintptr(dataSize),
	)

	if ret != 0 {
		return fmt.Errorf("error setting installation_mode: error code %d", ret)
	}

	fmt.Printf("  ✓ Blocked Firefox extension: %s\n", extensionID)
	return nil
}

func addToBlocklist(baseKeyPath, blocklistPath, extensionID string) error {
	fullPath := baseKeyPath
	if blocklistPath != "" {
		fullPath = baseKeyPath + "\\" + blocklistPath
	}

	keyPtr, err := syscall.UTF16PtrFromString(fullPath)
	if err != nil {
		return fmt.Errorf("error converting key path: %v", err)
	}

	var hKey windows.Handle
	var disposition uint32
	
	// Try to open or create the key
	ret, _, _ := regCreateKeyExW.Call(
		uintptr(windows.HKEY_LOCAL_MACHINE),
		uintptr(unsafe.Pointer(keyPtr)),
		0,
		0,
		0,
		uintptr(windows.KEY_READ|windows.KEY_WRITE),
		0,
		uintptr(unsafe.Pointer(&hKey)),
		uintptr(unsafe.Pointer(&disposition)),
	)
	
	if ret != 0 {
		return fmt.Errorf("error creating/opening blocklist key: error code %d", ret)
	}
	defer windows.RegCloseKey(hKey)

	// Read existing values to find the next available index
	existingValues, _ := readKeyValues(baseKeyPath, blocklistPath)
	
	// Check if extension ID already exists
	for _, value := range existingValues {
		if value == extensionID {
			fmt.Printf("  ℹ️  Extension ID %s already in blocklist\n", extensionID)
			return nil
		}
	}

	// Find the next available numeric index
	nextIndex := 1
	for {
		indexStr := fmt.Sprintf("%d", nextIndex)
		if _, exists := existingValues[indexStr]; !exists {
			break
		}
		nextIndex++
	}

	// Add the extension ID to the blocklist
	indexName := fmt.Sprintf("%d", nextIndex)
	indexNamePtr, _ := syscall.UTF16PtrFromString(indexName)
	
	// Convert extension ID to UTF-16
	extensionIDUTF16, _ := syscall.UTF16FromString(extensionID)
	dataSize := uint32(len(extensionIDUTF16) * 2)

	ret, _, _ = regSetValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(indexNamePtr)),
		0,
		uintptr(windows.REG_SZ),
		uintptr(unsafe.Pointer(&extensionIDUTF16[0])),
		uintptr(dataSize),
	)

	if ret != 0 {
		return fmt.Errorf("error setting blocklist value: error code %d", ret)
	}

	fmt.Printf("  ✓ Added extension ID %s to blocklist at index %s\n", extensionID, indexName)
	return nil
}

func removeExtensionSettingsForID(baseKeyPath, extensionID string, state *RegState) {
	fmt.Printf("  🔍 Checking for extension settings: %s\n", extensionID)
	fmt.Printf("  📊 Scanning %d subkeys and %d values...\n", len(state.Subkeys), len(state.Values))
	
	// Find and remove settings at: *\*\3rdparty\extensions\{extension-id}
	var settingsToRemove map[string]bool
	
	// Use index for O(1) lookup if available
	if extensionIndex != nil {
		paths := extensionIndex.GetPaths(extensionID)
		settingsToRemove = make(map[string]bool, len(paths))
		for _, p := range paths {
			fmt.Printf("  🎯 Found (indexed): %s\n", p)
			settingsToRemove[p] = true
		}
	} else {
		// Fallback: scan all keys
		settingsToRemove = make(map[string]bool)
		
		// Check subkeys
		for subkeyPath := range state.Subkeys {
			if containsIgnoreCase(subkeyPath, "3rdparty") &&
			   containsIgnoreCase(subkeyPath, "extensions") &&
			   containsIgnoreCase(subkeyPath, extensionID) {
				fmt.Printf("  🎯 Found matching subkey: %s\n", subkeyPath)
				settingsToRemove[subkeyPath] = true
			}
		}
		
		// Check values
		for valuePath := range state.Values {
			if containsIgnoreCase(valuePath, "3rdparty") &&
			   containsIgnoreCase(valuePath, "extensions") &&
			   containsIgnoreCase(valuePath, extensionID) {
				fmt.Printf("  🎯 Found matching value: %s\n", valuePath)
				
				// Extract extension settings path (up to extension ID)
				parts := splitPath(valuePath)
				for i := 0; i < len(parts); i++ {
					if parts[i] == extensionID {
						settingsPath := strings.Join(parts[:i+1], "\\")
						fmt.Printf("  📍 Extracted settings path: %s\n", settingsPath)
						settingsToRemove[settingsPath] = true
						break
					}
				}
			}
		}
	}
	
	if len(settingsToRemove) == 0 {
		fmt.Printf("  ℹ️  No extension settings found for %s\n", extensionID)
		return
	}
	
	fmt.Printf("  🗑️  Found %d setting path(s) to remove\n", len(settingsToRemove))
	
	// Delete the settings
	for settingsPath := range settingsToRemove {
		fmt.Printf("  🗑️  Deleting extension settings: %s\n", settingsPath)
		err := deleteRegistryKeyRecursive(baseKeyPath, settingsPath)
		if err != nil {
			fmt.Printf("  ⚠️  Failed to delete settings: %v\n", err)
		} else {
			fmt.Printf("  ✓ Successfully removed settings for %s\n", extensionID)
			// Remove from state
			delete(state.Subkeys, settingsPath)
			// Remove all child keys and values using optimized cleanup
			removeSubtreeFromState(state, settingsPath)
			// Update index
			if extensionIndex != nil {
				extensionIndex.Remove(extensionID)
			}
		}
	}
}

func removeSubtreeFromState(state *RegState, prefix string) {
	// Efficiently remove all keys/values under prefix
	for keyPath := range state.Subkeys {
		if strings.HasPrefix(keyPath, prefix) && len(keyPath) > len(prefix) {
			delete(state.Subkeys, keyPath)
		}
	}
	for valName := range state.Values {
		if strings.HasPrefix(valName, prefix) && len(valName) > len(prefix) {
			delete(state.Values, valName)
		}
	}
}

func processExistingPolicies(keyPath string, state *RegState) {
	if !isAdmin() {
		fmt.Println("\n⚠️  Not running as Administrator - skipping existing policy processing")
		return
	}

	fmt.Println("\n========================================")
	fmt.Println("Checking for existing extension policies...")
	fmt.Println("========================================")
	
	hasExistingPolicies := false
	
	// Check for Chrome ExtensionInstallForcelist values
	for valuePath, value := range state.Values {
		if isChromeExtensionForcelist(valuePath) {
			hasExistingPolicies = true
			fmt.Printf("\n[EXISTING CHROME POLICY DETECTED]\n")
			fmt.Printf("Path: %s\n", valuePath)
			fmt.Printf("Value: %s\n", value.Data)
			
			// Extract extension ID
			extensionID := extractExtensionIDFromValue(value.Data)
			if extensionID != "" {
				fmt.Printf("🔍 Extension ID: %s\n", extensionID)
				
				// Get the key path (parent of the value)
				lastSlash := -1
				for i := len(valuePath) - 1; i >= 0; i-- {
					if valuePath[i] == '\\' {
						lastSlash = i
						break
					}
				}
				
				if lastSlash >= 0 {
					forcelistKeyPath := valuePath[:lastSlash]
					
					// Read ALL values from the forcelist key before processing
					allValues, err := readKeyValues(keyPath, forcelistKeyPath)
					if err != nil {
						fmt.Printf("⚠️  Could not read forcelist values: %v\n", err)
					} else {
						fmt.Printf("📋 Processing all extension IDs in forcelist...\n")
						
						blocklistKeyPath := getBlocklistKeyPath(forcelistKeyPath)
						allowlistKeyPath := getAllowlistKeyPath(forcelistKeyPath)
						
						// Process each extension ID
						for _, valueData := range allValues {
							extensionID := extractExtensionIDFromValue(valueData)
							if extensionID != "" {
								fmt.Printf("🔍 Extension ID: %s\n", extensionID)
								
								// Add to blocklist
								fmt.Printf("📝 Adding to Chrome blocklist: %s\n", blocklistKeyPath)
								err := addToBlocklist(keyPath, blocklistKeyPath, extensionID)
								if err != nil {
									fmt.Printf("⚠️  Failed to add to blocklist: %v\n", err)
								}
								
								// Remove from allowlist
								fmt.Printf("🔍 Checking Chrome allowlist: %s\n", allowlistKeyPath)
								err = removeFromAllowlist(keyPath, allowlistKeyPath, extensionID)
								if err != nil {
									fmt.Printf("⚠️  Failed to remove from allowlist: %v\n", err)
								}
								
								// Remove extension settings for this ID
								removeExtensionSettingsForID(keyPath, extensionID, state)
							}
						}
					}
					
					// Delete the forcelist key
					fmt.Printf("🗑️  Deleting Chrome forcelist key: %s\n", forcelistKeyPath)
					err = deleteRegistryKeyRecursive(keyPath, forcelistKeyPath)
					if err != nil {
						fmt.Printf("❌ Failed to delete key: %v\n", err)
					} else {
						fmt.Printf("✓ Successfully removed forcelist key\n")
						// Remove from state since we deleted it
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
		
		// Check for Firefox ExtensionSettings policies
		if isFirefoxExtensionSettings(valuePath) && contains(valuePath, "installation_mode") {
			if value.Data == "force_installed" || value.Data == "normal_installed" {
				hasExistingPolicies = true
				fmt.Printf("\n[EXISTING FIREFOX POLICY DETECTED]\n")
				fmt.Printf("Path: %s\n", valuePath)
				fmt.Printf("Value: %s\n", value.Data)
				
				// Extract extension ID
				extensionID := extractFirefoxExtensionID(valuePath)
				if extensionID != "" {
					fmt.Printf("🔍 Extension ID: %s\n", extensionID)
					
					// Block the extension
					fmt.Printf("📝 Blocking Firefox extension\n")
					err := blockFirefoxExtension(keyPath, extensionID)
					if err != nil {
						fmt.Printf("⚠️  Failed to block extension: %v\n", err)
					}
					
					// Delete the extension's key
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
						err = deleteRegistryKeyRecursive(keyPath, extensionKeyPath)
						if err != nil {
							fmt.Printf("❌ Failed to delete key: %v\n", err)
						} else {
							fmt.Printf("✓ Successfully removed install policy\n")
							// Remove from state since we deleted it
							delete(state.Subkeys, extensionKeyPath)
							for valName := range state.Values {
								if len(valName) > len(extensionKeyPath) && 
								   valName[:len(extensionKeyPath)] == extensionKeyPath {
									delete(state.Values, valName)
								}
							}
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

func cleanupAllowlists(keyPath string, state *RegState) {
	if !isAdmin() {
		return
	}

	fmt.Println("Checking for ExtensionInstallAllowlist keys...")
	
	allowlistsFound := false
	allowlistKeys := make(map[string]bool)
	
	// Find all allowlist keys
	for subkeyPath := range state.Subkeys {
		if contains(subkeyPath, "ExtensionInstallAllowlist") {
			allowlistsFound = true
			allowlistKeys[subkeyPath] = true
		}
	}
	
	if !allowlistsFound {
		fmt.Println("✓ No ExtensionInstallAllowlist keys found")
		return
	}
	
	// Delete each allowlist key
	for allowlistPath := range allowlistKeys {
		fmt.Printf("\n[REMOVING ALLOWLIST]\n")
		fmt.Printf("Path: %s\n", allowlistPath)
		
		values, err := readKeyValues(keyPath, allowlistPath)
		if err == nil && len(values) > 0 {
			fmt.Printf("Found %d extension(s) in allowlist:\n", len(values))
			for valueName, valueData := range values {
				extensionID := extractExtensionIDFromValue(valueData)
				if extensionID != "" {
					fmt.Printf("  - %s: %s\n", valueName, extensionID)
				}
			}
		}
		
		fmt.Printf("🗑️  Deleting allowlist key: %s\n", allowlistPath)
		err = deleteRegistryKeyRecursive(keyPath, allowlistPath)
		if err != nil {
			fmt.Printf("❌ Failed to delete allowlist: %v\n", err)
		} else {
			fmt.Printf("✓ Successfully deleted allowlist\n")
			// Remove from state
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

func getBlockedExtensionIDs(keyPath string, state *RegState) map[string]bool {
	blockedIDs := make(map[string]bool)
	
	fmt.Println("  📋 Scanning for blocked extension IDs...")
	
	// Find all blocklist keys and read their values
	for subkeyPath := range state.Subkeys {
		if contains(subkeyPath, "ExtensionInstallBlocklist") {
			fmt.Printf("  🔍 Found blocklist: %s\n", subkeyPath)
			values, err := readKeyValues(keyPath, subkeyPath)
			if err == nil {
				for valueName, valueData := range values {
					extensionID := extractExtensionIDFromValue(valueData)
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
	
	// Also check for Firefox blocked extensions
	for valuePath, value := range state.Values {
		if isFirefoxExtensionSettings(valuePath) && 
		   contains(valuePath, "installation_mode") && 
		   value.Data == "blocked" {
			extensionID := extractFirefoxExtensionID(valuePath)
			if extensionID != "" {
				fmt.Printf("  🦊 Firefox blocked: %s\n", extensionID)
				blockedIDs[extensionID] = true
			}
		}
	}
	
	return blockedIDs
}

func cleanupExtensionSettings(keyPath string, state *RegState) {
	if !isAdmin() {
		return
	}

	fmt.Println("\n========================================")
	fmt.Println("Cleaning up extension settings...")
	fmt.Println("========================================")
	fmt.Println("Checking for extension settings of blocked extensions...")
	fmt.Println("Note: This removes settings for ALL extensions in blocklists,")
	fmt.Println("      regardless of whether they were added via forcelist or manually.")
	
	// Get all blocked extension IDs from blocklists
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
	
	// Use removeExtensionSettingsForID for each blocked extension
	for extensionID := range blockedIDs {
		fmt.Printf("\n[CHECKING SETTINGS FOR BLOCKED EXTENSION]\n")
		fmt.Printf("Extension ID: %s\n", extensionID)
		removeExtensionSettingsForID(keyPath, extensionID, state)
	}
	
	fmt.Println("========================================\n")
}

func main() {
	// Check if running as administrator
	if !isAdmin() {
		fmt.Println("⚠️  WARNING: Not running as Administrator")
		fmt.Println("Registry deletion requires elevated privileges.")
		fmt.Print("Attempting to elevate permissions... ")
		
		err := elevatePrivileges()
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
			fmt.Println("\nPlease run this program as Administrator to enable key deletion.")
			fmt.Println("Press Enter to continue in read-only mode...")
			fmt.Scanln()
		} else {
			fmt.Println("✓ Relaunching with elevated privileges...")
			// Exit this instance as the elevated one is starting
			return
		}
	} else {
		fmt.Println("✓ Running with Administrator privileges")
	}

	// Open the registry key to monitor
	keyPath := `SOFTWARE\Policies`
	
	// Test if we can actually delete keys
	canDelete := canDeleteRegistryKey(keyPath)
	if !canDelete {
		fmt.Println("⚠️  WARNING: Insufficient permissions to delete registry keys")
		fmt.Println("Key deletion features will be disabled.")
	} else {
		fmt.Println("✓ Registry deletion permissions verified")
	}
	
	key, err := syscall.UTF16PtrFromString(keyPath)
	if err != nil {
		fmt.Println("Error converting key path:", err)
		return
	}

	var hKey windows.Handle
	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, key, 0, windows.KEY_NOTIFY|windows.KEY_READ|windows.DELETE, &hKey)
	if err != nil {
		fmt.Println("Error opening registry key:", err)
		return
	}
	defer windows.RegCloseKey(hKey)

	// Capture initial state
	fmt.Println("Capturing initial registry state...")
	previousState, err := captureRegistryState(hKey, keyPath)
	if err != nil {
		fmt.Println("Error capturing initial state:", err)
		return
	}
	fmt.Printf("Initial state: %d subkeys, %d values\n", len(previousState.Subkeys), len(previousState.Values))

	// Build extension path index for O(1) lookups
	fmt.Println("Building extension path index...")
	extensionIndex = NewExtensionPathIndex()
	extensionIndex.BuildFromState(previousState)
	fmt.Printf("Index built: tracking %d unique extension IDs\n", len(extensionIndex.pathsByExtID))

	// Process any existing extension policies at startup
	processExistingPolicies(keyPath, previousState)
	
	// Clean up any allowlists
	cleanupAllowlists(keyPath, previousState)
	
	// Clean up extension settings for blocked extensions
	cleanupExtensionSettings(keyPath, previousState)

	// Create an event for notifications
	event, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		fmt.Println("Error creating event:", err)
		return
	}
	defer windows.CloseHandle(event)

	// Start monitoring the registry key
	err = windows.RegNotifyChangeKeyValue(hKey, true, windows.REG_NOTIFY_CHANGE_NAME|windows.REG_NOTIFY_CHANGE_LAST_SET, event, true)
	if err != nil {
		fmt.Println("Error setting up registry notification:", err)
		return
	}

	fmt.Println("Monitoring registry changes...")

	for {
		// Wait for a change notification
		status, err := windows.WaitForSingleObject(event, windows.INFINITE)
		if err != nil {
			fmt.Println("Error waiting for event:", err)
			return
		}

		if status == windows.WAIT_OBJECT_0 {
			// Capture new state
			newState, err := captureRegistryState(hKey, keyPath)
			if err != nil {
				fmt.Println("Error capturing new state:", err)
			} else {
				// Print the differences
				printDiff(previousState, newState, keyPath)
				// Update previous state
				previousState = newState
			}

			// Re-arm the notification
			err = windows.RegNotifyChangeKeyValue(hKey, true, windows.REG_NOTIFY_CHANGE_NAME|windows.REG_NOTIFY_CHANGE_LAST_SET, event, true)
			if err != nil {
				fmt.Println("Error re-arming registry notification:", err)
				return
			}
		}
	}
}
