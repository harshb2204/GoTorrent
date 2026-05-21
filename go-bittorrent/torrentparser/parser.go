package torrentparser

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackpal/bencode-go"
)

const BLOCK_LENGTH = 16384 // 2^14 bytes

type TorrentFile struct {
	Announce     string
	AnnounceList [][]string
	Info         TorrentInfo
	RawInfoHash  [20]byte
}

type TorrentInfo struct {
	Name        string
	PieceLength int64
	Pieces      string
	Length      int64
	Files       []FileInfo
}

type FileInfo struct {
	Length int64
	Path   []string
}

// Open reads and parses a torrent file entirely from the raw bencode tree
// to avoid jackpal/bencode-go panics with nested list types
func Open(filePath string) (*TorrentFile, error) {
	fmt.Println("====== opening the torrent file ======")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Use Decode which returns interface{} — no struct reflection panics
	decoded, err := bencode.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode torrent: %v", err)
	}

	root, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("torrent file is not a dictionary")
	}

	torrent := &TorrentFile{}

	// announce
	if v, ok := root["announce"]; ok {
		torrent.Announce, _ = v.(string)
	}

	// announce-list
	if rawAL, ok := root["announce-list"]; ok {
		if tiers, ok := rawAL.([]interface{}); ok {
			for _, tier := range tiers {
				if tierList, ok := tier.([]interface{}); ok {
					var urls []string
					for _, u := range tierList {
						if s, ok := u.(string); ok {
							urls = append(urls, s)
						}
					}
					if len(urls) > 0 {
						torrent.AnnounceList = append(torrent.AnnounceList, urls)
					}
				}
			}
		}
	}

	// info dictionary
	infoRaw, ok := root["info"]
	if !ok {
		return nil, fmt.Errorf("torrent file missing info dictionary")
	}

	infoMap, ok := infoRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("info is not a dictionary")
	}

	// Calculate info hash by re-encoding the raw info dict
	var infoBuf bytes.Buffer
	err = bencode.Marshal(&infoBuf, infoRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to encode info dict: %v", err)
	}
	torrent.RawInfoHash = sha1.Sum(infoBuf.Bytes())

	// Parse info fields
	if v, ok := infoMap["name"]; ok {
		torrent.Info.Name, _ = v.(string)
	}
	if v, ok := infoMap["piece length"]; ok {
		torrent.Info.PieceLength, _ = v.(int64)
	}
	if v, ok := infoMap["pieces"]; ok {
		torrent.Info.Pieces, _ = v.(string)
	}
	if v, ok := infoMap["length"]; ok {
		torrent.Info.Length, _ = v.(int64)
	}

	// Multi-file: parse files list
	if filesRaw, ok := infoMap["files"]; ok {
		if filesList, ok := filesRaw.([]interface{}); ok {
			for _, fileRaw := range filesList {
				if fileMap, ok := fileRaw.(map[string]interface{}); ok {
					fi := FileInfo{}
					if v, ok := fileMap["length"]; ok {
						fi.Length, _ = v.(int64)
					}
					if pathRaw, ok := fileMap["path"]; ok {
						if pathList, ok := pathRaw.([]interface{}); ok {
							for _, p := range pathList {
								if s, ok := p.(string); ok {
									fi.Path = append(fi.Path, s)
								}
							}
						}
					}
					torrent.Info.Files = append(torrent.Info.Files, fi)
				}
			}
		}
	}

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

func Size(torrent *TorrentFile) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(TotalLength(torrent)))
	return buf
}

func InfoHash(torrent *TorrentFile) ([]byte, error) {
	hash := torrent.RawInfoHash
	return hash[:], nil
}

func TotalPieces(torrent *TorrentFile) int {
	return len(torrent.Info.Pieces) / 20
}

func PieceHash(torrent *TorrentFile, pieceIndex int) []byte {
	start := pieceIndex * 20
	end := start + 20
	if end > len(torrent.Info.Pieces) {
		return nil
	}
	return []byte(torrent.Info.Pieces[start:end])
}

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

func BlocksPerPiece(torrent *TorrentFile, pieceIndex int) int {
	pieceLength := PieceLen(torrent, pieceIndex)
	return int((pieceLength + BLOCK_LENGTH - 1) / BLOCK_LENGTH)
}

func BlockLen(torrent *TorrentFile, pieceIndex int, blockIndex int) int {
	pieceLength := PieceLen(torrent, pieceIndex)
	lastBlockLength := int(pieceLength % BLOCK_LENGTH)
	lastBlockIndex := int(pieceLength / BLOCK_LENGTH)

	if blockIndex == lastBlockIndex && lastBlockLength != 0 {
		return lastBlockLength
	}
	return BLOCK_LENGTH
}

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
