package encode

// DataType represents column data types.
type DataType int32

const (
	TypeInt8          DataType = 1
	TypeInt16         DataType = 2
	TypeInt32         DataType = 3
	TypeInt64         DataType = 4
	TypeFloat16       DataType = 5
	TypeFloat32       DataType = 6
	TypeFloat64       DataType = 7
	TypeString        DataType = 8
	TypeBinary        DataType = 9
	TypeStruct        DataType = 10
	TypeList          DataType = 11
	TypeMap           DataType = 12
	TypeFixedSizeList DataType = 13
)

// EncodingType represents page encoding strategies.
type EncodingType int32

const (
	EncodingMiniBlock EncodingType = 1
	EncodingFullZip   EncodingType = 2
	EncodingConstant  EncodingType = 3
	EncodingFlat      EncodingType = 4
	EncodingAllNull   EncodingType = 5
)

// CompressionType represents compression algorithms.
type CompressionType int32

const (
	CompressionNone CompressionType = 0
	CompressionZstd CompressionType = 1
	CompressionLz4  CompressionType = 2
)

// BufferInfo describes a buffer within a page.
type BufferInfo struct {
	Offset      int64
	Length      int64
	Compression CompressionType
}

// PageLayout describes the encoding layout of a data page.
type PageLayout struct {
	Encoding           EncodingType
	MiniBlockNumBlocks int32
	MiniBlockBlockSize int32
	Buffers            []*BufferInfo
}
