package main

import (
	"fmt"
	"log"
	"os"

	"github.com/nyasuto/moz/internal/kvstore"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	store := kvstore.New()

	switch command {
	case "put":
		if len(os.Args) != 4 {
			fmt.Println("Usage: moz put <key> <value>")
			os.Exit(1)
		}
		key, value := os.Args[2], os.Args[3]
		if err := store.Put(key, value); err != nil {
			log.Fatalf("Error putting key-value: %v", err)
		}
		fmt.Printf("✅ Stored: %s = %s\n", key, value)

	case "get":
		if len(os.Args) != 3 {
			fmt.Println("Usage: moz get <key>")
			os.Exit(1)
		}
		key := os.Args[2]
		value, err := store.Get(key)
		if err != nil {
			log.Fatalf("Error getting key: %v", err)
		}
		fmt.Printf("%s\n", value)

	case "del", "delete":
		if len(os.Args) != 3 {
			fmt.Println("Usage: moz del <key>")
			os.Exit(1)
		}
		key := os.Args[2]
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

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("🔨 Moz KVストア - コマンドライン使用法:")
	fmt.Println("")
	fmt.Println("基本操作:")
	fmt.Println("  moz put <key> <value>  - キー・バリューの保存")
	fmt.Println("  moz get <key>          - キーの値を取得")
	fmt.Println("  moz del <key>          - キーを削除")
	fmt.Println("  moz list               - 全キー・バリューを表示")
	fmt.Println("")
	fmt.Println("管理操作:")
	fmt.Println("  moz compact            - ストレージ最適化")
}