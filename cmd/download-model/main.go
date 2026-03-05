// cmd/download-model/main.go
// One-shot utility to pre-download the ONNX model before starting the server.
// Usage: go run ./cmd/download-model/  [MODEL_DIR]
//
// By default downloads to ./models/
// The server also auto-downloads on startup if MODEL_DIR is set and model is absent.

package main

import (
	"fmt"
	"os"

	"github.com/knights-analytics/hugot"
)

const modelID = "sentence-transformers/all-MiniLM-L6-v2"

func main() {
	dest := "./models"
	if len(os.Args) > 1 {
		dest = os.Args[1]
	}

	fmt.Printf("⬇ Downloading %s → %s\n", modelID, dest)

	session, err := hugot.NewSession(hugot.WithTelemetry(false))
	if err != nil {
		fmt.Fprintf(os.Stderr, "hugot session: %v\n", err)
		os.Exit(1)
	}
	defer session.Destroy()

	path, err := hugot.DownloadModel(session, modelID, dest, hugot.NewDownloadOptions())
	if err != nil {
		fmt.Fprintf(os.Stderr, "download: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Model ready at: %s\n", path)
	fmt.Println("Set MODEL_DIR=" + dest + " in your .env to enable ONNX embedding.")
}
