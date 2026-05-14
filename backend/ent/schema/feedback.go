package schema

import (
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/ent/schema/mixins"
	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Feedback struct {
	ent.Schema
}

func (Feedback) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "feedbacks"},
	}
}

func (Feedback) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.SoftDeleteMixin{},
	}
}

func (Feedback) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("category").
			MaxLen(20).
			Default(domain.FeedbackCategoryOther),
		field.String("title").
			MaxLen(200).
			NotEmpty(),
		field.String("content").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			NotEmpty(),
		field.JSON("images", []string{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("contact").
			MaxLen(255).
			Default(""),
		field.String("priority").
			MaxLen(10).
			Default(domain.FeedbackPriorityLow),
		field.String("status").
			MaxLen(20).
			Default(domain.FeedbackStatusPending),
		field.Int("reply_count").
			Default(0),
		field.Time("last_reply_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("last_reply_role").
			MaxLen(20).
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

func (Feedback) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("feedbacks").
			Field("user_id").
			Unique().
			Required(),
		edge.To("replies", FeedbackReply.Type),
	}
}

func (Feedback) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("category"),
		index.Fields("priority"),
		index.Fields("created_at"),
		index.Fields("last_reply_at"),
		index.Fields("deleted_at"),
	}
}
