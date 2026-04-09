package torrentparser

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jackpal/bencode-go"
)

const BLOCK_LENGTH = 16384 // 2^14 bytes

// represents a .torrent file
type TorrentFile struct {
	Announce     string      `bencode:"announce"`      // Tracker URL
	AnnounceList [][]string  `bencode:"announce-list"`  // Multiple tracker URLs
	Info         TorrentInfo `bencode:"info"`           // Info Dictionary
	RawInfoHash  [20]byte   // SHA-1 of the raw info dictionary bytes
}

type TorrentInfo struct {
	Name        string     `bencode:"name"`         // Name of the file/folder
	PieceLength int64      `bencode:"piece length"` // Size of each piece
	Pieces      string     `bencode:"pieces"`       // Binary string of SHA-1 hashes
	Length      int64      `bencode:"length"`       // Single file size
	Files       []FileInfo `bencode:"files"`        // Multiple files (if multi-file torrent)
}

type FileInfo struct {
	Length int64    `bencode:"length"` // File size
	Path   []string `bencode:"path"`  // File path components
}

// Open reads and parses a torrent file
func Open(filePath string) (*TorrentFile, error) {
	fmt.Println("====== opening the torrent file ======")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	torrent := &TorrentFile{}
	err = bencode.Unmarshal(bytes.NewReader(data), torrent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse torrent file: %v", err)
	}

	// Calculate info hash from the raw info dictionary bytes
	// We must hash the original bytes, not re-encoded ones
	var rawTorrent map[string]interface{}
	err = bencode.Unmarshal(bytes.NewReader(data), &rawTorrent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse raw torrent: %v", err)
	}

	infoDict, ok := rawTorrent["info"]
	if !ok {
		return nil, fmt.Errorf("torrent file missing info dictionary")
	}

	// Re-encode just the info dict from the raw parsed map
	// This preserves the original field ordering and values
	var infoBuf bytes.Buffer
	err = bencode.Marshal(&infoBuf, infoDict)
	if err != nil {
		return nil, fmt.Errorf("failed to encode info dict: %v", err)
	}

	torrent.RawInfoHash = sha1.Sum(infoBuf.Bytes())

	// Print torrent info
	fmt.Printf("Torrent Name: %s\n", torrent.Info.Name)
	fmt.Printf("Announce URL: %s\n", torrent.Announce)
	fmt.Printf("Piece Length: %d bytes\n", torrent.Info.PieceLength)
	fmt.Printf("Number of Pieces: %d\n", len(torrent.Info.Pieces)/20)
	fmt.Printf("Info Hash: %x\n", torrent.RawInfoHash)

	if len(torrent.AnnounceList) > 0 {
		fmt.Printf("Tracker tiers: %d\n", len(torrent.AnnounceList))
	}

	if len(torrent.Info.Files) > 0 {
		fmt.Printf("Files: %d\n", len(torrent.Info.Files))
		var totalSize int64
		for i, file := range torrent.Info.Files {
			path := filepath.Join(file.Path...)
			fmt.Printf("  File %d: %s (%d bytes)\n", i+1, path, file.Length)
			totalSize += file.Length
		}
		fmt.Printf("Total Size: %d bytes\n", totalSize)
	} else {
		fmt.Printf("File Size: %d bytes\n", torrent.Info.Length)
	}

	return torrent, nil
}

// TotalLength returns the total size of all files in the torrent
func TotalLength(torrent *TorrentFile) int64 {
	if len(torrent.Info.Files) > 0 {
		var size int64
		for _, file := range torrent.Info.Files {
			size += file.Length
		}
		return size
	}
	return torrent.Info.Length
}

// Size returns total size as 8-byte big-endian (for tracker protocol)
func Size(torrent *TorrentFile) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(TotalLength(torrent)))
	return buf
}

// InfoHash returns the precomputed SHA-1 hash of the info dictionary
func InfoHash(torrent *TorrentFile) ([]byte, error) {
	hash := torrent.RawInfoHash
	return hash[:], nil
}

// TotalPieces returns the number of pieces in the torrent
func TotalPieces(torrent *TorrentFile) int {
	return len(torrent.Info.Pieces) / 20
}

// PieceHash returns the expected SHA-1 hash for a given piece index
func PieceHash(torrent *TorrentFile, pieceIndex int) []byte {
	start := pieceIndex * 20
	end := start + 20
	if end > len(torrent.Info.Pieces) {
		return nil
	}
	return []byte(torrent.Info.Pieces[start:end])
}

// PieceLen returns the length of a piece at the given index
func PieceLen(torrent *TorrentFile, pieceIndex int) int64 {
	totalLength := TotalLength(torrent)
	pieceLength := torrent.Info.PieceLength

	lastPieceIndex := int(totalLength / pieceLength)
	lastPieceLength := totalLength % pieceLength

	if pieceIndex == lastPieceIndex && lastPieceLength != 0 {
		return lastPieceLength
	}
	return pieceLength
}

// BlocksPerPiece returns the number of blocks in a piece
func BlocksPerPiece(torrent *TorrentFile, pieceIndex int) int {
	pieceLength := PieceLen(torrent, pieceIndex)
	return int((pieceLength + BLOCK_LENGTH - 1) / BLOCK_LENGTH)
}

// BlockLen returns the length of a block at the given piece and block index
func BlockLen(torrent *TorrentFile, pieceIndex int, blockIndex int) int {
	pieceLength := PieceLen(torrent, pieceIndex)
	lastBlockLength := int(pieceLength % BLOCK_LENGTH)
	lastBlockIndex := int(pieceLength / BLOCK_LENGTH)

	if blockIndex == lastBlockIndex && lastBlockLength != 0 {
		return lastBlockLength
	}
	return BLOCK_LENGTH
}

// GetTrackerURLs returns all tracker URLs (announce + announce-list), deduplicated
func GetTrackerURLs(torrent *TorrentFile) []string {
	seen := map[string]bool{}
	var urls []string

	if torrent.Announce != "" {
		urls = append(urls, torrent.Announce)
		seen[torrent.Announce] = true
	}

	for _, tier := range torrent.AnnounceList {
		for _, u := range tier {
			if !seen[u] && u != "" {
				urls = append(urls, u)
				seen[u] = true
			}
		}
	}

	return urls
}

// Helper function to read torrent file for info hash calculation (unused, kept for reference)
func readTorrentFile(filePath string) (map[string]interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var torrent map[string]interface{}
	err = bencode.Unmarshal(bytes.NewReader(data), &torrent)
	if err != nil {
		return nil, err
	}

	return torrent, nil
}
