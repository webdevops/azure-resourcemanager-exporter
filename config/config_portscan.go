package config

type (
	CollectorPortscan struct {
		*CollectorBase `yaml:",inline"`

		Scanner struct {
			Parallel int `json:"parallel"`
			Threads  int `json:"threads"`
			Timeout  int `json:"timeout"`

			Ports []string `json:"ports"`
		} `json:"scanner"`
	}
)
