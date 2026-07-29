// Package logstore 封装 MongoDB 日志存储。
package logstore

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/kvm-inspection/common"
)

// Store MongoDB 日志存储
type Store struct {
	col *mongo.Collection
	db  *mongo.Database
}

// New 连接 MongoDB 并返回 Store
func New(ctx context.Context, uri, db, coll string) (*Store, error) {
	cli, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := cli.Ping(ctx, nil); err != nil {
		return nil, err
	}
	database := cli.Database(db)
	c := database.Collection(coll)
	// 创建常用索引
	_, _ = c.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "node_id", Value: 1}}},
		{Keys: bson.D{{Key: "timestamp", Value: -1}}},
		{Keys: bson.D{{Key: "is_violation", Value: 1}}},
		{Keys: bson.D{{Key: "vm_id", Value: 1}}},
	})
	return &Store{col: c, db: database}, nil
}

// Database 暴露底层 database，供其它 collection 复用同一连接池
func (s *Store) Database() *mongo.Database { return s.db }

// Insert 写入一条事件
func (s *Store) Insert(ctx context.Context, ev *common.NetworkEvent) error {
	_, err := s.col.InsertOne(ctx, ev)
	return err
}

// InsertMany 批量写入
func (s *Store) InsertMany(ctx context.Context, evs []*common.NetworkEvent) error {
	if len(evs) == 0 {
		return nil
	}
	docs := make([]any, 0, len(evs))
	for _, e := range evs {
		docs = append(docs, e)
	}
	_, err := s.col.InsertMany(ctx, docs)
	return err
}

// Query 查询条件
type Query struct {
	NodeID      string
	VMID        string
	DstIP       string
	Domain      string
	IsViolation *bool
	From        time.Time
	To          time.Time
	Limit       int
	Skip        int
}

// Query 分页查询事件
func (s *Store) Query(ctx context.Context, q Query) ([]*common.NetworkEvent, int64, error) {
	filter := bson.D{}
	if q.NodeID != "" {
		filter = append(filter, bson.E{Key: "node_id", Value: q.NodeID})
	}
	if q.VMID != "" {
		filter = append(filter, bson.E{Key: "vm_id", Value: q.VMID})
	}
	if q.DstIP != "" {
		filter = append(filter, bson.E{Key: "dst_ip", Value: q.DstIP})
	}
	if q.Domain != "" {
		filter = append(filter, bson.E{Key: "domain", Value: q.Domain})
	}
	if q.IsViolation != nil {
		filter = append(filter, bson.E{Key: "is_violation", Value: *q.IsViolation})
	}
	if !q.From.IsZero() {
		filter = append(filter, bson.E{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: q.From}}})
	}
	if !q.To.IsZero() {
		filter = append(filter, bson.E{Key: "timestamp", Value: bson.D{{Key: "$lte", Value: q.To}}})
	}

	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}).SetLimit(int64(limit)).SetSkip(int64(q.Skip))

	cur, err := s.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var out []*common.NetworkEvent
	if err := cur.All(ctx, &out); err != nil {
		return nil, 0, err
	}
	total, err := s.col.CountDocuments(ctx, filter)
	if err != nil {
		return out, 0, err
	}
	return out, total, nil
}
