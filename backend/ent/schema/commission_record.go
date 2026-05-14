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

// CommissionRecord holds the schema definition for the CommissionRecord entity.
//
// 记录每一笔分佣/奖励流水：代理商消耗分佣、首充双向奖励、普通用户邀请奖励。
// 删除策略：不可删除（仅追加，财务流水）
type CommissionRecord struct {
	ent.Schema
}

func (CommissionRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "commission_records"},
	}
}

func (CommissionRecord) Fields() []ent.Field {
	return []ent.Field{
		// 获得分佣/奖励的用户（代理商或普通邀请人）
		field.Int64("beneficiary_id").
			Comment("获得分佣/奖励的用户 ID"),
		// 触发分佣的用户（消费者/充值者）
		field.Int64("user_id").
			Comment("触发分佣的用户 ID（消费方/充值方）"),
		// 分佣金额
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Comment("分佣/奖励金额（已入账到 beneficiary 余额的金额）"),
		// 触发来源金额
		field.Float("source_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Comment("触发分佣的原始金额（API 消费额或充值额）"),
		// 分佣类型
		field.String("type").
			MaxLen(50).
			Comment("分佣类型：consumption_commission / first_recharge_invitee_bonus / first_recharge_referral_bonus"),
		// 关联来源记录 ID（usage_log.id 或 redeem_code.id），可选
		field.Int64("source_id").
			Optional().
			Nillable().
			Comment("关联的来源记录 ID（usage_log.id 或 redeem_code.id）"),
		// 备注
		field.String("note").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Optional().
			Nillable().
			Comment("备注"),
		// 创建时间（不可变）
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Comment("记录创建时间"),
	}
}

func (CommissionRecord) Edges() []ent.Edge {
	return nil
}

func (CommissionRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("beneficiary_id"),
		index.Fields("user_id"),
		index.Fields("type"),
		index.Fields("created_at"),
		index.Fields("beneficiary_id", "created_at"),
		index.Fields("beneficiary_id", "type"),
	}
}
