package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	oi "github.com/kpiljoong/orderlyid"
)

func main() {
	var (
		prefix    = flag.String("prefix", "", "type prefix (e.g., order, user)")
		tenant    = flag.Uint("tenant", 0, "tenant id (0-65535)")
		shard     = flag.Uint("shard", 0, "shard id (0-65535)")
		shardFrom = flag.String("shard-from", "", "derive shard from bytes of this string (overrides -shard)")
		checksum  = flag.Bool("checksum", false, "append 4-char checksum")
		bucket    = flag.Int("bucket", 0, "bucket seconds for time privacy (0 = none)")
		count     = flag.Int("n", 1, "how many IDs to generate")
		parseOnly = flag.String("parse", "", "parse and inspect an existing OrderlyID")
		tamper    = flag.String("tamper", "", "modify the last char and attempt parse (should fail)")
	)
	flag.Parse()

	// Tamper demo: flips last char so checksum fails.
	if *tamper != "" {
		bad := *tamper
		if len(bad) == 0 {
			log.Fatalf("empty -tamper value")
		}
		last := bad[len(bad)-1]
		repl := byte('0')
		if last == '0' {
			repl = '1'
		}
		bad = bad[:len(bad)-1] + string(repl)
		fmt.Println("Tampered:", bad)
		if _, err := oi.Parse(bad); err != nil {
			fmt.Println("Parse error (expected):", err)
		} else {
			fmt.Println("Unexpected: checksum accepted")
			os.Exit(1)
		}
		return
	}

	// Parse path: verifies checksum automatically if present.
	if *parseOnly != "" {
		p, err := oi.Parse(*parseOnly)
		if err != nil {
			log.Fatalf("parse error: %v", err)
		}
		printParsed(*parseOnly, p)
		return
	}

	if *prefix == "" {
		fmt.Fprintf(os.Stderr, "usage: %s -prefix <type> [-tenant N] [-shard N|-shard-from STR] [-checksum] [-bucket SEC] [-n N]\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "       or:  -parse <id>   (verify and inspect)")
		fmt.Fprintln(os.Stderr, "       or:  -tamper <id>  (intentionally break checksum)")
		os.Exit(2)
	}

	var opts []oi.Option
	if *tenant > 0 {
		opts = append(opts, oi.WithTenant(uint16(*tenant)))
	}
	if *shardFrom != "" {
		opts = append(opts, oi.WithShardFromBytes([]byte(*shardFrom)))
	} else if *shard > 0 {
		opts = append(opts, oi.WithShard(uint16(*shard)))
	}
	if *checksum {
		opts = append(opts, oi.WithChecksum(true))
	}
	if *bucket > 0 {
		opts = append(opts, oi.WithBucketSeconds(*bucket))
	}

	for i := 0; i < *count; i++ {
		id := oi.New(*prefix, opts...)
		p, err := oi.Parse(id) // sanity check (also verifies checksum if present)
		if err != nil {
			log.Fatalf("internal parse error: %v", err)
		}
		fmt.Printf("%-34s  %s  tenant=%-5d shard=%-5d seq=%d\n",
			id,
			time.UnixMilli(p.TimeMs).UTC().Format(time.RFC3339Nano),
			p.Tenant, p.Shard, p.Seq)
	}
}

func printParsed(id string, p *oi.Parsed) {
	fmt.Printf("ID:         %s\n", id)
	fmt.Printf("prefix:     %s\n", p.Prefix)
	fmt.Printf("time (ms):  %d\n", p.TimeMs)
	fmt.Printf("time (iso): %s\n", time.UnixMilli(p.TimeMs).UTC().Format(time.RFC3339Nano))
	fmt.Printf("flags:      0x%02x\n", p.Flags)
	fmt.Printf("tenant:     %d\n", p.Tenant)
	fmt.Printf("seq:        %d\n", p.Seq)
	fmt.Printf("shard:      %d\n", p.Shard)
	fmt.Printf("random60:   0x%016x\n", p.Random)
}
