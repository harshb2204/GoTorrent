package main

import (
	"bittorrent/download"
	"bittorrent/torrentparser"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <torrent-file> [output-path]")
		os.Exit(1)
	}

	torrentFile := os.Args[1]

	torrent, err := torrentparser.Open(torrentFile)
	if err != nil {
		fmt.Printf("Error opening torrent file: %v\n", err)
		os.Exit(1)
	}

	// Determine output path
	downloadPath := torrent.Info.Name
	if len(os.Args) >= 3 {
		downloadPath = os.Args[2]
	}

	// For single-file torrents, don't nest inside a directory with the same name
	if len(torrent.Info.Files) == 0 {
		// Single file — downloadPath is the file itself
		fmt.Printf("Downloading file: %s\n", downloadPath)
	} else {
		// Multi-file — downloadPath is the directory
		downloadPath = filepath.Clean(downloadPath)
		fmt.Printf("Downloading to directory: %s\n", downloadPath)
	}

	err = download.Download(torrent, downloadPath)
	if err != nil {
		fmt.Printf("Error during download: %v\n", err)
		os.Exit(1)
	}
}
