package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nyasuto/moz/internal/batch"
	"github.com/nyasuto/moz/internal/daemon"
	"github.com/nyasuto/moz/internal/kvstore"
	"github.com/nyasuto/moz/internal/pool"
	"github.com/nyasuto/moz/internal/query"
)

func main() {
	var format = flag.String("format", "text", "Storage format: text or binary")
	var indexType = flag.String("index", "none", "Index type: hash, btree, or none")
	var help = flag.Bool("help", false, "Show help message")
	var useDaemon = flag.Bool("daemon", false, "Use daemon mode for high performance")
	var forceLocal = flag.Bool("local", false, "Force local execution (bypass daemon)")
	var partitions = flag.Int("partitions", 1, "Number of partitions for parallel writes (1-16)")
	flag.Parse()

	// Handle help flag
	if *help {
		printUsage()
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]

	// Handle daemon-specific commands first
	switch command {
	case "daemon":
		handleDaemonCommands(args[1:], *format, *indexType)
		return
	case "batch":
		handleBatchCommand(args[1:], *format, *indexType, *useDaemon || daemon.IsDaemonRunning())
		return
	case "pool":
		handlePoolCommands(args[1:], *format, *indexType)
		return
	}

	// Auto-optimization: try daemon first unless forced local
	if !*forceLocal && daemon.IsDaemonRunning() {
		if err := executeThroughDaemon(command, args[1:]); err == nil {
			return
		}
		// If daemon execution fails, fall back to local execution
	}

	// Create store with partition support
	store := createStoreOrPartitioned(*format, *indexType, *partitions)

	// Show partition info if using partitions
	if *partitions > 1 {
		fmt.Printf("🔄 Using %d partitions for parallel processing\n", *partitions)
	}

	switch command {
	case "put":
		if len(args) != 3 {
			fmt.Println("Usage: moz put <key> <value>")
			os.Exit(1)
		}
		key, value := args[1], args[2]
		if err := store.Put(key, value); err != nil {
			log.Fatalf("Error putting key-value: %v", err)
		}
		if *partitions > 1 {
			fmt.Printf("✅ Stored (partition): %s = %s\n", key, value)
		} else {
			fmt.Printf("✅ Stored: %s = %s\n", key, value)
		}

	case "get":
		if len(args) != 2 {
			fmt.Println("Usage: moz get <key>")
			os.Exit(1)
		}
		key := args[1]
		value, err := store.Get(key)
		if err != nil {
			log.Fatalf("Error getting key: %v", err)
		}
		fmt.Printf("%s\n", value)

	case "del", "delete":
		if len(args) != 2 {
			fmt.Println("Usage: moz del <key>")
			os.Exit(1)
		}
		key := args[1]
		if err := store.Delete(key); err != nil {
			log.Fatalf("Error deleting key: %v", err)
		}
		fmt.Printf("✅ Deleted: %s\n", key)

	case "list":
		keys, err := store.List()
		if err != nil {
			log.Fatalf("Error listing keys: %v", err)
		}
		if len(keys) == 0 {
			fmt.Println("No keys found")
		} else {
			for _, key := range keys {
				value, err := store.Get(key)
				if err != nil {
					fmt.Printf("%s: <error: %v>\n", key, err)
				} else {
					fmt.Printf("%s: %s\n", key, value)
				}
			}
		}

	case "compact":
		if err := store.Compact(); err != nil {
			log.Fatalf("Error compacting store: %v", err)
		}
		fmt.Println("✅ Store compacted")

	case "stats":
		fmt.Printf("📊 Storage Statistics:\n")
		
		// Try to get extended stats if available
		if extStore, ok := store.(ExtendedStoreInterface); ok {
			stats, err := extStore.GetCompactionStats()
			if err != nil {
				log.Fatalf("Error getting compaction stats: %v", err)
			}

			indexStats, err := extStore.GetIndexStats()
			if err != nil {
				log.Fatalf("Error getting index stats: %v", err)
			}

			fmt.Printf("  Format: %s\n", *format)
			fmt.Printf("  Index: %s\n", *indexType)
			fmt.Printf("  Auto-compaction: %v\n", stats.Enabled)
			fmt.Printf("  Operations since last compaction: %d\n", stats.OperationCount)
			fmt.Printf("  File size: %d bytes\n", stats.FileSize)
			fmt.Printf("  Deleted entries ratio: %.2f%%\n", stats.DeletedRatio*100)
			fmt.Printf("  Operations until next compaction: %d\n", stats.NextCompactionAt)
			if stats.LastCompaction > 0 {
				fmt.Printf("  Last compaction: %v\n", time.Unix(stats.LastCompaction, 0).Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("  Last compaction: Never\n")
			}

			fmt.Printf("\n🔍 Index Statistics:\n")
			fmt.Printf("  Index enabled: %v\n", indexStats["enabled"])
			fmt.Printf("  Index type: %s\n", indexStats["type"])
			fmt.Printf("  Index size: %d entries\n", indexStats["size"])
			fmt.Printf("  Index memory usage: %d bytes\n", indexStats["memory_usage"])
		} else {
			// For partitioned stores, show basic info
			fmt.Printf("  Format: %s\n", *format)
			fmt.Printf("  Partitions: %d\n", *partitions)
			fmt.Printf("  Basic statistics available\n")
		}

	case "convert":
		if len(args) != 3 {
			fmt.Println("Usage: moz convert <from_format> <to_format>")
			fmt.Println("Formats: text, binary")
			os.Exit(1)
		}
		fromFormat, toFormat := args[1], args[2]

		if fromFormat == toFormat {
			fmt.Printf("Source and target formats are the same: %s\n", fromFormat)
			os.Exit(1)
		}

		var converter *kvstore.FormatConverter
		if fromFormat == "text" && toFormat == "binary" {
			converter = kvstore.NewFormatConverter("moz.log", "moz.bin")
			if err := converter.TextToBinary(); err != nil {
				log.Fatalf("Error converting text to binary: %v", err)
			}
		} else if fromFormat == "binary" && toFormat == "text" {
			converter = kvstore.NewFormatConverter("moz.log", "moz.bin")
			if err := converter.BinaryToText(); err != nil {
				log.Fatalf("Error converting binary to text: %v", err)
			}
		} else {
			fmt.Printf("Unsupported conversion: %s to %s\n", fromFormat, toFormat)
			fmt.Println("Supported conversions: text to binary, binary to text")
			os.Exit(1)
		}
		fmt.Printf("✅ Successfully converted from %s to %s format\n", fromFormat, toFormat)

	case "validate":
		if len(args) < 2 {
			fmt.Println("Usage: moz validate <file_format>")
			fmt.Println("Formats: text, binary")
			os.Exit(1)
		}
		fileFormat := args[1]

		switch fileFormat {
		case "binary":
			if err := kvstore.ValidateBinaryFile("moz.bin"); err != nil {
				log.Fatalf("Binary file validation failed: %v", err)
			}
		case "text":
			fmt.Println("Text file validation not implemented yet")
		default:
			fmt.Printf("Unknown format: %s\n", fileFormat)
			os.Exit(1)
		}

	case "range":
		if len(args) != 3 {
			fmt.Println("Usage: moz range <start_key> <end_key>")
			os.Exit(1)
		}
		startKey, endKey := args[1], args[2]

		if extStore, ok := store.(ExtendedStoreInterface); ok {
			results, err := extStore.GetRange(startKey, endKey)
			if err != nil {
				log.Fatalf("Error performing range query: %v", err)
			}

			if len(results) == 0 {
				fmt.Printf("No keys found in range [%s, %s]\n", startKey, endKey)
			} else {
				fmt.Printf("🔍 Range query [%s, %s] (%d results):\n", startKey, endKey, len(results))
				for key, value := range results {
					fmt.Printf("  %s: %s\n", key, value)
				}
			}
		} else {
			fmt.Println("❌ Range queries not supported with partitioned stores")
		}

	case "prefix":
		if len(args) != 2 {
			fmt.Println("Usage: moz prefix <prefix>")
			os.Exit(1)
		}
		prefix := args[1]

		if extStore, ok := store.(ExtendedStoreInterface); ok {
			results, err := extStore.PrefixSearch(prefix)
			if err != nil {
				log.Fatalf("Error performing prefix search: %v", err)
			}

			if len(results) == 0 {
				fmt.Printf("No keys found with prefix '%s'\n", prefix)
			} else {
				fmt.Printf("🔍 Prefix search '%s' (%d results):\n", prefix, len(results))
				for key, value := range results {
					fmt.Printf("  %s: %s\n", key, value)
				}
			}
		} else {
			fmt.Println("❌ Prefix search not supported with partitioned stores")
		}

	case "sorted":
		if extStore, ok := store.(ExtendedStoreInterface); ok {
			keys, err := extStore.ListSorted()
			if err != nil {
				log.Fatalf("Error getting sorted keys: %v", err)
			}

			if len(keys) == 0 {
				fmt.Println("No keys found")
			} else {
				fmt.Printf("📋 Sorted keys (%d total):\n", len(keys))
				for _, key := range keys {
					value, err := store.Get(key)
					if err != nil {
						fmt.Printf("  %s: <error: %v>\n", key, err)
					} else {
						fmt.Printf("  %s: %s\n", key, value)
					}
				}
			}
		} else {
			// Fall back to regular list for partitioned stores
			keys, err := store.List()
			if err != nil {
				log.Fatalf("Error listing keys: %v", err)
			}
			fmt.Printf("📋 Keys (%d total):\n", len(keys))
			for _, key := range keys {
				value, err := store.Get(key)
				if err != nil {
					fmt.Printf("  %s: <error: %v>\n", key, err)
				} else {
					fmt.Printf("  %s: %s\n", key, value)
				}
			}
		}

	case "rebuild-index":
		if extStore, ok := store.(ExtendedStoreInterface); ok {
			if err := extStore.RebuildIndex(); err != nil {
				log.Fatalf("Error rebuilding index: %v", err)
			}
			fmt.Println("✅ Index rebuilt successfully")
		} else {
			fmt.Println("❌ Index operations not supported with partitioned stores")
		}

	case "validate-index":
		if extStore, ok := store.(ExtendedStoreInterface); ok {
			if err := extStore.ValidateIndex(); err != nil {
				log.Fatalf("Index validation failed: %v", err)
			}
			fmt.Println("✅ Index validation passed")
		} else {
			fmt.Println("❌ Index operations not supported with partitioned stores")
		}

	case "query":
		if len(args) < 2 {
			fmt.Println("Usage: moz query \"SELECT * FROM moz WHERE key = 'value'\"")
			os.Exit(1)
		}
		queryStr := strings.Join(args[1:], " ")

		// Parse and execute query
		lexer := query.NewLexer(queryStr)
		parser := query.NewParser(lexer)
		stmt := parser.ParseQuery()

		if len(parser.Errors()) > 0 {
			fmt.Printf("❌ Query parsing errors:\n")
			for _, err := range parser.Errors() {
				fmt.Printf("  - %s\n", err)
			}
			os.Exit(1)
		}

		if kvStore, ok := store.(*kvstore.KVStore); ok {
			executor := query.NewExecutor(kvStore)
			result := executor.Execute(stmt)

			if result.Error != nil {
				log.Fatalf("Query execution error: %v", result.Error)
			}

			// Display results
			selectStmt, ok := stmt.(*query.SelectStatement)
			if !ok {
				fmt.Println("❌ Invalid statement type")
				os.Exit(1)
			}

			if len(selectStmt.Fields) > 0 {
				if _, ok := selectStmt.Fields[0].(*query.FunctionExpression); ok {
					// Aggregation query
					fmt.Printf("Count: %d\n", result.Count)
				} else {
					// Regular SELECT query
					if len(result.Rows) == 0 {
						fmt.Println("No results found")
					} else {
						fmt.Printf("🔍 Query results (%d rows):\n", len(result.Rows))
						for i, row := range result.Rows {
							fmt.Printf("%d. ", i+1)
							for field, value := range row {
								fmt.Printf("%s: %s  ", field, value)
							}
							fmt.Println()
						}
					}
				}
			}
		} else {
			fmt.Println("❌ Query language not supported with partitioned stores")
		}

	case "help":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// executeThroughDaemon executes command through daemon for high performance
func executeThroughDaemon(command string, args []string) error {
	client := daemon.NewClient()

	switch command {
	case "put":
		if len(args) != 2 {
			return fmt.Errorf("put requires exactly 2 arguments")
		}
		err := client.Put(args[0], args[1])
		if err == nil {
			fmt.Printf("✅ Stored: %s = %s\n", args[0], args[1])
		}
		return err

	case "get":
		if len(args) != 1 {
			return fmt.Errorf("get requires exactly 1 argument")
		}
		value, err := client.Get(args[0])
		if err == nil {
			fmt.Printf("%s\n", value)
		}
		return err

	case "del", "delete":
		if len(args) != 1 {
			return fmt.Errorf("delete requires exactly 1 argument")
		}
		err := client.Delete(args[0])
		if err == nil {
			fmt.Printf("✅ Deleted: %s\n", args[0])
		}
		return err

	case "list":
		entries, err := client.List()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No keys found")
		} else {
			for key, value := range entries {
				fmt.Printf("%s: %s\n", key, value)
			}
		}
		return nil

	case "compact":
		err := client.Compact()
		if err == nil {
			fmt.Println("✅ Store compacted")
		}
		return err

	case "stats":
		stats, err := client.Stats()
		if err != nil {
			return err
		}
		fmt.Printf("📊 Storage Statistics (via daemon):\n")
		fmt.Printf("%+v\n", stats)
		return nil

	default:
		return fmt.Errorf("command not supported in daemon mode: %s", command)
	}
}

// handleDaemonCommands handles daemon management commands
func handleDaemonCommands(args []string, format, indexType string) {
	if len(args) < 1 {
		fmt.Println("Usage: moz daemon <start|stop|status|restart>")
		os.Exit(1)
	}

	subcommand := args[0]

	switch subcommand {
	case "start":
		if daemon.IsDaemonRunning() {
			fmt.Println("⚠️  Daemon is already running")
			return
		}

		// Create store
		store := createStore(format, indexType)

		// Create and start daemon
		dm := daemon.NewDaemonManager(store)
		if err := dm.Start(); err != nil {
			log.Fatalf("Failed to start daemon: %v", err)
		}

		// Write PID file
		if err := daemon.WritePIDFile(); err != nil {
			log.Printf("Warning: Failed to write PID file: %v", err)
		}

		fmt.Println("🚀 Daemon started successfully")
		fmt.Printf("Socket: %s\n", dm.GetSocketPath())

		// Set up signal handling for graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Wait for shutdown signal
		<-sigCh
		fmt.Println("\n📴 Shutting down daemon...")

		if err := dm.Stop(); err != nil {
			log.Printf("Error stopping daemon: %v", err)
		}

		if err := daemon.RemovePIDFile(); err != nil {
			log.Printf("Warning: Failed to remove PID file: %v", err)
		}
		fmt.Println("✅ Daemon stopped")

	case "stop":
		if !daemon.IsDaemonRunning() {
			fmt.Println("⚠️  Daemon is not running")
			return
		}

		pid, err := daemon.GetDaemonPID()
		if err != nil {
			log.Fatalf("Failed to get daemon PID: %v", err)
		}

		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			log.Fatalf("Failed to stop daemon: %v", err)
		}

		fmt.Println("📴 Daemon stopped")

	case "status":
		if daemon.IsDaemonRunning() {
			pid, _ := daemon.GetDaemonPID()
			fmt.Printf("✅ Daemon is running (PID: %d)\n", pid)
		} else {
			fmt.Println("❌ Daemon is not running")
		}

	case "restart":
		handleDaemonCommands([]string{"stop"}, format, indexType)
		time.Sleep(1 * time.Second)
		handleDaemonCommands([]string{"start"}, format, indexType)

	default:
		fmt.Printf("Unknown daemon command: %s\n", subcommand)
		fmt.Println("Available commands: start, stop, status, restart")
		os.Exit(1)
	}
}

// handleBatchCommand handles batch operations
func handleBatchCommand(args []string, format, indexType string, useDaemon bool) {
	if len(args) < 1 {
		fmt.Println("Usage: moz batch <operation1> [args...] <operation2> [args...] ...")
		fmt.Println("Example: moz batch put user1 alice put user2 bob get user1")
		os.Exit(1)
	}

	// Parse batch operations
	operations, err := batch.ParseBatchCommand(args)
	if err != nil {
		log.Fatalf("Error parsing batch command: %v", err)
	}

	if len(operations) == 0 {
		fmt.Println("No operations specified")
		os.Exit(1)
	}

	fmt.Printf("🔄 Executing %d batch operations...\n", len(operations))

	// Try daemon first if available and requested
	if useDaemon && daemon.IsDaemonRunning() {
		fmt.Println("📡 Using daemon for high-performance batch execution")
		client := daemon.NewClient()

		start := time.Now()
		successCount := 0

		for i, op := range operations {
			_, err := client.ExecuteCommand(op.Type, op.Arguments...)
			if err != nil {
				fmt.Printf("❌ Operation %d failed: %v\n", i+1, err)
			} else {
				fmt.Printf("✅ Operation %d: %s\n", i+1, op.Type)
				successCount++
			}
		}

		duration := time.Since(start)
		fmt.Printf("\n📊 Batch Summary:\n")
		fmt.Printf("  Total operations: %d\n", len(operations))
		fmt.Printf("  Successful: %d\n", successCount)
		fmt.Printf("  Failed: %d\n", len(operations)-successCount)
		fmt.Printf("  Total time: %v\n", duration)
		fmt.Printf("  Operations/sec: %.2f\n", float64(len(operations))/duration.Seconds())

		return
	}

	// Local batch execution
	store := createStore(format, indexType)
	executor := batch.NewBatchExecutor(store)

	results := executor.Execute(operations)

	// Display results
	for i, result := range results {
		if result.Success {
			fmt.Printf("✅ Operation %d: %s (%.2fms)\n", i+1, result.Operation.Type, float64(result.Duration.Nanoseconds())/1e6)
		} else {
			fmt.Printf("❌ Operation %d: %s - %s\n", i+1, result.Operation.Type, result.Error)
		}
	}

	// Display summary
	summary := batch.GenerateSummary(results)
	fmt.Printf("\n📊 Batch Summary:\n")
	fmt.Printf("  Total operations: %d\n", summary.TotalOperations)
	fmt.Printf("  Successful: %d\n", summary.SuccessfulOps)
	fmt.Printf("  Failed: %d\n", summary.FailedOps)
	fmt.Printf("  Total time: %v\n", summary.TotalDuration)
	fmt.Printf("  Average time: %v\n", summary.AverageDuration)
	fmt.Printf("  Operations/sec: %.2f\n", summary.OperationsPerSec)
}

// handlePoolCommands handles process pool commands
func handlePoolCommands(args []string, format, indexType string) {
	if len(args) < 1 {
		fmt.Println("Usage: moz pool <start|status|test> [workers] [jobs]")
		os.Exit(1)
	}

	subcommand := args[0]

	switch subcommand {
	case "start":
		workerSize := 4
		queueSize := 100

		if len(args) > 1 {
			if _, err := fmt.Sscanf(args[1], "%d", &workerSize); err != nil {
				log.Printf("Warning: Invalid worker size, using default: %v", err)
			}
		}
		if len(args) > 2 {
			if _, err := fmt.Sscanf(args[2], "%d", &queueSize); err != nil {
				log.Printf("Warning: Invalid queue size, using default: %v", err)
			}
		}

		store := createStore(format, indexType)
		pool := pool.NewProcessPool(workerSize, queueSize, store)

		if err := pool.Start(); err != nil {
			log.Fatalf("Failed to start process pool: %v", err)
		}

		fmt.Printf("🏊 Process pool started with %d workers\n", workerSize)

		// Set up signal handling
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Wait for shutdown signal
		<-sigCh
		fmt.Println("\n📴 Shutting down process pool...")

		if err := pool.Stop(); err != nil {
			log.Printf("Error stopping pool: %v", err)
		}

		fmt.Println("✅ Process pool stopped")

	case "test":
		workerSize := 4
		testJobs := 100

		if len(args) > 1 {
			if _, err := fmt.Sscanf(args[1], "%d", &workerSize); err != nil {
				log.Printf("Warning: Invalid worker size, using default: %v", err)
			}
		}
		if len(args) > 2 {
			if _, err := fmt.Sscanf(args[2], "%d", &testJobs); err != nil {
				log.Printf("Warning: Invalid test jobs, using default: %v", err)
			}
		}

		store := createStore(format, indexType)
		pool := pool.NewProcessPool(workerSize, 1000, store)

		if err := pool.Start(); err != nil {
			log.Fatalf("Failed to start process pool: %v", err)
		}
		defer func() {
			if err := pool.Stop(); err != nil {
				log.Printf("Error stopping pool: %v", err)
			}
		}()

		fmt.Printf("🧪 Testing process pool with %d workers, %d jobs\n", workerSize, testJobs)

		start := time.Now()
		successCount := 0

		for i := 0; i < testJobs; i++ {
			key := fmt.Sprintf("test_key_%d", i)
			value := fmt.Sprintf("test_value_%d", i)

			result, err := pool.SubmitJob("put", key, value)
			if err != nil {
				fmt.Printf("❌ Job %d submission failed: %v\n", i+1, err)
			} else if result.Success {
				successCount++
			}
		}

		duration := time.Since(start)
		stats := pool.GetStats()

		fmt.Printf("\n📊 Pool Test Results:\n")
		fmt.Printf("  Test jobs: %d\n", testJobs)
		fmt.Printf("  Successful: %d\n", successCount)
		fmt.Printf("  Failed: %d\n", testJobs-successCount)
		fmt.Printf("  Total time: %v\n", duration)
		fmt.Printf("  Jobs/sec: %.2f\n", float64(testJobs)/duration.Seconds())
		fmt.Printf("  Pool stats: %+v\n", stats)

	default:
		fmt.Printf("Unknown pool command: %s\n", subcommand)
		fmt.Println("Available commands: start, test")
		os.Exit(1)
	}
}

// createStore creates a KVStore with the specified configuration
func createStore(format, indexType string) *kvstore.KVStore {
	storageConfig := kvstore.StorageConfig{
		Format:     format,
		TextFile:   "moz.log",
		BinaryFile: "moz.bin",
		IndexType:  indexType,
		IndexFile:  "moz.idx",
	}

	compactionConfig := kvstore.CompactionConfig{
		Enabled:         true,
		MaxFileSize:     1024 * 1024, // 1MB
		MaxOperations:   1000,
		CompactionRatio: 0.5,
	}

	return kvstore.NewWithConfig(compactionConfig, storageConfig)
}

// StoreInterface defines the common interface for both regular and partitioned stores
type StoreInterface interface {
	Put(key, value string) error
	Get(key string) (string, error)
	Delete(key string) error
	List() ([]string, error)
	Compact() error
}

// ExtendedStoreInterface extends StoreInterface with additional methods
type ExtendedStoreInterface interface {
	StoreInterface
	GetRange(start, end string) (map[string]string, error)
	PrefixSearch(prefix string) (map[string]string, error)
	ListSorted() ([]string, error)
	GetCompactionStats() (kvstore.CompactionStats, error)
	GetIndexStats() (map[string]interface{}, error)
	RebuildIndex() error
	ValidateIndex() error
}

func createStoreOrPartitioned(format, indexType string, partitions int) StoreInterface {
	if partitions <= 1 {
		return createStore(format, indexType)
	}

	// Validate partition count
	if partitions > 16 {
		log.Printf("Warning: partition count %d exceeds maximum 16, using 16", partitions)
		partitions = 16
	}

	config := kvstore.PartitionConfig{
		NumPartitions: partitions,
		DataDir:       ".",
		BatchSize:     100,
		FlushInterval: 100 * time.Millisecond,
	}

	store, err := kvstore.NewPartitionedKVStore(config)
	if err != nil {
		log.Fatalf("Failed to create partitioned store: %v", err)
	}

	return store
}

func printUsage() {
	fmt.Println("🔨 Moz KVストア - コマンドライン使用法:")
	fmt.Println("")
	fmt.Println("Global Flags:")
	fmt.Println("  --format <text|binary>  - ストレージフォーマット指定 (default: text)")
	fmt.Println("  --index <hash|btree|none> - インデックス方式指定 (default: none)")
	fmt.Println("  --daemon                - デーモンモード使用（高性能）")
	fmt.Println("  --local                 - ローカル実行強制（デーモンバイパス）")
	fmt.Println("  --help                  - ヘルプメッセージ表示")
	fmt.Println("")
	fmt.Println("基本操作:")
	fmt.Println("  moz put <key> <value>  - キー・バリューの保存")
	fmt.Println("  moz get <key>          - キーの値を取得")
	fmt.Println("  moz del <key>          - キーを削除")
	fmt.Println("  moz list               - 全キー・バリューを表示")
	fmt.Println("  moz help               - ヘルプメッセージ表示")
	fmt.Println("")
	fmt.Println("🚀 高性能モード:")
	fmt.Println("  moz daemon start       - デーモン開始（9倍高速化）")
	fmt.Println("  moz daemon stop        - デーモン停止")
	fmt.Println("  moz daemon status      - デーモン状態確認")
	fmt.Println("  moz batch put key1 val1 put key2 val2 - バッチ処理（30倍高速化）")
	fmt.Println("  moz pool start 8       - プロセスプール開始（8ワーカー）")
	fmt.Println("  moz pool test 4 100    - プール性能テスト（4ワーカー、100ジョブ）")
	fmt.Println("")
	fmt.Println("高速検索操作:")
	fmt.Println("  moz range <start> <end> - 範囲検索")
	fmt.Println("  moz prefix <prefix>     - プレフィックス検索")
	fmt.Println("  moz sorted              - ソート済み一覧")
	fmt.Println("")
	fmt.Println("クエリ言語:")
	fmt.Println("  moz query \"SELECT * FROM moz WHERE key = 'value'\"")
	fmt.Println("  moz query \"SELECT * FROM moz WHERE key LIKE 'user%'\"")
	fmt.Println("  moz query \"SELECT COUNT(*) FROM moz WHERE value CONTAINS 'admin'\"")
	fmt.Println("")
	fmt.Println("管理操作:")
	fmt.Println("  moz compact            - ストレージ最適化")
	fmt.Println("  moz stats              - ストレージ統計表示")
	fmt.Println("  moz rebuild-index      - インデックス再構築")
	fmt.Println("  moz validate-index     - インデックス検証")
	fmt.Println("")
	fmt.Println("フォーマット操作:")
	fmt.Println("  moz convert <from> <to> - フォーマット変換 (text ↔ binary)")
	fmt.Println("  moz validate <format>   - ファイル整合性検証")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  moz daemon start                    # デーモン開始")
	fmt.Println("  moz --daemon put user alice         # デーモン経由で高速保存")
	fmt.Println("  moz batch put user1 alice put user2 bob get user1  # バッチ処理")
	fmt.Println("  moz --format=binary put key value   # バイナリ形式で保存")
	fmt.Println("  moz --index=hash put user alice     # Hash Index使用")
	fmt.Println("  moz --index=btree range a z         # B-Tree Index範囲検索")
	fmt.Println("  moz query \"SELECT * FROM moz WHERE key LIKE 'user%'\" # SQLライククエリ")
	fmt.Println("")
	fmt.Println("🎯 Performance Tips:")
	fmt.Println("  • デーモンモードで9倍高速化: moz daemon start")
	fmt.Println("  • バッチ処理で30倍高速化: moz batch <operations>")
	fmt.Println("  • Hash/B-Treeインデックスで30倍高速検索")
	fmt.Println("  • 自動最適化：デーモンが起動中なら自動で高速実行")
}
