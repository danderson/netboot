package dhcp6

import (
	"bytes"
	"fmt"
)

// MessageType contains ID identifying DHCP message type. See RFC 3315
type MessageType uint8

// Constants for each of the dhcp message types defined in RFC 3315
const (
	MsgSolicit MessageType = iota + 1
	MsgAdvertise
	MsgRequest
	MsgConfirm
	MsgRenew
	MsgRebind
	MsgReply
	MsgRelease
	MsgDecline
	MsgReconfigure
	MsgInformationRequest
	MsgRelayForw
	MsgRelayRepl
)

// Packet represents a DHCPv6 packet
type Packet struct {
	Type          MessageType
	TransactionID [3]byte
	Options       Options
}

// Unmarshal creates a Packet out of its serialized representation
func Unmarshal(bs []byte, packetLength int) (*Packet, error) {
	options, err := UnmarshalOptions(bs[4:packetLength])
	if err != nil {
		return nil, fmt.Errorf("packet has malformed options section: %s", err)
	}
	ret := &Packet{Type: MessageType(bs[0]), Options: options}
	copy(ret.TransactionID[:], bs[1:4])
	return ret, nil
}

// Marshal serializes the Packet
func (p *Packet) Marshal() ([]byte, error) {
	marshalledOptions, err := p.Options.Marshal()
	if err != nil {
		return nil, fmt.Errorf("packet has malformed options section: %s", err)
	}

	ret := make([]byte, len(marshalledOptions)+4)
	ret[0] = byte(p.Type)
	copy(ret[1:], p.TransactionID[:])
	copy(ret[4:], marshalledOptions)

	return ret, nil
}

// ShouldDiscard returns true if the Packet fails validation
func (p *Packet) ShouldDiscard(serverDuid []byte) error {
	switch p.Type {
	case MsgSolicit:
		return shouldDiscardSolicit(p)
	case MsgRequest:
		return shouldDiscardRequest(p, serverDuid)
	case MsgInformationRequest:
		return shouldDiscardInformationRequest(p, serverDuid)
	case MsgRelease:
		return nil // FIX ME!
	default:
		return fmt.Errorf("Unknown packet")
	}
}

func shouldDiscardSolicit(p *Packet) error {
	options := p.Options
	if !options.HasBootFileURLOption() {
		return fmt.Errorf("'Solicit' packet doesn't have file url option")
	}
	if !options.HasClientID() {
		return fmt.Errorf("'Solicit' packet has no Client id option")
	}
	if options.HasServerID() {
		return fmt.Errorf("'Solicit' packet has server id option")
	}
	return nil
}

func shouldDiscardRequest(p *Packet, serverDuid []byte) error {
	options := p.Options
	if !options.HasBootFileURLOption() {
		return fmt.Errorf("'Request' packet doesn't have file url option")
	}
	if !options.HasClientID() {
		return fmt.Errorf("'Request' packet has no Client id option")
	}
	if !options.HasServerID() {
		return fmt.Errorf("'Request' packet has no server id option")
	}
	if !bytes.Equal(options.ServerID(), serverDuid) {
		return fmt.Errorf("'Request' packet's server id option (%d) is different from ours (%d)", options.ServerID(), serverDuid)
	}
	return nil
}

func shouldDiscardInformationRequest(p *Packet, serverDuid []byte) error {
	options := p.Options
	if !options.HasBootFileURLOption() {
		return fmt.Errorf("'Information-request' packet doesn't have boot file url option")
	}
	if options.HasIaNa() || options.HasIaTa() {
		return fmt.Errorf("'Information-request' packet has an IA option present")
	}
	if options.HasServerID() && (!bytes.Equal(options.ServerID(), serverDuid)) {
		return fmt.Errorf("'Information-request' packet's server id option (%d) is different from ours (%d)", options.ServerID(), serverDuid)
	}
	return nil
}
