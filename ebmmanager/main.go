package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"git.dolansoft.org/lorenz/metanoia-ebm/bootloader"
	"git.dolansoft.org/lorenz/metanoia-ebm/ebm"
	"github.com/mdlayher/packet"
)

var (
	iface  = flag.String("if", "", "Network interface the modem is connected to")
	fwPath = flag.String("fw", "", "Path to the firmware file in Motorola S-REC format")
)

func main() {
	flag.Parse()
	if *iface == "" {
		log.Fatalf("if argument needs to be set")
	}
	if *fwPath == "" {
		log.Fatalf("fw argument needs to be set")
	}
	metanoiaIf, err := net.InterfaceByName(*iface)
	if err != nil {
		log.Fatalln(err)
	}

	pktConn, err := packet.Listen(metanoiaIf, packet.Datagram, 0x6120, &packet.Config{})
	if err != nil {
		log.Fatalf("failed to create socket: %v", err)
	}

	deviceId := make([]byte, 3)
	if _, err := rand.Read(deviceId); err != nil {
		log.Fatalf("failed to get randomness: %v", err)
	}

	assignedAddr := net.HardwareAddr{0xde, 0x21, 0x65, deviceId[0], deviceId[1], deviceId[2]}

	fw, err := os.Open(*fwPath)
	if err != nil {
		log.Fatalf("failed to open firmware file: %v", err)
	}

	err = bootloader.DownloadAndBoot(pktConn, assignedAddr, fw)
	if err != nil {
		log.Fatalf("failed to boot: %v", err)
	}

	c := ebm.NewConn(pktConn, assignedAddr)
	c.Logger = os.Stderr

	if err := c.Dial(); err != nil {
		log.Fatalf("failed to connect: %v", err)
	}

	// Enable log and console output
	if err := c.WriteMIB(&ebm.OidLogControl, uint32(0xfe)); err != nil {
		log.Fatalf("failed to write log control: %v", err)
	}
	if err := c.WriteMIB(&ebm.OidConsoleControl, uint32(2)); err != nil {
		log.Fatalf("failed to write console control: %v", err)
	}
	// Enable Modem
	if err := c.WriteMIB(&ebm.OidHostCommand, uint8(1)); err != nil {
		log.Fatalf("failed to write host command: %v", err)
	}
	if err := c.WriteMIB(&ebm.OidRepeatCommand, uint8(1)); err != nil {
		log.Fatalf("failed to write repeat command: %v", err)
	}
	if err := c.WriteMIB(&ebm.OidCmdStatus, true); err != nil {
		log.Fatalf("failed to write command status: %v", err)
	}

	for {
		time.Sleep(5 * time.Second)
		ticks, err := c.ReadMIB(&ebm.OidTicks)
		if err != nil {
			log.Fatalf("failed to read ticks: %v", err)
		}
		fmt.Printf("Ticks: %d\n", ticks.(uint32))
	}

}
