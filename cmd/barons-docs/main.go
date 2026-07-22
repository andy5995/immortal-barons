// Command barons-docs assembles the documentation website's MkDocs source tree
// from the repo's committed Markdown (README, docs/, and the in-game help
// content), so the site and the in-game help share one source of truth. It is
// build-time tooling for CI, not part of the game.
//
//	go run ./cmd/barons-docs            # assemble into ./build/docs
//	go run ./cmd/barons-docs -out DIR   # assemble into DIR
//
// Then: mkdocs build (reads <out>/mkdocs.yml).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/andy5995/immortal-barons/internal/docsite"
)

func main() {
	repoRoot := flag.String("root", ".", "repository root to read doc sources from")
	out := flag.String("out", "build/docs", "output directory for site-src/ and mkdocs.yml")
	flag.Parse()

	if err := docsite.Assemble(*repoRoot, *out); err != nil {
		fmt.Fprintln(os.Stderr, "barons-docs:", err)
		os.Exit(1)
	}
	fmt.Printf("assembled MkDocs source into %s (site-src/ + mkdocs.yml)\n", *out)
}
