package table

// DataFile is a physical file storing encoded column data.
type DataFile struct {
	Path     string
	ColumnID int32
	NumRows  int64
	FileSize int64
}

// DeletionFile tracks deleted row offsets within a fragment.
type DeletionFile struct {
	Path    string
	NumRows int64
}

// Fragment is a horizontal slice of a table.
type Fragment struct {
	ID           int32
	NumRows      int64
	DataFiles    []*DataFile
	DeletionFile *DeletionFile
	PhysicalRows int64
}

// NewFragment creates an empty Fragment with the given ID.
func NewFragment(id int32) *Fragment {
	return &Fragment{ID: id}
}

// AddDataFile appends a DataFile to the fragment.
func (f *Fragment) AddDataFile(df *DataFile) {
	f.DataFiles = append(f.DataFiles, df)
}

// SetDeletionFile sets the deletion file and updates PhysicalRows.
func (f *Fragment) SetDeletionFile(df *DeletionFile) {
	f.DeletionFile = df
	if df != nil {
		f.PhysicalRows = f.NumRows - df.NumRows
	} else {
		f.PhysicalRows = f.NumRows
	}
}

// NumDataFiles returns the number of data files in the fragment.
func (f *Fragment) NumDataFiles() int {
	return len(f.DataFiles)
}

// ColumnIDs returns all column IDs present in this fragment.
func (f *Fragment) ColumnIDs() []int32 {
	ids := make([]int32, len(f.DataFiles))
	for i, df := range f.DataFiles {
		ids[i] = df.ColumnID
	}
	return ids
}
