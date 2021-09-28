package server

import (
	"encoding/binary"
	"fmt"
	"net"

	smp "github.com/netsys-lab/scion-path-discovery/api"
	"github.com/netsys-lab/scion-path-discovery/packets"
	"github.com/netsys-lab/scion-path-discovery/pathselection"
	log "github.com/sirupsen/logrus"

	"github.com/martenwallewein/torrent-client/bitfield"
	"github.com/martenwallewein/torrent-client/handshake"
	"github.com/martenwallewein/torrent-client/message"
	"github.com/martenwallewein/torrent-client/peers"
	"github.com/martenwallewein/torrent-client/torrentfile"
	"github.com/scionproto/scion/go/lib/snet"
)

// A Client is a TCP connection with a peer
type Server struct {
	Conns       []packets.UDPConn
	Choked      bool
	peers       []peers.Peer
	infoHash    [20]byte
	lAddr       string
	localAddr   *snet.UDPAddr
	listener    *net.Listener
	Bitfield    bitfield.Bitfield
	torrentFile *torrentfile.TorrentFile
}

//LastSelection users could add more fields
type ServerSelection struct {
	lastSelectedPathSet pathselection.PathSet
}

//CustomPathSelectAlg this is where the user actually wants to implement its logic in
func (lastSel *ServerSelection) CustomPathSelectAlg(pathSet *pathselection.PathSet) (*pathselection.PathSet, error) {
	return pathSet.GetPathSmallHopCount(2), nil
}

func NewServer(lAddr string, torrentFile *torrentfile.TorrentFile) (*Server, error) {
	// localAddr, err := net.ResolveTCPAddr("tcp", lAddr)
	//if err != nil {
	//	return nil, err
	//}
	localAddr, _ := snet.ParseUDPAddr(lAddr)
	s := &Server{
		peers:       make([]peers.Peer, 0),
		Conns:       make([]packets.UDPConn, 0),
		lAddr:       lAddr,
		localAddr:   localAddr,
		torrentFile: torrentFile,
	}

	// TODO: Correct length!
	s.Bitfield = make([]byte, len(torrentFile.PieceHashes))
	for i := range torrentFile.PieceHashes {
		s.Bitfield.SetPiece(i)
	}

	return s, nil
}

func (s *Server) ListenHandshake() error {
	var err error
	// sock := socket.NewSocket("scion")

	// s.listener, err = net.ListenTCP("tcp", s.localAddr)

	// fmt.Printf("Listen TCP on %s\n", s.localAddr)

	// TODO: FIx loop with MPSock implementation
	// for {
	// Listen for an incoming connection.
	// conn, err := sock.Accept()
	sock := smp.NewMPPeerSock(s.lAddr, nil, &smp.MPSocketOptions{
		Transport:                   "QUIC",
		PathSelectionResponsibility: "CLIENT", // TODO: Change to server
	})
	err = sock.Listen()
	// s.listener, err = sock.Listen(s.localAddr.String())
	if err != nil {
		return err
	}
	// sel := ServerSelection{}
	// TODO: Add pathselection
	remote, err := sock.WaitForPeerConnect(nil)
	if err != nil {
		return err
	}

	sock.SetPeer(remote)

	// TODO: Remove public fields
	// connections := sock.UnderlaySocket.GetConnections()
	// TODO: Listen for connection change and update slice

	/*fmt.Printf("Accepted SCION/QUIC Connection on %s\n", conn.LocalAddr())
	if err != nil {
		fmt.Println("Error accepting: ", err.Error())
		return err
	}*/
	/*for i, conn := range connections {
		s.Conns = append(s.Conns, conn)
		log.Infof("Starting reading on conn %d with handshake %d", i, i == 0)
		go s.handleConnection(conn, true)
	}*/
	for {
		conns := <-sock.OnConnectionsChange
		log.Infof("Got new connections %d", len(conns))
		// if len(s.Conns) < len(conns) {
		//	for i := len(conns) - len(s.Conns); i < len(conns); i++ {
		//		log.Infof("Appending new Conn")
		//		s.Conns = append(s.Conns, conns[i])
		//		go s.handleConnection(conns[i], true)
		//	}
		//}
		if len(conns) < 2 {
			continue
		}
		for i, conn := range conns {
			s.Conns = append(s.Conns, conn)
			log.Infof("Starting reading on conn %d with handshake %d", i, i == 0)
			go s.handleConnection(conn, true)

		}
	}

	// TODO: Wait for next connection
	// time.Sleep(100 * time.Second)

	// return nil

	// }
}

func (s *Server) handleConnection(conn packets.UDPConn, waitForHandshake bool) error {
	if waitForHandshake {
		fmt.Printf("Handling handshake on conn %p\n", conn)
		s.handleIncomingHandshake(conn)
	}

	for {
		// fmt.Printf("Reading on Conn %p\n", conn)
		msg, err := message.Read(conn)
		if err != nil {
			return err
		}

		if msg == nil { // keep-alive
			return nil
		}
		// fmt.Printf("Got message %d\n", msg.ID)
		switch msg.ID {
		case message.MsgInterested:
			retMsg := message.Message{ID: message.MsgUnchoke, Payload: []byte{}}
			_, err := conn.Write(retMsg.Serialize())
			if err != nil {
				return err
			}
			fmt.Println("Sent back unChoke")
		case message.MsgRequest:
			index, begin, length := message.ParseRequest(msg)
			// fmt.Printf("Got request msg with index %d, begin %d, length %d\n", index, begin, length)
			buf := make([]byte, 8)
			binary.BigEndian.PutUint32(buf[0:4], uint32(index))
			binary.BigEndian.PutUint32(buf[4:8], uint32(begin))
			buf = append(buf, s.torrentFile.Content[(index*s.torrentFile.PieceLength)+begin:(index*s.torrentFile.PieceLength)+begin+length]...)
			// fmt.Println(buf[:128])
			retMsg := message.Message{ID: message.MsgPiece, Payload: buf}
			_, err := conn.Write(retMsg.Serialize())
			if err != nil {
				return err
			}
		}
	}
}

func (s *Server) handleIncomingHandshake(conn packets.UDPConn) error {
	fmt.Printf("%p: Waiting for Handshake message\n", conn)
	hs, err := handshake.Read(conn)
	fmt.Printf("%p: Got for Handshake message\n", conn)
	if err != nil {
		return err
	}

	_, err = conn.Write(hs.Serialize())
	if err != nil {
		return err
	}
	fmt.Printf("%p: Sent back Handshake message\n", conn)
	fmt.Printf("%p: Sending back bitfield\n", conn)
	msg := message.Message{ID: message.MsgBitfield, Payload: s.Bitfield}
	_, err = conn.Write(msg.Serialize())
	if err != nil {
		return err
	}
	fmt.Printf("%p: Sent back bitfield\n", conn)

	return nil
}
