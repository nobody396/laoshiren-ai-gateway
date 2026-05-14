package schema

import (
	"github.com/bozhouDev/DragonCode-sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// AdminRole holds the schema definition for the AdminRole entity.
type AdminRole struct {
	ent.Schema
}

func (AdminRole) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "admin_roles"},
	}
}

func (AdminRole) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AdminRole) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			MaxLen(50).
			NotEmpty().
			Unique(),
		field.String("description").
			MaxLen(500).
			Default(""),
		field.Bool("is_super_admin").
			Default(false),
		field.String("status").
			MaxLen(20).
			Default("active"),
	}
}
