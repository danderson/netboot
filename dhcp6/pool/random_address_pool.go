package pool

import (
	"fmt"
	"hash/fnv"
	"math/big"
	"math/rand"
	"net"
	"sync"
	"time"

	"go.universe.tf/netboot/dhcp6"
)

type associationExpiration struct {
	expiresAt time.Time
	ia        *dhcp6.IdentityAssociation
}

type fifo struct{ q []interface{} }

func newFifo() fifo {
	return fifo{q: make([]interface{}, 0, 1000)}
}

func (f *fifo) Push(v interface{}) {
	f.q = append(f.q, v)
}

func (f *fifo) Shift() interface{} {
	var ret interface{}
	ret, f.q = f.q[0], f.q[1:]
	return ret
}

func (f *fifo) Size() int {
	return len(f.q)
}

func (f *fifo) Peek() interface{} {
	if len(f.q) == 0 {
		return nil
	}
	return f.q[0]
}

// RandomAddressPool that returns a random IP address from a pool of available addresses
type RandomAddressPool struct {
	poolStartAddress               *big.Int
	poolSize                       uint64
	identityAssociations           map[uint64]*dhcp6.IdentityAssociation
	usedIps                        map[uint64]struct{}
	identityAssociationExpirations fifo
	validLifetime                  uint32 // in seconds
	timeNow                        func() time.Time
	lock                           sync.Mutex
}

// NewRandomAddressPool creates a new RandomAddressPool using pool start IP address, pool size, and valid lifetime of
// interface associations
func NewRandomAddressPool(poolStartAddress net.IP, poolSize uint64, validLifetime uint32) *RandomAddressPool {
	ret := &RandomAddressPool{}
	ret.validLifetime = validLifetime
	ret.poolStartAddress = big.NewInt(0)
	ret.poolStartAddress.SetBytes(poolStartAddress)
	ret.poolSize = poolSize
	ret.identityAssociations = make(map[uint64]*dhcp6.IdentityAssociation)
	ret.usedIps = make(map[uint64]struct{})
	ret.identityAssociationExpirations = newFifo()
	ret.timeNow = func() time.Time { return time.Now() }
	return ret
}

// ReserveAddresses creates new or retrieves active associations for interfaces in interfaceIDs list.
func (p *RandomAddressPool) ReserveAddresses(clientID []byte, interfaceIDs [][]byte) ([]*dhcp6.IdentityAssociation, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.expireIdentityAssociations()

	ret := make([]*dhcp6.IdentityAssociation, 0, len(interfaceIDs))
	rng := rand.New(rand.NewSource(p.timeNow().UnixNano()))

	for _, interfaceID := range interfaceIDs {
		clientIDHash := p.calculateIAIDHash(clientID, interfaceID)
		association, exists := p.identityAssociations[clientIDHash]

		if exists {
			ret = append(ret, association)
			continue
		}
		if uint64(len(p.usedIps)) == p.poolSize {
			return ret, fmt.Errorf("No more free ip addresses are currently available in the pool")
		}

		for {
			// we assume that ip addresses adhere to high 64 bits for net and subnet ids, low 64 bits are for host id rule
			hostOffset := rng.Uint64() % p.poolSize
			newIP := big.NewInt(0).Add(p.poolStartAddress, big.NewInt(0).SetUint64(hostOffset))
			_, exists := p.usedIps[newIP.Uint64()]
			if !exists {
				timeNow := p.timeNow()
				association := &dhcp6.IdentityAssociation{ClientID: clientID,
					InterfaceID: interfaceID,
					IPAddress:   newIP.Bytes(),
					CreatedAt:   timeNow}
				p.identityAssociations[clientIDHash] = association
				p.usedIps[newIP.Uint64()] = struct{}{}
				p.identityAssociationExpirations.Push(&associationExpiration{expiresAt: p.calculateAssociationExpiration(timeNow), ia: association})
				ret = append(ret, association)
				break
			}
		}
	}

	return ret, nil
}

// ReleaseAddresses returns IP addresses associated with ClientID and interfaceIDs back into the address pool
func (p *RandomAddressPool) ReleaseAddresses(clientID []byte, interfaceIDs [][]byte) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, interfaceID := range interfaceIDs {
		association, exists := p.identityAssociations[p.calculateIAIDHash(clientID, interfaceID)]
		if !exists {
			continue
		}
		delete(p.usedIps, big.NewInt(0).SetBytes(association.IPAddress).Uint64())
		delete(p.identityAssociations, p.calculateIAIDHash(clientID, interfaceID))
	}
}

// expireIdentityAssociations releases IP addresses in identity associations that reached the end of valid lifetime
// back into the address pool. Note it should be called from under the RandomAddressPool.lock.
func (p *RandomAddressPool) expireIdentityAssociations() {
	for {
		if p.identityAssociationExpirations.Size() < 1 {
			break
		}
		expiration := p.identityAssociationExpirations.Peek().(*associationExpiration)
		if p.timeNow().Before(expiration.expiresAt) {
			break
		}
		p.identityAssociationExpirations.Shift()
		delete(p.identityAssociations, p.calculateIAIDHash(expiration.ia.ClientID, expiration.ia.InterfaceID))
		delete(p.usedIps, big.NewInt(0).SetBytes(expiration.ia.IPAddress).Uint64())
	}
}

func (p *RandomAddressPool) calculateAssociationExpiration(now time.Time) time.Time {
	return now.Add(time.Duration(p.validLifetime) * time.Second)
}

func (p *RandomAddressPool) calculateIAIDHash(clientID, interfaceID []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(clientID)
	_, _ = h.Write(interfaceID)
	return h.Sum64()
}
