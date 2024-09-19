package metastore

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	metastorev1 "github.com/grafana/pyroscope/api/gen/proto/go/metastore/v1"
	"github.com/grafana/pyroscope/pkg/model"
)

func (m *Metastore) QueryMetadata(
	ctx context.Context,
	request *metastorev1.QueryMetadataRequest,
) (*metastorev1.QueryMetadataResponse, error) {
	// TODO(kolesnikovae): ReadIndex
	return m.state.listBlocksForQuery(ctx, request)
}

type metadataQuery struct {
	startTime      int64
	endTime        int64
	tenants        map[string]struct{}
	serviceMatcher *labels.Matcher
}

func newMetadataQuery(request *metastorev1.QueryMetadataRequest) (*metadataQuery, error) {
	if len(request.TenantId) == 0 {
		return nil, fmt.Errorf("tenant_id is required")
	}
	q := &metadataQuery{
		startTime: request.StartTime,
		endTime:   request.EndTime,
		tenants:   make(map[string]struct{}, len(request.TenantId)),
	}
	for _, tenant := range request.TenantId {
		q.tenants[tenant] = struct{}{}
	}
	selectors, err := parser.ParseMetricSelector(request.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse label selectors: %w", err)
	}
	for _, m := range selectors {
		if m.Name == model.LabelNameServiceName {
			q.serviceMatcher = m
			break
		}
	}
	// We could also validate that the service has the profile type
	// queried, but that's not really necessary: querying an irrelevant
	// profile type is rather a rare/invalid case.
	return q, nil
}

func (q *metadataQuery) matchService(s *metastorev1.Dataset) bool {
	_, ok := q.tenants[s.TenantId]
	if !ok {
		return false
	}
	if !inRange(s.MinTime, s.MaxTime, q.startTime, q.endTime) {
		return false
	}
	if q.serviceMatcher != nil {
		return q.serviceMatcher.Matches(s.Name)
	}
	return true
}

func inRange(blockStart, blockEnd, queryStart, queryEnd int64) bool {
	return blockStart <= queryEnd && blockEnd >= queryStart
}

func (i *index) listBlocksForQuery(q *metadataQuery) []*metastorev1.BlockMeta {
	md := make(map[string]*metastorev1.BlockMeta, 32)
	i.run(func() {
		level.Info(i.logger).Log("msg", "querying metastore", "query", q)
		blocks, err := i.findBlocksInRange(q.startTime, q.endTime, q.tenants)
		if err != nil {
			level.Error(i.logger).Log("msg", "failed to list metastore blocks", "err", err)
			return
		}
		level.Debug(i.logger).Log("msg", "found blocks for query", "block_count", len(blocks), "query", q)
		for _, block := range blocks {
			var clone *metastorev1.BlockMeta
			for _, svc := range block.Datasets {
				if q.matchService(svc) {
					if clone == nil {
						clone = cloneBlockForQuery(block)
						md[clone.Id] = clone
					}
					clone.Datasets = append(clone.Datasets, svc)
				}
			}
		}
	})

	blocks := make([]*metastorev1.BlockMeta, 0, len(md))
	for _, block := range md {
		blocks = append(blocks, block)
	}

	return blocks
}

func cloneBlockForQuery(b *metastorev1.BlockMeta) *metastorev1.BlockMeta {
	datasets := b.Datasets
	b.Datasets = nil
	c := b.CloneVT()
	b.Datasets = datasets
	c.Datasets = make([]*metastorev1.Dataset, 0, len(b.Datasets))
	return c
}

func (m *metastoreState) listBlocksForQuery(
	ctx context.Context,
	request *metastorev1.QueryMetadataRequest,
) (*metastorev1.QueryMetadataResponse, error) {
	q, err := newMetadataQuery(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	var resp metastorev1.QueryMetadataResponse
	blocks := m.index.listBlocksForQuery(q)
	resp.Blocks = append(resp.Blocks, blocks...)
	slices.SortFunc(resp.Blocks, func(a, b *metastorev1.BlockMeta) int {
		return strings.Compare(a.Id, b.Id)
	})
	return &resp, nil
}
