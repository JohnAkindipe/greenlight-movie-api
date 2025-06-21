package data

import (
	"fmt"
	"greenlight-movie-api/internal/validator"
	"math"
)

type Filters struct {
	Page     int
	PageSize int
	Sort     string
	SortSafeList []string
}

type PageMetadata struct {
    CurrentPage  int    `json:"current_page,omitempty"`
    PageSize     int    `json:"page_size,omitempty"`
    FirstPage    int    `json:"first_page,omitempty"`
    LastPage     int    `json:"last_page,omitempty"`
    TotalRecords int    `json:"total_records,omitempty"`
}

func CalculatePageMetadata(totalRecords, pageSize, currentPage int) PageMetadata {
	if totalRecords == 0 {
		return PageMetadata{}
	}

	return PageMetadata{
		CurrentPage: currentPage,
		PageSize: pageSize,
		FirstPage: 1,
		LastPage: int(math.Ceil(float64(totalRecords)/float64(pageSize))),
		TotalRecords: totalRecords,
	}
}

// var allowedSortValues = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

func ValidateFilters(filterValidatorPtr *validator.Validator, filters Filters) {

	//page no. can't be < 0 or >10_000_000, default: 1
	filterValidatorPtr.Check(
		filters.Page > 0 && filters.Page < 10_000_001,
		"page",
		"must be from (including) 1 upto (including) 10_000_000",
	)

	//page size (no. of items on a page) can't be >100 or <0, default: 20
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

func (filter Filters) offset() int {
	//e.g. page = 2, page-size = 10
	//offset = (2 - 1) * 10
	return (filter.Page - 1) * filter.PageSize
}

func (filter Filters) limit() int {
	//e.g. page-size = 10
	//offset = 10
	return filter.PageSize
}