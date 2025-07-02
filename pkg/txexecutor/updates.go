package txexecutor

import (
	"fmt"
)

// Additional implementations of Transaction for test cases.

// Deposit deposits Amount to To account.
type Deposit struct {
	To     string
	Amount uint
}

func (d Deposit) Updates(st AccountState) ([]AccountUpdate, error) {
	if d.Amount == 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	return []AccountUpdate{
		{Name: d.To, BalanceChange: int(d.Amount)},
	}, nil
}

// Withdraw withdraws Amount from From account.
type Withdraw struct {
	From   string
	Amount uint
}

func (w Withdraw) Updates(st AccountState) ([]AccountUpdate, error) {
	acc := st.GetAccount(w.From)
	if acc.Balance < w.Amount {
		return nil, fmt.Errorf("insufficient funds on %s", w.From)
	}
	return []AccountUpdate{
		{Name: w.From, BalanceChange: -int(w.Amount)},
	}, nil
}

// BatchTransfer transfers Amount from From to To every account in Tos list.
type BatchTransfer struct {
	From   string
	Tos    []string
	Amount uint
}

func (b BatchTransfer) Updates(st AccountState) ([]AccountUpdate, error) {
	from := st.GetAccount(b.From)
	total := uint(len(b.Tos)) * b.Amount
	if from.Balance < total {
		return nil, fmt.Errorf("not enough in %s for batch", b.From)
	}
	ups := make([]AccountUpdate, 0, len(b.Tos)+1)
	ups = append(ups, AccountUpdate{Name: b.From, BalanceChange: -int(total)})
	for _, to := range b.Tos {
		ups = append(ups, AccountUpdate{Name: to, BalanceChange: int(b.Amount)})
	}
	return ups, nil
}

// Interest pays RatePercent to Accounts.
type Interest struct {
	Accounts    []string
	RatePercent uint
}

func (in Interest) Updates(st AccountState) ([]AccountUpdate, error) {
	var ups []AccountUpdate
	for _, name := range in.Accounts {
		acc := st.GetAccount(name)
		delta := int(acc.Balance * in.RatePercent / 100)
		ups = append(ups, AccountUpdate{Name: name, BalanceChange: delta})
	}
	return ups, nil
}

// FeeSplit deducts Fee from Account and distributes it betwwen Receivers.
type FeeSplit struct {
	Account   string
	Fee       uint
	Receivers []string
}

func (f FeeSplit) Updates(st AccountState) ([]AccountUpdate, error) {
	acc := st.GetAccount(f.Account)
	if acc.Balance < f.Fee {
		return nil, fmt.Errorf("insufficient funds on %s for fee", f.Account)
	}
	n := uint(len(f.Receivers))
	if n == 0 {
		return nil, fmt.Errorf("no receivers for fee split")
	}
	per := f.Fee / n
	ups := []AccountUpdate{{Name: f.Account, BalanceChange: -int(f.Fee)}}
	for _, r := range f.Receivers {
		ups = append(ups, AccountUpdate{Name: r, BalanceChange: int(per)})
	}
	return ups, nil
}
