package metastore

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/log/level"

	"github.com/hashicorp/raft"
	"go.etcd.io/bbolt"

	metastorev1 "github.com/grafana/pyroscope/api/gen/proto/go/metastore/v1"
)

func (m *Metastore) AddBlock(_ context.Context, req *metastorev1.AddBlockRequest) (*metastorev1.AddBlockResponse, error) {
	_ = level.Info(m.logger).Log(
		"msg", "adding block",
		"block_id", req.Block.Id,
		"shard", req.Block.Shard,
		"raft_commit_index", m.raft.CommitIndex(),
		"raft_last_index", m.raft.LastIndex(),
		"raft_applied_index", m.raft.AppliedIndex())
	t1 := time.Now()
	defer func() {
		m.metrics.raftAddBlockDuration.Observe(time.Since(t1).Seconds())
		level.Debug(m.logger).Log("msg", "add block duration", "block_id", req.Block.Id, "shard", req.Block.Shard, "duration", time.Since(t1))
	}()
	_, resp, err := applyCommand[*metastorev1.AddBlockRequest, *metastorev1.AddBlockResponse](m.raft, req, m.config.Raft.ApplyTimeout)
	if err != nil {
		_ = level.Error(m.logger).Log("msg", "failed to apply add block", "block_id", req.Block.Id, "shard", req.Block.Shard, "err", err)
	}
	return resp, err
}

func (m *metastoreState) applyAddBlock(log *raft.Log, request *metastorev1.AddBlockRequest) (*metastorev1.AddBlockResponse, error) {
	err := m.db.boltdb.Update(func(tx *bbolt.Tx) error {
		err := m.persistBlock(tx, request.Block)
		if err != nil {
			return err
		}
		if err = m.compactBlock(request.Block, tx, log.Index); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = level.Error(m.logger).Log(
			"msg", "failed to add block",
			"block", request.Block.Id,
			"err", err,
		)
		return nil, err
	}
	err = m.index.InsertBlock(request.Block)
	if err != nil {
		return nil, err
	}
	return &metastorev1.AddBlockResponse{}, nil
}

func (m *metastoreState) persistBlock(tx *bbolt.Tx, block *metastorev1.BlockMeta) error {
	key := []byte(block.Id)
	value, err := block.MarshalVT()
	if err != nil {
		return err
	}

	partMeta, err := m.index.GetOrCreatePartitionMeta(block)
	if err != nil {
		return err
	}

	return updateBlockMetadataBucket(tx, partMeta, block.Shard, block.TenantId, func(bucket *bbolt.Bucket) error {
		return bucket.Put(key, value)
	})
}

func (m *metastoreState) deleteBlock(tx *bbolt.Tx, shard uint32, tenant, blockId string) error {
	partKey := m.index.GetPartitionKey(blockId)
	partMeta := m.index.FindPartitionMeta(partKey)
	if partMeta == nil {
		return fmt.Errorf("partition meta not found for %s", partKey)
	}
	return updateBlockMetadataBucket(tx, partMeta, shard, tenant, func(bucket *bbolt.Bucket) error {
		return bucket.Delete([]byte(blockId))
	})
}
