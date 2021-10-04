package server

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/martenwallewein/torrent-client/bitfield"
	"github.com/martenwallewein/torrent-client/handshake"
	"github.com/martenwallewein/torrent-client/message"
	"github.com/martenwallewein/torrent-client/peers"
	"github.com/martenwallewein/torrent-client/socket"
	"github.com/martenwallewein/torrent-client/torrentfile"
	"github.com/scionproto/scion/go/lib/snet"
)

// A Client is a TCP connection with a peer
type Server struct {
	Conns       []*net.Conn
	Choked      bool
	peers       []peers.Peer
	infoHash    [20]byte
	lAddr       string
	localAddr   *snet.UDPAddr
	listener    *net.Listener
	Bitfield    bitfield.Bitfield
	torrentFile *torrentfile.TorrentFile
}

func NewServer(lAddr string, torrentFile *torrentfile.TorrentFile) (*Server, error) {
	// localAddr, err := net.ResolveTCPAddr("tcp", lAddr)
	//if err != nil {
	//	return nil, err
	//}
	localAddr, _ := snet.ParseUDPAddr(lAddr)
	s := &Server{
		peers:       make([]peers.Peer, 0),
		Conns:       make([]*net.Conn, 0),
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
	sock := socket.NewSocket("scion")
	s.listener, err = sock.Listen(s.localAddr.String())
	if err != nil {
		return err
	}
	// s.listener, err = net.ListenTCP("tcp", s.localAddr)

	// fmt.Printf("Listen TCP on %s\n", s.localAddr)

	for {
		// Listen for an incoming connection.
		conn, err := sock.Accept()
		fmt.Printf("Accepted TCP Connection on %s\n", conn.LocalAddr())
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			return err
		}
		s.Conns = append(s.Conns, &conn)
		go s.handleConnection(&conn)
	}
}

func (s *Server) handleConnection(conn *net.Conn) error {
	s.handleIncomingHandshake(conn)

	for {
		msg, err := message.Read(*conn)
		if err != nil {
			return err
		}

		if msg == nil { // keep-alive
			return nil
		}
		// fmt.Println("Got message %d\n", msg.ID)
		switch msg.ID {
		case message.MsgInterested:
			retMsg := message.Message{ID: message.MsgUnchoke, Payload: []byte{}}
			_, err := (*conn).Write(retMsg.Serialize())
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
			_, err := (*conn).Write(retMsg.Serialize())
			if err != nil {
				return err
			}
		}
	}
}

func (s *Server) handleIncomingHandshake(conn *net.Conn) error {
	fmt.Println("Waiting for Handshake message")
	hs, err := handshake.Read(*conn)
	fmt.Println("Got for Handshake message")
	if err != nil {
		return err
	}

	_, err = (*conn).Write(hs.Serialize())
	if err != nil {
		return err
	}
	fmt.Println("Sent back Handshake message")
	fmt.Println("Sending back bitfield")
	msg := message.Message{ID: message.MsgBitfield, Payload: s.Bitfield}
	_, err = (*conn).Write(msg.Serialize())
	if err != nil {
		return err
	}
	fmt.Println("Sent back bitfield")

	return nil
}
