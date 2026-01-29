package main

import (
	"sync"
)

// ============================================================================
// BUFFER POOL - Reusable buffers for registry operations
// ============================================================================

var (
	// Pool for UTF-16 name buffers (typically 256 uint16)
	nameBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]uint16, 256)
			return &buf
		},
	}
	
	// Pool for data buffers (start with 4KB, can grow)
	dataBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 4096)
			return &buf
		},
	}
	
	// Pool for large data buffers (16KB for large values)
	largeDataBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 16384)
			return &buf
		},
	}
)

// getNameBuffer gets a name buffer from the pool
func getNameBuffer() *[]uint16 {
	return nameBufferPool.Get().(*[]uint16)
}

// putNameBuffer returns a name buffer to the pool
func putNameBuffer(buf *[]uint16) {
	// Clear sensitive data before returning to pool
	for i := range *buf {
		(*buf)[i] = 0
	}
	nameBufferPool.Put(buf)
}

// getDataBuffer gets a data buffer from the pool
func getDataBuffer() *[]byte {
	return dataBufferPool.Get().(*[]byte)
}

// putDataBuffer returns a data buffer to the pool
func putDataBuffer(buf *[]byte) {
	// Clear sensitive data before returning to pool
	for i := range *buf {
		(*buf)[i] = 0
	}
	dataBufferPool.Put(buf)
}

// getLargeDataBuffer gets a large data buffer from the pool
func getLargeDataBuffer() *[]byte {
	return largeDataBufferPool.Get().(*[]byte)
}

// putLargeDataBuffer returns a large data buffer to the pool
func putLargeDataBuffer(buf *[]byte) {
	// Clear sensitive data before returning to pool
	for i := range *buf {
		(*buf)[i] = 0
	}
	largeDataBufferPool.Put(buf)
}

// ============================================================================
// EXTENSION PATH INDEX - Fast O(1) lookup for extension settings
// ============================================================================

type ExtensionPathIndex struct {
	// Maps extension ID to list of registry paths containing settings
	pathsByExtID map[string][]string
	mu           sync.RWMutex
}

func NewExtensionPathIndex() *ExtensionPathIndex {
	return &ExtensionPathIndex{
		pathsByExtID: make(map[string][]string),
	}
}

// BuildFromState scans registry state once and builds index
func (idx *ExtensionPathIndex) BuildFromState(state *RegState) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	// Clear existing index
	idx.pathsByExtID = make(map[string][]string)
	
	// Index subkeys containing extension IDs
	for subkeyPath := range state.Subkeys {
		if containsIgnoreCase(subkeyPath, "3rdparty") && containsIgnoreCase(subkeyPath, "extensions") {
			extID := extractExtensionIDFromPath(subkeyPath, "extensions")
			if extID != "" {
				idx.pathsByExtID[extID] = append(idx.pathsByExtID[extID], subkeyPath)
			}
		}
	}
	
	// Index values containing extension IDs
	for valuePath := range state.Values {
		if containsIgnoreCase(valuePath, "3rdparty") && containsIgnoreCase(valuePath, "extensions") {
			extID := extractExtensionIDFromPath(valuePath, "extensions")
			if extID != "" {
				// Get the key path (parent of value)
				if parent, ok := getParentPath(valuePath); ok {
					// Check if not already in list
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

// GetPaths returns all registry paths for an extension ID
func (idx *ExtensionPathIndex) GetPaths(extensionID string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	paths := idx.pathsByExtID[extensionID]
	if paths == nil {
		return []string{}
	}
	
	// Return copy to avoid concurrent modification
	result := make([]string, len(paths))
	copy(result, paths)
	return result
}

// Remove removes an extension from the index
func (idx *ExtensionPathIndex) Remove(extensionID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.pathsByExtID, extensionID)
}

// ============================================================================
// PREFIX TREE - Efficient hierarchical key storage
// ============================================================================

type PrefixTreeNode struct {
	children map[string]*PrefixTreeNode
	isKey    bool
	value    *RegValue // For values (leaf nodes)
}

type PrefixTree struct {
	root *PrefixTreeNode
	mu   sync.RWMutex
}

func NewPrefixTree() *PrefixTree {
	return &PrefixTree{
		root: &PrefixTreeNode{
			children: make(map[string]*PrefixTreeNode),
		},
	}
}

// Insert adds a path to the tree
func (pt *PrefixTree) Insert(path string, value *RegValue) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	
	parts := splitPath(path)
	node := pt.root
	
	for _, part := range parts {
		if node.children[part] == nil {
			node.children[part] = &PrefixTreeNode{
				children: make(map[string]*PrefixTreeNode),
			}
		}
		node = node.children[part]
	}
	
	node.isKey = true
	if value != nil {
		node.value = value
	}
}

// DeletePrefix removes all paths under a prefix
func (pt *PrefixTree) DeletePrefix(prefix string) int {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	
	parts := splitPath(prefix)
	if len(parts) == 0 {
		return 0
	}
	
	// Navigate to parent of node to delete
	node := pt.root
	for i := 0; i < len(parts)-1; i++ {
		next := node.children[parts[i]]
		if next == nil {
			return 0 // Path doesn't exist
		}
		node = next
	}
	
	// Count nodes before deletion
	lastPart := parts[len(parts)-1]
	toDelete := node.children[lastPart]
	if toDelete == nil {
		return 0
	}
	
	count := pt.countNodes(toDelete)
	delete(node.children, lastPart)
	return count
}

// countNodes counts all nodes in a subtree
func (pt *PrefixTree) countNodes(node *PrefixTreeNode) int {
	if node == nil {
		return 0
	}
	
	count := 0
	if node.isKey {
		count = 1
	}
	
	for _, child := range node.children {
		count += pt.countNodes(child)
	}
	
	return count
}

// HasPrefix checks if any path starts with prefix
func (pt *PrefixTree) HasPrefix(prefix string) bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	
	parts := splitPath(prefix)
	node := pt.root
	
	for _, part := range parts {
		next := node.children[part]
		if next == nil {
			return false
		}
		node = next
	}
	
	return true
}
