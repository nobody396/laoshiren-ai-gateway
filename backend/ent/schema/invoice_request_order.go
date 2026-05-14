package schema

import (
	"github.com/bozhouDev/DragonCode-sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// InvoiceRequestOrder holds the schema definition for the InvoiceRequestOrder entity.
type InvoiceRequestOrder struct {
	ent.Schema
}

func (InvoiceRequestOrder) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "invoice_request_orders"},
	}
}

func (InvoiceRequestOrder) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (InvoiceRequestOrder) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("invoice_request_id"),
		field.Int64("topup_order_id"),
	}
}

func (InvoiceRequestOrder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("invoice_request", InvoiceRequest.Type).
			Ref("request_orders").
			Field("invoice_request_id").
			Required().
			Unique(),
		edge.From("topup_order", TopupOrder.Type).
			Ref("invoice_request_orders").
			Field("topup_order_id").
			Required().
			Unique(),
	}
}

func (InvoiceRequestOrder) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("invoice_request_id", "topup_order_id").Unique(),
		index.Fields("topup_order_id").Unique(),
	}
}
