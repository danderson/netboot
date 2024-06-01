package dhcp6

import (
	"encoding/binary"
	"hash/fnv"
	"net"
)

// PacketBuilder is used for generating responses to requests received from dhcp clients
type PacketBuilder struct {
	PreferredLifetime uint32
	ValidLifetime     uint32
}

// MakePacketBuilder creates a new PacketBuilder and initializes it with preferred and valid lifetimes
func MakePacketBuilder(preferredLifetime, validLifetime uint32) *PacketBuilder {
	return &PacketBuilder{PreferredLifetime: preferredLifetime, ValidLifetime: validLifetime}
}

// BuildResponse generates a response packet for a packet received from a client
func (b *PacketBuilder) BuildResponse(in *Packet, serverDUID []byte, configuration BootConfiguration, addresses AddressPool) (*Packet, error) {
	switch in.Type {
	case MsgSolicit:
		bootFileURL, err := configuration.GetBootURL(b.extractLLAddressOrID(in.Options.ClientID()), in.Options.ClientArchType())
		if err != nil {
			return nil, err
		}
		associations, err := addresses.ReserveAddresses(in.Options.ClientID(), in.Options.IaNaIDs())
		if err != nil {
			return b.makeMsgAdvertiseWithNoAddrsAvailable(in.TransactionID, serverDUID, in.Options.ClientID(), err), err
		}
		return b.makeMsgAdvertise(in.TransactionID, serverDUID, in.Options.ClientID(),
			in.Options.ClientArchType(), associations, bootFileURL, configuration.GetPreference(), configuration.GetRecursiveDNS()), nil
	case MsgRequest:
		bootFileURL, err := configuration.GetBootURL(b.extractLLAddressOrID(in.Options.ClientID()), in.Options.ClientArchType())
		if err != nil {
			return nil, err
		}
		associations, err := addresses.ReserveAddresses(in.Options.ClientID(), in.Options.IaNaIDs())
		return b.makeMsgReply(in.TransactionID, serverDUID, in.Options.ClientID(),
			in.Options.ClientArchType(), associations, iasWithoutAddesses(associations, in.Options.IaNaIDs()), bootFileURL,
			configuration.GetRecursiveDNS(), err), err
	case MsgInformationRequest:
		bootFileURL, err := configuration.GetBootURL(b.extractLLAddressOrID(in.Options.ClientID()), in.Options.ClientArchType())
		if err != nil {
			return nil, err
		}
		return b.makeMsgInformationRequestReply(in.TransactionID, serverDUID, in.Options.ClientID(),
			in.Options.ClientArchType(), bootFileURL, configuration.GetRecursiveDNS()), nil
	case MsgRelease:
		addresses.ReleaseAddresses(in.Options.ClientID(), in.Options.IaNaIDs())
		return b.makeMsgReleaseReply(in.TransactionID, serverDUID, in.Options.ClientID()), nil
	default:
		return nil, nil
	}
}

func (b *PacketBuilder) makeMsgAdvertise(transactionID [3]byte, serverDUID, clientID []byte, clientArchType uint16,
	associations []*IdentityAssociation, bootFileURL, preference []byte, dnsServers []net.IP) *Packet {
	retOptions := make(Options)
	retOptions.Add(MakeOption(OptClientID, clientID))
	for _, association := range associations {
		retOptions.Add(MakeIaNaOption(association.InterfaceID, b.calculateT1(), b.calculateT2(),
			MakeIaAddrOption(association.IPAddress, b.PreferredLifetime, b.ValidLifetime)))
	}
	retOptions.Add(MakeOption(OptServerID, serverDUID))
	if 0x10 == clientArchType { // HTTPClient
		retOptions.Add(MakeOption(OptVendorClass, []byte{0, 0, 0, 0, 0, 10, 72, 84, 84, 80, 67, 108, 105, 101, 110, 116})) // HTTPClient
	}
	retOptions.Add(MakeOption(OptBootfileURL, bootFileURL))
	if preference != nil {
		retOptions.Add(MakeOption(OptPreference, preference))
	}
	if len(dnsServers) > 0 {
		retOptions.Add(MakeDNSServersOption(dnsServers))
	}

	return &Packet{Type: MsgAdvertise, TransactionID: transactionID, Options: retOptions}
}

func (b *PacketBuilder) makeMsgReply(transactionID [3]byte, serverDUID, clientID []byte, clientArchType uint16,
	associations []*IdentityAssociation, iasWithoutAddresses [][]byte, bootFileURL []byte, dnsServers []net.IP, err error) *Packet {
	retOptions := make(Options)
	retOptions.Add(MakeOption(OptClientID, clientID))
	for _, association := range associations {
		retOptions.Add(MakeIaNaOption(association.InterfaceID, b.calculateT1(), b.calculateT2(),
			MakeIaAddrOption(association.IPAddress, b.PreferredLifetime, b.ValidLifetime)))
	}
	for _, ia := range iasWithoutAddresses {
		retOptions.Add(MakeIaNaOption(ia, b.calculateT1(), b.calculateT2(),
			MakeStatusOption(2, err.Error())))
	}
	retOptions.Add(MakeOption(OptServerID, serverDUID))
	if 0x10 == clientArchType { // HTTPClient
		retOptions.Add(MakeOption(OptVendorClass, []byte{0, 0, 0, 0, 0, 10, 72, 84, 84, 80, 67, 108, 105, 101, 110, 116})) // HTTPClient
	}
	retOptions.Add(MakeOption(OptBootfileURL, bootFileURL))
	if len(dnsServers) > 0 {
		retOptions.Add(MakeDNSServersOption(dnsServers))
	}

	return &Packet{Type: MsgReply, TransactionID: transactionID, Options: retOptions}
}

func (b *PacketBuilder) makeMsgInformationRequestReply(transactionID [3]byte, serverDUID, clientID []byte, clientArchType uint16,
	bootFileURL []byte, dnsServers []net.IP) *Packet {
	retOptions := make(Options)
	retOptions.Add(MakeOption(OptClientID, clientID))
	retOptions.Add(MakeOption(OptServerID, serverDUID))
	if 0x10 == clientArchType { // HTTPClient
		retOptions.Add(MakeOption(OptVendorClass, []byte{0, 0, 0, 0, 0, 10, 72, 84, 84, 80, 67, 108, 105, 101, 110, 116})) // HTTPClient
	}
	retOptions.Add(MakeOption(OptBootfileURL, bootFileURL))
	if len(dnsServers) > 0 {
		retOptions.Add(MakeDNSServersOption(dnsServers))
	}

	return &Packet{Type: MsgReply, TransactionID: transactionID, Options: retOptions}
}

func (b *PacketBuilder) makeMsgReleaseReply(transactionID [3]byte, serverDUID, clientID []byte) *Packet {
	retOptions := make(Options)

	retOptions.Add(MakeOption(OptClientID, clientID))
	retOptions.Add(MakeOption(OptServerID, serverDUID))
	v := make([]byte, 19)
	copy(v[2:], []byte("Release received."))
	retOptions.Add(MakeOption(OptStatusCode, v))

	return &Packet{Type: MsgReply, TransactionID: transactionID, Options: retOptions}
}

func (b *PacketBuilder) makeMsgAdvertiseWithNoAddrsAvailable(transactionID [3]byte, serverDUID, clientID []byte, err error) *Packet {
	retOptions := make(Options)
	retOptions.Add(MakeOption(OptClientID, clientID))
	retOptions.Add(MakeOption(OptServerID, serverDUID))
	retOptions.Add(MakeStatusOption(2, err.Error())) // NoAddrAvailable
	return &Packet{Type: MsgAdvertise, TransactionID: transactionID, Options: retOptions}
}

func (b *PacketBuilder) calculateT1() uint32 {
	return b.PreferredLifetime / 2
}

func (b *PacketBuilder) calculateT2() uint32 {
	return (b.PreferredLifetime * 4) / 5
}

func (b *PacketBuilder) extractLLAddressOrID(optClientID []byte) []byte {
	idType := binary.BigEndian.Uint16(optClientID[0:2])
	switch idType {
	case 1:
		return optClientID[8:]
	case 3:
		return optClientID[4:]
	default:
		return optClientID[2:]
	}
}

func iasWithoutAddesses(availableAssociations []*IdentityAssociation, allIAs [][]byte) [][]byte {
	ret := make([][]byte, 0)
	iasWithAddresses := make(map[uint64]bool)

	for _, association := range availableAssociations {
		iasWithAddresses[calculateIAIDHash(association.InterfaceID)] = true
	}

	for _, ia := range allIAs {
		_, exists := iasWithAddresses[calculateIAIDHash(ia)]
		if !exists {
			ret = append(ret, ia)
		}
	}
	return ret
}

func calculateIAIDHash(interfaceID []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(interfaceID)
	return h.Sum64()
}
