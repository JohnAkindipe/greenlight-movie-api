package data

import (
	"fmt"
	"greenlight-movie-api/internal/validator"
)

type Filters struct {
	Page     int
	PageSize int
	Sort     string
	SortSafeList []string
}

// var allowedSortValues = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

func ValidateFilters(filterValidatorPtr *validator.Validator, filters Filters) {

	filterValidatorPtr.Check(
		filters.Page > 0 && filters.Page < 10_000_001,
		"page",
		"must be from (including) 1 upto (including) 10_000_000",
	)

	filterValidatorPtr.Check(
		filters.PageSize > 0 && filters.PageSize <101,
		"page_size",
		"must be from (including) 1 upto (including) 100",
	)

	filterValidatorPtr.Check(
		validator.PermittedValue(filters.Sort, filters.SortSafeList...),
		"sort",
		fmt.Sprintf("must be a member of the following array: %+v", filters.SortSafeList),
	)
}