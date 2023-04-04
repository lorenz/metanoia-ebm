// Package bootloader implements the bootloader part of the Ethernet Boot
// Management protocol until a firmware has been loaded and the modem has
// been booted.
package bootloader

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"time"

	"git.dolansoft.org/lorenz/metanoia-ebm/srec"
	"github.com/mdlayher/packet"
)

type message struct {
	SequenceNumber uint16
	Type           uint16
	Payload        []byte
}

func (m *message) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	if len(m.Payload) > math.MaxUint16 {
		return nil, fmt.Errorf("payload larger than 2^16, invalid")
	}

	binary.Write(&buf, binary.BigEndian, m.SequenceNumber)
	binary.Write(&buf, binary.BigEndian, uint16(len(m.Payload)))
	binary.Write(&buf, binary.BigEndian, m.Type)
	buf.Write(m.Payload)
	for buf.Len() < 46 {
		buf.WriteByte(0)
	}
	return buf.Bytes(), nil
}

func parseMessage(data []byte) (*message, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("too short message")
	}
	var msg message
	msg.SequenceNumber = binary.BigEndian.Uint16(data[0:2])
	payloadLen := binary.BigEndian.Uint16(data[2:4])
	msg.Type = binary.BigEndian.Uint16(data[4:6])
	msg.Payload = data[6 : payloadLen+6]
	return &msg, nil
}

const (
	typeAssociateReq   = 0x01
	typeAssociateRes   = 0x02
	typeDownloadBegin  = 0x11
	typeDownloadRecord = 0x12
	typeDownloadEnd    = 0x13
	typeAck            = 0x14
)

func associateRequest(hwaddr net.HardwareAddr) *message {
	var buf bytes.Buffer

	binary.Write(&buf, binary.BigEndian, uint32(0x20304))
	buf.Write([]byte(hwaddr))
	binary.Write(&buf, binary.BigEndian, uint32(1))
	binary.Write(&buf, binary.BigEndian, uint32(2))
	binary.Write(&buf, binary.BigEndian, uint32(3))
	return &message{
		Type:    typeAssociateReq,
		Payload: buf.Bytes(),
	}
}

func downloadBegin() *message {
	return &message{
		Type:    typeDownloadBegin,
		Payload: []byte{0xba, 0, 0, 0, 0x00, 0x01, 0x02, 0x03, 0x0a, 0x0b, 0x0c, 0x0d},
	}
}

func downloadRecord(payload []byte) *message {
	return &message{
		Type:    typeDownloadRecord,
		Payload: payload,
	}
}

func downloadEnd(checksum uint32) *message {
	payload := [8]byte{0, 0, 0, 0, 0xf4, 0xee, 0x00, 0xdd}
	binary.BigEndian.PutUint32(payload[:4], checksum)
	return &message{
		Type:    typeDownloadEnd,
		Payload: payload[:],
	}
}

type conn struct {
	c     *packet.Conn
	addr  net.HardwareAddr
	seqNo uint16
}

func NewConn(iface *net.Interface) (*conn, error) {
	c, err := packet.Listen(iface, packet.Datagram, 0x6120, &packet.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}
	return &conn{c: c, addr: metanoiaDefaultAddr, seqNo: 1}, nil
}

func (c *conn) SetAddr(newAddr net.HardwareAddr) {
	c.addr = newAddr
}

func (c *conn) Exchange(req *message) (*message, error) {
	req.SequenceNumber = c.seqNo
	c.seqNo++
	reqRaw, err := req.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal req: %w", err)
	}
	buf := make([]byte, 1600)
	for i := 0; i < 5; i++ {
		if _, err := c.c.WriteTo(reqRaw, &packet.Addr{
			HardwareAddr: c.addr,
		}); err != nil {
			return nil, fmt.Errorf("failed to send packet: %w", err)
		}
		c.c.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, _, err := c.c.ReadFrom(buf)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			fmt.Println("No response in 1s, retrying")
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("error reading response: %w", err)
		}
		res, err := parseMessage(buf[:n])
		if err != nil {
			return nil, fmt.Errorf("error parsing response: %w", err)
		}
		if res.SequenceNumber != req.SequenceNumber {
			fmt.Printf("Bad sequence number %d, expected %d, dropping", res.SequenceNumber, req.SequenceNumber)
			continue
		}
		return res, nil
	}
	return nil, errors.New("no response after 5 tries")
}

var (
	metanoiaDefaultAddr = net.HardwareAddr{0x00, 0x0e, 0xad, 0x33, 0x44, 0x55}
)

type XorStream struct {
	W   io.Writer
	Key []byte
	n   int
}

func (s *XorStream) Write(data []byte) (int, error) {
	processedData := make([]byte, len(data))
	for i := range data {
		processedData[i] = data[i] ^ s.Key[s.n%len(s.Key)]
		s.n++
	}
	return s.W.Write(processedData)
}

// DownloadAndBoot connects to the modem attached to the pc connection, assigns
// it hwAddr as a MAC address, downloads the firmware in S-Record format (only
// S3 records/32 bit addresses supported) and boots it.
func DownloadAndBoot(pc *packet.Conn, hwAddr net.HardwareAddr, firmwareSrec io.Reader) error {
	c := conn{
		c:     pc,
		addr:  metanoiaDefaultAddr,
		seqNo: 1,
	}

	res, err := c.Exchange(associateRequest(hwAddr))
	if err != nil {
		return fmt.Errorf("error exchanging EBM message: %w", err)
	}
	if res.Type != typeAssociateRes {
		return fmt.Errorf("invalid response to AssociateRequest: %+v", res)
	}
	if res.Payload[0] != 0 {
		return fmt.Errorf("error status %d in AssociateRespone", res.Payload[0])
	}
	c.SetAddr(hwAddr)

	res2, err := c.Exchange(downloadBegin())
	if err != nil {
		return fmt.Errorf("error exchanging EBM message: %w", err)
	}
	if res2.Type != typeAck {
		return fmt.Errorf("invalid response to DownloadBegin: %+v", res)
	}
	if res2.Payload[0] != 0 {
		return fmt.Errorf("error status %d in DownloadAck", res2.Payload[0])
	}

	key, err := hex.DecodeString("b4df157369be2ae7d37c55cea6f8ab9d4df1573b9be2ae7637c55ced6f8ab9dadf1573b4be2ae7697c55ced3f8ab9da6f1573b4de2ae769bc55ced378ab9da6f1573b4df2ae769be55ced37cab9da6f8573b4df1ae769be25ced37c5b9da6f8a73b4df15e769be2aced37c559da6f8ab3b4df157769be2aeed37c55cda6f8ab9")
	if err != nil {
		panic(err)
	}
	var buf bytes.Buffer
	os := XorStream{
		W:   &buf,
		Key: key,
	}

	fwS := bufio.NewScanner(firmwareSrec)
	for fwS.Scan() {
		buf.Reset()
		typ, payload, err := srec.ParseGeneric(fwS.Text())
		if err != nil {
			return fmt.Errorf("error parsing S-Record %q: %w", fwS.Text(), err)
		}
		if typ != 3 {
			continue
		}
		addr := payload[0:4]
		data := payload[4:]
		os.Write(addr)
		binary.Write(&os, binary.BigEndian, uint32(len(data)/4))
		os.Write(data)

		res, err := c.Exchange(downloadRecord(buf.Bytes()))
		if err != nil {
			return fmt.Errorf("failed to exchange firmware packet: %w", err)
		}
		if res.Type != typeAck {
			return fmt.Errorf("invalid response to Download: %+v", res)
		}
		if res.Payload[0] != 0 {
			return fmt.Errorf("error status %d in DownloadAck", res.Payload[0])
		}
	}

	// TODO: Figure out checksum (probably CRC32 IEEE, but over what?)
	res3, err := c.Exchange(downloadEnd(0x02792767))
	if err != nil {
		return fmt.Errorf("error exchanging EBM message: %w", err)
	}
	if res3.Type != typeAck {
		return fmt.Errorf("invalid response to DownloadEnd: %+v", res)
	}
	if res3.Payload[0] != 0 {
		return fmt.Errorf("error status %d in DownloadAck", res2.Payload[0])
	}
	return nil
}
