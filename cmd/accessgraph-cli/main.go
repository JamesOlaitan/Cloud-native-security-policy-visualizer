package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/jamesolaitan/accessgraph/internal/config"
	"github.com/jamesolaitan/accessgraph/internal/policy"
	"github.com/jamesolaitan/accessgraph/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg := config.Load()

	command := os.Args[1]

	switch command {
	case "snapshots":
		handleSnapshots(cfg)
	case "findings":
		handleFindings(cfg)
	case "graph":
		handleGraph(cfg)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`AccessGraph CLI

Usage:
  accessgraph-cli snapshots ls
  accessgraph-cli snapshots diff --a <idA> --b <idB>
  accessgraph-cli findings --snapshot <id> [--format table|json]
  accessgraph-cli graph path --from <principalID> --to <resourceID>
`)
}

func handleSnapshots(cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Println("Usage: accessgraph-cli snapshots <ls|diff>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer st.Close()

	switch subcommand {
	case "ls":
		snapshots, err := st.ListSnapshots()
		if err != nil {
			log.Fatalf("Failed to list snapshots: %v", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tCREATED\tLABEL\tNODES\tEDGES")
		for _, snap := range snapshots {
			nodeCount, _ := st.CountNodes(snap.ID)
			edgeCount, _ := st.CountEdges(snap.ID)
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\n",
				snap.ID, snap.CreatedAt.Format("2006-01-02 15:04:05"),
				snap.Label, nodeCount, edgeCount)
		}
		w.Flush()

	case "diff":
		fs := flag.NewFlagSet("diff", flag.ExitOnError)
		idA := fs.String("a", "", "First snapshot ID")
		idB := fs.String("b", "", "Second snapshot ID")
		_ = fs.Parse(os.Args[3:])

		if *idA == "" || *idB == "" {
			fmt.Println("Usage: accessgraph-cli snapshots diff --a <idA> --b <idB>")
			os.Exit(1)
		}

		edgesA, err := st.GetEdges(*idA)
		if err != nil {
			log.Fatalf("Failed to get edges for %s: %v", *idA, err)
		}

		edgesB, err := st.GetEdges(*idB)
		if err != nil {
			log.Fatalf("Failed to get edges for %s: %v", *idB, err)
		}

		// Create edge maps
		edgeMapA := make(map[string]bool)
		edgeMapB := make(map[string]bool)

		for _, e := range edgesA {
			key := fmt.Sprintf("%s->%s:%s", e.Src, e.Dst, e.Kind)
			edgeMapA[key] = true
		}

		for _, e := range edgesB {
			key := fmt.Sprintf("%s->%s:%s", e.Src, e.Dst, e.Kind)
			edgeMapB[key] = true
		}

		// Find added and removed
		added := []string{}
		removed := []string{}

		for key := range edgeMapB {
			if !edgeMapA[key] {
				added = append(added, key)
			}
		}

		for key := range edgeMapA {
			if !edgeMapB[key] {
				removed = append(removed, key)
			}
		}

		fmt.Printf("Snapshot Diff: %s vs %s\n\n", *idA, *idB)
		fmt.Printf("Summary:\n")
		fmt.Printf("  Added: %d\n", len(added))
		fmt.Printf("  Removed: %d\n", len(removed))
		fmt.Printf("  Changed: %d\n\n", 0)

		if len(added) > 0 {
			fmt.Println("Added edges:")
			for _, edge := range added {
				fmt.Printf("  + %s\n", edge)
			}
			fmt.Println()
		}

		if len(removed) > 0 {
			fmt.Println("Removed edges:")
			for _, edge := range removed {
				fmt.Printf("  - %s\n", edge)
			}
		}

	default:
		fmt.Printf("Unknown subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

func handleFindings(cfg *config.Config) {
	fs := flag.NewFlagSet("findings", flag.ExitOnError)
	snapshotID := fs.String("snapshot", "", "Snapshot ID")
	format := fs.String("format", "table", "Output format (table|json)")
	_ = fs.Parse(os.Args[2:])

	if *snapshotID == "" {
		fmt.Println("Usage: accessgraph-cli findings --snapshot <id> [--format table|json]")
		os.Exit(1)
	}

	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer st.Close()

	g, err := st.LoadSnapshot(*snapshotID)
	if err != nil {
		log.Fatalf("Failed to load snapshot: %v", err)
	}

	input := policy.BuildInput(g)

	opaClient := policy.NewClient(cfg.OPAUrl)
	findings, err := opaClient.Evaluate(input)
	if err != nil {
		log.Fatalf("Failed to evaluate policies: %v", err)
	}

	if *format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(findings)
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "RULE_ID\tSEVERITY\tENTITY\tREASON")
		for _, f := range findings {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.RuleID, f.Severity, f.EntityRef, f.Reason)
		}
		w.Flush()
	}
}

func handleGraph(cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Println("Usage: accessgraph-cli graph path --from <principalID> --to <resourceID>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	if subcommand != "path" {
		fmt.Printf("Unknown subcommand: %s\n", subcommand)
		os.Exit(1)
	}

	fs := flag.NewFlagSet("path", flag.ExitOnError)
	from := fs.String("from", "", "Source node ID")
	to := fs.String("to", "", "Destination node ID")
	_ = fs.Parse(os.Args[3:])

	if *from == "" || *to == "" {
		fmt.Println("Usage: accessgraph-cli graph path --from <principalID> --to <resourceID>")
		os.Exit(1)
	}

	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer st.Close()

	// Get most recent snapshot
	snapshots, err := st.ListSnapshots()
	if err != nil {
		log.Fatalf("Failed to list snapshots: %v", err)
	}
	if len(snapshots) == 0 {
		log.Fatal("No snapshots found")
	}

	snapshotID := snapshots[0].ID

	g, err := st.LoadSnapshot(snapshotID)
	if err != nil {
		log.Fatalf("Failed to load snapshot: %v", err)
	}

	nodes, edges, err := g.ShortestPath(*from, *to, 8)
	if err != nil {
		log.Fatalf("Failed to find path: %v", err)
	}

	fmt.Printf("Path from %s to %s (length: %d):\n\n", *from, *to, len(nodes))

	for i, node := range nodes {
		fmt.Printf("%d. %s [%s]\n", i+1, node.ID, node.Kind)
		if i < len(edges) {
			fmt.Printf("   --[%s]-->\n", edges[i].Kind)
		}
	}
}
