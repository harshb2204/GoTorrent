package tracker

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

type Peer struct {
	IP   string
	Port uint16
}

type ConnectResponse struct {
	Action        uint32
	TransactionId uint32
	ConnectionId  []byte
}

type AnnounceResponse struct {
	Action        uint32
	TransactionId uint32
	Interval      uint32
	Leechers      uint32
	Seeders       uint32
	Peers         []Peer
}

// GetPeers tries all tracker URLs and returns the first successful peer list
func GetPeers(torrent *torrentparser.TorrentFile) ([]Peer, error) {
	trackerURLs := torrentparser.GetTrackerURLs(torrent)
	if len(trackerURLs) == 0 {
		return nil, fmt.Errorf("no tracker URLs found in torrent")
	}

	var lastErr error
	for _, trackerURL := range trackerURLs {
		fmt.Printf("\nTrying tracker: %s\n", trackerURL)

		announceURL, err := url.Parse(trackerURL)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse URL %s: %v", trackerURL, err)
			continue
		}

		var peers []Peer
		switch announceURL.Scheme {
		case "http", "https":
			peers, err = getHTTPPeers(torrent, announceURL)
		case "udp":
			peers, err = getUDPPeers(torrent, announceURL)
		default:
			err = fmt.Errorf("unsupported scheme: %s", announceURL.Scheme)
		}

		if err != nil {
			fmt.Printf("Tracker %s failed: %v\n", trackerURL, err)
			lastErr = err
			continue
		}

		if len(peers) > 0 {
			fmt.Printf("Got %d peers from %s\n", len(peers), trackerURL)
			return peers, nil
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all trackers failed, last error: %v", lastErr)
	}
	return nil, fmt.Errorf("no peers found from any tracker")
}

func getUDPPeers(torrent *torrentparser.TorrentFile, announceURL *url.URL) ([]Peer, error) {
	host := announceURL.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(announceURL.Hostname(), "80")
	}

	// Resolve DNS first
	udpAddr, err := net.ResolveUDPAddr("udp", host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %v", host, err)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(15 * time.Second))

	// 1. Send connect request
	connectReq := buildConnectReq()
	_, err = conn.Write(connectReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send connect request: %v", err)
	}

	// 2. Receive connect response
	buf := make([]byte, 16)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to receive connect response: %v", err)
	}
	if n < 16 {
		return nil, fmt.Errorf("connect response too short: %d bytes", n)
	}

	connectResp := parseConnectResp(buf[:n])
	if connectResp.Action != 0 {
		return nil, fmt.Errorf("unexpected connect action: %d", connectResp.Action)
	}

	// 3. Send announce request
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	announceReq, err := buildAnnounceReq(connectResp.ConnectionId, torrent, 6881)
	if err != nil {
		return nil, fmt.Errorf("failed to build announce request: %v", err)
	}

	_, err = conn.Write(announceReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send announce request: %v", err)
	}

	// 4. Receive announce response
	buf = make([]byte, 4096)
	n, err = conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to receive announce response: %v", err)
	}
	if n < 20 {
		return nil, fmt.Errorf("announce response too short: %d bytes", n)
	}

	announceResp := parseAnnounceResp(buf[:n])
	if announceResp.Action != 1 {
		return nil, fmt.Errorf("unexpected announce action: %d (error?)", announceResp.Action)
	}

	fmt.Printf("Seeders: %d, Leechers: %d, Peers: %d\n",
		announceResp.Seeders, announceResp.Leechers, len(announceResp.Peers))

	return announceResp.Peers, nil
}

func buildConnectReq() []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf[0:8], 0x41727101980)
	binary.BigEndian.PutUint32(buf[8:12], 0)
	transactionId := make([]byte, 4)
	rand.Read(transactionId)
	copy(buf[12:16], transactionId)
	return buf
}

func parseConnectResp(resp []byte) *ConnectResponse {
	return &ConnectResponse{
		Action:        binary.BigEndian.Uint32(resp[0:4]),
		TransactionId: binary.BigEndian.Uint32(resp[4:8]),
		ConnectionId:  resp[8:16],
	}
}

func buildAnnounceReq(connectionId []byte, torrent *torrentparser.TorrentFile, port uint16) ([]byte, error) {
	buf := make([]byte, 98)
	copy(buf[0:8], connectionId)
	binary.BigEndian.PutUint32(buf[8:12], 1)

	transactionId := make([]byte, 4)
	rand.Read(transactionId)
	copy(buf[12:16], transactionId)

	infoHash, err := torrentparser.InfoHash(torrent)
	if err != nil {
		return nil, err
	}
	copy(buf[16:36], infoHash)
	copy(buf[36:56], util.GenId())

	binary.BigEndian.PutUint64(buf[56:64], 0)                   // downloaded
	copy(buf[64:72], torrentparser.Size(torrent))                // left
	binary.BigEndian.PutUint64(buf[72:80], 0)                   // uploaded
	binary.BigEndian.PutUint32(buf[80:84], 2)                   // event: started
	binary.BigEndian.PutUint32(buf[84:88], 0)                   // IP
	key := make([]byte, 4)
	rand.Read(key)
	copy(buf[88:92], key)
	binary.BigEndian.PutUint32(buf[92:96], 0xFFFFFFFF)          // num_want: -1
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

	peerData := resp[20:]
	for i := 0; i+6 <= len(peerData); i += 6 {
		ip := fmt.Sprintf("%d.%d.%d.%d",
			peerData[i], peerData[i+1], peerData[i+2], peerData[i+3])
		port := binary.BigEndian.Uint16(peerData[i+4 : i+6])
		if ip != "0.0.0.0" && port != 0 {
			result.Peers = append(result.Peers, Peer{IP: ip, Port: port})
		}
	}

	return result
}

// HTTP Tracker Response structures
type HTTPTrackerResponse struct {
	Interval   int64  `bencode:"interval"`
	Peers      string `bencode:"peers"`
	Complete   int64  `bencode:"complete,omitempty"`
	Incomplete int64  `bencode:"incomplete,omitempty"`
}

func getHTTPPeers(torrent *torrentparser.TorrentFile, announceURL *url.URL) ([]Peer, error) {
	infoHash, err := torrentparser.InfoHash(torrent)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate info hash: %v", err)
	}

	// Build the URL manually to avoid double-encoding the info_hash
	params := url.Values{}
	params.Add("peer_id", string(util.GenId()))
	params.Add("port", "6881")
	params.Add("uploaded", "0")
	params.Add("downloaded", "0")
	params.Add("left", strconv.FormatInt(torrentparser.TotalLength(torrent), 10))
	params.Add("compact", "1")
	params.Add("event", "started")

	// info_hash must be URL-encoded byte-by-byte, not as a UTF-8 string
	encodedHash := ""
	for _, b := range infoHash {
		encodedHash += fmt.Sprintf("%%%02x", b)
	}

	fullURL := announceURL.String() + "?info_hash=" + encodedHash + "&" + params.Encode()

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read tracker response: %v", err)
	}

	// Check for error response
	var errorResp struct {
		FailureReason string `bencode:"failure reason"`
	}
	bencode.Unmarshal(bytes.NewReader(body), &errorResp)
	if errorResp.FailureReason != "" {
		return nil, fmt.Errorf("tracker error: %s", errorResp.FailureReason)
	}

	var trackerResp HTTPTrackerResponse
	err = bencode.Unmarshal(bytes.NewReader(body), &trackerResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tracker response: %v", err)
	}

	fmt.Printf("Seeders: %d, Leechers: %d\n", trackerResp.Complete, trackerResp.Incomplete)

	peers := []Peer{}
	peerData := []byte(trackerResp.Peers)
	for i := 0; i+6 <= len(peerData); i += 6 {
		ip := fmt.Sprintf("%d.%d.%d.%d",
			peerData[i], peerData[i+1], peerData[i+2], peerData[i+3])
		port := binary.BigEndian.Uint16(peerData[i+4 : i+6])
		if ip != "0.0.0.0" && port != 0 {
			peers = append(peers, Peer{IP: ip, Port: port})
		}
	}

	return peers, nil
}
