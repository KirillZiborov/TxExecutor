package txexecutor

import "fmt"

// Transfer transfers some value from From account to To account.
type Transfer struct {
	From  string
	To    string
	Value uint
}

// Updates implements Transaction interface.
func (t Transfer) Updates(state AccountState) ([]AccountUpdate, error) {
	fromAcc := state.GetAccount(t.From)
	if fromAcc.Balance < t.Value {
		return nil, fmt.Errorf("insufficient balance on %s", t.From)
	}
	return []AccountUpdate{
		{Name: t.From, BalanceChange: -int(t.Value)},
		{Name: t.To, BalanceChange: int(t.Value)},
	}, nil
}
