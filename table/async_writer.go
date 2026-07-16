package table

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/storage"
)

const (
	defaultBatchChanCap  = 256
	defaultResultChanCap = 16
)

// FlushResult records the outcome of a flush operation.
type FlushResult struct {
	Fragment *Fragment
	Manifest *Manifest
	Error    error
}

// AsyncWriter buffers RecordBatch writes in a channel and flushes them
// to disk via a background goroutine. It batches multiple writes into
// a single fragment when possible.
type AsyncWriter struct {
	store         storage.Store
	manifestStore *ManifestStore
	schema        *Schema
	tablePath     string
	compression   encode.CompressionType

	batchCh  chan *RecordBatch // incoming write requests
	resultCh chan *FlushResult // flush results
	flushCh  chan chan error   // manual flush trigger
	closeCh  chan struct{}     // close signal

	maxBatchSize  int           // max rows per fragment
	flushInterval time.Duration // auto-flush interval
	currentBatch  *RecordBatch  // accumulating batch
	currentRows   int64
	fragmentID    int32
	nextVersion   int64 // tracked for manifest commits
	done          chan struct{}
	started       atomic.Bool
	closeOnce     sync.Once
	closeErr      error
}

// NewAsyncWriter creates an AsyncWriter with the given configuration.
func NewAsyncWriter(store storage.Store, manifestStore *ManifestStore, schema *Schema, tablePath string, compression encode.CompressionType, maxBatchSize int, flushInterval time.Duration) *AsyncWriter {
	return &AsyncWriter{
		store:         store,
		manifestStore: manifestStore,
		schema:        schema,
		tablePath:     tablePath,
		compression:   compression,
		batchCh:       make(chan *RecordBatch, defaultBatchChanCap),
		resultCh:      make(chan *FlushResult, defaultResultChanCap),
		flushCh:       make(chan chan error),
		closeCh:       make(chan struct{}),
		maxBatchSize:  maxBatchSize,
		flushInterval: flushInterval,
		done:          make(chan struct{}),
	}
}

// Start launches the background goroutine. It reads the latest version
// from the manifest store to initialize nextVersion and fragmentID;
// if no versions exist, it starts at 1.
func (w *AsyncWriter) Start(ctx context.Context) error {
	versions, err := w.manifestStore.ListVersions(ctx)
	if err != nil {
		return fmt.Errorf("table: %w", err)
	}
	if len(versions) == 0 {
		w.nextVersion = 1
		w.fragmentID = 0
	} else {
		latest := versions[len(versions)-1]
		w.nextVersion = latest + 1
		// Read the latest manifest to get the current MaxFragmentID.
		manifest, rErr := w.manifestStore.Read(ctx, latest)
		if rErr == nil {
			w.fragmentID = manifest.MaxFragmentID + 1
		}
	}
	w.started.Store(true)
	go w.run(ctx)
	return nil
}

// Write enqueues a batch for asynchronous processing. It is non-blocking
// when the channel has capacity. It returns an error if the writer is
// closed, not running, or the channel is full.
func (w *AsyncWriter) Write(ctx context.Context, batch *RecordBatch) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("table: %w", err)
	}
	select {
	case w.batchCh <- batch:
		return nil
	case <-w.closeCh:
		return fmt.Errorf("table: writer is closed")
	case <-w.done:
		return fmt.Errorf("table: writer is not running")
	default:
		return fmt.Errorf("table: write channel is full")
	}
}

// Flush triggers a synchronous flush of the current accumulated batch.
// It blocks until the flush completes or the writer is closed.
func (w *AsyncWriter) Flush() error {
	respCh := make(chan error, 1)
	select {
	case w.flushCh <- respCh:
		select {
		case err := <-respCh:
			return err
		case <-w.closeCh:
			return fmt.Errorf("table: writer is closed")
		case <-w.done:
			return fmt.Errorf("table: writer is not running")
		}
	case <-w.closeCh:
		return fmt.Errorf("table: writer is closed")
	case <-w.done:
		return fmt.Errorf("table: writer is not running")
	}
}

// Close signals shutdown, waits for the background goroutine to exit,
// and flushes any pending data first. It returns the error from the
// final flush, if any.
func (w *AsyncWriter) Close() error {
	w.closeOnce.Do(func() { close(w.closeCh) })
	if w.started.Load() {
		<-w.done
	}
	return w.closeErr
}

// Results returns a read-only channel for consumers to receive flush outcomes.
func (w *AsyncWriter) Results() <-chan *FlushResult {
	return w.resultCh
}

// run is the background goroutine loop.
func (w *AsyncWriter) run(ctx context.Context) {
	defer close(w.done)

	var timer *time.Timer
	var timerC <-chan time.Time
	if w.flushInterval > 0 {
		timer = time.NewTimer(w.flushInterval)
		timerC = timer.C
		defer timer.Stop()
	}

	for {
		select {
		case batch := <-w.batchCh:
			if err := w.accumulate(ctx, batch); err != nil {
				w.sendResult(nil, nil, err)
				continue
			}
			if w.maxBatchSize > 0 && w.currentRows >= int64(w.maxBatchSize) {
				if err := w.flush(ctx); err != nil {
					w.sendResult(nil, nil, err)
				}
			}
		case respCh := <-w.flushCh:
			w.drainPending(ctx)
			respCh <- w.flush(ctx)
		case <-w.closeCh:
			w.drainPending(ctx)
			w.closeErr = w.flush(ctx)
			return
		case <-ctx.Done():
			w.drainPending(ctx)
			w.closeErr = w.flush(ctx)
			return
		case <-timerC:
			if w.currentBatch != nil && w.currentRows > 0 {
				w.flush(ctx)
			}
			timer.Reset(w.flushInterval)
		}
	}
}

// drainPending accumulates any batches waiting in batchCh without blocking.
func (w *AsyncWriter) drainPending(ctx context.Context) {
	for {
		select {
		case batch := <-w.batchCh:
			if err := w.accumulate(ctx, batch); err != nil {
				w.sendResult(nil, nil, err)
				return
			}
		default:
			return
		}
	}
}

// accumulate merges a batch into the current accumulated batch.
func (w *AsyncWriter) accumulate(ctx context.Context, batch *RecordBatch) error {
	if w.currentBatch == nil {
		w.currentBatch = NewRecordBatch(w.schema, 0)
		w.currentRows = 0
	}
	for colID, data := range batch.Columns {
		existing, ok := w.currentBatch.Columns[colID]
		if !ok {
			w.currentBatch.Columns[colID] = data
			continue
		}
		merged, err := mergeColumn(existing, data)
		if err != nil {
			return fmt.Errorf("table: %w", err)
		}
		w.currentBatch.Columns[colID] = merged
	}
	w.currentRows += batch.NumRows
	w.currentBatch.NumRows = w.currentRows
	return nil
}

// flush writes the current accumulated batch to disk as a new fragment
// and commits a new manifest version. It appends to existing fragments.
func (w *AsyncWriter) flush(ctx context.Context) error {
	if w.currentBatch == nil || w.currentRows == 0 {
		return nil
	}

	fragmentID := w.fragmentID
	fw := NewFragmentWriter(w.store, w.schema, fragmentID, w.tablePath, w.compression)
	if err := fw.WriteBatch(ctx, w.currentBatch); err != nil {
		return fmt.Errorf("table: async flush fragment %d: %w", fragmentID, err)
	}
	frag, err := fw.Finish()
	if err != nil {
		return fmt.Errorf("table: async finish fragment %d: %w", fragmentID, err)
	}

	w.fragmentID++

	// Read the latest manifest and append the new fragment.
	manifest := NewManifest(w.nextVersion, w.schema)
	latest, err := w.manifestStore.LatestVersion(ctx)
	if err != nil {
		// No previous versions — this is the first fragment.
		manifest.Fragments = []*Fragment{frag}
		manifest.MaxFragmentID = frag.ID
	} else {
		latestM, err := w.manifestStore.Read(ctx, latest)
		if err != nil {
			return fmt.Errorf("table: async flush: read v%d manifest: %w", latest, err)
		}
		manifest.Fragments = make([]*Fragment, 0, len(latestM.Fragments)+1)
		manifest.Fragments = append(manifest.Fragments, latestM.Fragments...)
		manifest.Fragments = append(manifest.Fragments, frag)
		if frag.ID > latestM.MaxFragmentID {
			manifest.MaxFragmentID = frag.ID
		} else {
			manifest.MaxFragmentID = latestM.MaxFragmentID
		}
	}
	if err := w.manifestStore.Write(ctx, manifest); err != nil {
		return fmt.Errorf("table: async flush: write v%d manifest: %w", manifest.Version, err)
	}
	w.nextVersion++

	w.currentBatch = nil
	w.currentRows = 0

	w.sendResult(frag, manifest, nil)
	return nil
}

// sendResult sends a FlushResult to resultCh without blocking.
func (w *AsyncWriter) sendResult(frag *Fragment, manifest *Manifest, err error) {
	select {
	case w.resultCh <- &FlushResult{Fragment: frag, Manifest: manifest, Error: err}:
	default:
	}
}

// mergeColumn appends incoming data to existing data via type-switching.
// mergeColumn handles all types supported by inferNumRows.
func mergeColumn(existing, incoming interface{}) (interface{}, error) {
	switch a := existing.(type) {
	case []int8:
		b, ok := incoming.([]int8)
		if !ok {
			return nil, fmt.Errorf("cannot merge %T into []int8", incoming)
		}
		return append(a, b...), nil
	case []int16:
		b, ok := incoming.([]int16)
		if !ok {
			return nil, fmt.Errorf("cannot merge %T into []int16", incoming)
		}
		return append(a, b...), nil
	case []int32:
		b, ok := incoming.([]int32)
		if !ok {
			return nil, fmt.Errorf("cannot merge %T into []int32", incoming)
		}
		return append(a, b...), nil
	case []int64:
		b, ok := incoming.([]int64)
		if !ok {
			return nil, fmt.Errorf("cannot merge %T into []int64", incoming)
		}
		return append(a, b...), nil
	case []float32:
		b, ok := incoming.([]float32)
		if !ok {
			return nil, fmt.Errorf("cannot merge %T into []float32", incoming)
		}
		return append(a, b...), nil
	case []float64:
		b, ok := incoming.([]float64)
		if !ok {
			return nil, fmt.Errorf("cannot merge %T into []float64", incoming)
		}
		return append(a, b...), nil
	case []string:
		b, ok := incoming.([]string)
		if !ok {
			return nil, fmt.Errorf("cannot merge %T into []string", incoming)
		}
		return append(a, b...), nil
	case [][]byte:
		b, ok := incoming.([][]byte)
		if !ok {
			return nil, fmt.Errorf("cannot merge %T into [][]byte", incoming)
		}
		return append(a, b...), nil
	default:
		return nil, fmt.Errorf("unsupported column type %T for merge", existing)
	}
}

// mergeRecordBatches merges multiple RecordBatches into one by concatenating
// column data. All batches must have compatible schemas (same column IDs).
func mergeRecordBatches(batches []*RecordBatch) (*RecordBatch, error) {
	if len(batches) == 0 {
		return nil, fmt.Errorf("no batches to merge")
	}
	if len(batches) == 1 {
		return batches[0], nil
	}

	schema := batches[0].Schema
	var totalRows int64
	for _, b := range batches {
		if b.Schema != schema {
			return nil, fmt.Errorf("schema mismatch in batch insert")
		}
		totalRows += b.NumRows
	}

	merged := NewRecordBatch(schema, totalRows)
	for colID := range batches[0].Columns {
		result := batches[0].Columns[colID]
		for i := 1; i < len(batches); i++ {
			data, ok := batches[i].Columns[colID]
			if !ok {
				continue
			}
			var err error
			result, err = mergeColumn(result, data)
			if err != nil {
				return nil, fmt.Errorf("merge column %d: %w", colID, err)
			}
		}
		merged.SetColumn(colID, result)
	}
	return merged, nil
}
