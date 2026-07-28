package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type currentMonthlyAccountRepoStub struct {
	AccountRepository
	accounts map[int64][]Account
}

func (s *currentMonthlyAccountRepoStub) ListByGroup(_ context.Context, groupID int64) ([]Account, error) {
	return s.accounts[groupID], nil
}

type currentMonthlyProgramRepoStub struct {
	settings AffiliateProgramSettings
}

func (s *currentMonthlyProgramRepoStub) GetSettings(context.Context) (*AffiliateProgramSettings, error) {
	copy := s.settings
	return &copy, nil
}

func (s *currentMonthlyProgramRepoStub) UpdateSettings(context.Context, *AffiliateProgramSettings, int64) error {
	panic("unexpected UpdateSettings call")
}

func guardedMonthlyTestGroup(id int64, name, platform string, rate, limit float64) Group {
	return Group{
		ID:                  id,
		Name:                name,
		Platform:            platform,
		RateMultiplier:      rate,
		Status:              StatusActive,
		SubscriptionType:    SubscriptionTypeCredit,
		MonthlyLimitUSD:     &limit,
		DefaultValidityDays: 31,
	}
}

func TestCurrentMonthlyCardGenerationGuardAcceptsFreshCompleteBundle(t *testing.T) {
	settings := DefaultAffiliateProgramSettings()
	gpt := guardedMonthlyTestGroup(101, "GPT Plus 月卡组", PlatformOpenAI, 0.50, 220)
	claude := guardedMonthlyTestGroup(102, "Claude Plus 月卡组", PlatformAnthropic, 2.40, 220)
	svc := &adminServiceImpl{
		accountRepo: &currentMonthlyAccountRepoStub{accounts: map[int64][]Account{
			101: {{ID: 1, Status: StatusActive, Schedulable: true}},
			102: {{ID: 2, Status: StatusActive, Schedulable: true}},
		}},
		affiliateProgram: NewAffiliateProgramService(&currentMonthlyProgramRepoStub{settings: settings}),
	}

	current, err := svc.guardCurrentMonthlyCardGeneration(context.Background(), []Group{gpt, claude}, 31, 249)

	require.NoError(t, err)
	require.True(t, current)
}

func TestCurrentMonthlyCardGenerationGuardRejectsPartialBundle(t *testing.T) {
	settings := DefaultAffiliateProgramSettings()
	gpt := guardedMonthlyTestGroup(101, "GPT Plus 月卡组", PlatformOpenAI, 0.50, 220)
	svc := &adminServiceImpl{
		accountRepo: &currentMonthlyAccountRepoStub{accounts: map[int64][]Account{
			101: {{ID: 1, Status: StatusActive, Schedulable: true}},
		}},
		affiliateProgram: NewAffiliateProgramService(&currentMonthlyProgramRepoStub{settings: settings}),
	}

	current, err := svc.guardCurrentMonthlyCardGeneration(context.Background(), []Group{gpt}, 31, 249)

	require.True(t, current)
	require.ErrorContains(t, err, "complete GPT and Claude group bundle")
}

func TestCurrentMonthlyCardGenerationGuardRejectsUnpricedFaceValue(t *testing.T) {
	settings := DefaultAffiliateProgramSettings()
	gpt := guardedMonthlyTestGroup(101, "GPT Plus 月卡组", PlatformOpenAI, 0.50, 220)
	claude := guardedMonthlyTestGroup(102, "Claude Plus 月卡组", PlatformAnthropic, 2.40, 220)
	svc := &adminServiceImpl{
		accountRepo: &currentMonthlyAccountRepoStub{accounts: map[int64][]Account{
			101: {{ID: 1, Status: StatusActive, Schedulable: true}},
			102: {{ID: 2, Status: StatusActive, Schedulable: true}},
		}},
		affiliateProgram: NewAffiliateProgramService(&currentMonthlyProgramRepoStub{settings: settings}),
	}

	current, err := svc.guardCurrentMonthlyCardGeneration(context.Background(), []Group{gpt, claude}, 31, 1)

	require.True(t, current)
	require.ErrorContains(t, err, "face value must match")
}

func TestCurrentMonthlyCatalogGroupShapeIsImmutable(t *testing.T) {
	group := guardedMonthlyTestGroup(101, "GPT Plus 月卡组", PlatformOpenAI, 0.50, 220)
	require.NoError(t, validateCurrentMonthlyCatalogGroupShape(group))

	group.RateMultiplier = 0.51
	require.ErrorContains(t, validateCurrentMonthlyCatalogGroupShape(group), "immutable")
}
