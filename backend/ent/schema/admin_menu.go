package schema

import (
	"github.com/bozhouDev/DragonCode-sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AdminMenu 菜单资源 (扁平列表, type 固定为 'menu').
// 历史上曾有 directory / button 两种类型以及 parent_id 父子关系,
// 已由 migration 119/120 清理并扁平化.
type AdminMenu struct {
	ent.Schema
}

func (AdminMenu) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "admin_menus"},
	}
}

func (AdminMenu) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AdminMenu) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		field.String("name_en").
			MaxLen(100).
			Default("").
			Comment("english menu name for locale-aware sidebar rendering"),
		field.String("type").
			MaxLen(20).
			NotEmpty().
			Default("menu").
			Comment("fixed to 'menu' after migration 120"),
		field.String("path").
			MaxLen(255).
			Default("").
			Comment("frontend route path (menu type)"),
		field.String("component").
			MaxLen(255).
			Default("").
			Comment("frontend component path"),
		field.String("icon").
			MaxLen(100).
			Default("").
			Comment("icon identifier"),
		field.String("permission_key").
			MaxLen(100).
			NotEmpty().
			Unique().
			Comment("permission identifier, e.g. admin:users or user:create"),
		field.Int("sort_order").
			Default(0),
		field.String("status").
			MaxLen(20).
			Default("active"),
	}
}

func (AdminMenu) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("type"),
		index.Fields("permission_key"),
	}
}
