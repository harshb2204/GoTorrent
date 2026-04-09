package pieces

import (
	"bittorrent/torrentparser"
	"sync"
)

type Pieces struct {
	requested           [][]bool
	received            [][]bool
	totalBlocks         int
	totalReceivedBlocks int
	mu                  sync.Mutex
}

func NewPieces(torrent *torrentparser.TorrentFile) *Pieces {
	// Use the actual piece count from the pieces hash string
	nPieces := torrentparser.TotalPieces(torrent)

	requested := make([][]bool, nPieces)
	received := make([][]bool, nPieces)
	totalBlocks := 0

	for i := 0; i < nPieces; i++ {
		nBlocks := torrentparser.BlocksPerPiece(torrent, i)
		requested[i] = make([]bool, nBlocks)
		received[i] = make([]bool, nBlocks)
		totalBlocks += nBlocks
	}

	return &Pieces{
		requested:           requested,
		received:            received,
		totalBlocks:         totalBlocks,
		totalReceivedBlocks: 0,
	}
}

func (p *Pieces) AddRequested(pieceIndex int, begin int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	blockIndex := begin / torrentparser.BLOCK_LENGTH
	if pieceIndex < len(p.requested) && blockIndex < len(p.requested[pieceIndex]) {
		p.requested[pieceIndex][blockIndex] = true
	}
}

func (p *Pieces) AddReceived(pieceIndex int, begin int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	blockIndex := begin / torrentparser.BLOCK_LENGTH
	if pieceIndex < len(p.received) && blockIndex < len(p.received[pieceIndex]) {
		if !p.received[pieceIndex][blockIndex] {
			p.received[pieceIndex][blockIndex] = true
			p.totalReceivedBlocks++
		}
	}
}

// Needed checks if a block needs to be requested
func (p *Pieces) Needed(pieceIndex int, begin int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	blockIndex := begin / torrentparser.BLOCK_LENGTH
	if pieceIndex >= len(p.requested) || blockIndex >= len(p.requested[pieceIndex]) {
		return false
	}

	// If already received, not needed
	if p.received[pieceIndex][blockIndex] {
		return false
	}

	// Check if all blocks have been requested at least once
	allRequested := true
	for _, blocks := range p.requested {
		for _, req := range blocks {
			if !req {
				allRequested = false
				break
			}
		}
		if !allRequested {
			break
		}
	}

	// If all blocks have been requested, reset unreceived ones to allow re-request
	if allRequested {
		for i := range p.requested {
			for j := range p.requested[i] {
				p.requested[i][j] = p.received[i][j]
			}
		}
	}

	return !p.requested[pieceIndex][blockIndex]
}

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

// ResetPiece resets all blocks of a piece (used when SHA-1 verification fails)
func (p *Pieces) ResetPiece(pieceIndex int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if pieceIndex >= len(p.received) {
		return
	}

	blocksReset := 0
	for j := range p.received[pieceIndex] {
		if p.received[pieceIndex][j] {
			blocksReset++
		}
		p.received[pieceIndex][j] = false
		p.requested[pieceIndex][j] = false
	}
	p.totalReceivedBlocks -= blocksReset
}

func (p *Pieces) GetProgress() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.totalBlocks == 0 {
		return 0
	}
	return float64(p.totalReceivedBlocks) / float64(p.totalBlocks) * 100
}

func (p *Pieces) TotalReceivedBlocks() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.totalReceivedBlocks
}

func (p *Pieces) TotalBlocks() int {
	return p.totalBlocks
}
