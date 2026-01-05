package tracker

/*
The tracker is a server that tells you which peers have the file. You ask it:
"Who has this torrent?" It replies with a list of IP addresses and ports.
*/

import (
	"bittorrent/torrentparser"
	"bittorrent/util"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"
)

// represents another comp, you can connect to
type Peer struct {
	IP   string // ip address
	Port uint16 // port number
}

// The tracker's response to UDP connect request.
// You send a connect request with a random transaction ID
// The tracker replies with the same transaction ID and a connection ID
// Use that connection ID in your announce request
type ConnectResponse struct {
	Action        uint32 // 0 means this is a connect res
	TransactionId uint32
	ConnectionId  []byte
}

// The tracker's response to your announce request, containing the peer list.
type AnnounceResponse struct {
	Action        uint32 // 1 means this is an announce res
	TransactionId uint32 // matches your announce req
	Interval      uint32 // how often to check back with tracker (sec)
	Leechers      uint32 // count of peers still downloading
	Seeders       uint32 // count of peers with entire files
	Peers         []Peer // list of peers you can connect to
}

// GetPeers connects to the tracker and retrieves the list of peers
// input is torrent file (for tracker URL)
// output is a list of peer structs (IP plus Port)
func GetPeers(torrent *torrentparser.TorrentFile) ([]Peer, error) {
	announceURL, err := url.Parse(torrent.Announce) // extract tracker url
	if err != nil {
		return nil, fmt.Errorf("failed to parse announce URL: %v", err)
	}

	// Support both UDP and HTTP trackers
	if announceURL.Scheme == "http" || announceURL.Scheme == "https" {
		return getHTTPPeers(torrent, announceURL)
	} else if announceURL.Scheme != "udp" {
		return nil, fmt.Errorf("unsupported tracker scheme: %s", announceURL.Scheme)
	} // if not udp then return error

	addr := announceURL.Host           // extract the host
	conn, err := net.Dial("udp", addr) // open a udp connection
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tracker: %v", err)
	}
	defer conn.Close() //ensures the connection closes when done

	// Set timeout 15 seconds
	conn.SetDeadline(time.Now().Add(15 * time.Second))

	// 1. Send connect request
	connectReq := buildConnectReq() // builds a 16 byte connect request
	_, err = conn.Write(connectReq) // send it over udp this is the first message to the tracker
	if err != nil {
		return nil, fmt.Errorf("failed to send connect request: %v", err)
	}

	// 2. Receive connect response
	buf := make([]byte, 16)  // 16 byte buffer
	n, err := conn.Read(buf) // read trackers response
	if err != nil {
		return nil, fmt.Errorf("failed to receive connect response: %v", err)
	}

	connectResp := parseConnectResp(buf[:n]) // parse into connectresp
	fmt.Printf("\n===== received connect response =====\n%+v\n", connectResp)

	// 3. Send announce request
	announceReq, err := buildAnnounceReq(connectResp.ConnectionId, torrent, 6881)
	// build 98 byte announce reques using connection id from connectResp
	if err != nil {
		return nil, fmt.Errorf("failed to build announce request: %v", err)
	}

	_, err = conn.Write(announceReq) // send it over the same udp connection
	if err != nil {
		return nil, fmt.Errorf("failed to send announce request: %v", err)
	}

	// 4. Receive announce response
	buf = make([]byte, 1024)
	n, err = conn.Read(buf) // read the tracker response
	if err != nil {
		return nil, fmt.Errorf("failed to receive announce response: %v", err)
	}

	announceResp := parseAnnounceResp(buf[:n])
	//Parses it into an AnnounceResponse (action, transaction ID, interval, leechers, seeders, peers)
	fmt.Printf("\n===== received announce response =====\n")
	fmt.Printf("Seeders: %d, Leechers: %d, Peers: %d\n",
		announceResp.Seeders, announceResp.Leechers, len(announceResp.Peers))

	return announceResp.Peers, nil // returns the list of peer structs
}

// builds a 16 byte UDP packer to send to the tracker as a connect request
// A magic constant is a fixed value used to identify a protocol or format.
// In this case: 0x41727101980 is the BitTorrent UDP tracker protocol identifier.
// The tracker checks this value; if it's wrong, the request is rejected.

/*
Byte Position:  0-7        8-11       12-15

	┌─────────┬─────────┬─────────┐
	│  Magic  │ Action  │Trans ID │
	│  Number │   (0)   │ (random)│
	│ 8 bytes │ 4 bytes │ 4 bytes │
	└─────────┴─────────┴─────────┘

Total: 16 bytes
*/
func buildConnectReq() []byte {
	buf := make([]byte, 16)

	// Connection ID (put int first 7 bytes the magic constant)(used big endian network byte order)
	binary.BigEndian.PutUint64(buf[0:8], 0x41727101980)

	// Action (0 = connect)
	binary.BigEndian.PutUint32(buf[8:12], 0)

	// Transaction ID (random)
	transactionId := make([]byte, 4)
	rand.Read(transactionId) // generate 4 random bytes
	copy(buf[12:16], transactionId)

	return buf
}

// Converts the tracker's raw 16-byte response into a ConnectResponse struct.

/*
Byte Position:  0-3        4-7        8-15
                ┌─────────┬─────────┬─────────┐
                │ Action  │Trans ID │Conn ID  │
                │ 4 bytes │ 4 bytes │ 8 bytes │
                └─────────┴─────────┴─────────┘
Total: 16 bytes

*/

func parseConnectResp(resp []byte) *ConnectResponse {
	return &ConnectResponse{
		Action:        binary.BigEndian.Uint32(resp[0:4]),
		TransactionId: binary.BigEndian.Uint32(resp[4:8]),
		ConnectionId:  resp[8:16],
	}
}

func buildAnnounceReq(connectionId []byte, torrent *torrentparser.TorrentFile, port uint16) ([]byte, error) {
	buf := make([]byte, 98)

	// Connection ID
	copy(buf[0:8], connectionId)

	// Action (1 = announce)
	binary.BigEndian.PutUint32(buf[8:12], 1)

	// Transaction ID (random)
	transactionId := make([]byte, 4)
	rand.Read(transactionId)
	copy(buf[12:16], transactionId)

	// Info hash
	infoHash, err := torrentparser.InfoHash(torrent)
	if err != nil {
		return nil, err
	}
	copy(buf[16:36], infoHash)

	// Peer ID
	copy(buf[36:56], util.GenId())

	// Downloaded (8 bytes, 0 for now)
	binary.BigEndian.PutUint64(buf[56:64], 0)

	// Left (8 bytes, total file size)
	copy(buf[64:72], torrentparser.Size(torrent))

	// Uploaded (8 bytes, 0 for now)
	binary.BigEndian.PutUint64(buf[72:80], 0)

	// Event (0 = none, 1 = completed, 2 = started, 3 = stopped)
	binary.BigEndian.PutUint32(buf[80:84], 0)

	// IP address (0 = default)
	binary.BigEndian.PutUint32(buf[84:88], 0)

	// Key (random)
	key := make([]byte, 4)
	rand.Read(key)
	copy(buf[88:92], key)

	// Num want (-1 = default)
	binary.BigEndian.PutUint32(buf[92:96], 0xFFFFFFFF)

	// Port
	binary.BigEndian.PutUint16(buf[96:98], port)

	return buf, nil
}

func parseAnnounceResp(resp []byte) *AnnounceResponse {
	result := &AnnounceResponse{
		Action:        binary.BigEndian.Uint32(resp[0:4]),
		TransactionId: binary.BigEndian.Uint32(resp[4:8]),
		Interval:      binary.BigEndian.Uint32(resp[8:12]),
		Leechers:      binary.BigEndian.Uint32(resp[12:16]),
		Seeders:       binary.BigEndian.Uint32(resp[16:20]),
		Peers:         []Peer{},
	}

	// Parse peers (6 bytes each: 4 bytes IP + 2 bytes port)
	peerData := resp[20:]
	for i := 0; i < len(peerData); i += 6 {
		if i+6 > len(peerData) {
			break
		}
		peer := Peer{
			IP: fmt.Sprintf("%d.%d.%d.%d",
				peerData[i], peerData[i+1], peerData[i+2], peerData[i+3]),
			Port: binary.BigEndian.Uint16(peerData[i+4 : i+6]),
		}
		result.Peers = append(result.Peers, peer)
	}

	return result
}

// HTTP Tracker Response structures
type HTTPTrackerResponse struct {
	Interval   int64  `bencode:"interval"`
	Peers      string `bencode:"peers"` // Compact format (binary)
	Complete   int64  `bencode:"complete,omitempty"`
	Incomplete int64  `bencode:"incomplete,omitempty"`
}

// getHTTPPeers implements HTTP tracker protocol
func getHTTPPeers(torrent *torrentparser.TorrentFile, announceURL *url.URL) ([]Peer, error) {
	infoHash, err := torrentparser.InfoHash(torrent)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate info hash: %v", err)
	}

	// Build query parameters
	params := url.Values{}
	params.Add("info_hash", string(infoHash))
	params.Add("peer_id", string(util.GenId()))
	params.Add("port", "6881")
	params.Add("uploaded", "0")
	params.Add("downloaded", "0")
	params.Add("left", strconv.FormatUint(binary.BigEndian.Uint64(torrentparser.Size(torrent)), 10))
	params.Add("compact", "1")
	params.Add("event", "started")

	// Build full URL
	fullURL := announceURL.String() + "?" + params.Encode()

	// Make HTTP GET request
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to contact HTTP tracker: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracker returned status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read tracker response: %v", err)
	}

	// Parse bencode response
	var trackerResp HTTPTrackerResponse
	err = bencode.Unmarshal(bytes.NewReader(body), &trackerResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tracker response: %v", err)
	}

	fmt.Printf("\n===== received HTTP tracker response =====\n")
	fmt.Printf("Seeders: %d, Leechers: %d\n", trackerResp.Complete, trackerResp.Incomplete)

	// Parse compact peer list
	peers := []Peer{}
	peerData := []byte(trackerResp.Peers)

	for i := 0; i < len(peerData); i += 6 {
		if i+6 > len(peerData) {
			break
		}
		peer := Peer{
			IP: fmt.Sprintf("%d.%d.%d.%d",
				peerData[i], peerData[i+1], peerData[i+2], peerData[i+3]),
			Port: binary.BigEndian.Uint16(peerData[i+4 : i+6]),
		}
		peers = append(peers, peer)
	}

	return peers, nil
}
