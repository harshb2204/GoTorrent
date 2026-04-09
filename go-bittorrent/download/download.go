package download

import (
	"bittorrent/message"
	"bittorrent/pieces"
	"bittorrent/queue"
	"bittorrent/torrentparser"
	"bittorrent/tracker"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	maxPipeline     = 5               // max concurrent block requests per peer
	peerTimeout     = 30 * time.Second
	handshakeTimeout = 10 * time.Second
	connectTimeout  = 10 * time.Second
)

// Download initiates the download process
func Download(torrent *torrentparser.TorrentFile, downloadPath string) error {
	peers, err := tracker.GetPeers(torrent)
	if err != nil {
		return fmt.Errorf("failed to get peers: %v", err)
	}

	fmt.Printf("Found %d peers\n", len(peers))

	if len(peers) == 0 {
		return fmt.Errorf("no peers available")
	}

	piecesTracker := pieces.NewPieces(torrent)

	// Set up files
	isSingleFile := len(torrent.Info.Files) == 0
	var file *os.File

	if isSingleFile {
		// Single file: write directly as the named file
		dir := filepath.Dir(downloadPath)
		if dir != "." && dir != "" {
			os.MkdirAll(dir, 0755)
		}
		file, err = os.Create(downloadPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %v", downloadPath, err)
		}
		defer file.Close()

		// Pre-allocate
		file.Truncate(torrent.Info.Length)
	} else {
		// Multi-file: create directory and all files
		err = os.MkdirAll(downloadPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %v", err)
		}

		// Create a single concatenated file for simplicity
		// We'll split into individual files after download
		concatPath := downloadPath + ".tmp"
		file, err = os.Create(concatPath)
		if err != nil {
			return fmt.Errorf("failed to create temp file: %v", err)
		}
		defer file.Close()

		totalLen := torrentparser.TotalLength(torrent)
		file.Truncate(totalLen)
	}

	// Piece buffer storage: stores assembled piece data for SHA-1 verification
	pieceBuffers := &pieceBufferStore{
		buffers: make(map[int][]byte),
	}

	// Download from all peers concurrently
	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(p tracker.Peer) {
			defer wg.Done()
			downloadFromPeer(p, torrent, piecesTracker, file, pieceBuffers)
		}(peer)
	}

	wg.Wait()

	if piecesTracker.IsDone() {
		fmt.Println("\n\n********** DOWNLOAD COMPLETE **********")

		// For multi-file torrents, split the concatenated file into individual files
		if !isSingleFile {
			file.Close()
			err = splitMultiFile(torrent, downloadPath)
			if err != nil {
				return fmt.Errorf("failed to split files: %v", err)
			}
			os.Remove(downloadPath + ".tmp")
		}
	} else {
		fmt.Printf("\n\n********** DOWNLOAD INCOMPLETE (%.2f%%) **********\n", piecesTracker.GetProgress())
	}

	return nil
}

// pieceBufferStore stores piece data in memory for SHA-1 verification before writing to disk
type pieceBufferStore struct {
	buffers map[int][]byte
	mu      sync.Mutex
}

func (s *pieceBufferStore) getOrCreate(pieceIndex int, pieceLen int64) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	if buf, ok := s.buffers[pieceIndex]; ok {
		return buf
	}
	buf := make([]byte, pieceLen)
	s.buffers[pieceIndex] = buf
	return buf
}

func (s *pieceBufferStore) get(pieceIndex int) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffers[pieceIndex]
}

func (s *pieceBufferStore) remove(pieceIndex int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.buffers, pieceIndex)
}

func downloadFromPeer(peer tracker.Peer, torrent *torrentparser.TorrentFile,
	piecesTracker *pieces.Pieces, file *os.File, pieceBuffers *pieceBufferStore) {

	addr := fmt.Sprintf("%s:%d", peer.IP, peer.Port)
	conn, err := net.DialTimeout("tcp", addr, connectTimeout)
	if err != nil {
		return
	}
	defer conn.Close()

	// Send handshake
	handshake, err := message.BuildHandshake(torrent)
	if err != nil {
		return
	}

	conn.SetDeadline(time.Now().Add(handshakeTimeout))
	_, err = conn.Write(handshake)
	if err != nil {
		return
	}

	// Read handshake response
	hsResp := make([]byte, 68)
	_, err = io.ReadFull(conn, hsResp)
	if err != nil {
		return
	}

	// Validate handshake
	if hsResp[0] != 19 || string(hsResp[1:20]) != "BitTorrent protocol" {
		return
	}

	// Verify info hash matches
	infoHash, _ := torrentparser.InfoHash(torrent)
	peerInfoHash := hsResp[28:48]
	if !bytesEqual(infoHash, peerInfoHash) {
		fmt.Printf("Peer %s: info hash mismatch, disconnecting\n", peer.IP)
		return
	}

	fmt.Printf("Connected to peer: %s\n", peer.IP)

	// Send interested message
	conn.SetDeadline(time.Now().Add(peerTimeout))
	conn.Write(message.BuildInterested())

	// Create queue for this peer
	q := queue.NewQueue(torrent)

	// Message loop
	savedBuffer := []byte{}
	buf := make([]byte, 16384)
	pendingRequests := 0

	for {
		conn.SetDeadline(time.Now().Add(peerTimeout))
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		savedBuffer = append(savedBuffer, buf[:n]...)

		// Process complete messages from buffer
		for {
			if len(savedBuffer) < 4 {
				break
			}

			msgLength := int(binary.BigEndian.Uint32(savedBuffer[0:4])) + 4

			if msgLength < 4 {
				// Invalid length, discard
				savedBuffer = savedBuffer[4:]
				continue
			}

			if len(savedBuffer) < msgLength {
				break
			}

			msgBytes := savedBuffer[:msgLength]
			savedBuffer = savedBuffer[msgLength:]

			if msgLength == 4 {
				// Keep-alive
				continue
			}

			parsedMsg, err := message.Parse(msgBytes)
			if err != nil {
				continue
			}

			switch parsedMsg.ID {
			case message.MsgChoke:
				q.SetChoked(true)
				pendingRequests = 0

			case message.MsgUnchoke:
				q.SetChoked(false)
				pendingRequests = 0
				pendingRequests = fillPipeline(conn, piecesTracker, q, pendingRequests)

			case message.MsgHave:
				if len(parsedMsg.Payload) >= 4 {
					pieceIndex := binary.BigEndian.Uint32(parsedMsg.Payload[0:4])
					q.Enqueue(int(pieceIndex))
					if !q.IsChoked() {
						pendingRequests = fillPipeline(conn, piecesTracker, q, pendingRequests)
					}
				}

			case message.MsgBitfield:
				for i, b := range parsedMsg.Payload {
					for j := 0; j < 8; j++ {
						if (b & (1 << uint(7-j))) != 0 {
							pieceIdx := i*8 + j
							if pieceIdx < torrentparser.TotalPieces(torrent) {
								q.Enqueue(pieceIdx)
							}
						}
					}
				}
				if !q.IsChoked() {
					pendingRequests = fillPipeline(conn, piecesTracker, q, pendingRequests)
				}

			case message.MsgPiece:
				pendingRequests--
				if pendingRequests < 0 {
					pendingRequests = 0
				}

				pieceBlock, err := message.ParsePieceBlock(parsedMsg.Payload)
				if err != nil {
					continue
				}

				pieceLen := torrentparser.PieceLen(torrent, int(pieceBlock.Index))
				pieceBuf := pieceBuffers.getOrCreate(int(pieceBlock.Index), pieceLen)

				// Copy block data into piece buffer
				begin := int(pieceBlock.Begin)
				if begin+len(pieceBlock.Block) <= len(pieceBuf) {
					copy(pieceBuf[begin:], pieceBlock.Block)
				}

				piecesTracker.AddReceived(int(pieceBlock.Index), int(pieceBlock.Begin))

				// Check if entire piece is complete
				if piecesTracker.IsPieceComplete(int(pieceBlock.Index)) {
					// Verify SHA-1
					fullPiece := pieceBuffers.get(int(pieceBlock.Index))
					if fullPiece != nil {
						hash := sha1.Sum(fullPiece)
						expectedHash := torrentparser.PieceHash(torrent, int(pieceBlock.Index))

						if expectedHash != nil && bytesEqual(hash[:], expectedHash) {
							// Write verified piece to file
							offset := int64(pieceBlock.Index) * torrent.Info.PieceLength
							file.WriteAt(fullPiece, offset)
							pieceBuffers.remove(int(pieceBlock.Index))
						} else {
							// Hash mismatch — re-download this piece
							fmt.Printf("\nPiece %d failed SHA-1 check, re-downloading\n", pieceBlock.Index)
							piecesTracker.ResetPiece(int(pieceBlock.Index))
							pieceBuffers.remove(int(pieceBlock.Index))
							// Re-enqueue all blocks of this piece
							q.Enqueue(int(pieceBlock.Index))
						}
					}
				}

				if piecesTracker.IsDone() {
					return
				}

				fmt.Printf("\rDownloading... %.2f%%", piecesTracker.GetProgress())

				// Keep pipeline full
				if !q.IsChoked() {
					pendingRequests = fillPipeline(conn, piecesTracker, q, pendingRequests)
				}
			}
		}
	}
}

// fillPipeline sends requests to fill up to maxPipeline concurrent requests
func fillPipeline(conn net.Conn, piecesTracker *pieces.Pieces, q *queue.Queue, pending int) int {
	for pending < maxPipeline && q.Length() > 0 {
		pieceBlock, ok := q.Dequeue()
		if !ok {
			break
		}

		if piecesTracker.Needed(int(pieceBlock.Index), int(pieceBlock.Begin)) {
			requestMsg := message.BuildRequest(pieceBlock.Index, pieceBlock.Begin, pieceBlock.Length)
			_, err := conn.Write(requestMsg)
			if err != nil {
				return pending
			}
			piecesTracker.AddRequested(int(pieceBlock.Index), int(pieceBlock.Begin))
			pending++
		}
	}
	return pending
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// splitMultiFile splits the concatenated temp file into individual files
func splitMultiFile(torrent *torrentparser.TorrentFile, downloadPath string) error {
	tmpFile, err := os.Open(downloadPath + ".tmp")
	if err != nil {
		return err
	}
	defer tmpFile.Close()

	var offset int64
	for _, fileInfo := range torrent.Info.Files {
		// Build full file path
		pathParts := append([]string{downloadPath}, fileInfo.Path...)
		filePath := filepath.Join(pathParts...)

		// Create parent directories
		dir := filepath.Dir(filePath)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}

		// Create file and copy data
		outFile, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %v", filePath, err)
		}

		_, err = tmpFile.Seek(offset, 0)
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.CopyN(outFile, tmpFile, fileInfo.Length)
		outFile.Close()
		if err != nil {
			return fmt.Errorf("failed to write file %s: %v", filePath, err)
		}

		offset += fileInfo.Length
	}

	return nil
}
