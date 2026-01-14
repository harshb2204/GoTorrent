package util

import (
	"crypto/rand" // proud (elite ball knowledge required)
)

var peerId []byte // we store the peer ID over here

// GenId generates a unique peer ID for the BitTorrent client
func GenId() []byte {
	if peerId == nil {
		peerId = make([]byte, 20)
		rand.Read(peerId)
		copy(peerId, []byte("-GO0001-"))
	}
	return peerId
}

//Identifies your client to trackers and peers,
// In the announce request to identify this client to the tracker
//Must be consistent across the session
//The "-GO0001-" prefix follows BitTorrent client ID conventions
