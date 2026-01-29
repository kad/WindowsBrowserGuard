package admin

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32       = syscall.NewLazyDLL("shell32.dll")
	shellExecuteW = shell32.NewProc("ShellExecuteW")

	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getModuleFileNameW = kernel32.NewProc("GetModuleFileNameW")
)

// IsAdmin checks if the current process is running with administrator privileges
func IsAdmin() bool {
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

// GetExecutablePath returns the full path to the current executable
func GetExecutablePath() (string, error) {
	buf := make([]uint16, windows.MAX_PATH)
	ret, _, _ := getModuleFileNameW.Call(0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ret == 0 {
		return "", fmt.Errorf("failed to get executable path")
	}
	return syscall.UTF16ToString(buf), nil
}

// ElevatePrivileges attempts to restart the current process with elevated privileges
func ElevatePrivileges() error {
	exePath, err := GetExecutablePath()
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

// CanDeleteRegistryKey checks if the current process has permission to delete the specified registry key
func CanDeleteRegistryKey(keyPath string) bool {
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

// CheckAdminAndElevate checks admin status and handles elevation or dry-run mode
// Returns true if the process has write permissions, false otherwise
func CheckAdminAndElevate(dryRun bool) bool {
	if dryRun {
		fmt.Println("🔍 DRY-RUN MODE: Running in read-only mode")
		fmt.Println("   No changes will be made to the registry")
		fmt.Println("   All write/delete operations will be simulated\n")
		return false
	}

	if !IsAdmin() {
		fmt.Println("⚠️  WARNING: Not running as Administrator")
		fmt.Println("Registry deletion requires elevated privileges.")
		fmt.Print("Attempting to elevate permissions... ")

		err := ElevatePrivileges()
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
