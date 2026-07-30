package blockscout

import "encoding/json"

type Tx struct {
	BlockNumber   json.Number `json:"blockNumber"`
	Confirmations json.Number `json:"confirmations"`
	From          string      `json:"from"`
	GasLimit      json.Number `json:"gasLimit"`
	GasPrice      json.Number `json:"gasPrice"`
	GasUsed       json.Number `json:"gasUsed"`
	Hash          string      `json:"hash"`
	To            string      `json:"to"`
	Success       bool        `json:"success"`
	Timestamp     json.Number `json:"timeStamp"`
	Value         json.Number `json:"value"`
}

type ContractCreator struct {
	BlockNumber     json.Number `json:"blockNumber"`
	ContractAddress string      `json:"contractAddress"`
	ContractCreator string      `json:"contractCreator"`
	ContractFactory string      `json:"contractFactory"`
	Timestamp       json.Number `json:"timestamp"`
	TxHash          string      `json:"txHash"`
}

type Contract struct {
	ContractName     string `json:"ContractName"`
	CompilerVersion  string `json:"CompilerVersion"`
	OptimizationUsed string `json:"OptimizationUsed"`
	EVMVersion       string `json:"EVMVersion"`
	Filename         string `json:"FileName"`
	Address          string `json:"Address"`
	ABI              string `json:"ABI"`
}
