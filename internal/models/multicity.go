package models

type MultiCityRequest struct {
	Legs    []SearchCriteria
	Filters SearchFilters
	Sort    SortOption
}

type MultiCityResult struct {
	Legs     []LegResult
	Metadata SearchMetadata
}

type LegResult struct {
	Criteria SearchCriteria
	Flights  []Flight
}
