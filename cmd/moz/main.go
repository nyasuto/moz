package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nyasuto/moz/internal/kvstore"
)

func main() {
	var format = flag.String("format", "text", "Storage format: text or binary")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]

	// Create store with specified format
	storageConfig := kvstore.StorageConfig{
		Format:     *format,
		TextFile:   "moz.log",
		BinaryFile: "moz.bin",
	}

	compactionConfig := kvstore.CompactionConfig{
		Enabled:         true,
		MaxFileSize:     1024 * 1024, // 1MB
		MaxOperations:   1000,
		CompactionRatio: 0.5,
	}

	store := kvstore.NewWithConfig(compactionConfig, storageConfig)

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
		fmt.Printf("✅ Stored: %s = %s\n", key, value)

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
		stats, err := store.GetCompactionStats()
		if err != nil {
			log.Fatalf("Error getting compaction stats: %v", err)
		}
		fmt.Printf("📊 Storage Statistics:\n")
		fmt.Printf("  Format: %s\n", storageConfig.Format)
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

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("🔨 Moz KVストア - コマンドライン使用法:")
	fmt.Println("")
	fmt.Println("Global Flags:")
	fmt.Println("  --format <text|binary>  - ストレージフォーマット指定 (default: text)")
	fmt.Println("")
	fmt.Println("基本操作:")
	fmt.Println("  moz put <key> <value>  - キー・バリューの保存")
	fmt.Println("  moz get <key>          - キーの値を取得")
	fmt.Println("  moz del <key>          - キーを削除")
	fmt.Println("  moz list               - 全キー・バリューを表示")
	fmt.Println("")
	fmt.Println("管理操作:")
	fmt.Println("  moz compact            - ストレージ最適化")
	fmt.Println("  moz stats              - ストレージ統計表示")
	fmt.Println("")
	fmt.Println("フォーマット操作:")
	fmt.Println("  moz convert <from> <to> - フォーマット変換 (text ↔ binary)")
	fmt.Println("  moz validate <format>   - ファイル整合性検証")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  moz --format=binary put key value  # バイナリ形式で保存")
	fmt.Println("  moz convert text binary             # テキスト→バイナリ変換")
	fmt.Println("  moz validate binary                 # バイナリファイル検証")
}
