package service

import (
	"sync"
	"time"
)

const grokRuntimeBlockFallback = 2 * time.Minute

func isOpenAICompatibleAccount(account *Account) bool {
	return account != nil && account.IsOpenAICompatible()
}

func (s *OpenAIGatewayService) openAIAccountRuntimeBlockLock(accountID int64) *sync.Mutex {
	actual, _ := s.openaiAccountRuntimeBlockLocks.LoadOrStore(accountID, &sync.Mutex{})
	mu, ok := actual.(*sync.Mutex)
	if !ok {
		mu = &sync.Mutex{}
		s.openaiAccountRuntimeBlockLocks.Store(accountID, mu)
	}
	return mu
}

func (s *OpenAIGatewayService) BlockAccountScheduling(account *Account, until time.Time, _ string) {
	if s == nil || !isOpenAICompatibleAccount(account) {
		return
	}
	mu := s.openAIAccountRuntimeBlockLock(account.ID)
	mu.Lock()
	defer mu.Unlock()
	_, _ = s.blockAccountSchedulingLocked(account, until, "")
}

func (s *OpenAIGatewayService) blockAccountSchedulingLocked(account *Account, until time.Time, _ string) (uint64, bool) {
	if s == nil || !isOpenAICompatibleAccount(account) {
		return 0, false
	}
	if until.IsZero() || !until.After(time.Now()) {
		until = time.Now().Add(grokRuntimeBlockFallback)
	}
	generation := s.openaiAccountRuntimeBlockSequence.Add(1)
	s.openaiAccountRuntimeBlockGeneration.Store(account.ID, generation)
	if current, ok := s.openaiAccountRuntimeBlockUntil.Load(account.ID); ok {
		if currentUntil, ok := current.(time.Time); ok && !until.After(currentUntil) {
			return generation, false
		}
	}
	s.openaiAccountRuntimeBlockUntil.Store(account.ID, until)
	return generation, true
}

func (s *OpenAIGatewayService) ClearAccountSchedulingBlock(accountID int64) {
	if s == nil || accountID <= 0 {
		return
	}
	mu := s.openAIAccountRuntimeBlockLock(accountID)
	mu.Lock()
	defer mu.Unlock()
	s.openaiAccountRuntimeBlockUntil.Delete(accountID)
	s.openaiAccountRuntimeBlockGeneration.Store(accountID, s.openaiAccountRuntimeBlockSequence.Add(1))
}

func (s *OpenAIGatewayService) isOpenAIAccountRuntimeBlocked(account *Account) bool {
	if s == nil || !isOpenAICompatibleAccount(account) {
		return false
	}
	mu := s.openAIAccountRuntimeBlockLock(account.ID)
	mu.Lock()
	defer mu.Unlock()
	value, ok := s.openaiAccountRuntimeBlockUntil.Load(account.ID)
	if !ok {
		return false
	}
	until, ok := value.(time.Time)
	if ok && time.Now().Before(until) {
		return true
	}
	s.openaiAccountRuntimeBlockUntil.Delete(account.ID)
	s.openaiAccountRuntimeBlockGeneration.Store(account.ID, s.openaiAccountRuntimeBlockSequence.Add(1))
	return false
}
