package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 回归：分组绑定优先级（account_groups.priority）必须压过账号全局优先级。
// 背景：负载感知选择路径历史上只读 accounts.priority，导致分组主备配置不生效
// （2026-08-19 月卡探针全红事故：备用账号全局优先级与主力相同，LRU/负载决胜
// 下备用账号吸走全部探针与流量）。

func accountWithGroupPriority(id int64, globalPriority int, groupID int64, groupPriority int) *Account {
	return &Account{
		ID:       id,
		Priority: globalPriority,
		Status:   StatusActive,
		AccountGroups: []AccountGroup{
			{AccountID: id, GroupID: groupID, Priority: groupPriority},
		},
	}
}

func TestEffectivePriorityForGroup(t *testing.T) {
	acc := accountWithGroupPriority(1, 5, 42, 1)

	p, ok := acc.PriorityForGroup(42)
	require.True(t, ok)
	require.Equal(t, 1, p)

	gid := int64(42)
	require.Equal(t, 1, acc.EffectivePriorityForGroup(&gid))

	// 未绑定的分组 / 无分组语境回退全局优先级
	otherGid := int64(7)
	require.Equal(t, 5, acc.EffectivePriorityForGroup(&otherGid))
	require.Equal(t, 5, acc.EffectivePriorityForGroup(nil))

	_, ok = acc.PriorityForGroup(0)
	require.False(t, ok)
}

func TestFilterByMinPriorityUsesGroupBindingPriority(t *testing.T) {
	gid := int64(42)
	// 全局优先级相同（都是 1），绑定优先级不同：primary=1，backup=2
	primary := accountWithGroupPriority(14, 1, gid, 1)
	backup := accountWithGroupPriority(34, 1, gid, 2)
	load := &AccountLoadInfo{}

	accounts := []accountWithLoad{
		{account: backup, loadInfo: load},
		{account: primary, loadInfo: load},
	}

	result := filterByMinPriority(accounts, &gid)
	require.Len(t, result, 1)
	require.Equal(t, primary.ID, result[0].account.ID)

	// 无分组语境回退到全局优先级：两者同级，都保留
	result = filterByMinPriority(accounts, nil)
	require.Len(t, result, 2)
}

func TestFilterByMinPriorityGroupBindingOverridesGlobal(t *testing.T) {
	gid := int64(42)
	// 全局优先级更低（更优）但绑定优先级更高的账号，在分组内必须输给绑定 priority 1
	globalFavorite := accountWithGroupPriority(34, 0, gid, 2)
	groupFavorite := accountWithGroupPriority(14, 9, gid, 1)
	load := &AccountLoadInfo{}

	accounts := []accountWithLoad{
		{account: globalFavorite, loadInfo: load},
		{account: groupFavorite, loadInfo: load},
	}

	result := filterByMinPriority(accounts, &gid)
	require.Len(t, result, 1)
	require.Equal(t, groupFavorite.ID, result[0].account.ID)
}

func TestSortAccountsByPriorityAndLastUsedGroupBinding(t *testing.T) {
	gid := int64(42)
	older := time.Now().Add(-time.Hour)
	newer := time.Now()

	// backup 更久未用（旧代码会因此排在前面），但绑定优先级更低
	primary := accountWithGroupPriority(14, 1, gid, 1)
	primary.LastUsedAt = &newer
	backup := accountWithGroupPriority(34, 1, gid, 2)
	backup.LastUsedAt = &older

	accounts := []*Account{backup, primary}
	sortAccountsByPriorityAndLastUsed(accounts, false, &gid)
	require.Equal(t, primary.ID, accounts[0].ID)
	require.Equal(t, backup.ID, accounts[1].ID)

	// 同组内同绑定优先级：仍按 LastUsedAt 决胜
	peer1 := accountWithGroupPriority(21, 1, gid, 2)
	peer1.LastUsedAt = &newer
	peer2 := accountWithGroupPriority(22, 1, gid, 2)
	peer2.LastUsedAt = &older
	peers := []*Account{peer1, peer2}
	sortAccountsByPriorityAndLastUsed(peers, false, &gid)
	require.Equal(t, peer2.ID, peers[0].ID)
}

func TestSameAccountGroupRespectsGroupBinding(t *testing.T) {
	gid := int64(42)
	a := accountWithGroupPriority(14, 1, gid, 1)
	b := accountWithGroupPriority(34, 1, gid, 2)

	// 全局优先级相同但绑定优先级不同，不应视为同一打散组
	require.False(t, sameAccountGroup(a, b, &gid))
	// 无分组语境下两者同组（维持旧行为）
	require.True(t, sameAccountGroup(a, b, nil))
}
