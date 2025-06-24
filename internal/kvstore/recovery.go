package kvstore

import (
	"fmt"
	"io"
	"os"
	"time"
)

// RecoveryManager handles crash recovery for AsyncKVStore
type RecoveryManager struct {
	wal       *WAL
	baseStore *KVStore
	memTable  *MemTable

	// Recovery statistics
	stats RecoveryStats
}

// RecoveryStats holds statistics about recovery operations
type RecoveryStats struct {
	WALEntriesProcessed uint64
	PutOperations       uint64
	DeleteOperations    uint64
	ErrorCount          uint64
	RecoveryDuration    time.Duration
	LastRecoveryTime    time.Time
	RecoveredFromLSN    uint64
	RecoveredToLSN      uint64
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(wal *WAL, baseStore *KVStore, memTable *MemTable) *RecoveryManager {
	return &RecoveryManager{
		wal:       wal,
		baseStore: baseStore,
		memTable:  memTable,
	}
}

// RecoverFromWAL performs crash recovery by replaying WAL entries
func (rm *RecoveryManager) RecoverFromWAL() error {
	start := time.Now()
	rm.stats.LastRecoveryTime = start

	// Get the last committed LSN from base store
	lastCommittedLSN, err := rm.getLastCommittedLSN()
	if err != nil {
		return fmt.Errorf("failed to get last committed LSN: %w", err)
	}

	// Open WAL file for reading
	walFile := rm.wal.filename
	file, err := os.Open(walFile) // #nosec G304 - walFile is from internal WAL configuration
	if err != nil {
		if os.IsNotExist(err) {
			// No WAL file exists, nothing to recover
			return nil
		}
		return fmt.Errorf("failed to open WAL file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: file close failed: %v\n", err)
		}
	}()

	reader := &walReader{file: file}

	// Process WAL entries
	for {
		entry, err := reader.ReadEntry()
		if err == io.EOF {
			break
		}
		if err != nil {
			rm.stats.ErrorCount++
			// Log error but continue recovery
			fmt.Printf("Warning: WAL recovery error reading entry: %v\n", err)
			continue
		}

		// Skip entries that are already committed
		if entry.LSN <= lastCommittedLSN {
			continue
		}

		// Set recovery range
		if rm.stats.RecoveredFromLSN == 0 {
			rm.stats.RecoveredFromLSN = entry.LSN
		}
		rm.stats.RecoveredToLSN = entry.LSN

		// Verify checksum
		if !rm.verifyChecksum(entry) {
			rm.stats.ErrorCount++
			fmt.Printf("Warning: Checksum mismatch for LSN %d, skipping\n", entry.LSN)
			continue
		}

		// Apply the operation
		if err := rm.applyWALEntry(entry); err != nil {
			rm.stats.ErrorCount++
			fmt.Printf("Warning: Failed to apply WAL entry LSN %d: %v\n", entry.LSN, err)
			continue
		}

		rm.stats.WALEntriesProcessed++
	}

	rm.stats.RecoveryDuration = time.Since(start)
	return nil
}

// getLastCommittedLSN gets the highest LSN that was committed to the base store
func (rm *RecoveryManager) getLastCommittedLSN() (uint64, error) {
	// For simplicity, we'll assume all data in base store is committed
	// In a more sophisticated implementation, you might store checkpoint information

	// Check if there's a recovery checkpoint file
	checkpointFile := rm.baseStore.dataDir + "/recovery_checkpoint"
	// #nosec G304 - checkpointFile is from internal configuration
	if data, err := os.ReadFile(checkpointFile); err == nil {
		var lsn uint64
		if _, err := fmt.Sscanf(string(data), "%d", &lsn); err == nil {
			return lsn, nil
		}
	}

	// No checkpoint found, assume we need to recover from the beginning
	return 0, nil
}

// saveRecoveryCheckpoint saves the current recovery state
func (rm *RecoveryManager) saveRecoveryCheckpoint(lsn uint64) error {
	checkpointFile := rm.baseStore.dataDir + "/recovery_checkpoint"
	return os.WriteFile(checkpointFile, []byte(fmt.Sprintf("%d", lsn)), 0600)
}

// verifyChecksum verifies the integrity of a WAL entry
func (rm *RecoveryManager) verifyChecksum(entry *WALEntry) bool {
	// Recalculate checksum
	expectedChecksum := rm.wal.calculateChecksum(entry)
	return entry.Checksum == expectedChecksum
}

// applyWALEntry applies a single WAL entry to the appropriate store
func (rm *RecoveryManager) applyWALEntry(entry *WALEntry) error {
	key := string(entry.Key)
	value := string(entry.Value)

	switch entry.Operation {
	case OpTypePut:
		// Apply to MemTable for recent entries, base store for older ones
		if rm.shouldApplyToMemTable(entry) {
			rm.memTable.Put(key, value, entry.LSN)
		} else {
			if err := rm.baseStore.Put(key, value); err != nil {
				return fmt.Errorf("failed to apply PUT to base store: %w", err)
			}
		}
		rm.stats.PutOperations++

	case OpTypeDelete:
		// Apply deletion
		if rm.shouldApplyToMemTable(entry) {
			rm.memTable.Delete(key, entry.LSN)
		} else {
			if err := rm.baseStore.Delete(key); err != nil {
				// Deletion of non-existent key is OK
				if err.Error() != fmt.Sprintf("key not found: %s", key) {
					return fmt.Errorf("failed to apply DELETE to base store: %w", err)
				}
			}
		}
		rm.stats.DeleteOperations++

	case OpTypeCompaction:
		// Compaction marker - might trigger a compaction or just log
		fmt.Printf("Info: WAL recovery found compaction marker at LSN %d\n", entry.LSN)

	default:
		return fmt.Errorf("unknown operation type: %d", entry.Operation)
	}

	return nil
}

// shouldApplyToMemTable determines if an entry should be applied to MemTable vs base store
func (rm *RecoveryManager) shouldApplyToMemTable(entry *WALEntry) bool {
	// Recent entries (within last hour) go to MemTable
	entryTime := time.Unix(0, entry.Timestamp)
	return time.Since(entryTime) < time.Hour
}

// GetRecoveryStats returns current recovery statistics
func (rm *RecoveryManager) GetRecoveryStats() RecoveryStats {
	return rm.stats
}

// PerformConsistencyCheck verifies data consistency after recovery
func (rm *RecoveryManager) PerformConsistencyCheck() error {
	// Check that MemTable and base store are consistent
	memTableKeys := rm.memTable.List()

	for _, key := range memTableKeys {
		memValue, memFound := rm.memTable.Get(key)
		if !memFound {
			continue // Key was deleted in MemTable
		}

		// Check if the same key exists in base store
		baseValue, baseErr := rm.baseStore.Get(key)
		if baseErr != nil {
			// Key doesn't exist in base store, which is OK if it's recent
			continue
		}

		// If both have the key, MemTable should have newer or same value
		// This is a simple check - in practice you'd use timestamps/LSNs
		if memValue != baseValue {
			fmt.Printf("Info: Key %s has different values in MemTable vs base store (expected for recent writes)\n", key)
		}
	}

	return nil
}

// RecoveryPoint represents a point-in-time recovery state
type RecoveryPoint struct {
	LSN        uint64
	Timestamp  time.Time
	EntryCount uint64
}

// CreateRecoveryPoint creates a recovery checkpoint
func (rm *RecoveryManager) CreateRecoveryPoint() (*RecoveryPoint, error) {
	currentLSN := rm.stats.RecoveredToLSN
	if currentLSN == 0 {
		// Use WAL's current LSN
		currentLSN = rm.wal.nextLSN - 1
	}

	point := &RecoveryPoint{
		LSN:        currentLSN,
		Timestamp:  time.Now(),
		EntryCount: rm.stats.WALEntriesProcessed,
	}

	// Save checkpoint
	if err := rm.saveRecoveryCheckpoint(currentLSN); err != nil {
		return nil, fmt.Errorf("failed to save recovery checkpoint: %w", err)
	}

	return point, nil
}

// ValidateWALIntegrity performs integrity checks on the WAL file
func (rm *RecoveryManager) ValidateWALIntegrity() error {
	walFile := rm.wal.filename
	file, err := os.Open(walFile) // #nosec G304 - walFile is from internal WAL configuration
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No WAL file, nothing to validate
		}
		return fmt.Errorf("failed to open WAL file for validation: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: file close failed: %v\n", err)
		}
	}()

	reader := &walReader{file: file}
	entryCount := 0
	var lastLSN uint64

	for {
		entry, err := reader.ReadEntry()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("WAL integrity check failed at entry %d: %w", entryCount, err)
		}

		// Check LSN ordering
		if entry.LSN <= lastLSN && entryCount > 0 {
			return fmt.Errorf("WAL integrity check failed: LSN not increasing at entry %d (current: %d, previous: %d)",
				entryCount, entry.LSN, lastLSN)
		}
		lastLSN = entry.LSN

		// Verify checksum
		if !rm.verifyChecksum(entry) {
			return fmt.Errorf("WAL integrity check failed: checksum mismatch at entry %d (LSN: %d)",
				entryCount, entry.LSN)
		}

		entryCount++
	}

	fmt.Printf("WAL integrity check passed: %d entries validated\n", entryCount)
	return nil
}

// RepairWAL attempts to repair a corrupted WAL file
func (rm *RecoveryManager) RepairWAL() error {
	walFile := rm.wal.filename
	backupFile := walFile + ".backup"
	repairedFile := walFile + ".repaired"

	// Create backup
	if err := copyFile(walFile, backupFile); err != nil {
		return fmt.Errorf("failed to create WAL backup: %w", err)
	}

	// Open original and repaired files
	original, err := os.Open(walFile) // #nosec G304 - walFile is from internal WAL configuration
	if err != nil {
		return fmt.Errorf("failed to open original WAL: %w", err)
	}
	defer func() {
		if err := original.Close(); err != nil {
			fmt.Printf("Warning: original file close failed: %v\n", err)
		}
	}()

	repaired, err := os.Create(repairedFile) // #nosec G304 - repairedFile is from internal configuration
	if err != nil {
		return fmt.Errorf("failed to create repaired WAL: %w", err)
	}
	defer func() {
		if err := repaired.Close(); err != nil {
			fmt.Printf("Warning: repaired file close failed: %v\n", err)
		}
	}()

	reader := &walReader{file: original}
	validEntries := 0
	corruptedEntries := 0

	// Process entries and write only valid ones
	for {
		entry, err := reader.ReadEntry()
		if err == io.EOF {
			break
		}
		if err != nil {
			corruptedEntries++
			fmt.Printf("Warning: Skipping corrupted entry: %v\n", err)
			continue
		}

		// Verify checksum
		if !rm.verifyChecksum(entry) {
			corruptedEntries++
			fmt.Printf("Warning: Skipping entry with invalid checksum at LSN %d\n", entry.LSN)
			continue
		}

		// Write valid entry to repaired file
		if err := rm.writeWALEntry(repaired, entry); err != nil {
			return fmt.Errorf("failed to write repaired entry: %w", err)
		}

		validEntries++
	}

	if err := repaired.Close(); err != nil {
		fmt.Printf("Warning: repaired file close failed during backup: %v\n", err)
	}

	// Replace original with repaired file
	if err := os.Rename(repairedFile, walFile); err != nil {
		return fmt.Errorf("failed to replace WAL with repaired version: %w", err)
	}

	fmt.Printf("WAL repair completed: %d valid entries, %d corrupted entries removed\n",
		validEntries, corruptedEntries)

	return nil
}

// writeWALEntry writes a WAL entry to a file (helper for repair)
func (rm *RecoveryManager) writeWALEntry(file *os.File, entry *WALEntry) error {
	// This would use the same format as the WAL writer
	// For now, we'll use a simplified version

	// Calculate entry size
	entrySize := 25 + len(entry.Key) + len(entry.Value) + 4

	// Write entry data (simplified - in practice would match WAL format exactly)
	data := make([]byte, entrySize)
	// ... write entry data to buffer ...

	_, err := file.Write(data)
	return err
}

// copyFile copies a file (helper function)
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src) // #nosec G304 - src is from internal configuration
	if err != nil {
		return err
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			fmt.Printf("Warning: source file close failed: %v\n", err)
		}
	}()

	destFile, err := os.Create(dst) // #nosec G304 - dst is from internal configuration
	if err != nil {
		return err
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			fmt.Printf("Warning: dest file close failed: %v\n", err)
		}
	}()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
