package main

import (
	"errors"
	"fmt"
	"strconv"
)

// parse --portscan-range
func argparserParsePortrange() (errorMessage error) {
	var err error
	var firstPort int64
	var lastPort int64

	if len(opts.PortscanPortRange) > 0 {
		opts.portscanPortRange = []Portrange{}

		for _, portrange := range opts.PortscanPortRange {
			// parse via regexp
			portscanRangeSubMatch := portrangeRegexp.FindStringSubmatch(portrange)

			if len(portscanRangeSubMatch) == 0 {
				// portrange is invalid
				errorMessage = errors.New(fmt.Sprintf("Unable to parse \"--portscan-range\", has to be format \"nnn-mmm\""))
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
				errorMessage = errors.New(fmt.Sprintf("Failed to parse \"--portscan-range\": %v", err))
				return
			}

			// parse last port (optional)
			if portscanRangeSubMatchResult["last"] != "" {
				lastPort, err = strconv.ParseInt(portscanRangeSubMatchResult["last"], 10, 32)
				if err != nil {
					errorMessage = errors.New(fmt.Sprintf("Failed to parse \"--portscan-range\": %v", err))
					return
				}
			} else {
				// single port only
				lastPort = firstPort
			}

			// check min port
			if firstPort < 1 {
				errorMessage = errors.New(fmt.Sprintf("Failed to parse \"--portscan-range\": first port cannot be smaller then 0 (%v -> %v)", firstPort, lastPort))
				return
			}

			// check max port
			if lastPort > 65535 {
				errorMessage = errors.New(fmt.Sprintf("Failed to parse \"--portscan-range\": last port cannot be bigger then 65535 (%v -> %v)", firstPort, lastPort))
				return
			}

			// check if range is ok
			if firstPort > lastPort {
				errorMessage = errors.New(fmt.Sprintf("Failed to parse \"--portscan-range\": first port cannot be beyond last port (%v -> %v)", firstPort, lastPort))
				return
			}

			// add to portlist
			opts.portscanPortRange = append(
				opts.portscanPortRange,
				Portrange{FirstPort: int(firstPort), LastPort: int(lastPort)},
			)
		}
	} else {
		errorMessage = errors.New("No port range available, set via \"--portscan-range\"")
		return
	}

	return
}
