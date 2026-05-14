package schema

import (
	"github.com/bozhouDev/DragonCode-sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AdminAPI 后端 API 资源, 用于 RBAC 细粒度控制接口级权限.
// 通过启动时扫描 Gin 路由自动生成, 管理员可补充 description / group / sort_order.
type AdminAPI struct {
	ent.Schema
}

func (AdminAPI) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "admin_apis"},
	}
}

func (AdminAPI) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AdminAPI) Fields() []ent.Field {
	return []ent.Field{
		field.String("group").
			MaxLen(100).
			Default("").
			Comment("API 分组名 (如 用户管理组)"),
		field.String("path").
			MaxLen(255).
			NotEmpty().
			Comment("API 路径 (含参数占位符, 如 /admin/users/:id)"),
		field.String("method").
			MaxLen(10).
			NotEmpty().
			Comment("HTTP method: GET/POST/PUT/DELETE/PATCH"),
		field.String("description").
			MaxLen(255).
			Default("").
			Comment("API 简介"),
		field.Int("sort_order").
			Default(0),
		field.String("status").
			MaxLen(20).
			Default("active"),
	}
}

func (AdminAPI) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("method", "path").Unique(),
		index.Fields("group"),
	}
}
