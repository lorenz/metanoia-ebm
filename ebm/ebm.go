// Package ebm implements the Metanoia Ethernet Boot Management protocol as
// used in the MT-G5321, minus the bootloader which speaks a different
// protocol.
package ebm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/mdlayher/packet"
)

// Message is the generic structure is used by every EBM command and event.
type Message struct {
	Type           uint8
	SequenceNumber uint32
	Status         uint8
	Payload        []byte
}

func (m *Message) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(m.Type)
	binary.Write(&buf, binary.BigEndian, m.SequenceNumber)
	binary.Write(&buf, binary.BigEndian, uint16(len(m.Payload)))
	buf.WriteByte(m.Status)
	buf.Write(m.Payload)
	for buf.Len() < 46 {
		buf.WriteByte(0)
	}
	return buf.Bytes(), nil
}

func ParseMessage(data []byte) (*Message, error) {
	if len(data) < 7 {
		return nil, fmt.Errorf("too short message")
	}
	var msg Message
	msg.Type = data[0]
	msg.SequenceNumber = binary.BigEndian.Uint32(data[1:5])
	payloadLen := binary.BigEndian.Uint16(data[5:7])
	msg.Status = data[7]
	msg.Payload = data[8 : payloadLen+8]
	return &msg, nil
}

var statusDesc = map[uint8]string{
	0:   "OK",
	1:   "GTPI_NOT_FOUND",
	2:   "INVALID_ACCESSING",
	3:   "LENGTH_MISMATCH",
	4:   "INVALID_VALUE",
	5:   "PSD_ERROR",
	6:   "RMSC_ERROR",
	7:   "CONNECTED",
	16:  "LENGTH_EXCEEDS_PAYLOAD_SIZE",
	17:  "INCOMPLETE_CMD",
	18:  "ACCESS_DENIED",
	177: "DISCONNECTED",
	224: "QUESTION",
	225: "ANSWER_CORRECT",
	226: "ANSWER_WRONG",
	227: "OCCUPIED",
	228: "FORCED_CONNECT",
	255: "DEFAULT_STATUS",
}

const (
	StatusOk                       = 0
	StatusGTPINotFound             = 1
	StatusInvalidAccessing         = 2
	StatusLengthMismatch           = 3
	StatusInvalidValue             = 4
	StatusPSDError                 = 5
	StatusRMSCError                = 6
	StatusConnected                = 7
	StatusLengthExceedsPayloadSize = 16
	StatusIncompleteCommand        = 17
	StatusAccessDenied             = 18
	StatusDisconnected             = 177
	StatusQuestion                 = 224
	StatusAnswerCorrect            = 225
	StatusAnswerWrong              = 226
	StatusOccupied                 = 227
	StatusForcedConnect            = 228
	StatusDefault                  = 255
)

var typeDesc = map[uint8]string{
	1:    "READ_MEMORY",
	2:    "WRITE_MEMORY",
	6:    "READ_MIB",
	7:    "WRITE_MIB",
	0x30: "SEARCH_DEVICE",
	0x31: "CONNECT",
	0x33: "REBOOT_UPGRADE",
	0x40: "CONSOLE_INPUT",
	0x50: "SDP_DISCONNECT",
	0x60: "CONSOLE_OUTPUT",
	0x61: "LOGGER_OUTPUT",
	0x70: "DEVICE_DISCONNECT",
	0x81: "READ_MEMORY_RESP",
	0x82: "WRITE_MEMORY_RESP",
	0x86: "READ_MIB_RESP",
	0x87: "WRITE_MIB_RESP",
	0xb0: "SEARCH_DEVICE_RESP",
	0xb1: "CONNECT_RESP",
	0xb2: "DISCONNECT_RESP",
}

const (
	TypeReadMemory       = 1
	TypeWriteMemory      = 2
	TypeReadMIB          = 6
	TypeWriteMIB         = 7
	TypeSearchDevice     = 0x30
	TypeConnect          = 0x31
	TypeRebootUpgrade    = 0x33
	TypeConsoleInput     = 0x40
	TypeSDPDisconnect    = 0x50
	TypeConsoleOutput    = 0x60
	TypeLoggerOutput     = 0x61
	TypeDeviceDisconnect = 0x70
	TypeConnectResp      = 0xb1
)

func (m *Message) String() string {
	typeName := typeDesc[m.Type]
	if typeName == "" {
		typeName = fmt.Sprintf("UNK_%d", m.Type)
	}
	statusName := statusDesc[m.Status]
	if statusName == "" {
		statusName = fmt.Sprintf("UNK_%d", m.Status)
	}
	return fmt.Sprintf("type=%s seq=%d status=%s payload=%x", typeName, m.SequenceNumber, statusName, m.Payload)
}

type Conn struct {
	c     *packet.Conn
	addr  net.HardwareAddr
	seqNo uint32

	exchReq   chan *Message
	exchRes   chan *Message
	exchMutex sync.Mutex
	rxMsgChan chan []byte

	Logger          io.Writer
	HandleChallenge func(c uint32) uint32
}

func DefaultChallengeHandler(c uint32) uint32 {
	switch c {
	case 0x95743926:
		return 0x6e6f6961
	default:
		log.Printf("unknown challenge %d, returning 0", c)
		return 0
	}
}

func NewConnFromIf(iface *net.Interface, addr net.HardwareAddr) (*Conn, error) {
	c, err := packet.Listen(iface, packet.Datagram, 0x6120, &packet.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}
	return NewConn(c, addr), nil
}

func NewConn(c *packet.Conn, addr net.HardwareAddr) *Conn {
	c.SetReadDeadline(time.Time{})
	return &Conn{
		c:               c,
		addr:            addr,
		seqNo:           2,
		exchReq:         make(chan *Message),
		exchRes:         make(chan *Message),
		rxMsgChan:       make(chan []byte, 10),
		HandleChallenge: DefaultChallengeHandler,
	}
}

func (c *Conn) listener() {
	for {
		buf := make([]byte, 1514)
		n, _, err := c.c.ReadFrom(buf)
		if err != nil {
			fmt.Fprintf(c.Logger, "read error, quitting: %v\n", err)
			close(c.rxMsgChan)
			return
		}
		c.rxMsgChan <- buf[:n]
	}

}

func (c *Conn) reactor() {
	var curReq *Message
	curReqTimer := time.NewTimer(1 * time.Second)
	curReqTimer.Stop()
	for {
		select {
		case rxMsg, ok := <-c.rxMsgChan:
			if !ok {
				close(c.exchRes)
				return
			}
			res, err := ParseMessage(rxMsg)
			if err != nil {
				fmt.Fprintf(c.Logger, "error parsing message, ignoring: %v\n", err)
			}
			switch res.Type {
			case TypeConsoleOutput:
				c.Logger.Write(res.Payload)
			case TypeLoggerOutput:
				logType := binary.BigEndian.Uint16(res.Payload[20:22])
				switch logType {
				case 1:
					fmt.Printf("Modem Status: %v\n", modemStatusDesc[binary.BigEndian.Uint32(res.Payload[24:28])])
				case 4:
					fmt.Printf("Error: %v\n", errorDesc[binary.BigEndian.Uint32(res.Payload[24:28])])
				default:
					fmt.Printf("Log Type %v: %x", logTypeDesc[logType], res.Payload)
				}
			case TypeDeviceDisconnect:
				fmt.Fprintf(c.Logger, "device disconnect, closing: %v\n", err)
				close(c.exchReq)
				return
			default:
				if curReq == nil {
					fmt.Fprintf(c.Logger, "unknown message %v received, no requests pending\n", err)
					continue
				}
				if curReq.SequenceNumber != res.SequenceNumber {
					fmt.Fprintf(c.Logger, "WARNING: Sequence number mismatch %d != %d\n", curReq.SequenceNumber, res.SequenceNumber)
				}
				c.exchRes <- res
				if !curReqTimer.Stop() {
					<-curReqTimer.C
				}
				curReq = nil
			}
		case req := <-c.exchReq:
			req.SequenceNumber = c.seqNo
			reqRaw, err := req.MarshalBinary()
			if err != nil {
				fmt.Fprintf(c.Logger, "failed to marshal: %v\n", err)
				c.exchRes <- nil
				continue
			}
			if _, err := c.c.WriteTo(reqRaw, &packet.Addr{
				HardwareAddr: c.addr,
			}); err != nil {
				fmt.Fprintf(c.Logger, "failed to send: %v\n", err)
				c.exchRes <- nil
				continue
			}
			c.seqNo++
			curReq = req
			curReqTimer.Reset(1 * time.Second)
		case <-curReqTimer.C:
			fmt.Fprintf(c.Logger, "retrying send\n")
			reqRaw, err := curReq.MarshalBinary()
			if err != nil {
				c.exchRes <- nil
				continue
			}
			if _, err := c.c.WriteTo(reqRaw, &packet.Addr{
				HardwareAddr: c.addr,
			}); err != nil {
				fmt.Fprintf(c.Logger, "failed to send: %v\n", err)
				c.exchRes <- nil
				continue
			}
			curReqTimer.Reset(1 * time.Second)
		}
	}
}

func (c *Conn) Exchange(req *Message, exp uint8) (*Message, error) {
	c.exchMutex.Lock()
	defer c.exchMutex.Unlock()
	c.exchReq <- req
	res, ok := <-c.exchRes
	if !ok {
		return nil, errors.New("connection has shut down")
	}
	if res == nil {
		return nil, errors.New("an error occurred while processing")
	}
	return res, nil
}

func (c *Conn) connect(challangeRes, flags uint32) (*Message, error) {
	var p [8]byte
	binary.BigEndian.PutUint32(p[:4], challangeRes)
	binary.BigEndian.PutUint32(p[4:], flags)
	return c.Exchange(&Message{
		Type:    TypeConnect,
		Payload: p[:],
		Status:  StatusDefault,
	}, TypeConnectResp)
}

func (c *Conn) Dial() error {
	go c.listener()
	go c.reactor()
	res, err := c.connect(0xffffffff, 0x3c)
	if err != nil {
		return err
	}
	if res.Status == StatusForcedConnect || res.Status == StatusAnswerCorrect {
		return nil // We're connected
	}
	if res.Status == StatusQuestion {
		resp := c.HandleChallenge(binary.BigEndian.Uint32(res.Payload[4:8]))
		res, err = c.connect(resp, 0)
		if err != nil {
			return err
		}
		if res.Status == StatusForcedConnect || res.Status == StatusAnswerCorrect {
			return nil
		}
		return fmt.Errorf("connection request failed: %s", res)
	}
	return fmt.Errorf("connection request failed: %s", res)
}

func (c *Conn) ReadMIB(o *OID) (any, error) {
	req, err := o.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OID request: %w", err)
	}
	resRaw, err := c.Exchange(&Message{
		Type:    TypeReadMIB,
		Status:  StatusDefault,
		Payload: req,
	}, TypeReadMIB)
	if err != nil {
		return nil, fmt.Errorf("failed to request OID: %w", err)
	}
	if resRaw.Status != StatusOk {
		return nil, fmt.Errorf("failed to request OID: %v", resRaw)
	}
	res, err := ParseOID(resRaw.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return res, nil
}
func (c *Conn) WriteMIB(o *OID, value any) error {
	req, err := MarshalOID(o, value)
	if err != nil {
		return fmt.Errorf("failed to marshal OID write: %w", err)
	}
	resRaw, err := c.Exchange(&Message{
		Type:    TypeWriteMIB,
		Status:  StatusDefault,
		Payload: req,
	}, TypeWriteMIB)
	if err != nil {
		return fmt.Errorf("failed to write OID: %w", err)
	}
	if resRaw.Status != StatusOk {
		return fmt.Errorf("failed to write OID: %v", resRaw)
	}
	return nil
}

// type (2 bytes)
var logTypeDesc = map[uint16]string{
	0: "eyebox",
	1: "modem status",
	2: "training SNR",
	3: "showtime SNR",
	4: "SOC message error",
	5: "OLR",
	6: "overheating",
	7: "snapshot",
}

// data + 24
var errorDesc = map[uint32]string{
	4:  "synchro 1-1 failed",
	7:  "o-signature failed",
	8:  "synchro 1 failed",
	9:  "timeline sequencer timeout",
	10: "SOC message error",
	11: "high BER event",
	16: "high BER event 2",
	32: "pilot low SNR",
	33: "RMC low SNR",
	35: "ETR < ETR_MIN",
}

type ModemStatus uint32

const (
	ModemStatusIdle   = 0
	ModemStatusSilent = 1
)

// modem status (data + 24)
var modemStatusDesc = map[uint32]string{
	0:  "idle",
	1:  "silent",
	2:  "init handshake",
	3:  "init train",
	4:  "showtime",
	5:  "selftest",
	6:  "unit fail",
	7:  "deactivating 1",
	8:  "deactivating 2",
	9:  "init handshake only",
	10: "init train only",
	12: "quick showtime",
	13: "AFE TX test",
	14: "AFE RX test",
	15: "AFE loopback",
}
