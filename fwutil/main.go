package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"os"

	"git.dolansoft.org/lorenz/metanoia-ebm/srec"
)

var (
	fwPackPath = flag.String("fw-pack", "", "Path to the Metanoia firmware pack")
	outPath    = flag.String("out", "", "Path where the Motorola S-Rec file with the deobfuscated firmware should be created")
)

func main() {
	flag.Parse()
	if *fwPackPath == "" {
		log.Fatalln("fw-pack needs to be set")
	}
	if *outPath == "" {
		log.Fatalln("out needs to be set")
	}
	firmware, err := os.Open(*fwPackPath)
	if err != nil {
		log.Fatal(err)
	}
	key, err := hex.DecodeString("b4df157369be2ae7d37c55cea6f8ab9d4df1573b9be2ae7637c55ced6f8ab9dadf1573b4be2ae7697c55ced3f8ab9da6f1573b4de2ae769bc55ced378ab9da6f1573b4df2ae769be55ced37cab9da6f8573b4df1ae769be25ced37c5b9da6f8a73b4df15e769be2aced37c559da6f8ab3b4df157769be2aeed37c55cda6f8ab9")
	if err != nil {
		log.Fatal(err)
	}

	outHex, err := os.Create(*outPath)
	if err != nil {
		log.Fatal(err)
	}

	header := make([]byte, 512)
	if _, err := io.ReadFull(firmware, header); err != nil {
		log.Fatal(err)
	}
	packSignature := binary.BigEndian.Uint32(header[:4])
	if packSignature != 0x61232321 {
		log.Fatalf("Bad signature %x", packSignature)
	}
	packVersion := binary.BigEndian.Uint32(header[16:20])
	if packVersion != 0x20000 {
		log.Fatalf("Unknown pack version %x", packVersion)
	}
	numberOfFirmwares := binary.BigEndian.Uint32(header[4:8])
	if numberOfFirmwares*32 >= uint32(len(header)) {
		log.Fatalf("More firmwares than fit in the header: %d", numberOfFirmwares)
	}
	var mt5321FwIdx int = -1
	for i := 1; i <= int(numberOfFirmwares); i++ {
		firmwareSig := binary.BigEndian.Uint32(header[i*32 : (i*32)+4])
		if firmwareSig == 0x23210010 {
			mt5321FwIdx = i
			break
		}
	}
	if mt5321FwIdx == -1 {
		log.Fatalf("No MT5321 firmware signature found in firmware file")
	}

	metaStart := 32 * mt5321FwIdx
	fwStartOffset := binary.BigEndian.Uint32(header[metaStart+16 : metaStart+20])
	numberOfRecords := binary.BigEndian.Uint32(header[metaStart+24 : metaStart+28])
	fwSize := binary.BigEndian.Uint32(header[metaStart+4 : metaStart+8])
	fmt.Printf("fwStartOffset: %d\n", fwStartOffset)
	fmt.Printf("numberOfRecords: %d\n", numberOfRecords)
	fmt.Printf("fwSize: %d\n", fwSize)
	_, err = firmware.Seek(int64(fwStartOffset), io.SeekStart)
	if err != nil {
		log.Fatal(err)
	}

	fwDataR := io.LimitReader(firmware, int64(fwSize))

	var signature, checksum uint32

	cksum := crc32.NewIEEE()

	binary.Read(fwDataR, binary.BigEndian, &signature)
	binary.Read(fwDataR, binary.BigEndian, &checksum)

	var fwData []byte

	buf := make([]byte, 4096)

	for {
		n, err := fwDataR.Read(buf)
		if errors.Is(err, io.EOF) {
			break
		}
		cksum.Write(buf[:n])
		for _, b := range buf[:n] {
			fwData = append(fwData, b^key[len(fwData)%len(key)])
		}
	}

	fmt.Printf("checksum raw: %x\n", cksum.Sum32())
	fmt.Printf("checksum deobfuscated: %x\n", crc32.ChecksumIEEE(fwData))
	fmt.Printf("checksum deobfuscated -4 bytes: %x\n", crc32.ChecksumIEEE(fwData[:len(fwData)-4]))

	outHex.WriteString(srec.S0("Generated from firmware_package.b by ebm-fwutil"))

	dataSum := crc32.NewIEEE()

	var ptr int
	for {
		if ptr+8 > len(fwData) {
			break
		}
		record := fwData[ptr : ptr+8]
		addr := binary.BigEndian.Uint32(record[:4])
		if addr == 0xffeeddcc {
			break
		}
		recordSize := int(record[7]) * 4
		if !bytes.Equal(record[4:7], []byte{0, 0, 0}) {
			fmt.Printf("data in reserved area: %x %v %v\n", addr, record[4:7], recordSize)
		}
		ptr += 8
		dataSum.Write(fwData[ptr : ptr+recordSize])
		outHex.WriteString(srec.S3(addr, fwData[ptr:ptr+recordSize]))
		ptr += recordSize
	}

	fmt.Printf("checksum data only: %x\n", dataSum.Sum32())

	outHex.Close()
	fmt.Println("done")
}
