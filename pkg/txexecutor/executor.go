// Package txexecutor implements a concurrent transaction executor for a blockchain.
// The executor processes transactions within a block concurrently,
// ensuring the global account state is updated consistently and safely.
package txexecutor

import (
	"log"
	"sort"
	"sync"
)

// Updates
type AccountUpdate struct {
	Name          string
	BalanceChange int
}

// Account
type AccountValue struct {
	Name    string // a name (a string) which uniquely identifies the account.
	Balance uint   // a balance (an unsigned integer) representing the account’s balance or state.
}

// AccountState is the snapshot of accounts at a given block.
// If the account does not exist, return zero balance
type AccountState interface {
	GetAccount(name string) AccountValue
}

// Transaction is a function that takes the current account state and returns a list of updates to the accounts.
// Each update is a key/value pair, where the key is the account name,
// and the value is the balance change for that account.
type Transaction interface {
	Updates(AccountState) ([]AccountUpdate, error)
}

// Block is an ordered sequence of transactions.
// If a transaction in a block fails other transactions can still be run.
type Block struct {
	Transactions []Transaction
}

// Workers process transactions.
var Workers = 5

// internal account representation with versioning and locking
type acct struct {
	mu  sync.Mutex
	bal uint
	ver uint64
}

var (
	state sync.Map // map from name to *acct
)

// ensureAcct returns the *acct for name or creates it.
func ensureAcct(name string) *acct {
	if v, ok := state.Load(name); ok {
		return v.(*acct)
	}
	a := &acct{}
	real, _ := state.LoadOrStore(name, a)
	return real.(*acct)
}

// txCtx implements AccountState.
type txCtx struct {
	reads map[string]uint64 // name -> version
}

// newTxCtx initializes the execution context for transaction.
func newTxCtx() *txCtx {
	return &txCtx{reads: make(map[string]uint64)}
}

// GetAccount implementation for txCtx.
func (c *txCtx) GetAccount(name string) AccountValue {
	ac := ensureAcct(name)
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if _, seen := c.reads[name]; !seen {
		c.reads[name] = ac.ver
	}
	return AccountValue{Name: name, Balance: ac.bal}
}

type txResult struct {
	idx     int
	updates []AccountUpdate
	reads   map[string]uint64
	err     error
}

// ExecuteBlock implementation.
func ExecuteBlock(b Block) ([]AccountValue, error) {
	log.Printf("[INFO] ExecuteBlock: %d tx, workers=%d", len(b.Transactions), Workers)

	// channels
	execCh := make(chan int, len(b.Transactions))
	resCh := make(chan txResult, len(b.Transactions))

	// --- Execution Phase ---
	for i := 0; i < Workers; i++ {
		go func() {
			for idx := range execCh {
				ctx := newTxCtx()
				upd, err := b.Transactions[idx].Updates(ctx)
				resCh <- txResult{
					idx:     idx,
					updates: upd,
					reads:   ctx.reads,
					err:     err,
				}
			}
		}()
	}

	for i := range b.Transactions {
		execCh <- i
	}

	// --- Commit Phase (deterministic) ---
	ready := make(map[int]txResult) // processed txs
	k := 0                          // expected tx id to commit

	for k < len(b.Transactions) {
		var r txResult
		var ok bool

		if r, ok = ready[k]; !ok {
			// wait for new result
			r = <-resCh
			ready[r.idx] = r
			continue // try again
		}

		// try to commit
		if commitTx(r) {
			delete(ready, k)
			k++
		} else {
			// retry
			delete(ready, k)
			execCh <- k
		}
	}

	close(execCh)
	log.Printf("[INFO] ExecuteBlock complete")

	// collect final state
	var final []AccountValue
	state.Range(func(k, v any) bool {
		a := v.(*acct)
		final = append(final, AccountValue{Name: k.(string), Balance: a.bal})
		return true
	})
	sort.Slice(final, func(i, j int) bool { return final[i].Name < final[j].Name })
	return final, nil
}

// commitTx tries to commit tx.
// Returns true, if tx is finalized (success or tx error),
// false if there is a conflict and retry needed.
func commitTx(r txResult) bool {
	accSet := make(map[string]struct{}, len(r.reads)+len(r.updates))
	for n := range r.reads {
		accSet[n] = struct{}{}
	}
	for _, u := range r.updates {
		accSet[u.Name] = struct{}{}
	}

	names := make([]string, 0, len(accSet))
	for n := range accSet {
		names = append(names, n)
	}
	sort.Strings(names) // global lex order

	locked := make([]*acct, 0, len(names))
	for _, n := range names {
		a := ensureAcct(n)
		a.mu.Lock()
		locked = append(locked, a)
	}

	// ensure consistent version
	for n, v := range r.reads {
		if ensureAcct(n).ver != v {
			for _, a := range locked {
				a.mu.Unlock()
			}
			log.Printf("[RETRY] transaction #%d due to stale read", r.idx)
			return false
		}
	}

	// update failed
	if r.err != nil || len(r.updates) == 0 {
		for _, a := range locked {
			a.mu.Unlock()
		}
		log.Printf("[SKIP] transaction #%d error=%v", r.idx, r.err)
		return true
	}

	// check balances
	for _, u := range r.updates {
		ac := ensureAcct(u.Name)
		if int(ac.bal)+u.BalanceChange < 0 {
			for _, a := range locked {
				a.mu.Unlock()
			}
			return true
		}
	}

	// commit
	for _, u := range r.updates {
		ac := ensureAcct(u.Name)
		ac.bal += uint(u.BalanceChange)
		ac.ver++
	}
	for _, a := range locked {
		a.mu.Unlock()
	}
	log.Printf("[COMMIT] transaction #%d applied", r.idx)
	return true
}
