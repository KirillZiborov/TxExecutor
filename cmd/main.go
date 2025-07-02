package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/KirillZiborov/TxExecutor/pkg/txexecutor"
)

func main() {
	flag.IntVar(&txexecutor.Workers, "workers", txexecutor.Workers, "number of workers")
	flag.Parse()

	txexecutor.ResetState([]txexecutor.AccountValue{{"A", 20}, {"B", 30}, {"C", 40}})
	block := txexecutor.Block{Transactions: []txexecutor.Transaction{
		txexecutor.Transfer{"A", "B", 5},
		txexecutor.Transfer{"B", "C", 10},
		txexecutor.Transfer{"B", "C", 30},
	}}
	finalState, err := txexecutor.ExecuteBlock(block)
	if err != nil {
		log.Fatal(err)
	}
	data, _ := json.MarshalIndent(finalState, "", "  ")
	fmt.Println(string(data))
}
