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

// AdminRoleMenu 角色-菜单 关联表.
type AdminRoleMenu struct {
	ent.Schema
}

func (AdminRoleMenu) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "admin_role_menus"},
	}
}

func (AdminRoleMenu) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("role_id"),
		field.Int64("menu_id"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{
				dialect.Postgres: "timestamptz",
			}),
	}
}

func (AdminRoleMenu) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("role_id", "menu_id").Unique(),
		index.Fields("role_id"),
		index.Fields("menu_id"),
	}
}
