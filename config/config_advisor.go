package config

type (
	CollectorAdvisor struct {
		*CollectorBase `yaml:",inline"`

		ProblemMaxLength  int `json:"problemMaxLength"`
		SolutionMaxLength int `json:"solutionMaxLength"`
	}
)
