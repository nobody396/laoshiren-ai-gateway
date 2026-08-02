package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TopupOrder holds the schema definition for the TopupOrder entity.
type TopupOrder struct {
	ent.Schema
}

func (TopupOrder) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "topup_orders"},
	}
}

func (TopupOrder) Fields() []ent.Field {
	return []ent.Field{
		field.String("order_no").
			MaxLen(32).
			NotEmpty().
			Unique(),
		field.Int64("user_id"),
		// 充值金额，单位：分（人民币）。例如 2000 = ¥20
		field.Int("amount_cny_fen").
			Positive(),
		// 活动赠送额度，单位：分。历史订单默认为 0，避免按金额追溯套用新活动。
		field.Int("bonus_amount_cny_fen").
			NonNegative().
			Default(0),
		// 支付渠道：alipay 或 wechat
		field.String("pay_type").
			MaxLen(16),
		// 订单状态：pending / completed / expired
		field.String("status").
			MaxLen(20).
			Default("pending"),
		// 开票状态：none / applied / invoiced
		field.String("invoice_status").
			MaxLen(20).
			Default("none"),
		// 虎皮椒平台交易号，回调时写入
		field.String("xunhu_trade_no").
			MaxLen(64).
			Optional().
			Nillable(),
		// 二维码图片 URL
		field.Text("qr_code_url").
			Optional().
			Nillable(),
		field.Time("completed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (TopupOrder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("topup_orders").
			Field("user_id").
			Required().
			Unique(),
		edge.To("invoice_request_orders", InvoiceRequestOrder.Type),
	}
}

func (TopupOrder) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("invoice_status"),
		index.Fields("order_no"),
		index.Fields("user_id", "invoice_status", "created_at"),
		index.Fields("invoice_status", "created_at"),
	}
}
