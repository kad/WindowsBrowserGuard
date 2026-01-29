package main

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

func main() {
	printRegistryKeyValues(`SOFTWARE\Policies\Google\Chrome`)
}

/*
import (
        "fmt"
        "io/ioutil"
        "os"
        "path"

        "golang.org/x/sys/windows/registry"
)

    k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\YourPath`, registry.WRITE)
        if err != nil {
            log.Error(err)
            return err
        }
        defer k.Close()

        if err = k.SetStringValue(key, val); err != nil {
            log.Error(err)
            return
        }
*/

func printRegistryKeyValues(path string) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.READ)
	if err != nil {
		fmt.Printf("Error opening registry key %s: %s\n", path, err)
		return
	}
	defer key.Close()

	names, err := key.ReadValueNames(0)
	if err != nil {
		fmt.Printf("Error reading value names for path %s: %s\n", path, err)
		return
	}

	for _, name := range names {
		val, valType, err := key.GetValue(name, nil)
		if err != nil {
			fmt.Printf("Error reading value for key %s\\%s: %s\n", path, name, err)
			continue
		}
		if valType == registry.SZ {
			valStr, _, err := key.GetStringValue(name)
			if err != nil {
				fmt.Println("String value, but can't retrive", name, err)
			}
			fmt.Printf("Path: %s\\%s, Type: %d, Value: %#v\n", path, name, valType, valStr)
		} else {
			fmt.Printf("Path: %s\\%s, Type: %d, Value: %#v\n", path, name, valType, val)
		}
	}

	subKeys, err := key.ReadSubKeyNames(0)
	if err != nil {
		fmt.Printf("Error reading subkeys for path %s: %s\n", path, err)
		return
	}

	for _, subKey := range subKeys {
		subKeyPath := path + "\\" + subKey
		printRegistryKeyValues(subKeyPath)
	}
}
