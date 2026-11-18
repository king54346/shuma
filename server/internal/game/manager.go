package game

import (
	"log"
	"sync"

	"shuma-server/internal/login"
	"shuma-server/internal/playerstore"
)

// Manager routes game requests to per-account persistent Service instances.
type Manager struct {
	mu         sync.RWMutex
	store      *playerstore.Store
	accounts   *login.AccountStore
	defaultKey playerstore.AccountKey
	services   map[playerstore.AccountKey]*Service
	activeSvc  *Service
}

func NewManager(store *playerstore.Store, accounts *login.AccountStore, defaultKey playerstore.AccountKey) (*Manager, error) {
	m := &Manager{
		store:      store,
		accounts:   accounts,
		defaultKey: defaultKey,
		services:   make(map[playerstore.AccountKey]*Service),
	}
	svc, err := m.getOrLoad(defaultKey, 100001)
	if err != nil {
		return nil, err
	}
	m.activeSvc = svc
	return m, nil
}

func (m *Manager) getOrLoad(key playerstore.AccountKey, accountID int64) (*Service, error) {
	m.mu.RLock()
	if svc, ok := m.services[key]; ok {
		m.mu.RUnlock()
		return svc, nil
	}
	m.mu.RUnlock()

	svc, err := NewWithStore(m.store, key)
	if err != nil {
		return nil, err
	}
	svc.mu.Lock()
	if accountID > 0 {
		svc.accountID = accountID
	}
	svc.accountName = key.Account
	svc.mu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.services[key]; ok {
		return existing, nil
	}
	m.services[key] = svc
	return svc, nil
}

func (m *Manager) Handle(route string, req any) map[string]any {
	if route == "/game/login" && m.accounts != nil {
		if acct, servKey, ok := m.accounts.ConsumePendingSession(); ok && acct.Name != "" {
			key := playerstore.AccountKey{Account: acct.Name, ServerKey: servKey}
			svc, err := m.getOrLoad(key, acct.ID)
			if err != nil {
				log.Printf("[game] Failed to load account %s@%s: %v", acct.Name, servKey, err)
				return gameError("account_load_failed")
			}
			m.mu.Lock()
			m.activeSvc = svc
			m.mu.Unlock()
		}
	}

	m.mu.RLock()
	svc := m.activeSvc
	m.mu.RUnlock()
	return svc.Handle(route, req)
}
