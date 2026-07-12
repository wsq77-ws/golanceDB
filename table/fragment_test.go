package table

import (
	"reflect"
	"testing"
)

func TestNewFragment(t *testing.T) {
	f := NewFragment(5)
	if f.ID != 5 {
		t.Errorf("expected ID 5, got %d", f.ID)
	}
	if f.NumDataFiles() != 0 {
		t.Errorf("expected 0 data files, got %d", f.NumDataFiles())
	}
	if f.ColumnIDs() != nil && len(f.ColumnIDs()) != 0 {
		t.Errorf("expected empty ColumnIDs, got %v", f.ColumnIDs())
	}
}

func TestFragmentAddDataFile(t *testing.T) {
	f := NewFragment(1)
	f.AddDataFile(&DataFile{Path: "a.lance", ColumnID: 0, NumRows: 10})
	f.AddDataFile(&DataFile{Path: "b.lance", ColumnID: 1, NumRows: 10})

	if f.NumDataFiles() != 2 {
		t.Errorf("expected 2 data files, got %d", f.NumDataFiles())
	}
}

func TestFragmentColumnIDs(t *testing.T) {
	f := NewFragment(1)
	f.AddDataFile(&DataFile{ColumnID: 0})
	f.AddDataFile(&DataFile{ColumnID: 2})
	f.AddDataFile(&DataFile{ColumnID: 4})

	got := f.ColumnIDs()
	want := []int32{0, 2, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestFragmentSetDeletionFile(t *testing.T) {
	f := NewFragment(1)
	f.NumRows = 100
	f.PhysicalRows = 100

	f.SetDeletionFile(&DeletionFile{Path: "del.lance", NumRows: 30})
	if f.DeletionFile == nil {
		t.Fatal("expected deletion file to be set")
	}
	if f.DeletionFile.NumRows != 30 {
		t.Errorf("expected 30 deleted rows, got %d", f.DeletionFile.NumRows)
	}
	if f.PhysicalRows != 70 {
		t.Errorf("expected 70 physical rows, got %d", f.PhysicalRows)
	}

	f.SetDeletionFile(nil)
	if f.DeletionFile != nil {
		t.Error("expected deletion file to be nil")
	}
	if f.PhysicalRows != 100 {
		t.Errorf("expected physical rows to reset to 100, got %d", f.PhysicalRows)
	}
}

func TestFragmentNumDataFiles(t *testing.T) {
	f := NewFragment(1)
	if f.NumDataFiles() != 0 {
		t.Errorf("expected 0, got %d", f.NumDataFiles())
	}
	f.AddDataFile(&DataFile{ColumnID: 0})
	f.AddDataFile(&DataFile{ColumnID: 1})
	if f.NumDataFiles() != 2 {
		t.Errorf("expected 2, got %d", f.NumDataFiles())
	}
}
