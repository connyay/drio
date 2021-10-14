package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"

	"github.com/connyay/drio/cs"
	"github.com/davecgh/go-spew/spew"
)

func main() {
	docBytes, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatalf("failed reading %v", err)
	}
	doc, err := cs.Parse(docBytes)
	if err != nil {
		log.Fatalf("failed parsing %v", err)
	}
	requester := fmt.Sprintf("%x", sha256.Sum256([]byte("127.0.0.1")))
	spew.Dump(doc.ToStore(requester))
}
