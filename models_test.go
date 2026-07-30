package blockscout

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTx_Unmarshal(t *testing.T) {
	raw := `{
		"blockNumber": "18000000",
		"confirmations": "42",
		"from": "0xAAA",
		"gasLimit": "21000",
		"gasPrice": "1000000000",
		"gasUsed": "21000",
		"hash": "0xdeadbeef",
		"to": "0xBBB",
		"success": true,
		"timeStamp": "1700000000",
		"value": "1000000000000000000"
	}`

	var tx Tx
	require.NoError(t, json.Unmarshal([]byte(raw), &tx))

	assert.Equal(t, "18000000", tx.BlockNumber.String())
	assert.Equal(t, "42", tx.Confirmations.String())
	assert.Equal(t, "0xAAA", tx.From)
	assert.Equal(t, "0xBBB", tx.To)
	assert.Equal(t, "0xdeadbeef", tx.Hash)
	assert.True(t, tx.Success)
	assert.Equal(t, "1700000000", tx.Timestamp.String())
	assert.Equal(t, "1000000000000000000", tx.Value.String())
}

func TestTx_UnmarshalMissingFields(t *testing.T) {
	var tx Tx
	require.NoError(t, json.Unmarshal([]byte(`{}`), &tx))
	assert.Equal(t, "", tx.Hash)
	assert.Equal(t, "", tx.From)
	assert.False(t, tx.Success)
}

func TestContractCreator_Unmarshal(t *testing.T) {
	raw := `{
		"blockNumber": "17000000",
		"contractAddress": "0xCONTRACT",
		"contractCreator": "0xCREATOR",
		"contractFactory": "0xFACTORY",
		"timestamp": "1699000000",
		"txHash": "0xTXHASH"
	}`

	var cc ContractCreator
	require.NoError(t, json.Unmarshal([]byte(raw), &cc))

	assert.Equal(t, "17000000", cc.BlockNumber.String())
	assert.Equal(t, "0xCONTRACT", cc.ContractAddress)
	assert.Equal(t, "0xCREATOR", cc.ContractCreator)
	assert.Equal(t, "0xFACTORY", cc.ContractFactory)
	assert.Equal(t, "1699000000", cc.Timestamp.String())
	assert.Equal(t, "0xTXHASH", cc.TxHash)
}

func TestContract_Unmarshal(t *testing.T) {
	raw := `{
		"ContractName": "MyToken",
		"CompilerVersion": "v0.8.20+commit.a1b79de6",
		"OptimizationUsed": "1",
		"EVMVersion": "paris",
		"FileName": "MyToken.sol",
		"Address": "0xTOKEN",
		"ABI": "[{\"type\":\"function\"}]"
	}`

	var c Contract
	require.NoError(t, json.Unmarshal([]byte(raw), &c))

	assert.Equal(t, "MyToken", c.ContractName)
	assert.Equal(t, "v0.8.20+commit.a1b79de6", c.CompilerVersion)
	assert.Equal(t, "1", c.OptimizationUsed)
	assert.Equal(t, "paris", c.EVMVersion)
	assert.Equal(t, "MyToken.sol", c.Filename)
	assert.Equal(t, "0xTOKEN", c.Address)
	assert.Contains(t, c.ABI, "function")
}

func TestContract_UnmarshalMissingFields(t *testing.T) {
	var c Contract
	require.NoError(t, json.Unmarshal([]byte(`{}`), &c))
	assert.Equal(t, "", c.ContractName)
	assert.Equal(t, "", c.ABI)
}

func TestTx_RoundTrip(t *testing.T) {
	original := Tx{
		Hash:    "0xdeadbeef",
		From:    "0xAAA",
		To:      "0xBBB",
		Success: true,
	}
	b, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Tx
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, original.Hash, decoded.Hash)
	assert.Equal(t, original.From, decoded.From)
	assert.Equal(t, original.Success, decoded.Success)
}
