package schema

import (
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ChangelogEntry stores curated Build in Public updates.
type ChangelogEntry struct {
	ent.Schema
}

func (ChangelogEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "changelog_entries"},
	}
}

func (ChangelogEntry) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").
			MaxLen(180).
			NotEmpty().
			Unique().
			Comment("公开详情页稳定 slug"),
		field.String("title").
			MaxLen(200).
			NotEmpty().
			Comment("更新标题"),
		field.String("summary").
			MaxLen(500).
			NotEmpty().
			Comment("一句话摘要"),
		field.String("rationale").
			MaxLen(500).
			NotEmpty().
			Comment("为什么做"),
		field.String("content").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			NotEmpty().
			Comment("更新正文（Markdown）"),
		field.String("category").
			MaxLen(30).
			NotEmpty().
			Comment("分类: feature, model_config, improvement, fix"),
		field.JSON("related_products", []string{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}).
			Comment("相关产品或范围"),
		field.String("status").
			MaxLen(20).
			Default(domain.ChangelogStatusDraft).
			Comment("状态: draft, published, archived"),
		field.Time("published_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Comment("公开发布时间"),
		field.String("commit_sha").
			MaxLen(64).
			Optional().
			Nillable().
			Comment("内部 Git commit 追溯，不公开"),
		field.String("pull_request_url").
			MaxLen(500).
			Optional().
			Nillable().
			Comment("内部 PR 追溯，不公开"),
		field.Int64("created_by").
			Optional().
			Nillable().
			Comment("创建人用户ID"),
		field.Int64("updated_by").
			Optional().
			Nillable().
			Comment("更新人用户ID"),
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

func (ChangelogEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "published_at"),
		index.Fields("category"),
		index.Fields("created_at"),
	}
}
