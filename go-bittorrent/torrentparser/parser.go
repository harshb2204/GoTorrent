package torrentparser

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/jackpal/bencode-go"
)

const BLOCK_LENGTH = 16384 // 2^14 bytes
//Pieces are split into 16KB blocks for requests.

// represents a .torrent file
type TorrentFile struct {
	Announce string      `bencode:"announce"` // Tracker URL (where to find peers)
	Info     TorrentInfo `bencode:"info"`     // Info Dictionary The actual torrent metadata
}

type TorrentInfo struct {
	Name        string     // Name of the file/folder
	PieceLength int64      // Size of each piece (e.g., 262144 bytes = 256KB)
	Pieces      string     // Binary string: 20-byte SHA-1 hash for each piece
	Length      int64      // Single file size (if single-file torrent)
	Files       []FileInfo // Multiple files (if multi-file torrent)
}

//Pieces is a concatenated string of 20-byte hashes. If there are 100 pieces, it's 2000 bytes (100 × 20).

// Represents one file in a multi-file torrent.
type FileInfo struct {
	Length int64    // File size
	Path   []string // File path (e.g., ["folder", "file.pdf"])
}

// Open reads and parses a torrent file
/*
Unmarshal -> convert data from a file format (like JSON, bencode, XML)
             into Go structs/objects.

*/
func Open(filePath string) (*TorrentFile, error) { // takes a file path and returns a ptr to
	// TorrentFile
	fmt.Println("====== opening the torrent file ======")
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err // return nil and error on failing to open
	}
	defer file.Close() //Defer is used to ensure that a function call is performed later \
	// in a program’s  execution, usually for purposes of cleanup.
	//file object will be automatically called just before the enclosing function returns,
	// regardless of how the function exits

	torrent := &TorrentFile{}
	err = bencode.Unmarshal(file, torrent) //reads the bencoded file and fills the torrent struct
	if err != nil {
		return nil, err
	}

	// Print torrent info without the binary pieces field
	fmt.Printf("Torrent Name: %s\n", torrent.Info.Name)
	fmt.Printf("Announce URL: %s\n", torrent.Announce)
	fmt.Printf("Piece Length: %d bytes\n", torrent.Info.PieceLength)
	fmt.Printf("Number of Pieces: %d\n", len(torrent.Info.Pieces)/20)

	// Handling single vs multi file info printing
	if len(torrent.Info.Files) > 0 {
		fmt.Printf("Files: %d\n", len(torrent.Info.Files))
		for i, file := range torrent.Info.Files {
			fmt.Printf("  File %d: %s (%d bytes)\n", i+1, file.Path[0], file.Length)
		}
	} else {
		fmt.Printf("File Size: %d bytes\n", torrent.Info.Length)
	}

	return torrent, nil
}

// Size calculates the total size of all files in the torrent
// Tells tracker how much you have left to download
func Size(torrent *TorrentFile) []byte {
	var size int64 // to hold very large numbers
	if len(torrent.Info.Files) > 0 {
		for _, file := range torrent.Info.Files {
			size += file.Length
		}
	} else {
		size = torrent.Info.Length
	}

	buf := make([]byte, 8)                        // create a byte slice of 8 bytes
	binary.BigEndian.PutUint64(buf, uint64(size)) //Converts the size to bytes in big-endian order
	return buf
}

// InfoHash calculates the SHA-1 hash of the info dictionary
// Used in tracker requests to identify which torrent you want
// Used in peer handshakes to verify you're downloading the same torrent
// Must match exactly - any change to the info dict changes the hash
func InfoHash(torrent *TorrentFile) ([]byte, error) {
	// Re-encode just the info dictionary
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, torrent.Info) //Marshals torrent.Info back into bencode format
	if err != nil {
		return nil, err
	}

	hash := sha1.Sum(buf.Bytes())
	//sha1.Sum() returns a fixed-size array [20]byte
	return hash[:], nil //hash[:] converts the array to a slice
}

// PieceLen returns the length of a piece at the given index
func PieceLen(torrent *TorrentFile, pieceIndex int) int64 {
	totalLength := binary.BigEndian.Uint64(Size(torrent))
	//Converts from the 8-byte format back to a number

	pieceLength := torrent.Info.PieceLength //Gets the standard piece size
	// (e.g., 262144 bytes = 256KB)

	lastPieceLength := int64(totalLength) % pieceLength
	lastPieceIndex := int64(totalLength) / pieceLength

	if int64(pieceIndex) == lastPieceIndex {
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

// Helper function to read torrent file for info hash calculation
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
