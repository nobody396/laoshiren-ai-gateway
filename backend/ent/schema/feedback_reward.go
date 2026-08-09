package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// FeedbackReward is the immutable reward audit record. One feedback can be rewarded once.
type FeedbackReward struct{ ent.Schema }

func (FeedbackReward) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "feedback_rewards"}}
}

func (FeedbackReward) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("feedback_id"),
		field.Int64("user_id"),
		field.Float("amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.String("reason").MaxLen(64),
		field.String("batch_id").MaxLen(64),
		field.Int64("operator_user_id").Optional().Nillable(),
		field.Int64("account_change_record_id").Optional().Nillable(),
		field.Time("granted_at").Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (FeedbackReward) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("feedback_id").Unique(),
		index.Fields("user_id", "created_at"),
		index.Fields("batch_id"),
	}
}
