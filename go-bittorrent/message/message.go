package message

// Defines how peers talk to each other. Build and parses messages.
import (
	"bittorrent/torrentparser"
	"bittorrent/util"
	"encoding/binary"
	"fmt"
)

const (
	MsgChoke         = 0 // "I'm not allowing you to request data"
	MsgUnchoke       = 1 // "You can now request data from me"
	MsgInterested    = 2 // "I want data from you"
	MsgNotInterested = 3 // "I don't need anything from you"
	MsgHave          = 4 // "I just completed piece X"
	MsgBitfield      = 5 // "Here's what pieces I have (all at once)"
	MsgRequest       = 6 // "Please send me block X of piece Y"
	MsgPiece         = 7 // "Here's the block you requested"
	MsgCancel        = 8 // "Never mind, cancel that request"
	MsgPort          = 9 // "My DHT port is X" (not used in this client)
)

// represents a parsed message
type Message struct {
	Size    uint32 // Length of message (excluding the 4-byte length field)
	ID      uint8  // Message type (0-9)
	Payload []byte // Message-specific data
}

// represents a block of data
type PieceBlock struct {
	Index  uint32 // Which piece (0, 1, 2, ...)
	Begin  uint32 // Offset within piece (0, 16384, 32768, ...)
	Block  []byte // The actual data
	Length uint32 // Size of block
}

// BuildHandshake creates the initial handshake message
// Handshake is the first message sent to a peer.(68 bytes) Identifies you and torrent
/*
Byte:  0    1-19                   20-27      28-47       48-67
      ┌───┬──────────────────────┬──────────┬──────────┬──────────┐
      │19 │"BitTorrent protocol" │ reserved │info hash │ peer id  │
      │   │                      │ (zeros)  │ 20 bytes │20 bytes  │
      └───┴──────────────────────┴──────────┴──────────┴──────────┘
Total: 68 bytes

*/
func BuildHandshake(torrent *torrentparser.TorrentFile) ([]byte, error) {
	buf := make([]byte, 68)

	// pstrlen
	buf[0] = 19

	// pstr
	copy(buf[1:20], "BitTorrent protocol")

	// reserved (8 bytes of zeros)
	for i := 20; i < 28; i++ {
		buf[i] = 0
	}

	// info hash
	infoHash, err := torrentparser.InfoHash(torrent)
	if err != nil {
		return nil, err
	}
	copy(buf[28:48], infoHash)

	// peer id
	copy(buf[48:68], util.GenId())

	return buf, nil
}

// BuildKeepAlive creates a keep-alive message
// Format: 4 zero bytes.
func BuildKeepAlive() []byte {
	return make([]byte, 4)
}

// BuildChoke creates a choke message
// Format: [length: 1][ID: 0] = 5 bytes total.
func BuildChoke() []byte {
	buf := make([]byte, 5)
	binary.BigEndian.PutUint32(buf[0:4], 1)
	buf[4] = MsgChoke
	return buf
}

// BuildUnchoke creates an unchoke message
// Format: [length: 1][ID: 0] = 5 bytes total.
func BuildUnchoke() []byte {
	buf := make([]byte, 5)
	binary.BigEndian.PutUint32(buf[0:4], 1)
	buf[4] = MsgUnchoke
	return buf
}

// 5 bytes with length=1 and ID=2 or 3. (BuildInterested andBuildNotInterested  )
// BuildInterested creates an interested message

func BuildInterested() []byte {
	buf := make([]byte, 5)
	binary.BigEndian.PutUint32(buf[0:4], 1)
	buf[4] = MsgInterested
	return buf
}

// BuildNotInterested creates a not interested message

func BuildNotInterested() []byte {
	buf := make([]byte, 5)
	binary.BigEndian.PutUint32(buf[0:4], 1)
	buf[4] = MsgNotInterested
	return buf
}

// BuildHave creates a have message
// Example: "I completed piece #42"
func BuildHave(pieceIndex uint32) []byte {
	buf := make([]byte, 9)
	binary.BigEndian.PutUint32(buf[0:4], 5)
	buf[4] = MsgHave
	binary.BigEndian.PutUint32(buf[5:9], pieceIndex) // Which piece
	return buf
}

// BuildBitfield creates a bitfield message
// Format: [length][ID: 5][bitfield data]
// Purpose: Send all pieces you have at once. Each bit = 1 piece (1 = have it, 0 = don't).
// Example: If you have 10 pieces, the bitfield might be 2 bytes (16 bits),
// with bits set for pieces you have.
func BuildBitfield(bitfield []byte) []byte {
	buf := make([]byte, 5+len(bitfield))
	binary.BigEndian.PutUint32(buf[0:4], uint32(1+len(bitfield)))
	buf[4] = MsgBitfield
	copy(buf[5:], bitfield)
	return buf
}

// BuildRequest creates a request message for a piece block
// Example: "Send me piece #5, starting at byte 16384, length 16384"
func BuildRequest(index, begin, length uint32) []byte {
	buf := make([]byte, 17)
	binary.BigEndian.PutUint32(buf[0:4], 13)
	buf[4] = MsgRequest
	binary.BigEndian.PutUint32(buf[5:9], index)
	binary.BigEndian.PutUint32(buf[9:13], begin)
	binary.BigEndian.PutUint32(buf[13:17], length)
	return buf
}

// BuildPiece creates a piece message
// Purpose: Send the requested block data. Length varies with block size.
func BuildPiece(index, begin uint32, block []byte) []byte {
	buf := make([]byte, 13+len(block))
	binary.BigEndian.PutUint32(buf[0:4], uint32(9+len(block)))
	buf[4] = MsgPiece
	binary.BigEndian.PutUint32(buf[5:9], index)
	binary.BigEndian.PutUint32(buf[9:13], begin)
	copy(buf[13:], block)
	return buf
}

// BuildCancel creates a cancel message
func BuildCancel(index, begin, length uint32) []byte {
	buf := make([]byte, 17)
	binary.BigEndian.PutUint32(buf[0:4], 13)
	buf[4] = MsgCancel
	binary.BigEndian.PutUint32(buf[5:9], index)
	binary.BigEndian.PutUint32(buf[9:13], begin)
	binary.BigEndian.PutUint32(buf[13:17], length)
	return buf
}

// BuildPort creates a port message
// NOT USED!!!
func BuildPort(port uint16) []byte {
	buf := make([]byte, 7)
	binary.BigEndian.PutUint32(buf[0:4], 3)
	buf[4] = MsgPort
	binary.BigEndian.PutUint16(buf[5:7], port)
	return buf
}

// Parse parses a message from the peer
// Read firsr 4 bytes as length
// byte 5 is the message id
// everything after byte 5 is payload
func Parse(msg []byte) (*Message, error) {
	if len(msg) < 4 {
		return nil, fmt.Errorf("message too short")
	}

	size := binary.BigEndian.Uint32(msg[0:4])

	result := &Message{
		Size: size,
	}

	if len(msg) > 4 {
		result.ID = msg[4]
	}

	if len(msg) > 5 {
		result.Payload = msg[5:]
	}

	return result, nil
}

// ParsePieceBlock parses a piece message payload
func ParsePieceBlock(payload []byte) (*PieceBlock, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("piece payload too short")
	}

	return &PieceBlock{
		Index: binary.BigEndian.Uint32(payload[0:4]),
		Begin: binary.BigEndian.Uint32(payload[4:8]),
		Block: payload[8:],
	}, nil
}
