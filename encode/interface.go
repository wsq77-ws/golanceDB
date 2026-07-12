package encode

// ColumnEncoder encodes column data into bytes and decodes back.
type ColumnEncoder interface {
	Encode(data interface{}, dataType DataType) ([]byte, error)
	Decode(encoded []byte, dataType DataType, numRows int64) (interface{}, error)
}
