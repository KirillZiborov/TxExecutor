package txexecutor

import "fmt"

// Transfer transfers some value from from account to to account.
type Transfer struct {
	from  string
	to    string
	value uint
}

// Updates implements Transaction interface.
func (t Transfer) Updates(state AccountState) ([]AccountUpdate, error) {
	fromAcc := state.GetAccount(t.from)
	if fromAcc.Balance < t.value {
		return nil, fmt.Errorf("insufficient balance on %s", t.from)
	}
	return []AccountUpdate{
		{Name: t.from, BalanceChange: -int(t.value)},
		{Name: t.to, BalanceChange: int(t.value)},
	}, nil
}
