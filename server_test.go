package wi

import (
	"bytes"
	"encoding/hex"
	"net"
	"reflect"
	"testing"
)

func TestSignMarshal(t *testing.T) {
	sign := "default"
	uuid := "mock#00001"
	s := Sign{Sign: sign, UUID: uuid}
	pkt, _ := s.Marshal()
	t.Logf("pkt: [%v]", hex.EncodeToString(pkt))
}

func TestMsgMarshal(t *testing.T) {
	sign := "default"
	uuid := "mock#00001"
	s := Sign{Sign: sign, UUID: uuid}
	msg := SignedMessage{s, []byte("hello")}
	pkt, _ := msg.Marshal()
	t.Logf("pkt: [%v]", hex.EncodeToString(pkt))
}

func TestListenPacketUDP(t *testing.T) {
	// init data
	signAck := Ack{Block: 0}
	signAckBytes, err := signAck.Marshal()
	msgAck := Ack{Block: 0}
	msgAckBytes, err := msgAck.Marshal()

	sign := "test"

	uuid := "#00001"
	clientSign := Sign{sign, uuid}
	clientSignPkt, _ := clientSign.Marshal()
	t.Logf("sign pkt: [%v]", hex.EncodeToString(clientSignPkt))

	uuidA := "#00002"
	clientASign := Sign{sign, uuidA}

	text := "hello beautiful world"
	clientMsg := SignedMessage{Sign: clientSign, Payload: []byte(text)}
	clientMsgPkt, err := clientMsg.Marshal()

	textA := "beautiful world"
	clientAMsg := SignedMessage{Sign: clientASign, Payload: []byte(textA)}
	clientAMsgPkt, err := clientAMsg.Marshal()

	serverAddr := setUpServer(t)
	client, err := setUpClient(t)
	defer func() { _ = client.Close() }()

	// test send sign, server should ack
	buf := make([]byte, DatagramSize)
	sAddr, _ := net.ResolveUDPAddr("udp", serverAddr)

	// send sign
	_, err = client.WriteTo(clientSignPkt, sAddr)
	if err != nil {
		t.Fatal(err)
	}

	// read ack
	n, _, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(signAckBytes, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", signAckBytes, buf[:n])
	}

	// clientA send text
	clientA := Client{ServerAddr: serverAddr, Status: make(chan struct{}), UUID: uuidA, Sign: sign}
	go func() {
		clientA.ListenAndServe("127.0.0.1:")
	}()
	clientA.Ready()
	clientA.SendText(textA)

	// client read text. client should receive "beautiful world" from clientA
	n, _, err = client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(clientAMsgPkt, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", clientAMsgPkt, buf[:n])
	}
	// send ack
	client.WriteTo(msgAckBytes, sAddr)

	// client send text, server should ack
	_, err = client.WriteTo(clientMsgPkt, sAddr)
	if err != nil {
		t.Fatal(err)
	}
	// read ack
	n, _, err = client.ReadFrom(buf)
	if !bytes.Equal(msgAckBytes, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", signAckBytes, buf[:n])
	}
}

func TestPubSub(t *testing.T) {
	serverAddr := setUpServer(t)
	pub := newClient(serverAddr, "pub")
	sub := newClient(serverAddr, "sub")
	other := newClient(serverAddr, "other")
	_ = pub.PublishFile("test.txt", 1, 1)
	wrq := <-sub.FileMessages
	if wrq.FileId != 1 {
		t.Errorf("expected file id 1; actual file id %d", wrq.FileId)
	}
	if wrq.Filename != "test.txt" {
		t.Errorf("expected filename \"test.txt\"; actual filename %q", wrq.Filename)
	}
	q := <-other.FileMessages
	if !reflect.DeepEqual(wrq, q) {
		t.Errorf("expected file message %v; actual file message %v", wrq, q)
	}
	_ = sub.SubscribeFile(1, "pub")
	rrq := <-pub.SubMessages
	if rrq.FileId != 1 {
		t.Errorf("expected file id 1; actual file id %d", rrq.FileId)
	}
	if rrq.Subscriber != "sub" {
		t.Errorf("expected subscriber \"sub\"; actual subscriber %q", rrq.Subscriber)
	}
}

func newClient(serverAddr string, UUID string) *Client {
	client := Client{ServerAddr: serverAddr, Status: make(chan struct{}), UUID: UUID, Sign: "default"}
	go func() {
		client.ListenAndServe("127.0.0.1:")
	}()
	client.Ready()
	return &client
}

func setUpClient(t *testing.T) (net.PacketConn, error) {
	client, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	return client, err
}

func setUpServer(t *testing.T) string {
	s := Server{}
	serverAddr := "127.0.0.1:52000"
	go func() {
		t.Error(s.ListenAndServe(serverAddr))
	}()
	return serverAddr
}
