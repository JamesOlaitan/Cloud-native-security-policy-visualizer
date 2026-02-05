package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/jamesolaitan/accessgraph/internal/config"
	"github.com/jamesolaitan/accessgraph/internal/graph"
	redactlog "github.com/jamesolaitan/accessgraph/internal/log"
	"github.com/jamesolaitan/accessgraph/internal/policy"
	"github.com/jamesolaitan/accessgraph/internal/reco"
	"github.com/jamesolaitan/accessgraph/internal/store"
)

func main() {
	log.SetOutput(&redactlog.RedactWriter{Out: os.Stderr})

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg := config.Load()
	ctx := context.Background()

	command := os.Args[1]

	switch command {
	case "snapshots":
		handleSnapshots(ctx, cfg)
	case "findings":
		handleFindings(ctx, cfg)
	case "graph":
		handleGraph(ctx, cfg)
	case "attack-path":
		handleAttackPath(ctx, cfg)
	case "recommend":
		handleRecommend(ctx, cfg)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`AccessGraph CLI v1.1.0

Usage:
  accessgraph-cli snapshots ls
  accessgraph-cli snapshots diff --a <idA> --b <idB>
  accessgraph-cli findings --snapshot <id> [--format table|json]
  accessgraph-cli graph path --from <principalID> --to <resourceID>
  accessgraph-cli graph export --snapshot <id> --format cypher --out <file>
  accessgraph-cli attack-path --from <id> [--to <id>] [--tag sensitive] [--max-hops 8] [--out path.md] [--sarif findings.sarif]
  accessgraph-cli recommend --snapshot <id> --policy <policyId> [--target <id>] [--tag sensitive] [--cap 20] [--out reco.json]
`)
}

func handleSnapshots(ctx context.Context, cfg *config.Config) {
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
		snapshots, err := st.ListSnapshots(ctx)
		if err != nil {
			log.Fatalf("Failed to list snapshots: %v", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tCREATED\tLABEL\tNODES\tEDGES")
		for _, snap := range snapshots {
			nodeCount, err := st.CountNodes(ctx, snap.ID)
			if err != nil {
				log.Fatalf("Failed to count nodes for snapshot %s: %v", snap.ID, err)
			}

			edgeCount, err := st.CountEdges(ctx, snap.ID)
			if err != nil {
				log.Fatalf("Failed to count edges for snapshot %s: %v", snap.ID, err)
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\n",
				snap.ID, snap.CreatedAt.Format("2006-01-02 15:04:05"),
				snap.Label, nodeCount, edgeCount)
		}
		w.Flush()

	case "diff":
		fs := flag.NewFlagSet("diff", flag.ExitOnError)
		idA := fs.String("a", "", "First snapshot ID")
		idB := fs.String("b", "", "Second snapshot ID")
		if err := fs.Parse(os.Args[3:]); err != nil {
			log.Fatalf("Failed to parse flags: %v", err)
		}

		if *idA == "" || *idB == "" {
			fmt.Println("Usage: accessgraph-cli snapshots diff --a <idA> --b <idB>")
			os.Exit(1)
		}

		edgesA, err := st.GetEdges(ctx, *idA)
		if err != nil {
			log.Fatalf("Failed to get edges for %s: %v", *idA, err)
		}

		edgesB, err := st.GetEdges(ctx, *idB)
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

func handleFindings(ctx context.Context, cfg *config.Config) {
	fs := flag.NewFlagSet("findings", flag.ExitOnError)
	snapshotID := fs.String("snapshot", "", "Snapshot ID")
	format := fs.String("format", "table", "Output format (table|json)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if *snapshotID == "" {
		fmt.Println("Usage: accessgraph-cli findings --snapshot <id> [--format table|json]")
		os.Exit(1)
	}

	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer st.Close()

	g, err := st.LoadSnapshot(ctx, *snapshotID)
	if err != nil {
		log.Fatalf("Failed to load snapshot: %v", err)
	}

	input := policy.BuildInput(g)

	opaClient := policy.NewClient(cfg.OPAUrl)
	findings, err := opaClient.Evaluate(ctx, input)
	if err != nil {
		log.Fatalf("Failed to evaluate policies: %v", err)
	}

	if *format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(findings); err != nil {
			log.Fatalf("Failed to encode findings: %v", err)
		}
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "RULE_ID\tSEVERITY\tENTITY\tREASON")
		for _, f := range findings {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.RuleID, f.Severity, f.EntityRef, f.Reason)
		}
		w.Flush()
	}
}

func handleGraph(ctx context.Context, cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Println("Usage: accessgraph-cli graph <path|export>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "path":
		handleGraphPath(ctx, cfg)
	case "export":
		handleGraphExport(ctx, cfg)
	default:
		fmt.Printf("Unknown subcommand: %s\n", subcommand)
		fmt.Println("Usage: accessgraph-cli graph <path|export>")
		os.Exit(1)
	}
}

func handleGraphPath(ctx context.Context, cfg *config.Config) {
	fs := flag.NewFlagSet("path", flag.ExitOnError)
	from := fs.String("from", "", "Source node ID")
	to := fs.String("to", "", "Destination node ID")
	if err := fs.Parse(os.Args[3:]); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

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
	snapshots, err := st.ListSnapshots(ctx)
	if err != nil {
		log.Fatalf("Failed to list snapshots: %v", err)
	}
	if len(snapshots) == 0 {
		log.Fatal("No snapshots found")
	}

	snapshotID := snapshots[0].ID

	g, err := st.LoadSnapshot(ctx, snapshotID)
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

func handleGraphExport(ctx context.Context, cfg *config.Config) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	snapshotID := fs.String("snapshot", "", "Snapshot ID")
	format := fs.String("format", "cypher", "Export format (cypher)")
	outFile := fs.String("out", "", "Output file")
	if err := fs.Parse(os.Args[3:]); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if *snapshotID == "" || *outFile == "" {
		fmt.Println("Usage: accessgraph-cli graph export --snapshot <id> --format cypher --out <file>")
		os.Exit(1)
	}

	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer st.Close()

	g, err := st.LoadSnapshot(ctx, *snapshotID)
	if err != nil {
		log.Fatalf("Failed to load snapshot: %v", err)
	}

	var content string

	switch *format {
	case "cypher":
		content, err = g.ExportCypher()
		if err != nil {
			log.Fatalf("Failed to export Cypher: %v", err)
		}
	default:
		log.Fatalf("Unknown format: %s", *format)
	}

	if err := os.WriteFile(*outFile, []byte(content), 0644); err != nil {
		log.Fatalf("Failed to write file: %v", err)
	}

	fmt.Printf("Exported snapshot %s to %s (%d bytes)\n", *snapshotID, *outFile, len(content))
}

func handleAttackPath(ctx context.Context, cfg *config.Config) {
	fs := flag.NewFlagSet("attack-path", flag.ExitOnError)
	from := fs.String("from", "", "Source principal ID")
	to := fs.String("to", "", "Destination resource ID (optional with --tag)")
	tag := fs.String("tag", "", "Tag filter (e.g., 'sensitive')")
	maxHops := fs.Int("max-hops", 8, "Maximum hops")
	outMD := fs.String("out", "", "Output Markdown file")
	outSARIF := fs.String("sarif", "", "Output SARIF file")
	formatFlag := fs.String("format", "table", "Output format (table|json)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if *from == "" {
		fmt.Println("Usage: accessgraph-cli attack-path --from <id> [--to <id>] [--tag sensitive] [--max-hops 8] [--out path.md] [--sarif findings.sarif]")
		os.Exit(1)
	}

	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer st.Close()

	// Get most recent snapshot
	snapshots, err := st.ListSnapshots(ctx)
	if err != nil {
		log.Fatalf("Failed to list snapshots: %v", err)
	}
	if len(snapshots) == 0 {
		log.Fatal("No snapshots found")
	}

	snapshotID := snapshots[0].ID

	g, err := st.LoadSnapshot(ctx, snapshotID)
	if err != nil {
		log.Fatalf("Failed to load snapshot: %v", err)
	}

	// Build tags
	tags := []string{}
	if *tag != "" {
		tags = append(tags, *tag)
	}

	// Find attack path
	result, err := g.FindAttackPath(*from, *to, tags, *maxHops)
	if err != nil {
		log.Fatalf("Failed to find attack path: %v", err)
	}

	if !result.Found {
		fmt.Println("No attack path found")
		os.Exit(0)
	}

	// Display path
	if *formatFlag == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]interface{}{
			"found": result.Found,
			"nodes": result.Nodes,
			"edges": result.Edges,
			"hops":  len(result.Nodes) - 1,
		}); err != nil {
			log.Fatalf("Failed to encode output: %v", err)
		}
	} else {
		targetID := *to
		if targetID == "" && len(result.Nodes) > 0 {
			targetID = result.Nodes[len(result.Nodes)-1].ID
		}
		fmt.Printf("Attack Path: %s → %s (hops: %d)\n\n", *from, targetID, len(result.Nodes)-1)

		for i, node := range result.Nodes {
			fmt.Printf("%d. %s [%s]\n", i+1, node.ID, node.Kind)
			if i < len(result.Edges) {
				fmt.Printf("   --[%s]-->\n", result.Edges[i].Kind)
			}
		}
	}

	// Export to Markdown if requested
	if *outMD != "" {
		targetID := *to
		if targetID == "" && len(result.Nodes) > 0 {
			targetID = result.Nodes[len(result.Nodes)-1].ID
		}
		markdown, err := graph.ExportMarkdownAttackPath(*from, targetID, result.Nodes, result.Edges)
		if err != nil {
			log.Fatalf("Failed to export Markdown: %v", err)
		}

		if err := os.WriteFile(*outMD, []byte(markdown), 0644); err != nil {
			log.Fatalf("Failed to write Markdown file: %v", err)
		}

		fmt.Printf("\nMarkdown report saved to: %s\n", *outMD)
	}

	// Export to SARIF if requested
	if *outSARIF != "" {
		targetID := *to
		if targetID == "" && len(result.Nodes) > 0 {
			targetID = result.Nodes[len(result.Nodes)-1].ID
		}
		sarif, err := graph.ExportSARIFAttackPath(*from, targetID, result.Nodes, result.Edges)
		if err != nil {
			log.Fatalf("Failed to export SARIF: %v", err)
		}

		if err := os.WriteFile(*outSARIF, []byte(sarif), 0644); err != nil {
			log.Fatalf("Failed to write SARIF file: %v", err)
		}

		fmt.Printf("SARIF report saved to: %s\n", *outSARIF)
	}
}

func handleRecommend(ctx context.Context, cfg *config.Config) {
	fs := flag.NewFlagSet("recommend", flag.ExitOnError)
	snapshotID := fs.String("snapshot", "", "Snapshot ID")
	policyID := fs.String("policy", "", "Policy ID")
	target := fs.String("target", "", "Target resource ID (optional with --tag)")
	tag := fs.String("tag", "", "Tag filter (e.g., 'sensitive')")
	cap := fs.Int("cap", 20, "Maximum suggestions")
	outFile := fs.String("out", "", "Output JSON file")
	formatFlag := fs.String("format", "table", "Output format (table|json)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if *snapshotID == "" || *policyID == "" {
		fmt.Println("Usage: accessgraph-cli recommend --snapshot <id> --policy <policyId> [--target <id>] [--tag sensitive] [--cap 20] [--out reco.json]")
		os.Exit(1)
	}

	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer st.Close()

	g, err := st.LoadSnapshot(ctx, *snapshotID)
	if err != nil {
		log.Fatalf("Failed to load snapshot: %v", err)
	}

	recommender := reco.New(g)

	// Build tags
	tags := []string{}
	if *tag != "" {
		tags = append(tags, *tag)
	}

	rec, err := recommender.Recommend(*policyID, *target, tags, *cap)
	if err != nil {
		log.Fatalf("Failed to generate recommendation: %v", err)
	}

	// Display recommendation
	if *formatFlag == "json" || *outFile != "" {
		data, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal JSON: %v", err)
		}

		if *outFile != "" {
			if err := os.WriteFile(*outFile, data, 0644); err != nil {
				log.Fatalf("Failed to write file: %v", err)
			}
			fmt.Printf("Recommendation saved to: %s\n", *outFile)
		} else {
			fmt.Println(string(data))
		}
	} else {
		fmt.Printf("Least-Privilege Recommendation\n\n")
		fmt.Printf("Policy: %s\n\n", rec.PolicyID)
		fmt.Printf("Rationale:\n%s\n\n", rec.Rationale)

		if len(rec.SuggestedActions) > 0 {
			fmt.Printf("Suggested Actions (%d):\n", len(rec.SuggestedActions))
			for _, action := range rec.SuggestedActions {
				fmt.Printf("  - %s\n", action)
			}
			fmt.Println()
		}

		if len(rec.SuggestedResources) > 0 {
			fmt.Printf("Suggested Resources (%d):\n", len(rec.SuggestedResources))
			for _, resource := range rec.SuggestedResources {
				fmt.Printf("  - %s\n", resource)
			}
			fmt.Println()
		}

		fmt.Printf("JSON Patch:\n%s\n", rec.PatchJSON)
	}
}
