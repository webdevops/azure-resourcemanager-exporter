package main

import (
	"errors"
	"fmt"
	"strconv"
)

// parse --portscan-range
func parseConfigPortScannerPortrange() (errorMessage error) {
	var err error
	var firstPort int64
	var lastPort int64

	if len(Config.Collectors.Portscan.Scanner.Ports) > 0 {
		portscanPortRange = []Portrange{}

		for _, portrange := range Config.Collectors.Portscan.Scanner.Ports {
			// parse via regexp
			portscanRangeSubMatch := portrangeRegexp.FindStringSubmatch(portrange)

			if len(portscanRangeSubMatch) == 0 {
				// portrange is invalid
				errorMessage = fmt.Errorf("unable to parse collectors.portscan.scanner.ports, has to be format \"nnn-mmm\"")
				return
			}

			// get named submatches
			portscanRangeSubMatchResult := make(map[string]string)
			for i, name := range portrangeRegexp.SubexpNames() {
				if i != 0 && name != "" {
					portscanRangeSubMatchResult[name] = portscanRangeSubMatch[i]
				}
			}

			// parse first port
			firstPort, err = strconv.ParseInt(portscanRangeSubMatchResult["first"], 10, 32)
			if err != nil {
				errorMessage = fmt.Errorf("failed to parse collectors.portscan.scanner.ports: %w", err)
				return
			}

			// parse last port (optional)
			if portscanRangeSubMatchResult["last"] != "" {
				lastPort, err = strconv.ParseInt(portscanRangeSubMatchResult["last"], 10, 32)
				if err != nil {
					errorMessage = fmt.Errorf("failed to parse collectors.portscan.scanner.ports: %w", err)
					return
				}
			} else {
				// single port only
				lastPort = firstPort
			}

			// check min port
			if firstPort < 1 {
				errorMessage = fmt.Errorf("failed to parse collectors.portscan.scanner.ports: first port cannot be smaller then 0 (%v -> %v)", firstPort, lastPort)
				return
			}

			// check max port
			if lastPort > 65535 {
				errorMessage = fmt.Errorf("failed to parse collectors.portscan.scanner.ports: last port cannot be bigger then 65535 (%v -> %v)", firstPort, lastPort)
				return
			}

			// check if range is ok
			if firstPort > lastPort {
				errorMessage = fmt.Errorf("failed to parse collectors.portscan.scanner.ports: first port cannot be beyond last port (%v -> %v)", firstPort, lastPort)
				return
			}

			// add to portlist
			portscanPortRange = append(
				portscanPortRange,
				Portrange{FirstPort: int(firstPort), LastPort: int(lastPort)},
			)
		}
	} else {
		errorMessage = errors.New("no port range available, set via collectors.portscan.scanner.ports")
		return
	}

	return
}
