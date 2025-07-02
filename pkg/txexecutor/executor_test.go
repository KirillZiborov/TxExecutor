package txexecutor

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func TestExample1(t *testing.T) {
	ResetState([]AccountValue{{"A", 20}, {"B", 30}, {"C", 40}})
	block := Block{Transactions: []Transaction{
		Transfer{"A", "B", 5},
		Transfer{"B", "C", 10},
		Transfer{"B", "C", 30}, // should fail
	}}
	got, _ := ExecuteBlock(block)
	want := []AccountValue{{"A", 15}, {"B", 25}, {"C", 50}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestExample2(t *testing.T) {
	ResetState([]AccountValue{{"A", 10}, {"B", 20}, {"C", 30}, {"D", 40}})
	block := Block{Transactions: []Transaction{
		Transfer{"A", "B", 5},
		Transfer{"C", "D", 10},
	}}
	got, _ := ExecuteBlock(block)
	want := []AccountValue{{"A", 5}, {"B", 25}, {"C", 20}, {"D", 50}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestDepositAndWithdraw(t *testing.T) {
	ResetState([]AccountValue{{Name: "X", Balance: 10}})
	block := Block{Transactions: []Transaction{
		Deposit{To: "Y", Amount: 5},    // Y = 5
		Withdraw{From: "X", Amount: 7}, // X: 10-7 = 3
		Withdraw{From: "X", Amount: 5}, // fail
		Deposit{To: "X", Amount: 0},    // dummy
	}}
	got, err := ExecuteBlock(block)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []AccountValue{
		{Name: "X", Balance: 3},
		{Name: "Y", Balance: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestBatchTransfer(t *testing.T) {
	ResetState([]AccountValue{
		{Name: "A", Balance: 100},
		{Name: "B", Balance: 0},
		{Name: "C", Balance: 0},
	})
	block := Block{Transactions: []Transaction{
		BatchTransfer{From: "A", Tos: []string{"B", "C"}, Amount: 10}, // A=80, B=10, C=10
		BatchTransfer{From: "A", Tos: []string{"B", "C"}, Amount: 30}, // A=20, B=40, C=40
		BatchTransfer{From: "C", Tos: []string{"A", "B"}, Amount: 5},  // A=25, B=45, C=30
	}}
	got, _ := ExecuteBlock(block)
	want := []AccountValue{
		{Name: "A", Balance: 25},
		{Name: "B", Balance: 45},
		{Name: "C", Balance: 30},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestBatchTransfer2(t *testing.T) {
	ResetState([]AccountValue{
		{Name: "A", Balance: 100},
		{Name: "B", Balance: 0},
		{Name: "C", Balance: 0},
	})
	block := Block{Transactions: []Transaction{
		BatchTransfer{From: "A", Tos: []string{"B", "C"}, Amount: 10}, // A=80, B=10, C=10
		BatchTransfer{From: "A", Tos: []string{"B", "C"}, Amount: 50}, // fails
		BatchTransfer{From: "C", Tos: []string{"A", "B"}, Amount: 5},  // A=85, B=15, C=0
	}}
	got, _ := ExecuteBlock(block)
	want := []AccountValue{
		{Name: "A", Balance: 85},
		{Name: "B", Balance: 15},
		{Name: "C", Balance: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestInterestAccrual(t *testing.T) {
	ResetState([]AccountValue{
		{Name: "A", Balance: 100},
		{Name: "B", Balance: 200},
		{Name: "C", Balance: 33},
	})
	block := Block{Transactions: []Transaction{
		Interest{Accounts: []string{"A", "B", "C"}, RatePercent: 5}, // +5,+10,+1
		Interest{Accounts: []string{"A", "C"}, RatePercent: 10},     // recalc: A from 105→+10, C from 34→+3
	}}
	got, _ := ExecuteBlock(block)
	want := []AccountValue{
		{Name: "A", Balance: 115}, // 100+5+10
		{Name: "B", Balance: 210}, // 200+10
		{Name: "C", Balance: 37},  // 33+1+3
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestHighContentionTransfers(t *testing.T) {
	ResetState([]AccountValue{
		{Name: "A", Balance: 1000},
		{Name: "B", Balance: 0},
	})
	// 100 tansfers between A and B (roundtrip)
	var txs []Transaction
	for i := 0; i < 50; i++ {
		txs = append(txs,
			Transfer{From: "A", To: "B", Value: 1},
			Transfer{From: "B", To: "A", Value: 1},
		)
	}
	block := Block{Transactions: txs}
	got, _ := ExecuteBlock(block)

	want := []AccountValue{
		{Name: "A", Balance: 1000},
		{Name: "B", Balance: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestEdgeCases(t *testing.T) {
	ResetState(nil)
	block := Block{Transactions: []Transaction{
		Withdraw{From: "X", Amount: 1},         // fails
		Deposit{To: "Y", Amount: 0},            // fails
		Transfer{From: "X", To: "Y", Value: 1}, // fails
	}}
	got, _ := ExecuteBlock(block)
	want := []AccountValue{
		{Name: "X", Balance: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// helpers
type seqCtx struct{ st map[string]uint }

func (s *seqCtx) GetAccount(name string) AccountValue {
	return AccountValue{Name: name, Balance: s.st[name]}
}

func TestConcurrentVsSequentialDeterminism(t *testing.T) {
	ResetState([]AccountValue{
		{"A0", 1000},
		{"A1", 1000},
		{"A2", 1000},
		{"A3", 1000},
		{"A4", 1000},
	})

	var txs []Transaction

	// 15 transfers A0->A1->...->A4->A0
	for i := 0; i < 15; i++ {
		from := fmt.Sprintf("A%d", i%5)
		to := fmt.Sprintf("A%d", (i+1)%5)
		val := uint((i + 1) * 10)
		txs = append(txs, Transfer{From: from, To: to, Value: val})
	}
	// 5 BatchTransfers from A0 to A1, A2, A3
	for i := 0; i < 5; i++ {
		txs = append(txs, BatchTransfer{
			From:   "A0",
			Tos:    []string{"A1", "A2", "A3"},
			Amount: uint(20 + i*5),
		})
	}
	// 5 Interests: give 5% to A0,A2,A4
	for i := 0; i < 5; i++ {
		txs = append(txs, Interest{
			Accounts:    []string{"A0", "A2", "A4"},
			RatePercent: uint(5 + i), // 5%,6%,7%,8%,9%
		})
	}
	// 5 FeeSplits: deduct fees from A3 to A0,A1,A2
	for i := 0; i < 5; i++ {
		txs = append(txs, FeeSplit{
			Account:   "A3",
			Fee:       uint(30 + i*10),
			Receivers: []string{"A0", "A1", "A2"},
		})
	}

	block := Block{Transactions: txs}

	seqState := make(map[string]uint)
	for _, v := range []AccountValue{
		{"A0", 1000}, {"A1", 1000}, {"A2", 1000}, {"A3", 1000}, {"A4", 1000},
	} {
		seqState[v.Name] = v.Balance
	}

	for _, tx := range txs {
		ups, err := tx.Updates(&seqCtx{seqState})
		if err != nil {
			continue
		}
		for _, u := range ups {
			seqState[u.Name] += uint(u.BalanceChange)
		}
	}

	var want []AccountValue
	for name, bal := range seqState {
		want = append(want, AccountValue{Name: name, Balance: bal})
	}
	sort.Slice(want, func(i, j int) bool { return want[i].Name < want[j].Name })

	got, err := ExecuteBlock(block)
	if err != nil {
		t.Fatalf("concurrent executor returned error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf(
			"determinism violated:\nwant=%+v\ngot =%+v",
			want, got,
		)
	}
}
