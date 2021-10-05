package dht_node

import (
	"fmt"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/martenwallewein/torrent-client/peers"
	"github.com/netsys-lab/dht"
	"github.com/scionproto/scion/go/lib/snet"
	"sync/atomic"
	"time"
)

const announceInterval = 5 * 60 // announce every 5min

type DhtNode struct {
	node              *dht.Server
	announceTicker    *time.Ticker
	stats             *dhtStats
	infoHash          [20]byte
	peerPort          uint16
	onNewPeerReceived func(peer peers.Peer)
}

type dhtStats struct {
	receivedPeers     uint32
	blockedPeers      uint32
	numberOfAnnounces uint32
	zeroPortsReceived uint32
}

// New creates a new DHT Node.
// peerPort, the port the controlling peer is listening to
// onNewPeerReceived, a function to be executed when a new Peer was found, used for adding the new peer to the
// controlling peers storage
func New(torrentInfoHash [20]byte, startingNodes []dht.Addr, peerPort int, onNewPeerReceived func(peer peers.Peer)) (*DhtNode, error) {
	stats := &dhtStats{}

	dhtConf := dht.NewDefaultServerConfig()
	dhtConf.OnAnnouncePeer = func(infoHash metainfo.Hash, scionAddr snet.UDPAddr, port int, portOk bool) {
		var infoH [20]byte
		copy(infoH[:], infoHash.Bytes())
		if torrentInfoHash != infoH || !portOk || port == 0 {
			atomic.AddUint32(&stats.blockedPeers, 1)
			if port == 0 {
				atomic.AddUint32(&stats.zeroPortsReceived, 1)
			}
			fmt.Printf("rejected peer %v - %v - %v - %v", infoHash, scionAddr.String(), port, portOk)
			return
		}

		newPeer := peers.Peer{
			IP:    scionAddr.Host.IP,
			Port:  uint16(scionAddr.Host.Port),
			Addr:  scionAddr.String(),
			Index: 0,
		}
		onNewPeerReceived(newPeer)
		atomic.AddUint32(&stats.receivedPeers, 1)
		fmt.Printf("handled announce for %v - %v - %v - %v", infoHash, scionAddr.String(), port, portOk)
	}
	dhtConf.StartingNodes = func() ([]dht.Addr, error) {
		return startingNodes, nil
	}
	node, err := dht.NewServer(dhtConf)
	if err != nil {
		fmt.Printf("error creating dht node %s", err)
		return nil, err
	}

	ticker := time.NewTicker(announceInterval * time.Second)
	dhtNode := DhtNode{node: node, announceTicker: ticker, stats: stats}
	go func() {
		for range ticker.C {
			dhtNode.announce()
		}
	}()
	dhtNode.announce()
	return &dhtNode, nil
}

func (d *DhtNode) announce() {
	atomic.AddUint32(&d.stats.numberOfAnnounces, 1)
	ps, _ := d.node.Announce(d.infoHash, int(d.peerPort), false)
	go d.consumePeers(ps)
}

func convertPeer(peer dht.Peer) peers.Peer {
	return peers.Peer{
		IP:    peer.IP,
		Port:  uint16(peer.Port),
		Addr:  peer.String(),
		Index: 0,
	}
}

func (d *DhtNode) consumePeers(announce *dht.Announce) {
	for v := range announce.Peers {
		for _, cp := range v.Peers {
			atomic.AddUint32(&d.stats.receivedPeers, 1)
			if cp.Port == 0 {
				atomic.AddUint32(&d.stats.blockedPeers, 1)
				atomic.AddUint32(&d.stats.zeroPortsReceived, 1)
				continue
			}
			d.onNewPeerReceived(convertPeer(cp))
		}
	}
}

func (d *DhtNode) Close() {
	d.announceTicker.Stop()
	d.PrintStats()
	d.node.Close()
}

func (d *DhtNode) PrintStats() {
	fmt.Printf("Announced %d times, recieved %d peers, blocked %d peers, blocked 0-port %d peers",
		d.stats.numberOfAnnounces, d.stats.receivedPeers, d.stats.blockedPeers, d.stats.zeroPortsReceived)
}
