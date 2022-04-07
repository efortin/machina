package internal

import (
	"bufio"
	"fmt"
	"github.com/efortin/machina/utils"
	"io"
	"os"
	"regexp"
	"strings"
)

const (
	// LeasesPath is the path to dhcpd leases
	LeasesPath = "/var/db/dhcpd_leases"
)

var (
	leadingZeroRegexp = regexp.MustCompile(`0([A-Fa-f0-9](:|$))`)
)

// DHCPEntry holds a parsed DNS entry
type DHCPEntry struct {
	Name      string
	IPAddress string
	HWAddress string
	ID        string
	Lease     string
}

// GetIPAddressByMACAddress gets the IP address of a MAC address
func GetIPAddressByMACAddress(mac string) (string, error) {
	mac = trimMACAddress(mac)
	return getIPAddressFromFile(mac, LeasesPath)
}

func getIPAddressFromFile(mac, path string) (string, error) {
	utils.Logger.Debugf("Searching for %s in %s ...", mac, path)
	file, err := os.Open(path)
	if err != nil {
		return utils.Empty, err
	}
	defer file.Close()

	dhcpEntries, err := parseDHCPdLeasesFile(file)
	if err != nil {
		return utils.Empty, err
	}
	utils.Logger.Debugf("Found %d entries in %s!", len(dhcpEntries), path)
	for _, dhcpEntry := range dhcpEntries {
		utils.Logger.Tracef("dhcp entry: %+v", dhcpEntry)
		if dhcpEntry.HWAddress == mac {
			utils.Logger.Tracef("Found match: %s", mac)
			return dhcpEntry.IPAddress, nil
		}
	}
	return utils.Empty, fmt.Errorf("could not find an IP address for %s", mac)
}

func parseDHCPdLeasesFile(file io.Reader) ([]DHCPEntry, error) {
	var (
		dhcpEntry   *DHCPEntry
		dhcpEntries []DHCPEntry
	)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "{" {
			dhcpEntry = new(DHCPEntry)
			continue
		} else if line == "}" {
			dhcpEntries = append(dhcpEntries, *dhcpEntry)
			continue
		}

		split := strings.SplitN(line, "=", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("invalid line in dhcp leases file: %s", line)
		}
		key, val := split[0], split[1]
		switch key {
		case "name":
			dhcpEntry.Name = val
		case "ip_address":
			dhcpEntry.IPAddress = val
		case "hw_address":
			// The mac addresses have a '1,' at the start.
			dhcpEntry.HWAddress = val[2:]
		case "identifier":
			dhcpEntry.ID = val
		case "lease":
			dhcpEntry.Lease = val
		default:
			return dhcpEntries, fmt.Errorf("unable to parse line: %s", line)
		}
	}
	return dhcpEntries, scanner.Err()
}

// trimMacAddress trimming "0" of the ten's digit
func trimMACAddress(macAddress string) string {
	return leadingZeroRegexp.ReplaceAllString(macAddress, "$1")
}
