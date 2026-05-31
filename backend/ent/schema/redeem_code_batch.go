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

// RedeemCodeBatch groups generated card codes for billing reconciliation.
type RedeemCodeBatch struct {
	ent.Schema
}

func (RedeemCodeBatch) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "redeem_code_batches"},
	}
}

func (RedeemCodeBatch) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			MaxLen(128).
			NotEmpty(),
		field.String("purpose").
			MaxLen(32).
			Default(domain.RedeemCodePurposeSaleRecharge),
		field.Float("face_value").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.String("currency").
			MaxLen(16).
			Default("balance_unit"),
		field.String("sales_channel").
			MaxLen(32).
			Default("manual"),
		field.Text("external_url").
			Optional().
			Nillable(),
		field.Text("notes").
			Optional().
			Nillable(),
		field.Int64("created_by").
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
	}
}

func (RedeemCodeBatch) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("redeem_codes", RedeemCode.Type),
		edge.From("creator", User.Type).
			Ref("redeem_code_batches").
			Field("created_by").
			Unique(),
	}
}

func (RedeemCodeBatch) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("purpose"),
		index.Fields("created_at"),
	}
}
