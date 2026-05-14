package schema

import (
	"github.com/bozhouDev/DragonCode-sub2api/ent/schema/mixins"
	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// InvoiceRequest holds the schema definition for the InvoiceRequest entity.
type InvoiceRequest struct {
	ent.Schema
}

func (InvoiceRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "invoice_requests"},
	}
}

func (InvoiceRequest) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (InvoiceRequest) Fields() []ent.Field {
	return []ent.Field{
		field.String("serial_no").
			MaxLen(20).
			NotEmpty().
			Unique(),
		field.Int64("user_id"),
		field.Int64("profile_id").
			Optional().
			Nillable(),
		field.JSON("profile_snapshot_json", domain.InvoiceProfileSnapshot{}),
		field.Int64("total_amount_fen"),
		field.String("status").
			MaxLen(20).
			Default("pending"),
		field.String("export_batch_no").
			MaxLen(40).
			Optional().
			Nillable(),
		field.Time("exported_at").
			Optional().
			Nillable(),
		field.Int64("exported_by").
			Optional().
			Nillable(),
		field.Text("remark").
			Optional().
			Nillable(),
		field.String("reject_reason").
			MaxLen(255).
			Optional().
			Nillable(),
		field.Time("completed_at").
			Optional().
			Nillable(),
		field.Int64("completed_by").
			Optional().
			Nillable(),
	}
}

func (InvoiceRequest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("invoice_requests").
			Field("user_id").
			Required().
			Unique(),
		edge.From("profile", InvoiceProfile.Type).
			Ref("invoice_requests").
			Field("profile_id").
			Unique(),
		edge.To("request_orders", InvoiceRequestOrder.Type),
	}
}

func (InvoiceRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("serial_no"),
		index.Fields("created_at"),
		index.Fields("user_id", "status", "created_at"),
		index.Fields("status", "created_at"),
	}
}
