// Package ifstatstore 封装接口流量快照的 MongoDB 存储与查询。
package ifstatstore

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/kvm-inspection/common"
)

// Store 接口流量存储
type Store struct {
	col *mongo.Collection
}

// New 连接 MongoDB，返回接口流量存储
func New(ctx context.Context, db *mongo.Database, collName string) (*Store, error) {
	col := db.Collection(collName)
	_, _ = col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "node_id", Value: 1}, {Key: "name", Value: 1}}},
		{Keys: bson.D{{Key: "timestamp", Value: -1}}},
	})
	return &Store{col: col}, nil
}

// InsertMany 批量写入接口流量快照
func (s *Store) InsertMany(ctx context.Context, stats []common.IfaceStat) error {
	if len(stats) == 0 {
		return nil
	}
	docs := make([]any, 0, len(stats))
	for i := range stats {
		docs = append(docs, stats[i])
	}
	_, err := s.col.InsertMany(ctx, docs)
	return err
}

// IfStatQuery 接口流量查询条件
type IfStatQuery struct {
	NodeID string
	Name   string
	From   time.Time
	To     time.Time
	Limit  int
}

// Query 按条件查询接口流量历史（时间正序，便于前端绘图）
func (s *Store) Query(ctx context.Context, q IfStatQuery) ([]common.IfaceStat, error) {
	filter := bson.D{}
	if q.NodeID != "" {
		filter = append(filter, bson.E{Key: "node_id", Value: q.NodeID})
	}
	if q.Name != "" {
		filter = append(filter, bson.E{Key: "name", Value: q.Name})
	}
	if !q.From.IsZero() {
		filter = append(filter, bson.E{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: q.From}}})
	}
	if !q.To.IsZero() {
		filter = append(filter, bson.E{Key: "timestamp", Value: bson.D{{Key: "$lte", Value: q.To}}})
	}

	limit := q.Limit
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}}).SetLimit(int64(limit))

	cur, err := s.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []common.IfaceStat
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Latest 返回某节点每个接口的最新一条快照（用于接口列表卡片）。
func (s *Store) Latest(ctx context.Context, nodeID string) ([]common.IfaceStat, error) {
	// 用聚合：按 name 分组取 timestamp 最大的一条
	pipeline := []bson.D{
		{{Key: "$match", Value: bson.D{{Key: "node_id", Value: nodeID}}}},
		{{Key: "$sort", Value: bson.D{{Key: "timestamp", Value: -1}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$name"},
			{Key: "doc", Value: bson.D{{Key: "$first", Value: "$$ROOT"}}},
		}}},
	}
	cur, err := s.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var grouped []struct {
		Doc common.IfaceStat `bson:"doc"`
	}
	if err := cur.All(ctx, &grouped); err != nil {
		return nil, err
	}
	out := make([]common.IfaceStat, 0, len(grouped))
	for _, g := range grouped {
		out = append(out, g.Doc)
	}
	return out, nil
}
