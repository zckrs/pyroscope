package util

import (
	"slices"
	"sort"

	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
)

type labelNameCardinalitySort typesv1.LabelNamesResponse

func (s *labelNameCardinalitySort) Len() int {
	return len(s.Names)
}

func (s *labelNameCardinalitySort) Less(i, j int) bool {
	return s.Names[i] < s.Names[j]
}

func (s *labelNameCardinalitySort) Swap(i, j int) {
	s.Names[i], s.Names[j] = s.Names[j], s.Names[i]
	s.Cardinality[i], s.Cardinality[j] = s.Cardinality[j], s.Cardinality[i]
}

func SortLabelNamesResponse(r *typesv1.LabelNamesResponse) {
	if len(r.Cardinality) == 0 {
		slices.Sort(r.Names)
		return
	}

	sort.Sort((*labelNameCardinalitySort)(r))
}