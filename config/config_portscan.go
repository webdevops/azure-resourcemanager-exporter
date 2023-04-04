package config

type (
	CollectorPortscan struct {
		CollectorBase `yaml:",inline"`

		Scanner struct {
			Parallel int `yaml:"parallel"`
			Threads  int `yaml:"threads"`
			Timeout  int `yaml:"timeout"`

			Ports []string `yaml:"ports"`
		} `yaml:"scanner"`
	}
)
