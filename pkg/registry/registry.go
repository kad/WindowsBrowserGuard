package registry

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/kad/WindowsBrowserGuard/pkg/buffers"
	"github.com/kad/WindowsBrowserGuard/pkg/detection"
	"github.com/kad/WindowsBrowserGuard/pkg/pathutils"
	"golang.org/x/sys/windows"
)

var (
	advapi32         = syscall.NewLazyDLL("advapi32.dll")
	regEnumValueW    = advapi32.NewProc("RegEnumValueW")
	regQueryInfoKeyW = advapi32.NewProc("RegQueryInfoKeyW")
	regDeleteKeyW    = advapi32.NewProc("RegDeleteKeyW")
	regSetValueExW   = advapi32.NewProc("RegSetValueExW")
	regCreateKeyExW  = advapi32.NewProc("RegCreateKeyExW")
	regDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
)

type RegValue struct {
	Name string
	Type uint32
	Data string
}

type RegState struct {
	Subkeys map[string]bool
	Values  map[string]RegValue
}

type PerfMetrics struct {
	StartupTime      time.Duration
	IndexBuildTime   time.Duration
	InitialScanKeys  int
	InitialScanDepth int
}

const MaxRegistryDepth = 8

type ExtensionPathIndex struct {
	pathsByExtID map[string][]string
	mu           sync.RWMutex
}

func NewExtensionPathIndex() *ExtensionPathIndex {
	return &ExtensionPathIndex{
		pathsByExtID: make(map[string][]string),
	}
}

func (idx *ExtensionPathIndex) BuildFromState(state *RegState) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.pathsByExtID = make(map[string][]string)

	for subkeyPath := range state.Subkeys {
		if pathutils.ContainsIgnoreCase(subkeyPath, "3rdparty") && pathutils.ContainsIgnoreCase(subkeyPath, "extensions") {
			extID := pathutils.ExtractExtensionIDFromPath(subkeyPath, "extensions")
			if extID != "" {
				idx.pathsByExtID[extID] = append(idx.pathsByExtID[extID], subkeyPath)
			}
		}
	}

	for valuePath := range state.Values {
		if pathutils.ContainsIgnoreCase(valuePath, "3rdparty") && pathutils.ContainsIgnoreCase(valuePath, "extensions") {
			extID := pathutils.ExtractExtensionIDFromPath(valuePath, "extensions")
			if extID != "" {
				if parent, ok := pathutils.GetParentPath(valuePath); ok {
					paths := idx.pathsByExtID[extID]
					found := false
					for _, p := range paths {
						if p == parent {
							found = true
							break
						}
					}
					if !found {
						idx.pathsByExtID[extID] = append(idx.pathsByExtID[extID], parent)
					}
				}
			}
		}
	}
}

func (idx *ExtensionPathIndex) GetPaths(extensionID string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	paths := idx.pathsByExtID[extensionID]
	if paths == nil {
		return []string{}
	}

	result := make([]string, len(paths))
	copy(result, paths)
	return result
}

func (idx *ExtensionPathIndex) Remove(extensionID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.pathsByExtID, extensionID)
}

func (idx *ExtensionPathIndex) GetCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.pathsByExtID)
}

func CaptureKeyRecursive(hKey windows.Handle, relativePath string, state *RegState, depth int) error {
	const maxDepth = 8

	if depth > maxDepth {
		return nil
	}

	var index uint32
	subkeyNames := []string{}
	for {
		nameBuf := buffers.GetNameBuffer()
		nameLen := uint32(len(*nameBuf))
		err := windows.RegEnumKeyEx(hKey, index, &(*nameBuf)[0], &nameLen, nil, nil, nil, nil)
		if err == windows.ERROR_NO_MORE_ITEMS {
			buffers.PutNameBuffer(nameBuf)
			break
		}
		if err != nil {
			buffers.PutNameBuffer(nameBuf)
			return fmt.Errorf("error enumerating subkeys: %v", err)
		}
		subkeyName := syscall.UTF16ToString((*nameBuf)[:nameLen])
		buffers.PutNameBuffer(nameBuf)

		fullPath := pathutils.BuildPath(relativePath, subkeyName)

		state.Subkeys[fullPath] = true
		subkeyNames = append(subkeyNames, subkeyName)

		index++
	}

	index = 0
	for {
		nameBuf := buffers.GetLargeNameBuffer()
		nameLen := uint32(len(*nameBuf))
		var valueType uint32
		dataBuf := buffers.GetDataBuffer()
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
			buffers.PutLargeNameBuffer(nameBuf)
			buffers.PutDataBuffer(dataBuf)
			break
		}
		if ret != 0 {
			buffers.PutLargeNameBuffer(nameBuf)
			buffers.PutDataBuffer(dataBuf)
			return fmt.Errorf("error enumerating values: error code %d", ret)
		}

		valueName := syscall.UTF16ToString((*nameBuf)[:nameLen])
		valueData := detection.FormatRegValue(valueType, (*dataBuf)[:dataLen])
		buffers.PutLargeNameBuffer(nameBuf)
		buffers.PutDataBuffer(dataBuf)

		fullPath := pathutils.BuildPath(relativePath, valueName)

		state.Values[fullPath] = RegValue{
			Name: fullPath,
			Type: valueType,
			Data: valueData,
		}

		index++
	}

	for _, subkeyName := range subkeyNames {
		subkeyPtr, err := syscall.UTF16PtrFromString(subkeyName)
		if err != nil {
			continue
		}

		var hSubKey windows.Handle
		err = windows.RegOpenKeyEx(hKey, subkeyPtr, 0, windows.KEY_READ, &hSubKey)
		if err != nil {
			continue
		}

		fullPath := pathutils.BuildPath(relativePath, subkeyName)

		err = CaptureKeyRecursive(hSubKey, fullPath, state, depth+1)
		_ = windows.RegCloseKey(hSubKey)

		if err != nil {
			return err
		}
	}

	return nil
}

func ReadKeyValues(baseKeyPath, relativePath string) (map[string]string, error) {
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
	defer func() { _ = windows.RegCloseKey(hKey) }()

	values := make(map[string]string)
	var index uint32
	for {
		nameBuf := buffers.GetLargeNameBuffer()
		nameLen := uint32(len(*nameBuf))
		var valueType uint32
		dataBuf := buffers.GetDataBuffer()
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
			buffers.PutLargeNameBuffer(nameBuf)
			buffers.PutDataBuffer(dataBuf)
			break
		}
		if ret != 0 {
			buffers.PutLargeNameBuffer(nameBuf)
			buffers.PutDataBuffer(dataBuf)
			return nil, fmt.Errorf("error enumerating values: error code %d", ret)
		}

		valueName := syscall.UTF16ToString((*nameBuf)[:nameLen])
		valueData := detection.FormatRegValue(valueType, (*dataBuf)[:dataLen])
		buffers.PutLargeNameBuffer(nameBuf)
		buffers.PutDataBuffer(dataBuf)
		values[valueName] = valueData
		index++
	}

	return values, nil
}

func DeleteRegistryKey(baseKeyPath, relativePath string, dryRun bool) error {
	fullPath := baseKeyPath
	if relativePath != "" {
		fullPath = baseKeyPath + "\\" + relativePath
	}

	if dryRun {
		fmt.Printf("  [DRY-RUN] Would delete registry key: HKLM\\%s\n", fullPath)
		return nil
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

	ret, _, _ := regDeleteKeyW.Call(uintptr(hKey), uintptr(0))
	if ret != 0 {
		return fmt.Errorf("error deleting key: error code %d", ret)
	}

	return nil
}

func DeleteRegistryKeyRecursive(baseKeyPath, relativePath string, dryRun bool) error {
	fullPath := baseKeyPath
	if relativePath != "" {
		fullPath = baseKeyPath + "\\" + relativePath
	}

	if dryRun {
		fmt.Printf("  [DRY-RUN] Would recursively delete registry key: HKLM\\%s\n", fullPath)
		return nil
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
	defer func() { _ = windows.RegCloseKey(hKey) }()

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

		subkeyPath := relativePath
		if subkeyPath != "" {
			subkeyPath += "\\"
		}
		subkeyPath += subkeyName

		err = DeleteRegistryKeyRecursive(baseKeyPath, subkeyPath, dryRun)
		if err != nil {
			fmt.Printf("Warning: failed to delete subkey %s: %v\n", subkeyPath, err)
		}
	}

	_ = windows.RegCloseKey(hKey)

	parentPath := baseKeyPath
	if relativePath != "" {
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
		defer func() { _ = windows.RegCloseKey(hParentKey) }()

		ret, _, _ := regDeleteKeyW.Call(uintptr(hParentKey), uintptr(unsafe.Pointer(keyPtr)))
		if ret != 0 {
			return fmt.Errorf("error deleting key: error code %d", ret)
		}
	}

	return nil
}

func AddToBlocklist(baseKeyPath, blocklistPath, extensionID string, dryRun bool) error {
	fullPath := baseKeyPath
	if blocklistPath != "" {
		fullPath = baseKeyPath + "\\" + blocklistPath
	}

	if dryRun {
		fmt.Printf("  [DRY-RUN] Would add to blocklist: HKLM\\%s\n", fullPath)
		fmt.Printf("  [DRY-RUN]   Extension ID: %s\n", extensionID)
		return nil
	}

	keyPtr, err := syscall.UTF16PtrFromString(fullPath)
	if err != nil {
		return fmt.Errorf("error converting key path: %v", err)
	}

	var hKey windows.Handle
	var disposition uint32

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

	existingValues, _ := ReadKeyValues(baseKeyPath, blocklistPath)

	for _, value := range existingValues {
		if value == extensionID {
			fmt.Printf("  ℹ️  Extension ID %s already in blocklist\n", extensionID)
			return nil
		}
	}

	nextIndex := 1
	for {
		indexStr := fmt.Sprintf("%d", nextIndex)
		if _, exists := existingValues[indexStr]; !exists {
			break
		}
		nextIndex++
	}

	indexName := fmt.Sprintf("%d", nextIndex)
	indexNamePtr, _ := syscall.UTF16PtrFromString(indexName)

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

func BlockFirefoxExtension(baseKeyPath, extensionID string, dryRun bool) error {
	if dryRun {
		fmt.Printf("  [DRY-RUN] Would block Firefox extension: %s\n", extensionID)
		return nil
	}

	blocklistPath := detection.GetFirefoxBlocklistPath(extensionID)
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

func RemoveFromAllowlist(baseKeyPath, allowlistPath, extensionID string, dryRun bool) error {
	if dryRun {
		fmt.Printf("  [DRY-RUN] Would remove from allowlist: %s\n", extensionID)
		return nil
	}

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
		if err == windows.ERROR_FILE_NOT_FOUND {
			return nil
		}
		return fmt.Errorf("error opening allowlist key: %v", err)
	}
	defer windows.RegCloseKey(hKey)

	existingValues, err := ReadKeyValues(baseKeyPath, allowlistPath)
	if err != nil {
		return fmt.Errorf("error reading allowlist values: %v", err)
	}

	found := false
	for valueName, valueData := range existingValues {
		checkID := detection.ExtractExtensionIDFromValue(valueData)
		if checkID == extensionID {
			found = true
			fmt.Printf("  🔍 Found in allowlist at index %s\n", valueName)

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

func RemoveExtensionSettingsForID(baseKeyPath, extensionID string, dryRun bool, state *RegState, extensionIndex *ExtensionPathIndex) {
	fmt.Printf("  🔍 Checking for extension settings: %s\n", extensionID)
	fmt.Printf("  📊 Scanning %d subkeys and %d values...\n", len(state.Subkeys), len(state.Values))

	var settingsToRemove map[string]bool

	if extensionIndex != nil {
		paths := extensionIndex.GetPaths(extensionID)
		settingsToRemove = make(map[string]bool, len(paths))
		for _, p := range paths {
			fmt.Printf("  🎯 Found (indexed): %s\n", p)
			settingsToRemove[p] = true
		}
	} else {
		settingsToRemove = make(map[string]bool)

		for subkeyPath := range state.Subkeys {
			if pathutils.ContainsIgnoreCase(subkeyPath, "3rdparty") &&
				pathutils.ContainsIgnoreCase(subkeyPath, "extensions") &&
				pathutils.ContainsIgnoreCase(subkeyPath, extensionID) {
				fmt.Printf("  🎯 Found matching subkey: %s\n", subkeyPath)
				settingsToRemove[subkeyPath] = true
			}
		}

		for valuePath := range state.Values {
			if pathutils.ContainsIgnoreCase(valuePath, "3rdparty") &&
				pathutils.ContainsIgnoreCase(valuePath, "extensions") &&
				pathutils.ContainsIgnoreCase(valuePath, extensionID) {
				fmt.Printf("  🎯 Found matching value: %s\n", valuePath)

				parts := pathutils.SplitPath(valuePath)
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

	for settingsPath := range settingsToRemove {
		fmt.Printf("  🗑️  Deleting extension settings: %s\n", settingsPath)
		err := DeleteRegistryKeyRecursive(baseKeyPath, settingsPath, dryRun)
		if err != nil {
			fmt.Printf("  ⚠️  Failed to delete settings: %v\n", err)
		} else {
			fmt.Printf("  ✓ Successfully removed settings for %s\n", extensionID)
			delete(state.Subkeys, settingsPath)
			RemoveSubtreeFromState(state, settingsPath)
			if extensionIndex != nil {
				extensionIndex.Remove(extensionID)
			}
		}
	}
}

func RemoveSubtreeFromState(state *RegState, prefix string) {
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
