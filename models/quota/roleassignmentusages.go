package quota

type (
	RoleAssignmentUsage struct {
		RoleAssignmentsLimit          float64 `json:"roleAssignmentsLimit"`
		RoleAssignmentsCurrentCount   float64 `json:"roleAssignmentsCurrentCount"`
		RoleAssignmentsRemainingCount float64 `json:"roleAssignmentsRemainingCount"`
	}
)
