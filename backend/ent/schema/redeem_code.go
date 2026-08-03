package schema

import (
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RedeemCode holds the schema definition for the RedeemCode entity.
//
// 删除策略：硬删除
// RedeemCode 使用硬删除而非软删除，原因如下：
//   - 兑换码具有一次性使用特性，删除后无需保留历史记录
//   - 已使用的兑换码通过 status 和 used_at 字段追踪，无需依赖软删除
//   - 减少数据库存储压力和查询复杂度
//
// 如需审计已删除的兑换码，建议在删除前将关键信息写入审计日志表。
type RedeemCode struct {
	ent.Schema
}

func (RedeemCode) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "redeem_codes"},
	}
}

func (RedeemCode) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").
			MaxLen(32).
			NotEmpty().
			Unique(),
		field.String("type").
			MaxLen(20).
			Default(domain.RedeemTypeBalance),
		field.Float("value").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("paid_value").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0).
			Comment("Actual customer payment for affiliate and revenue attribution; zero keeps legacy value fallback"),
		field.String("status").
			MaxLen(20).
			Default(domain.StatusUnused),
		field.Int64("used_by").
			Optional().
			Nillable(),
		field.Time("used_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("notes").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("batch_id").
			Optional().
			Nillable(),
		field.String("purpose").
			MaxLen(32).
			Default(domain.RedeemCodePurposeSaleRecharge),
		field.String("sales_status").
			MaxLen(32).
			Default(domain.RedeemCodeSalesStatusInventory),
		field.Time("sold_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Text("sold_to_note").
			Optional().
			Nillable(),
		field.String("external_order_no").
			MaxLen(128).
			Optional().
			Nillable(),
		field.Text("external_order_url").
			Optional().
			Nillable(),
		field.Text("internal_notes").
			Optional().
			Nillable(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("group_id").
			Optional().
			Nillable(),
		field.JSON("group_ids", []int64{}).
			Default([]int64{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}).
			Comment("订阅组合兑换码关联的多个分组 ID；为空时兼容旧版 group_id"),
		field.Int("validity_days").
			Default(30),
	}
}

func (RedeemCode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("redeem_codes").
			Field("used_by").
			Unique(),
		edge.From("group", Group.Type).
			Ref("redeem_codes").
			Field("group_id").
			Unique(),
		edge.From("batch", RedeemCodeBatch.Type).
			Ref("redeem_codes").
			Field("batch_id").
			Unique(),
	}
}

func (RedeemCode) Indexes() []ent.Index {
	return []ent.Index{
		// code 字段已在 Fields() 中声明 Unique()，无需重复索引
		index.Fields("status"),
		index.Fields("used_by"),
		index.Fields("group_id"),
		index.Fields("batch_id"),
		index.Fields("purpose"),
		index.Fields("sales_status"),
		index.Fields("purpose", "sales_status"),
		index.Fields("used_at"),
		index.Fields("created_at"),
	}
}
