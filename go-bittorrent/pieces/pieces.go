package pieces

// Tracks which blocks have been requested and received during the download and overall progress
import (
	"bittorrent/torrentparser"
	"sync"
)

type Pieces struct {
	requested           [][]bool   // 2D array: [piece][block] → requested?
	received            [][]bool   // 2D array: [piece][block] → received?
	totalBlocks         int        // Total blocks in entire torrent
	totalReceivedBlocks int        // How many blocks received so far
	mu                  sync.Mutex // Lock for thread-safety
}

// 2D Arrays Concept
/*
Piece 0: [block0, block1, block2, block3]
Piece 1: [block0, block1, block2, block3]
Piece 2: [block0, block1, block2, block3]

requested[0][2] = true  → "I requested block 2 of piece 0"
received[1][0] = true   → "I received block 0 of piece 1"

*/

// NewPieces creates a new Pieces tracker
func NewPieces(torrent *torrentparser.TorrentFile) *Pieces {
	nPieces := calculateTotalPieces(torrent) // How many pieces?

	requested := make([][]bool, nPieces) // Create outer array
	received := make([][]bool, nPieces)  // Create outer array
	totalBlocks := 0

	// For each piece, create inner arrays
	for i := 0; i < nPieces; i++ {
		nBlocks := torrentparser.BlocksPerPiece(torrent, i) // Blocks in this piece
		requested[i] = make([]bool, nBlocks)                // Create array for this piece
		received[i] = make([]bool, nBlocks)                 // Create array for this piece
		totalBlocks += nBlocks                              // Count total blocks
	}

	return &Pieces{
		requested:           requested,
		received:            received,
		totalBlocks:         totalBlocks,
		totalReceivedBlocks: 0, // Nothing received yet
	}
}

/*
This uses ceiling division:
Formula: (length + pieceLength - 1) / pieceLength
Equivalent to: ceil(length / pieceLength)
*/
func calculateTotalPieces(torrent *torrentparser.TorrentFile) int {
	if len(torrent.Info.Files) > 0 {
		totalPieces := 0
		for _, file := range torrent.Info.Files {
			piecesInFile := (file.Length + torrent.Info.PieceLength - 1) / torrent.Info.PieceLength
			totalPieces += int(piecesInFile)
		}
		return totalPieces
	}
	return int((torrent.Info.Length + torrent.Info.PieceLength - 1) / torrent.Info.PieceLength)
}

// AddRequested marks a block as requested
// Locks the mutex Multiple goroutines may call this simultaneously.
/*
Example:
pieceIndex = 5, begin = 32768
blockIndex = 32768 / 16384 = 2
Sets requested[5][2] = true

*/
func (p *Pieces) AddRequested(pieceIndex int, begin int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	blockIndex := begin / torrentparser.BLOCK_LENGTH
	if pieceIndex < len(p.requested) && blockIndex < len(p.requested[pieceIndex]) {
		p.requested[pieceIndex][blockIndex] = true
	}
}

// AddReceived marks a block as received
func (p *Pieces) AddReceived(pieceIndex int, begin int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	blockIndex := begin / torrentparser.BLOCK_LENGTH
	if pieceIndex < len(p.received) && blockIndex < len(p.received[pieceIndex]) {
		p.received[pieceIndex][blockIndex] = true
		p.totalReceivedBlocks++
	}
}

// Needed checks if a block is needed (not yet requested)
/*
How it works
Phase 1: Check if all blocks have been requested
Scans all pieces/blocks
If any block hasn't been requested, allRequested = false
Phase 2: Smart reset (if all requested)
If all blocks have been requested at least once:
Reset requested to match received
This allows re-requesting blocks that failed


*/
func (p *Pieces) Needed(pieceIndex int, begin int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if every block has been requested once
	allRequested := true
	for _, blocks := range p.requested {
		for _, requested := range blocks {
			if !requested {
				allRequested = false
				break
			}
		}
		if !allRequested {
			break
		}
	}

	// If all blocks have been requested, reset requested to match received
	if allRequested {
		for i := range p.requested {
			for j := range p.requested[i] {
				p.requested[i][j] = p.received[i][j]
			}
		}
	}

	blockIndex := begin / torrentparser.BLOCK_LENGTH
	if pieceIndex >= len(p.requested) || blockIndex >= len(p.requested[pieceIndex]) {
		return false
	}
	return !p.requested[pieceIndex][blockIndex]
}

// IsDone checks if all pieces have been downloaded
func (p *Pieces) IsDone() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, blocks := range p.received {
		for _, received := range blocks {
			if !received {
				return false
			}
		}
	}
	return true
}

// IsPieceComplete checks if a specific piece is complete
func (p *Pieces) IsPieceComplete(pieceIndex int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if pieceIndex >= len(p.received) {
		return false
	}

	for _, received := range p.received[pieceIndex] {
		if !received {
			return false
		}
	}
	return true
}

// GetProgress returns the download progress as a percentage
func (p *Pieces) GetProgress() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.totalBlocks == 0 {
		return 0
	}
	return float64(p.totalReceivedBlocks) / float64(p.totalBlocks) * 100
}

// TotalReceivedBlocks returns the total number of received blocks
func (p *Pieces) TotalReceivedBlocks() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.totalReceivedBlocks
}

// TotalBlocks returns the total number of blocks
func (p *Pieces) TotalBlocks() int {
	return p.totalBlocks
}
