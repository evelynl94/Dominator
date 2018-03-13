package manager

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path"

	"github.com/Symantec/Dominator/lib/json"
	proto "github.com/Symantec/Dominator/proto/hypervisor"
)

func (m *Manager) addAddressesToPool(addresses []proto.Address,
	lock bool) error {
	for index := range addresses {
		addresses[index].Shrink()
	}
	if lock {
		m.mutex.Lock()
		defer m.mutex.Unlock()
	}
	for _, address := range addresses {
		foundSubnet := false
		for _, subnet := range m.subnets {
			subnetMask := net.IPMask(subnet.IpMask)
			subnetAddr := subnet.IpGateway.Mask(subnetMask)
			if address.IpAddress.Mask(subnetMask).Equal(subnetAddr) {
				foundSubnet = true
				break
			}
		}
		if !foundSubnet {
			return fmt.Errorf("no subnet matching %s", address.IpAddress)
		}
	}
	m.addressPool = append(m.addressPool, addresses...)
	return json.WriteToFile(path.Join(m.StateDir, "address-pool.json"),
		filePerms, "    ", m.addressPool)
}

func (m *Manager) loadAddressPool() error {
	var addressPool []proto.Address
	err := json.ReadFromFile(path.Join(m.StateDir, "address-pool.json"),
		&addressPool)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for index := range addressPool {
		addressPool[index].Shrink()
	}
	m.addressPool = addressPool
	return nil
}

func (m *Manager) getFreeAddress(subnetId string) (proto.Address, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if len(m.addressPool) < 1 {
		return proto.Address{}, errors.New("no free addresses in pool")
	}
	if subnetId == "" {
		err := json.WriteToFile(path.Join(m.StateDir, "address-pool.json"),
			filePerms, "    ", m.addressPool[1:])
		if err != nil {
			return proto.Address{}, err
		}
		address := m.addressPool[0]
		copy(m.addressPool, m.addressPool[1:])
		m.addressPool = m.addressPool[:len(m.addressPool)-1]
		return address, nil
	}
	subnet, ok := m.subnets[subnetId]
	if !ok {
		return proto.Address{}, fmt.Errorf("no such subnet: %s", subnetId)
	}
	subnetMask := net.IPMask(subnet.IpMask)
	subnetAddr := subnet.IpGateway.Mask(subnetMask)
	foundPos := -1
	for index, address := range m.addressPool {
		if address.IpAddress.Mask(subnetMask).Equal(subnetAddr) {
			foundPos = index
			break
		}
	}
	if foundPos < 0 {
		return proto.Address{},
			fmt.Errorf("no free address in subnet: %s", subnetId)
	}
	addressPool := make([]proto.Address, 0, len(m.addressPool)-1)
	for index, address := range m.addressPool {
		if index == foundPos {
			break
		}
		addressPool = append(addressPool, address)
	}
	err := json.WriteToFile(path.Join(m.StateDir, "address-pool.json"),
		filePerms, "    ", addressPool)
	if err != nil {
		return proto.Address{}, err
	}
	address := m.addressPool[foundPos]
	m.addressPool = addressPool
	return address, nil
}
