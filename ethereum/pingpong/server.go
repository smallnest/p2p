package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/smallnest/log"
)

type MsgType string

const (
	Ping MsgType = "Ping"
	Pong         = "Pong"
)

type Message struct {
	MsgType MsgType   `json:"msg_type,omitempty"`
	Data    string    `json:"data,omitempty"`
	Created time.Time `json:"created,omitempty"`
}

var pingPongProto = p2p.Protocol{
	Name:    "ping-pong",
	Version: 1,
	Length:  1,
	Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
		pingMsg := Message{
			MsgType: Ping,
			Data:    time.Now().String(),
			Created: time.Now(),
		}

		data, _ := json.Marshal(pingMsg)

		// send the message
		err := p2p.Send(rw, 0, data)
		if err != nil {
			log.Infof("failed to send ping: %v", err)
			return err
		}
		log.Infof("sending ping to %+v", p)

		for {
			pmsg, err := rw.ReadMsg()
			if err != nil {
				return err
			}

			data = data[:0]
			err = pmsg.Decode(&data)
			if err != nil {
				log.Infof("failed to decode msg: %v", err)
				return err
			}

			var msg Message
			json.Unmarshal(data, &msg)
			log.Infof("received message: %+v", msg)

			if msg.MsgType == Ping {
				pongMsg := Message{
					MsgType: Pong,
					Data:    time.Now().String(),
					Created: time.Now(),
				}

				data, _ = json.Marshal(pongMsg)
				// send the message
				err = p2p.Send(rw, 0, data)
				if err != nil {
					log.Infof("failed to send pong: %v", err)
					return err
				}
				log.Infof("sending pong to %+v", p)
			}
		}

		return nil
	},
}

func main() {
	// start server 1
	key1, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("failed to generate private key: %v", err)
	}
	srv1 := createServer(key1, "p2p_server_1", ":9981")
	err = srv1.Start()
	if err != nil {
		log.Fatalf("failed to start server1: %v", err)
	}
	defer srv1.Stop()

	event1C := make(chan *p2p.PeerEvent)
	sub1 := srv1.SubscribeEvents(event1C)
	go func() {
		for {
			select {
			case e := <-event1C:
				if e.Type == "add" {
					log.Infof("#1 received peer add event: %v", e.Peer)
				} else if e.Type == "msgrecv" {
					log.Infof("#1 received message event: %v", e)
				}
			case <-sub1.Err():
				return
			}
		}
	}()

	node1Self := srv1.Self()
	log.Infof("node1: IP=%s, TCP Port:%d, UDP Port:%d, ID: %s", node1Self.IP, node1Self.TCP, node1Self.UDP, node1Self.ID)

	// start server 2
	key2, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("failed to generate private key: %v", err)
	}
	srv2 := createServer(key2, "p2p_server_2", ":9982")
	err = srv2.Start()
	if err != nil {
		log.Fatalf("failed to start server2: %v", err)
	}
	defer srv2.Stop()

	event2C := make(chan *p2p.PeerEvent)
	sub2 := srv2.SubscribeEvents(event2C)
	go func() {
		for {
			select {
			case e := <-event2C:
				if e.Type == "add" {
					log.Infof("#2 received peer add event: %+v", e.Peer)
				} else if e.Type == "msgrecv" {
					log.Infof("#2 received message event: %+v", e)
				}
			case <-sub2.Err():
				return
			}
		}
	}()

	node2Self := srv2.Self()
	log.Infof("node2: IP=%s, TCP Port:%d, UDP Port:%d, ID: %s", node2Self.IP, node2Self.TCP, node2Self.UDP, node2Self.ID)

	// add peers
	node2 := &discover.Node{
		IP:  node2Self.IP,
		TCP: node2Self.TCP,
		UDP: node2Self.UDP,
		ID:  node2Self.ID,
	}
	srv1.AddPeer(node2)

	time.Sleep(5 * time.Second)
	log.Infof("server1 peers: %v, server2 peers: %v", srv1.Peers(), srv2.Peers())

	select {}
}

func createServer(key *ecdsa.PrivateKey, name string, addr string) *p2p.Server {
	cfg := p2p.Config{
		MaxPeers:   10,
		PrivateKey: key,
		Name:       name,
		ListenAddr: addr,
		Protocols:  []p2p.Protocol{pingPongProto},
	}
	srv := &p2p.Server{
		Config: cfg,
	}

	return srv
}
