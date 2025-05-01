package config

type (
	CollectorBudgets struct {
		*CollectorBase

		Scopes []string `json:"scopes"`
	}
)
