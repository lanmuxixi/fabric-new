package common

import (
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
)

type TableRecord struct {
	RecordName  string `json:"record_name"`
	RecordValue string `json:"record_value"`
	RecordType  string `json:"record_type"`
	RecordOwner string `json:"record_owner"`
	RecordTTL   int32  `json:"record_ttl"`
}

func BuildRecordFromRawRow(row shim.Row) TableRecord {
	var record TableRecord
	if row.GetColumns() == nil {
		return TableRecord{}
	}
	if row.GetColumns()[0] != nil {
		record.RecordName = row.Columns[0].GetString_()
	}
	if row.GetColumns()[1] != nil {
		record.RecordValue = row.Columns[1].GetString_()
	}
	if row.GetColumns()[2] != nil {
		record.RecordType = row.Columns[2].GetString_()
	}
	if row.GetColumns()[3] != nil {
		record.RecordOwner = row.Columns[3].GetString_()
	}
	if row.GetColumns()[4] != nil {
		record.RecordTTL = row.Columns[4].GetInt32()
	}
	return record
}

func BuildRowFromRecord(record TableRecord) shim.Row {
	var columns []*shim.Column
	columns = append(columns, &shim.Column{Value: &shim.Column_String_{String_: record.RecordName}})
	columns = append(columns, &shim.Column{Value: &shim.Column_String_{String_: record.RecordValue}})
	columns = append(columns, &shim.Column{Value: &shim.Column_String_{String_: record.RecordType}})
	columns = append(columns, &shim.Column{Value: &shim.Column_String_{String_: record.RecordOwner}})
	columns = append(columns, &shim.Column{Value: &shim.Column_Int32{Int32: record.RecordTTL}})
	return shim.Row{Columns: columns}
}

// InsertRecord Insert a record
func InsertRecord(stub shim.ChaincodeStubInterface, record TableRecord) (bool, error) {
	row := BuildRowFromRecord(record)
	return stub.InsertRow(DNS_RECORD_TABLE_NAME, row)
}

// UpdateRecord Query a record, if not exist, insert it
func UpdateRecord(stub shim.ChaincodeStubInterface, record TableRecord) (bool, error) {
	oldRecord, err := GetRecordByKey(stub, record.RecordName)
	if err != nil {
		return false, err
	}

	newRecord := TableRecord{
		RecordName:  record.RecordName,
		RecordValue: record.RecordValue,
		RecordType:  record.RecordType,
		RecordOwner: record.RecordOwner,
		RecordTTL:   record.RecordTTL,
	}
	if oldRecord == (TableRecord{}) {
		return stub.InsertRow(DNS_RECORD_TABLE_NAME, BuildRowFromRecord(newRecord))
	}
	return stub.ReplaceRow(DNS_RECORD_TABLE_NAME, BuildRowFromRecord(newRecord))
}

// DeleteRecord Delete a record
func DeleteRecord(stub shim.ChaincodeStubInterface, record TableRecord) error {
	row := BuildRowFromRecord(record)
	return stub.DeleteRow(DNS_RECORD_TABLE_NAME, []shim.Column{*row.Columns[0]})
}

// CheckRecordExist Check if a record exist
func CheckRecordExist(stub shim.ChaincodeStubInterface, record TableRecord) (bool, error) {
	row, err := stub.GetRow(DNS_RECORD_TABLE_NAME, []shim.Column{{Value: &shim.Column_String_{String_: record.RecordName}}})
	if err != nil {
		return false, err
	}
	return row.Columns != nil, nil
}

// GetRecordByKey Get a record
func GetRecordByKey(stub shim.ChaincodeStubInterface, key string) (TableRecord, error) {
	row, err := stub.GetRow(DNS_RECORD_TABLE_NAME, []shim.Column{{Value: &shim.Column_String_{String_: key}}})
	if err != nil {
		return TableRecord{}, err
	}
	return BuildRecordFromRawRow(row), nil
}

// GetAllRecords Get all records
func GetAllRecords(stub shim.ChaincodeStubInterface, table string) ([]TableRecord, error) {
	var records []TableRecord
	rowChannel, err := stub.GetRows(table, []shim.Column{})
	if err != nil {
		return nil, fmt.Errorf("get rows failed: %v", err)
	}

	for row := range rowChannel {
		record := BuildRecordFromRawRow(row)
		records = append(records, record)
	}
	return records, nil
}
