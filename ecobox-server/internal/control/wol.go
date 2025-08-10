package control

import (
	"fmt"
	"net"

)

// WoLSender handles Wake-on-LAN magic packet sending
type WoLSender struct{}

// NewWoLSender creates a new WoL sender instance
func NewWoLSender() *WoLSender {
	return &WoLSender{}
}

// SendMagicPacket sends a Wake-on-LAN magic packet to the specified MAC address
func (w *WoLSender) SendMagicPacket(macAddress string, broadcastAddr string) error {
	// Parse MAC address
	mac, err := net.ParseMAC(macAddress)
	if err != nil {
		return fmt.Errorf("invalid MAC address '%s': %w", macAddress, err)
	}

	// If no broadcast address specified, use default
	if broadcastAddr == "" {
		broadcastAddr = "255.255.255.255:9"
	}

	// Create magic packet
	magicPacket := w.createMagicPacket(mac)

	// Send packet via UDP
	conn, err := net.Dial("udp", broadcastAddr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection to %s: %w", broadcastAddr, err)
	}
	defer conn.Close()

	_, err = conn.Write(magicPacket)
	if err != nil {
		return fmt.Errorf("failed to send WoL packet: %w", err)
	}

	return nil
}

// createMagicPacket creates a Wake-on-LAN magic packet
func (w *WoLSender) createMagicPacket(mac net.HardwareAddr) []byte {
	// Magic packet format:
	// 6 bytes of 0xFF followed by 16 repetitions of the target MAC address
	packet := make([]byte, 102) // 6 + 16*6 = 102 bytes

	// Fill first 6 bytes with 0xFF
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}

	// Repeat MAC address 16 times
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:6+(i+1)*6], mac)
	}

	return packet
}

// SendMagicPacketMultiple sends magic packet to multiple broadcast addresses
func (w *WoLSender) SendMagicPacketMultiple(macAddress string, broadcastAddrs []string) error {
	if len(broadcastAddrs) == 0 {
		broadcastAddrs = []string{"255.255.255.255:9", "255.255.255.255:7"}
	}

	var lastErr error
	sent := false

	for _, addr := range broadcastAddrs {
		if err := w.SendMagicPacket(macAddress, addr); err != nil {
			lastErr = err
			continue
		}
		sent = true
	}

	if !sent && lastErr != nil {
		return fmt.Errorf("failed to send WoL packet to any address: %w", lastErr)
	}

	return nil
}
