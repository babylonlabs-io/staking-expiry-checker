package services

import "sync"

type TrackedSubscriptions struct {
	mu            sync.RWMutex
	subscriptions map[string]struct{} // Using empty struct as value since we only care about existence
}

func NewTrackedSubscriptions() *TrackedSubscriptions {
	return &TrackedSubscriptions{
		subscriptions: make(map[string]struct{}),
	}
}

func (ts *TrackedSubscriptions) IsSubscribed(stakingTxHash string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	_, exists := ts.subscriptions[stakingTxHash]
	return exists
}

func (ts *TrackedSubscriptions) GetSubscribedHashes() map[string]struct{} {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	subscribed := make(map[string]struct{}, len(ts.subscriptions))
	for hash := range ts.subscriptions {
		subscribed[hash] = struct{}{} // Just copying hashes
	}
	return subscribed
}

// AddSubscriptions adds multiple subscriptions at once
func (ts *TrackedSubscriptions) AddSubscriptions(stakingTxHashes []string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for _, hash := range stakingTxHashes {
		ts.subscriptions[hash] = struct{}{}
	}
}

func (ts *TrackedSubscriptions) AddSubscription(stakingTxHash string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.subscriptions[stakingTxHash] = struct{}{} // Empty struct uses no memory
}

func (ts *TrackedSubscriptions) RemoveSubscription(stakingTxHash string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.subscriptions, stakingTxHash)
}
