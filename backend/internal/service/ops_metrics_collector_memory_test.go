package service

import (
	"testing"

	"github.com/shirou/gopsutil/v4/mem"
	"github.com/stretchr/testify/require"
)

const (
	testMiB = 1024 * 1024
	testGiB = 1024 * testMiB
)

func TestResolveMemoryStatsUnlimitedCgroupFallsBackEntirelyToHost(t *testing.T) {
	const cgroupUsed = uint64(64573440)
	host := &mem.VirtualMemoryStat{Used: 16 * testGiB, Total: 24 * testGiB, UsedPercent: 66.7}

	usedMB, totalMB, pct := resolveMemoryStats(cgroupUsed, 0, true, host)

	require.NotNil(t, usedMB)
	require.NotNil(t, totalMB)
	require.NotNil(t, pct)
	require.Equal(t, int64(16*1024), *usedMB)
	require.Equal(t, int64(24*1024), *totalMB)
	require.InDelta(t, 66.7, *pct, 0.05)
	require.NotEqual(t, int64(cgroupUsed/testMiB), *usedMB)
}

func TestResolveMemoryStatsLimitedCgroupUsesOnlyCgroup(t *testing.T) {
	host := &mem.VirtualMemoryStat{Used: 16 * testGiB, Total: 24 * testGiB, UsedPercent: 66.7}

	usedMB, totalMB, pct := resolveMemoryStats(512*testMiB, 2*testGiB, true, host)

	require.Equal(t, int64(512), *usedMB)
	require.Equal(t, int64(2048), *totalMB)
	require.InDelta(t, 25.0, *pct, 0.05)
}

func TestResolveMemoryStatsNoCgroupUsesHost(t *testing.T) {
	host := &mem.VirtualMemoryStat{Used: 16 * testGiB, Total: 24 * testGiB, UsedPercent: 66.7}

	usedMB, totalMB, pct := resolveMemoryStats(0, 0, false, host)

	require.Equal(t, int64(16*1024), *usedMB)
	require.Equal(t, int64(24*1024), *totalMB)
	require.InDelta(t, 66.7, *pct, 0.05)
}

func TestResolveMemoryStatsNoDataReturnsNil(t *testing.T) {
	usedMB, totalMB, pct := resolveMemoryStats(0, 0, false, nil)
	require.Nil(t, usedMB)
	require.Nil(t, totalMB)
	require.Nil(t, pct)
}

func TestResolveMemoryStatsHostWithoutTotalKeepsHostPercentage(t *testing.T) {
	host := &mem.VirtualMemoryStat{Used: 8 * testGiB, UsedPercent: 42.5}

	usedMB, totalMB, pct := resolveMemoryStats(0, 0, false, host)

	require.Equal(t, int64(8*1024), *usedMB)
	require.Nil(t, totalMB)
	require.InDelta(t, 42.5, *pct, 0.05)
}
