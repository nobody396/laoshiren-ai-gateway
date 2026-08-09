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

// FeedbackEvent is the append-only business timeline visible to operators and users.
type FeedbackEvent struct{ ent.Schema }

func (FeedbackEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "feedback_events"}}
}

func (FeedbackEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("feedback_id"),
		field.String("event_type").MaxLen(40),
		field.String("actor_type").MaxLen(16),
		field.Int64("actor_user_id").Optional().Nillable(),
		field.String("summary").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("metadata", map[string]string{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (FeedbackEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("feedback_id", "created_at"),
		index.Fields("event_type"),
	}
}
