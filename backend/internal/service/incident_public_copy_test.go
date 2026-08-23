package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePublicIncidentCopyRejectsInternalRoutingAndCredentials(t *testing.T) {
	require.NoError(t, validatePublicIncidentCopy(IncidentPhaseResolved, "相关服务已经恢复，用户无需进行额外操作。"))
	require.Error(t, validatePublicIncidentCopy(IncidentPhaseInvestigating, "相关服务已经恢复，用户无需进行额外操作。"))
	for _, unsafe := range []string{
		"供应商线路已经切换",
		"account 53 recovered",
		"route aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa recovered",
		"provider Pomo JP endpoint 0123456789abcdef recovered",
		"内部账号 Pomo-HK 已经恢复",
		"联系 support@example.com 获取 token",
		"服务 10.0.0.8 已恢复",
		"请改用 https://internal.invalid",
		"API key [redacted]",
	} {
		require.Error(t, validatePublicIncidentCopy(IncidentPhaseInvestigating, unsafe), unsafe)
	}
}
