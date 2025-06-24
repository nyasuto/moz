package lsm

import (
	"fmt"
	"sort"
	"time"
)

// CompactionManager handles different compaction strategies for the LSM-Tree
type CompactionManager struct {
	lsm    *LSMTree
	config CompactionConfig
}

// CompactionConfig holds configuration for compaction operations
type CompactionConfig struct {
	MaxParallelCompactions int           // Maximum number of concurrent compactions
	CompactionTimeout      time.Duration // Timeout for compaction operations
	MinCompactionSize      int64         // Minimum size to trigger compaction
	MaxCompactionSize      int64         // Maximum size for a single compaction
}

// DefaultCompactionConfig returns default compaction configuration
func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		MaxParallelCompactions: 2,
		CompactionTimeout:      30 * time.Minute,
		MinCompactionSize:      4 * 1024 * 1024,   // 4MB
		MaxCompactionSize:      100 * 1024 * 1024, // 100MB
	}
}

// NewCompactionManager creates a new compaction manager
func NewCompactionManager(lsm *LSMTree, config CompactionConfig) *CompactionManager {
	return &CompactionManager{
		lsm:    lsm,
		config: config,
	}
}

// PerformLeveledCompaction performs leveled compaction between specified levels
func (cm *CompactionManager) PerformLeveledCompaction(sourceLevel int) error {
	if sourceLevel >= len(cm.lsm.levels)-1 {
		return fmt.Errorf("cannot compact last level")
	}

	sourceLevelData := &cm.lsm.levels[sourceLevel]
	targetLevelData := &cm.lsm.levels[sourceLevel+1]

	// Select SSTables for compaction
	sourceSSTables, targetSSTables, err := cm.selectSSTablesForLeveledCompaction(sourceLevelData, targetLevelData)
	if err != nil {
		return fmt.Errorf("failed to select SSTables: %w", err)
	}

	if len(sourceSSTables) == 0 {
		return nil // Nothing to compact
	}

	fmt.Printf("Compacting %d SSTables from L%d with %d SSTables from L%d\n",
		len(sourceSSTables), sourceLevel, len(targetSSTables), sourceLevel+1)

	// Perform the actual compaction
	newSSTables, err := cm.mergeSSTables(sourceSSTables, targetSSTables, sourceLevel+1)
	if err != nil {
		return fmt.Errorf("failed to merge SSTables: %w", err)
	}

	// Update levels with new SSTables
	if err := cm.updateLevelsAfterCompaction(sourceLevelData, targetLevelData, sourceSSTables, targetSSTables, newSSTables); err != nil {
		return fmt.Errorf("failed to update levels: %w", err)
	}

	// Clean up old SSTables
	cm.cleanupOldSSTables(append(sourceSSTables, targetSSTables...))

	return nil
}

// PerformSizeTieredCompaction performs size-tiered compaction
func (cm *CompactionManager) PerformSizeTieredCompaction(level int) error {
	if level >= len(cm.lsm.levels) {
		return fmt.Errorf("invalid level: %d", level)
	}

	levelData := &cm.lsm.levels[level]

	// Group SSTables by similar size
	sstableGroups := cm.groupSSTablesBySize(levelData.SSTables)

	// Compact groups that meet criteria
	for _, group := range sstableGroups {
		if cm.shouldCompactGroup(group) {
			if err := cm.compactSSTableGroup(group, level); err != nil {
				return fmt.Errorf("failed to compact SSTable group: %w", err)
			}
		}
	}

	return nil
}

// selectSSTablesForLeveledCompaction selects SSTables for leveled compaction
func (cm *CompactionManager) selectSSTablesForLeveledCompaction(sourceLevel, targetLevel *Level) ([]*SSTable, []*SSTable, error) {
	if len(sourceLevel.SSTables) == 0 {
		return nil, nil, nil
	}

	// For L0, select all SSTables (they can overlap)
	var sourceSSTables []*SSTable
	if sourceLevel.Level == 0 {
		sourceSSTables = sourceLevel.SSTables
	} else {
		// For L1+, select SSTables that exceed size threshold
		totalSize := int64(0)
		for _, sstable := range sourceLevel.SSTables {
			totalSize += sstable.FileSize
		}

		if totalSize > sourceLevel.Config.CompactionSize {
			// Select oldest SSTables up to reasonable compaction size
			sourceSSTables = cm.selectOldestSSTables(sourceLevel.SSTables, cm.config.MaxCompactionSize)
		}
	}

	if len(sourceSSTables) == 0 {
		return nil, nil, nil
	}

	// Find overlapping SSTables in target level
	var targetSSTables []*SSTable
	if targetLevel.Level > 0 {
		targetSSTables = cm.findOverlappingSSTables(sourceSSTables, targetLevel.SSTables)
	}

	return sourceSSTables, targetSSTables, nil
}

// selectOldestSSTables selects the oldest SSTables up to a maximum size
func (cm *CompactionManager) selectOldestSSTables(sstables []*SSTable, maxSize int64) []*SSTable {
	// Sort by creation time (oldest first)
	sorted := make([]*SSTable, len(sstables))
	copy(sorted, sstables)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].metadata.CreatedAt < sorted[j].metadata.CreatedAt
	})

	var selected []*SSTable
	var totalSize int64

	for _, sstable := range sorted {
		if totalSize+sstable.FileSize > maxSize && len(selected) > 0 {
			break
		}
		selected = append(selected, sstable)
		totalSize += sstable.FileSize
	}

	return selected
}

// findOverlappingSSTables finds SSTables in target level that overlap with source SSTables
func (cm *CompactionManager) findOverlappingSSTables(sourceSSTables []*SSTable, targetSSTables []*SSTable) []*SSTable {
	if len(sourceSSTables) == 0 {
		return nil
	}

	// Find min and max keys from source SSTables
	minKey := sourceSSTables[0].metadata.MinKey
	maxKey := sourceSSTables[0].metadata.MaxKey

	for _, sstable := range sourceSSTables {
		if sstable.metadata.MinKey < minKey {
			minKey = sstable.metadata.MinKey
		}
		if sstable.metadata.MaxKey > maxKey {
			maxKey = sstable.metadata.MaxKey
		}
	}

	// Find overlapping SSTables in target level
	var overlapping []*SSTable
	for _, sstable := range targetSSTables {
		if cm.keyRangesOverlap(minKey, maxKey, sstable.metadata.MinKey, sstable.metadata.MaxKey) {
			overlapping = append(overlapping, sstable)
		}
	}

	return overlapping
}

// keyRangesOverlap checks if two key ranges overlap
func (cm *CompactionManager) keyRangesOverlap(min1, max1, min2, max2 string) bool {
	return max1 >= min2 && max2 >= min1
}

// mergeSSTables merges multiple SSTables into new SSTables for the target level
func (cm *CompactionManager) mergeSSTables(sourceSSTables, targetSSTables []*SSTable, targetLevel int) ([]*SSTable, error) {
	// Collect all SSTables to merge
	allSSTables := append(sourceSSTables, targetSSTables...)

	if len(allSSTables) == 0 {
		return nil, nil
	}

	// Create iterators for all SSTables
	iterators := make([]*SSTableIterator, len(allSSTables))
	for i, sstable := range allSSTables {
		iterators[i] = sstable.Iterator()
	}

	// Merge using k-way merge algorithm
	mergedEntries, err := cm.kWayMerge(iterators)
	if err != nil {
		return nil, fmt.Errorf("failed to perform k-way merge: %w", err)
	}

	// Split merged entries into appropriately sized SSTables
	newSSTables, err := cm.createSSTablesFromEntries(mergedEntries, targetLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create new SSTables: %w", err)
	}

	return newSSTables, nil
}

// kWayMerge performs k-way merge of multiple SSTable iterators
func (cm *CompactionManager) kWayMerge(iterators []*SSTableIterator) ([]*SSTableEntry, error) {
	type iteratorItem struct {
		iterator *SSTableIterator
		entry    *SSTableEntry
		index    int
	}

	// Initialize priority queue with first entry from each iterator
	heap := make([]*iteratorItem, 0, len(iterators))
	for i, iter := range iterators {
		if iter.HasNext() {
			entry, err := iter.Next()
			if err != nil {
				return nil, err
			}
			heap = append(heap, &iteratorItem{
				iterator: iter,
				entry:    entry,
				index:    i,
			})
		}
	}

	// Sort initial heap
	sort.Slice(heap, func(i, j int) bool {
		return heap[i].entry.Key < heap[j].entry.Key
	})

	var result []*SSTableEntry
	var lastKey string

	for len(heap) > 0 {
		// Take the smallest entry
		item := heap[0]
		heap = heap[1:]

		// Only add if this is a new key or the newest version of the key
		if item.entry.Key != lastKey {
			// For duplicates, take the one with highest timestamp (newest)
			if !item.entry.Deleted {
				result = append(result, item.entry)
			}
			lastKey = item.entry.Key
		}

		// Get next entry from the same iterator
		if item.iterator.HasNext() {
			entry, err := item.iterator.Next()
			if err != nil {
				return nil, err
			}

			// Insert back into heap maintaining order
			newItem := &iteratorItem{
				iterator: item.iterator,
				entry:    entry,
				index:    item.index,
			}

			// Find insertion point to maintain sorted order
			insertIndex := sort.Search(len(heap), func(i int) bool {
				return heap[i].entry.Key > entry.Key
			})

			// Insert at the correct position
			heap = append(heap, nil)
			copy(heap[insertIndex+1:], heap[insertIndex:])
			heap[insertIndex] = newItem
		}
	}

	return result, nil
}

// createSSTablesFromEntries creates new SSTables from merged entries
func (cm *CompactionManager) createSSTablesFromEntries(entries []*SSTableEntry, level int) ([]*SSTable, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	var newSSTables []*SSTable
	var currentSSTable *SSTable
	var currentSize int64

	targetFileSize := cm.lsm.levels[level].Config.TargetFileSize

	for _, entry := range entries {
		// Create new SSTable if needed
		if currentSSTable == nil || currentSize >= targetFileSize {
			if currentSSTable != nil {
				// Finalize current SSTable
				if err := currentSSTable.Finalize(); err != nil {
					return nil, fmt.Errorf("failed to finalize SSTable: %w", err)
				}
				newSSTables = append(newSSTables, currentSSTable)
			}

			// Create new SSTable
			cm.lsm.nextSSTableID++
			sstableID := fmt.Sprintf("sstable_L%d_%d", level, cm.lsm.nextSSTableID)

			var err error
			currentSSTable, err = NewSSTable(sstableID, cm.lsm.dataDir, level)
			if err != nil {
				return nil, fmt.Errorf("failed to create SSTable: %w", err)
			}
			currentSize = 0
		}

		// Add entry to current SSTable
		if err := currentSSTable.Put(entry.Key, entry.Value, entry.Deleted); err != nil {
			return nil, fmt.Errorf("failed to write entry: %w", err)
		}

		// Estimate size (simplified)
		currentSize += int64(len(entry.Key) + len(entry.Value) + 32) // Overhead estimate
	}

	// Finalize last SSTable
	if currentSSTable != nil {
		if err := currentSSTable.Finalize(); err != nil {
			return nil, fmt.Errorf("failed to finalize final SSTable: %w", err)
		}
		newSSTables = append(newSSTables, currentSSTable)
	}

	return newSSTables, nil
}

// updateLevelsAfterCompaction updates the level structure after compaction
func (cm *CompactionManager) updateLevelsAfterCompaction(sourceLevel, targetLevel *Level, oldSourceSSTables, oldTargetSSTables, newSSTables []*SSTable) error {
	// Remove old SSTables from source level
	sourceLevel.SSTables = cm.removeSSTables(sourceLevel.SSTables, oldSourceSSTables)

	// Remove old SSTables from target level
	targetLevel.SSTables = cm.removeSSTables(targetLevel.SSTables, oldTargetSSTables)

	// Add new SSTables to target level
	targetLevel.SSTables = append(targetLevel.SSTables, newSSTables...)

	// Sort target level SSTables by key range
	sort.Slice(targetLevel.SSTables, func(i, j int) bool {
		return targetLevel.SSTables[i].metadata.MinKey < targetLevel.SSTables[j].metadata.MinKey
	})

	return nil
}

// removeSSTables removes specified SSTables from a slice
func (cm *CompactionManager) removeSSTables(sstables []*SSTable, toRemove []*SSTable) []*SSTable {
	removeSet := make(map[string]bool)
	for _, sstable := range toRemove {
		removeSet[sstable.ID] = true
	}

	var result []*SSTable
	for _, sstable := range sstables {
		if !removeSet[sstable.ID] {
			result = append(result, sstable)
		}
	}

	return result
}

// cleanupOldSSTables removes old SSTable files
func (cm *CompactionManager) cleanupOldSSTables(sstables []*SSTable) {
	for _, sstable := range sstables {
		if err := sstable.Close(); err != nil {
			fmt.Printf("Warning: failed to close SSTable %s: %v\n", sstable.ID, err)
		}

		// Remove bloom filter
		delete(cm.lsm.bloomFilters, sstable.ID)

		// TODO: Remove SSTable files from disk
		// This would involve deleting the .sst and .idx files
	}
}

// groupSSTablesBySize groups SSTables by similar size for size-tiered compaction
func (cm *CompactionManager) groupSSTablesBySize(sstables []*SSTable) [][]*SSTable {
	if len(sstables) == 0 {
		return nil
	}

	// Sort by size
	sorted := make([]*SSTable, len(sstables))
	copy(sorted, sstables)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FileSize < sorted[j].FileSize
	})

	var groups [][]*SSTable
	var currentGroup []*SSTable
	var currentSizeRange int64

	for _, sstable := range sorted {
		if len(currentGroup) == 0 {
			// Start new group
			currentGroup = []*SSTable{sstable}
			currentSizeRange = sstable.FileSize
		} else {
			// Check if SSTable fits in current group (within 2x size range)
			if sstable.FileSize <= currentSizeRange*2 {
				currentGroup = append(currentGroup, sstable)
			} else {
				// Start new group
				groups = append(groups, currentGroup)
				currentGroup = []*SSTable{sstable}
				currentSizeRange = sstable.FileSize
			}
		}
	}

	// Add last group
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	return groups
}

// shouldCompactGroup determines if a group of SSTables should be compacted
func (cm *CompactionManager) shouldCompactGroup(group []*SSTable) bool {
	if len(group) < 3 { // Need at least 3 SSTables to make compaction worthwhile
		return false
	}

	totalSize := int64(0)
	for _, sstable := range group {
		totalSize += sstable.FileSize
	}

	return totalSize >= cm.config.MinCompactionSize && totalSize <= cm.config.MaxCompactionSize
}

// compactSSTableGroup compacts a group of SSTables in size-tiered compaction
func (cm *CompactionManager) compactSSTableGroup(group []*SSTable, level int) error {
	if len(group) == 0 {
		return nil
	}

	fmt.Printf("Size-tiered compaction: merging %d SSTables in level %d\n", len(group), level)

	// Merge the group into new SSTables
	newSSTables, err := cm.mergeSSTables(group, nil, level)
	if err != nil {
		return fmt.Errorf("failed to merge SSTable group: %w", err)
	}

	// Update level
	levelData := &cm.lsm.levels[level]
	levelData.SSTables = cm.removeSSTables(levelData.SSTables, group)
	levelData.SSTables = append(levelData.SSTables, newSSTables...)

	// Clean up old SSTables
	cm.cleanupOldSSTables(group)

	return nil
}
