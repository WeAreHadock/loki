package local

import (
	"context"

	"github.com/cortexproject/cortex/pkg/chunk"
	"github.com/cortexproject/cortex/pkg/chunk/local"
	chunk_util "github.com/cortexproject/cortex/pkg/chunk/util"
	"go.etcd.io/bbolt"
)

type BoltdbIndexClientWithShipper struct {
	*local.BoltIndexClient
	shipper *Shipper
}

// NewBoltDBIndexClient creates a new IndexClient that used BoltDB.
func NewBoltDBIndexClient(cfg local.BoltDBConfig, archiveStoreClient chunk.ObjectClient, archiverCfg ShipperConfig) (chunk.IndexClient, error) {
	boltDBIndexClient, err := local.NewBoltDBIndexClient(cfg)
	if err != nil {
		return nil, err
	}

	shipper, err := NewShipper(archiverCfg, archiveStoreClient, boltDBIndexClient)
	if err != nil {
		return nil, err
	}

	indexClient := BoltdbIndexClientWithShipper{
		BoltIndexClient: boltDBIndexClient,
		shipper:         shipper,
	}

	return &indexClient, nil
}

func (b *BoltdbIndexClientWithShipper) Stop() {
	b.shipper.Stop()
	b.BoltIndexClient.Stop()
}

func (b *BoltdbIndexClientWithShipper) QueryPages(ctx context.Context, queries []chunk.IndexQuery, callback func(chunk.IndexQuery, chunk.ReadBatch) (shouldContinue bool)) error {
	return chunk_util.DoParallelQueries(ctx, b.query, queries, callback)
}

func (b *BoltdbIndexClientWithShipper) query(ctx context.Context, query chunk.IndexQuery, callback chunk_util.Callback) error {
	db, err := b.GetDB(query.TableName, local.DBOperationRead)
	if err != nil && err != local.ErrUnexistentBoltDB {
		return err
	}

	if db != nil {
		if err := b.QueryDB(ctx, db, query, callback); err != nil {
			return err
		}
	}

	return b.shipper.forEach(ctx, query.TableName, func(db *bbolt.DB) error {
		return b.QueryDB(ctx, db, query, callback)
	})
}
