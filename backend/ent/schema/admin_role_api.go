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

// AdminRoleAPI 角色-API 关联表.
type AdminRoleAPI struct {
	ent.Schema
}

func (AdminRoleAPI) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "admin_role_apis"},
	}
}

func (AdminRoleAPI) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("role_id"),
		field.Int64("api_id"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{
				dialect.Postgres: "timestamptz",
			}),
	}
}

func (AdminRoleAPI) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("role_id", "api_id").Unique(),
		index.Fields("role_id"),
		index.Fields("api_id"),
	}
}
