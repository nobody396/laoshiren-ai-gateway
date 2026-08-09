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

// UserNotification stores durable in-app notifications.
type UserNotification struct{ ent.Schema }

func (UserNotification) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "user_notifications"}}
}

func (UserNotification) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("feedback_id").Optional().Nillable(),
		field.String("type").MaxLen(40),
		field.String("title").MaxLen(200),
		field.String("body").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("action_url").MaxLen(500).Default(""),
		field.String("dedupe_key").MaxLen(128).Optional().Nillable(),
		field.Time("read_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (UserNotification) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "read_at", "created_at"),
		index.Fields("feedback_id"),
		index.Fields("dedupe_key").Unique(),
	}
}
