package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jamesolaitan/accessgraph/internal/config"
	"github.com/jamesolaitan/accessgraph/internal/graph"
	"github.com/jamesolaitan/accessgraph/internal/ingest"
	logpkg "github.com/jamesolaitan/accessgraph/internal/log"
	"github.com/jamesolaitan/accessgraph/internal/store"
)

func main() {
	var (
		awsDir     = flag.String("aws", "", "Path to AWS JSON directory")
		k8sDir     = flag.String("k8s", "", "Path to Kubernetes YAML directory")
		tfPlanPath = flag.String("tf", "", "Path to Terraform plan JSON (optional)")
		snapshotID = flag.String("snapshot", "", "Snapshot ID (required)")
	)

	flag.Parse()

	if *snapshotID == "" {
		fmt.Println("Error: --snapshot is required")
		flag.Usage()
		os.Exit(1)
	}

	cfg := config.Load()

	// Configure network mode (IMDS always blocked for security)
	if cfg.Offline {
		config.EnableOfflineMode(true)
		log.Println("Mode: OFFLINE (no network egress, IMDS blocked)")
	} else {
		config.EnableOnlineModeWithIMDSBlock()
		log.Println("Mode: ONLINE (IMDS blocked for security)")
	}

	log.Printf("Starting ingestion for snapshot: %s", logpkg.Redact(*snapshotID))

	// Initialize graph
	g := graph.New()

	var allNodes []ingest.Node
	var allEdges []ingest.Edge

	// Parse AWS if provided
	if *awsDir != "" {
		log.Printf("Parsing AWS IAM from: %s", *awsDir)
		result, err := ingest.ParseAWS(*awsDir)
		if err != nil {
			log.Fatalf("Failed to parse AWS: %v", err)
		}
		allNodes = append(allNodes, result.Nodes...)
		allEdges = append(allEdges, result.Edges...)
		log.Printf("Parsed %d AWS nodes and %d edges", len(result.Nodes), len(result.Edges))
	}

	// Parse K8s if provided
	if *k8sDir != "" {
		log.Printf("Parsing Kubernetes RBAC from: %s", *k8sDir)
		result, err := ingest.ParseK8s(*k8sDir)
		if err != nil {
			log.Fatalf("Failed to parse K8s: %v", err)
		}
		allNodes = append(allNodes, result.Nodes...)
		allEdges = append(allEdges, result.Edges...)
		log.Printf("Parsed %d K8s nodes and %d edges", len(result.Nodes), len(result.Edges))
	}

	// Parse Terraform if provided
	label := *snapshotID
	if *tfPlanPath != "" {
		log.Printf("Parsing Terraform plan from: %s", *tfPlanPath)
		result, isTF, err := ingest.ParseTerraform(*tfPlanPath)
		if err != nil {
			log.Fatalf("Failed to parse Terraform: %v", err)
		}
		if isTF {
			allNodes = append(allNodes, result.Nodes...)
			allEdges = append(allEdges, result.Edges...)
			label = *snapshotID + "-iac"
			log.Printf("Parsed %d Terraform nodes and %d edges", len(result.Nodes), len(result.Edges))
		}
	}

	// Build graph
	log.Println("Building graph...")
	for _, node := range allNodes {
		g.AddNode(node)
	}

	for _, edge := range allEdges {
		if err := g.AddEdge(edge); err != nil {
			// Skip edges with missing nodes
			continue
		}
	}

	log.Printf("Graph built: %d nodes, %d edges", len(allNodes), len(allEdges))

	// Save to SQLite
	log.Printf("Saving snapshot to: %s", cfg.SQLitePath)
	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer st.Close()

	if err := st.SaveSnapshot(*snapshotID, label, g); err != nil {
		log.Fatalf("Failed to save snapshot: %v", err)
	}

	log.Printf("Successfully saved snapshot: %s", *snapshotID)
}
