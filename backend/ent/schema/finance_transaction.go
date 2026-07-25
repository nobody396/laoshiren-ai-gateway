package schema

import (
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// FinanceTransaction holds the schema definition for the FinanceTransaction entity.
//
// 手工记账流水：真实现金进出（售卖收入、上游进货、服务器/域名等成本），
// 与 cost_accounting 的理论毛利率计算相互独立，互不依赖。
type FinanceTransaction struct {
	ent.Schema
}

func (FinanceTransaction) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "finance_transactions"},
	}
}

func (FinanceTransaction) Fields() []ent.Field {
	return []ent.Field{
		field.String("type").
			MaxLen(10).
			NotEmpty().
			Comment("income / expense"),
		field.String("category").
			MaxLen(30).
			NotEmpty().
			Comment("流水分类，取值受 type 约束"),
		field.Int64("amount_fen").
			Positive().
			Comment("金额，单位：分（人民币），恒为正，方向由 type 决定"),
		field.Time("occurred_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Comment("实际发生时间（可补记，非录入时间）"),
		field.Text("note").
			Optional().
			Nillable().
			Comment("备注"),
		field.String("receipt_key").
			MaxLen(255).
			Optional().
			Nillable().
			Comment("凭证图片的 S3 object key"),
		field.String("source").
			MaxLen(10).
			Default(domain.FinanceTransactionSourceManual).
			Comment("manual / skill"),
		field.Int64("created_by").
			Optional().
			Nillable().
			Comment("创建人用户ID（管理员）"),
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

func (FinanceTransaction) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("type"),
		index.Fields("category"),
		index.Fields("occurred_at"),
		index.Fields("created_at"),
	}
}
