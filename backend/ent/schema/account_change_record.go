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

// AccountChangeRecord stores balance/concurrency/subscription ledger entries.
type AccountChangeRecord struct {
	ent.Schema
}

func (AccountChangeRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "account_change_records"},
	}
}

func (AccountChangeRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("asset_type").
			MaxLen(20),
		field.String("reason").
			MaxLen(32),
		field.Float("delta").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.String("source_type").
			MaxLen(32).
			Default(""),
		field.Int64("source_id").
			Optional().
			Nillable(),
		field.String("reference_no").
			MaxLen(128).
			Optional().
			Nillable(),
		field.Int64("operator_user_id").
			Optional().
			Nillable(),
		field.String("notes").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("group_id").
			Optional().
			Nillable(),
		field.Int("validity_days").
			Default(0),
		field.String("dedupe_key").
			MaxLen(128).
			Optional().
			Nillable(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (AccountChangeRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("account_change_records").
			Field("user_id").
			Required().
			Unique(),
		edge.From("group", Group.Type).
			Ref("account_change_records").
			Field("group_id").
			Unique(),
	}
}

func (AccountChangeRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("asset_type"),
		index.Fields("reason"),
		index.Fields("source_type", "source_id"),
		index.Fields("user_id", "created_at"),
		index.Fields("user_id", "asset_type", "created_at"),
		index.Fields("dedupe_key").Unique(),
	}
}
