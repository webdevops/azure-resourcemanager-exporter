package quota

type (
	ListUsageResult struct {
		// REQUIRED; The list of compute resource usages.
		Value []*Usage

		// The URI to fetch the next page of compute resource usage information. Call ListNext() with this to fetch the next page
		// of compute resource usage information.
		NextLink *string
	}

	Usage struct {
		// REQUIRED; The current usage of the resource.
		CurrentValue *float64

		// REQUIRED; The maximum permitted usage of the resource.
		Limit *float64

		// REQUIRED; The name of the type of usage.
		Name *UsageName

		// REQUIRED; An enum describing the unit of usage measurement.
		Unit *string
	}

	// UsageName - The Usage Names.
	UsageName struct {
		// The localized name of the resource.
		LocalizedValue *string

		// The name of the resource.
		Value *string
	}
)
