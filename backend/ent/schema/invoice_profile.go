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

// InvoiceProfile holds the schema definition for the InvoiceProfile entity.
type InvoiceProfile struct {
	ent.Schema
}

func (InvoiceProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "invoice_profiles"},
	}
}

func (InvoiceProfile) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (InvoiceProfile) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("title").
			MaxRuneLen(200).
			NotEmpty(),
		field.String("tax_number").
			MaxRuneLen(20).
			NotEmpty(),
		field.String("email").
			MaxRuneLen(100).
			NotEmpty(),
		field.String("address").
			MaxRuneLen(500).
			Optional().
			Nillable(),
		field.String("phone").
			MaxRuneLen(40).
			Optional().
			Nillable(),
		field.String("bank_name").
			MaxRuneLen(200).
			Optional().
			Nillable(),
		field.String("bank_account").
			MaxRuneLen(100).
			Optional().
			Nillable(),
		field.Bool("is_default").
			Default(false),
	}
}

func (InvoiceProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("invoice_profiles").
			Field("user_id").
			Required().
			Unique(),
		edge.To("invoice_requests", InvoiceRequest.Type),
	}
}

func (InvoiceProfile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("user_id", "is_default"),
	}
}
