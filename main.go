package main

import (
	"fmt"
	"github.com/martenwallewein/torrent-client/config"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/martenwallewein/torrent-client/server"
	"github.com/martenwallewein/torrent-client/torrentfile"
)

func main() {
	inPath := os.Args[1]  // .torrent file
	outPath := os.Args[2] // file to write downloaded pieces to
	peer := os.Args[3]    // listening addr for seeder, target-peer addr for lecher
	seed := os.Args[4]    // seed or leech
	file := os.Args[5]    // original file (for seeders)
	numCons := os.Args[6]
	nCons, err := strconv.Atoi(numCons)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Input %s, Output %s, Peer %s, seed %s, file %s\n", inPath, outPath, peer, seed, file)
	tf, err := torrentfile.Open(inPath)
	if err != nil {
		log.Fatal(err)
	}

	peerDiscoveryConfig := config.DefaultPeerDisoveryConfig()

	if seed == "true" {
		fmt.Println("Loading file to RAM")
		tf.Content, err = ioutil.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Loaded file to RAM")
		i := 0
		startPort := 42423
		for port := startPort; port < startPort+nCons; port++ {
			peer := fmt.Sprintf("%s:%d", peer, startPort+i)
			server, _ := initServer(peer, &tf, &peerDiscoveryConfig)
			defer server.Close()
			peerDiscoveryConfig = config.NoPeerDisoveryConfig() // only first server does peer discovery for now
		}

		if err != nil {
			log.Fatal(err)
		}
	} else {
		err = tf.DownloadToFile(outPath, peer, nCons, &peerDiscoveryConfig)
		if err != nil {
			log.Fatal(err)
		}
	}

}

func initServer(peerAddr string, tf *torrentfile.TorrentFile, dc *config.PeerDiscoveryConfig) (*server.Server, error) {
	server, err := server.NewServer(peerAddr, tf, dc)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Created Server")

	err = server.ListenHandshake()
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	return server, err
}
