package srec

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

func ParseGeneric(record string) (int, []byte, error) {
	if !strings.HasPrefix(record, "S") {
		return 0, nil, fmt.Errorf("record does not start with S")
	}
	if len(record) < 10 {
		return 0, nil, fmt.Errorf("record contains less than 10 characters, invalid")
	}
	typ, err := strconv.Atoi(record[1:2])
	if err != nil {
		return 0, nil, fmt.Errorf("record does not have a valid type: %w", err)
	}
	byteCount, err := hex.DecodeString(record[2:4])
	if err != nil {
		return 0, nil, fmt.Errorf("failed to decode byte count hex: %w", err)
	}
	if hex.EncodedLen(int(byteCount[0])) > len(record)-4 {
		return 0, nil, fmt.Errorf("byte count larger than record")
	}
	payload := make([]byte, byteCount[0])
	_, err = hex.Decode(payload, []byte(record[4:]))
	if err != nil {
		return 0, nil, fmt.Errorf("failed to decode payload: %w", err)
	}
	var gotChecksum byte
	gotChecksum += byteCount[0]
	for _, b := range payload[:len(payload)-1] {
		gotChecksum += b
	}
	gotChecksum ^= 0xff
	expectedCksum := payload[len(payload)-1]
	if expectedCksum != gotChecksum {
		return 0, nil, fmt.Errorf("invalid checksum, expected %x, got %x", expectedCksum, gotChecksum)
	}
	return typ, payload[:len(payload)-1], nil
}

func genericRecord(typ int, addr any, payload []byte) string {
	if typ < 0 || typ > 9 {
		panic("wrong record type")
	}
	var rec bytes.Buffer
	recLen := binary.Size(addr) + len(payload) + 1
	if recLen > 255 {
		panic("record too long")
	}
	rec.WriteByte(byte(recLen))
	binary.Write(&rec, binary.BigEndian, addr)
	rec.Write(payload)
	var sum byte
	for _, b := range rec.Bytes() {
		sum += b
	}
	rec.WriteByte(sum ^ 0xff) // Checksum
	return fmt.Sprintf("S%d%X\n", typ, rec.Bytes())
}

func S0(comment string) string {
	return genericRecord(0, uint16(0), []byte(comment))
}

func S1(addr uint16, data []byte) string {
	return genericRecord(1, addr, data)
}

func S3(addr uint32, data []byte) string {
	return genericRecord(3, addr, data)
}

func S7(addr uint32) string {
	return genericRecord(7, addr, []byte{})
}
