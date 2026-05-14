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

type FeedbackReply struct {
	ent.Schema
}

func (FeedbackReply) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "feedback_replies"},
	}
}

func (FeedbackReply) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("feedback_id"),
		field.Int64("user_id"),
		field.String("role").
			MaxLen(20),
		field.String("content").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			NotEmpty(),
		field.JSON("images", []string{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (FeedbackReply) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("feedback", Feedback.Type).
			Ref("replies").
			Field("feedback_id").
			Unique().
			Required(),
		edge.From("user", User.Type).
			Ref("feedback_replies").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (FeedbackReply) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("feedback_id"),
	}
}
