package queue

// per peer request queue for blocks
import (
	"bittorrent/torrentparser"
	"sync"
)

/*
For one peer:
Peer announces: “I have piece 5.”
We call peerQueue.Enqueue(5) → queue now has all blocks of piece 5.
Peer unchokes us → peerQueue.SetChoked(false).
Loop:
	While not choked and queue not empty:
	  Take next block from queue (Dequeue).
	  If we still need it (Pieces.Needed):
		Mark requested.
		Send a Request message for that block.
Peer sends Piece messages back:
For each, we:
	Write data to disk.
	Mark block as received in Pieces.

*/

type PieceBlock struct {
	Index  uint32 // which piece
	Begin  uint32 // byte offse within that piece (0, 16384, 32768, …).
	Length uint32 // how many bytes to request in that block
}

type Queue struct {
	torrent *torrentparser.TorrentFile // reference to torrent metadata
	queue   []PieceBlock               // actual FIFO queue
	choked  bool                       // whether this peer is currently choking us
	mu      sync.Mutex                 // to protect the queue and choked from concurrent access
}

// NewQueue creates a new Queue for managing piece requests
func NewQueue(torrent *torrentparser.TorrentFile) *Queue {
	return &Queue{
		torrent: torrent,
		queue:   []PieceBlock{}, // takes an empty slice for queue
		choked:  true,           // by default, a new peer is considered choking us
		// until we receive an Unchoke message.
	}
}

// Enqueue adds all blocks of a piece to the queue
func (q *Queue) Enqueue(pieceIndex int) {
	q.mu.Lock()
	defer q.mu.Unlock()

	nBlocks := torrentparser.BlocksPerPiece(q.torrent, pieceIndex)

	for i := 0; i < nBlocks; i++ {
		pieceBlock := PieceBlock{
			Index:  uint32(pieceIndex),
			Begin:  uint32(i * torrentparser.BLOCK_LENGTH),
			Length: uint32(torrentparser.BlockLen(q.torrent, pieceIndex, i)),
		}
		q.queue = append(q.queue, pieceBlock)
	}
}

// Dequeue removes and returns the first piece block from the queue
func (q *Queue) Dequeue() (PieceBlock, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return PieceBlock{}, false
	}

	pieceBlock := q.queue[0]
	q.queue = q.queue[1:]
	return pieceBlock, true
}

// Peek returns the first piece block without removing it
func (q *Queue) Peek() (PieceBlock, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return PieceBlock{}, false
	}
	return q.queue[0], true
}

// Length returns the number of items in the queue
func (q *Queue) Length() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}

// SetChoked sets the choked status
func (q *Queue) SetChoked(choked bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.choked = choked
}

// IsChoked returns the choked status
func (q *Queue) IsChoked() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.choked
}
